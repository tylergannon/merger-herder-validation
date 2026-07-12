# P1 Ref Events Review - Claude Round 2

## Exact Prompt

> # P1 Ref Events Review - Claude Round 2
>
> ## Goal
>
> Re-review roadmap phase 1 after the round-1 findings. Determine whether the
> current implementation and real-Git tests are sufficient proof of fetch and ref
> transition contracts plus exact immutable pending event and queued workflow
> derivation.
>
> ## Round-1 Revisions
>
> Read the round-1 artifact and inspect the current diff.
>
> - Receive-pack CGI output is now buffered. A post-receive ref-snapshot failure
>   records an inspectable observation error and returns a server failure instead
>   of silently returning success without an event.
> - The proof now runs two concurrent pushes to distinct refs in the same
>   repository and asserts two correct zero-to-SHA events, no workflow runs, no
>   observation errors, and race-detector cleanliness.
> - Immutable push bodies now include `forced` and an App sender. The proof
>   distinguishes release creation, fast-forward, exact-lease non-fast-forward
>   rebuild, default landing, and deletion flags.
>
> ## Review Instructions
>
> Adjudicate every prior critical, bug, and design finding and check the revisions
> for new correctness, protocol, buffering, concurrency, or proof problems. Run
> the proof. Do not edit implementation, tests, contracts, roadmap, or docs. Write
> the exact prompt and findings only to:
>
> `ephemeral/reviews/20260711-p1-ref-events-claude-round2.md`
>
> Use these labels:
>
> - **critical:** must fix before proceeding.
> - **bug:** demonstrable incorrect behavior, broken contract, race, or
>   regression.
> - **design:** architecture, boundary, scope, maintainability, or proof issue
>   materially likely to cause problems.
> - **nit:** small cleanup that should not block progress.
>
> Finish with a consensus verdict scoped strictly to phase 1.

## Method

- Re-read the complete working-tree diff (`dtu/git.go`, `dtu/types.go`,
  `dtu/control.go`, `dtu/server.go`, `dtu/client.go`, roadmap), the revised
  proof (`dtu/ref_events_test.go`, `docs/p1-ref-events-proof.md`), and both
  prior artifacts (`…-claude-round1.md`, `…-codex.md`).
- Re-checked the roadmap `[x]` claims against `GITHUB_SERVICE_CONTRACT.md`
  (`GIT-01/02/03`, `HOOK-04`, `CI-02`).
- `go build ./...` and `go vet ./...`: clean.
- `go test ./dtu/ -race -count=1` (full package): PASS.
- `go test ./dtu/ -run TestGITRefTransitionsAndEventCreation -race -count=10`:
  PASS on all iterations (the printed `fatal: … not our ref 111…` line is the
  expected negative unavailable-SHA fetch). The concurrent-push subtest is clean
  under `-race` across all 10 runs.

## Adjudication Of Round-1 Findings

### RESOLVED — design: after-snapshot Git failure silently drops a real transition

`serveGit` no longer fails open. The post-CGI snapshot error path now calls
`recordObservationError(repository.id, "post-receive ref snapshot", err)` and
returns `500 Unable to confirm repository refs` instead of returning success
with no event (`dtu/git.go:95-101`). The observation error is surfaced in the
control snapshot (`ObservationErrors`), and the proof asserts
`len(after.ObservationErrors) == 0` on the happy path, so a regression would be
visible. This is the best achievable fail-closed posture short of transactional
Git — the CGI has already committed the ref by the time the snapshot runs, so
the ref moves while the client sees `500` and no event is created — but that
window is now _inspectable_ rather than silent, which is the correct phase-1
outcome. Resolved.

### RESOLVED — design: immutable push body omits `forced` and `sender`

`appendPushEvent` now serializes `forced` (computed by `forcedUpdate` via
`git merge-base --is-ancestor`, correctly `false` for creation/deletion) and a
`sender` object (`{id: appID, login: "dtu-app-N[bot]", type: "Bot"}`). The body
now carries every `HOOK-04`-named field: repository, installation, ref, before,
after, created, deleted, forced, sender. The proof's `assertPushEvent`
distinguishes the force-with-lease rebuild (`forced=true`,
`advancedSHA→repairSHA`) from the fast-forward and create cases byte-for-byte,
so the immutable body is now `HOOK-04`-complete and consumers can recover
`forced` from the frozen bytes alone. Resolved.

### PARTIALLY RESOLVED — design: receive-pack serialization has no regression proof

`proveConcurrentPushObservation` now spawns two real concurrent `git push`
processes and asserts exactly two disjoint immutable events, zero workflow runs,
zero observation errors, and race-detector cleanliness. This is a genuine
improvement and exercises the per-repo `receiveLock` under real contention. See
the residual design finding below for the remaining gap.

### PERSISTS — nit: `GIT-01` fetch coverage is one recorded SHA

Unchanged. `proveExactObjectFetch` still fetches a single recorded SHA
(`featureSHA`) plus the unavailable-SHA and unauthorized negatives, while
`GIT-01`'s scope enumerates the default-branch base, each batch source head, and
the release branch. The roadmap now marks `GIT-01` `[x]` ("proves exact
reachable objects"). Mechanism-complete, coverage-partial. Still a nit.

### PERSISTS — nit: internal `WorkflowRun.attempt` vs wire `run_attempt`

Unchanged. The control snapshot serializes `attempt`; the event body serializes
`run_attempt`. Both are pinned by the proof. Harmless, still divergent.

## New Findings On The Revisions

### design: the concurrency proof does not exercise stale-`before` misattribution

The codex fix (`receiveLocks`) was added for one specific hazard: overlapping
receive-packs snapshotting the same old refs and later reporting a transition
with a **stale `before`** on a _pre-existing_ ref. The new proof does not
reproduce that shape. Both concurrent pushes create **distinct new refs from
zero** (`main:refs/heads/concurrent-a` and `…-b`), so each ref's `before` is
unconditionally the zero OID regardless of locking — there is no prior value to
capture stale. What the disjoint-create test _does_ catch is duplicate fan-out
(without the lock, interleaved before/after snapshots could each observe
`{}→{a,b}` and emit four events); the `+2` assertion would fail in that case, so
it is a real regression guard on that axis. But the tightest stale-`before`
attribution path — two successful updates to the same existing ref where the
second's snapshot straddles the first's commit — remains reasoned, not
exercised. This is materially the highest-risk axis the prompt names, so the
residual gap is worth recording even though same-ref concurrent updates are
awkward to force (Git rejects the non-fast-forward loser). Non-blocking:
serialization is exercised and clean under `-race ×10`; the specific
misattribution scenario is not demonstrated. The proof doc (item 11) and the
round-2 prompt both describe the test honestly as "distinct refs" / "zero-to-SHA,"
so there is no overstatement — only an untested corner.

### nit: `appendPendingEvent` silently drops the event on a marshal failure

`dtu/git.go` `appendPendingEvent` returns early without appending or bumping
`nextEventID` if `json.Marshal(payload)` errors, after the ref has already
moved and the CGI response is about to flush success. This re-introduces the
exact silent-fail-open shape round-1 pushed to eliminate on the snapshot path.
It is unreachable in practice — the payloads are fixed structs of string/int/
bool fields that cannot fail to marshal — so it is a nit, not a bug, but for
consistency with the now-fail-closed snapshot path it would be cleaner to record
an observation error (or panic in the double) rather than drop silently.

### nit: receive-pack response is fully buffered in memory before flush

The `responseBuffer` accumulates the entire `git-receive-pack` CGI response
before `flush`. Correct for a validation double (status/headers/body are
faithfully replayed, and rejected pushes still return their `200` + `ng`
report-status so the client reports failure — confirmed by the proof's
`runFails` rejection cases). Worth a one-line note only: the buffer is unbounded,
which is immaterial at test scale but is a deliberate trade of streaming/progress
for the snapshot-ordering guarantee. No action needed for phase 1.

## Verdict

**Sufficient for phase 1.** All three round-1 design findings are addressed:
the after-snapshot path is now fail-closed and inspectable, the immutable `push`
body is `HOOK-04`-complete (`forced` + `sender`), and a real concurrent-push
regression test now guards the serialization. The required Git transport
semantics — fetch of a recorded object with explicit unavailable/unauthorized
failure, release-ref creation and fast-forward, exact `--force-with-lease`
guarding with stale-lease rejection, and default-branch fast-forward landing
with non-fast-forward rejection — are proven through real Git smart HTTP, with
`ls-remote` reconfirming that rejected updates move no ref. Pending `push` and
`workflow_run`/`requested` bodies plus the single queued run are derived solely
from the real committed ref diff, bound to exact before/after and head SHAs, and
rejected updates demonstrably create no events, runs, observation errors, or
mutations. `go build`, `go vet`, and `-race` (full package + proof ×10) are
clean.

No critical, bug, or blocking finding. The residual items are one design/proof
gap (the concurrency test exercises duplicate fan-out but not same-ref
stale-`before` misattribution) and three nits (`GIT-01` single-SHA coverage,
`attempt`/`run_attempt` naming, marshal-failure silent drop, in-memory
buffering) — all non-blocking and none of which undermines the derived
ref/event/run facts.

This verdict covers phase-1 ref transition, event, and queued-run derivation
only. It asserts nothing about later webhook signing/delivery, redelivery or
ordering, `workflow_run` transitions beyond `queued`, CI execution, or the
MergeHerder system proof, all of which this slice correctly declares out of
scope.
