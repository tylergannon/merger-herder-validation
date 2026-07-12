# P1 Ref Events Review - Claude Round 3

## Exact Prompt

> # P1 Ref Events Review - Claude Round 3
>
> ## Goal
>
> Make the final consensus determination for roadmap phase 1 after resolving
> round 2's remaining design proof gap.
>
> ## Revision
>
> The concurrency proof no longer creates two unrelated refs from zero. It first
> creates one real `race` ref at a known SHA, then launches two concurrent pushes
> with the same exact force-with-lease expectation and divergent candidate SHAs.
> Exactly one may succeed. The proof requires exactly one new pending event whose
> `before` is the known original SHA and whose `after` is the actual winning
> remote SHA, with no workflow run or observation error. The focused scenario
> passes under `-race -count=10`.
>
> The round-2 nits were also tightened: pending-event JSON marshal failure is loud
> rather than silently dropped, control snapshots use `run_attempt`, and
> exact-SHA fetch covers both base and source objects.
>
> ## Review Instructions
>
> Inspect the current revisions and prior reviews. Adjudicate the remaining
> design finding and check for regressions. Run the focused proof. Do not edit
> implementation, tests, contracts, roadmap, or docs. Write the exact prompt and
> findings only to:
>
> `ephemeral/reviews/20260711-p1-ref-events-claude-round3.md`
>
> Use the canonical critical, bug, design, and nit labels. Finish with a binary
> consensus verdict scoped strictly to phase 1.

## Method

- Re-read the working-tree diff (`dtu/git.go`, `dtu/types.go`, `dtu/control.go`,
  `dtu/server.go`, `dtu/client.go`), the revised proof
  (`dtu/ref_events_test.go`), the proof doc, and all three prior artifacts
  (`…-claude-round1.md`, `…-claude-round2.md`, `…-codex.md`).
- Focused on the round-3 delta: `proveConcurrentPushObservation`
  (`ref_events_test.go:206-254`), the marshal panic
  (`git.go:249-263`), the `run_attempt` control tag (`types.go:136`), and the
  two-SHA fetch (`ref_events_test.go:43`, `:99-116`).
- `go build ./...`: clean. `go vet ./dtu/`: clean.
- `go test ./dtu/ -run TestGITRefTransitionsAndEventCreation -race -count=10`:
  PASS (21.99s). The printed `fatal: … not our ref 111…` line is the expected
  negative unavailable-SHA fetch.
- `go test ./dtu/ -race -count=1` (full package): PASS (5.84s).

## Adjudication Of The Remaining Round-2 Design Finding

### RESOLVED — design: concurrency proof does not exercise stale-`before` misattribution

Round 2's residual gap was that both concurrent pushes created **distinct refs
from zero**, so each ref's `before` was unconditionally the zero OID and no
prior value could be captured stale — the highest-risk axis the prompt names
(same-ref concurrent contention with a real prior value) was reasoned, not
exercised.

The round-3 proof closes this precisely. `proveConcurrentPushObservation`
(`ref_events_test.go:206-254`) now:

1. Creates two commits `race-one`/`race-two`, both children of `main`
   (`baseSHA`), then creates a **real** remote ref `race` at `baseSHA` via
   `git push remote main:refs/heads/race` (`:218`). This establishes a
   pre-existing ref with a genuine **non-zero** `before` value.
2. Launches two concurrent `git push --force-with-lease=refs/heads/race:baseSHA`
   with divergent candidate SHAs (`:226-234`). Both leases pin the same expected
   value (`baseSHA`); the first accepted update moves `race` off `baseSHA`, so
   the other's lease check fails. **Exactly one** succeeds, asserted by
   `successes != 1` (`:242`).
3. Requires **exactly one** new pending event (`+1`, `:246`), zero new workflow
   runs, zero observation errors, a winning ref equal to one of the two
   candidates (`:249-252`), and a push body with `before == baseSHA`,
   `after == winner`, `forced == false` (`:253`).

This is a real regression guard on the exact hazard `receiveLocks` exists to
prevent. Without the per-repo lock (`git.go:62-63`), the **losing** handler —
whose push Git rejects, so it moves no ref itself — can still snapshot
`beforeRefs = {race: baseSHA}` _before_ the winner commits and
`afterRefs = {race: winnerSHA}` _after_, and `recordRefChanges`
(`git.go:165-178`) would then emit a **phantom** `baseSHA→winnerSHA` event
attributed to the rejected push, yielding two events. The lock serializes each
handler's snapshot→update→snapshot→record sequence, so the loser observes
`{race: winnerSHA}` on both sides and emits nothing. The `+1` exact-count
assertion fails if that phantom appears, and the `before == baseSHA` /
`after == winner` assertion pins correct attribution. This is now demonstrated,
not merely argued.

This is the tightest shape achievable in real Git: two _successful_ same-ref
updates cannot be forced concurrently (fast-forward and `--force-with-lease`
rules reject the loser), so the accepted-transition count is intrinsically one.
The proof exercises the full contention, the observation straddle, and the
attribution — which is exactly what the finding asked for. Resolved.

## Adjudication Of Prior Nits

### RESOLVED — nit: marshal failure silently drops the event

`appendPendingEvent` (`git.go:249-263`) now `panic`s with
`marshal pending %s event: %v` on a `json.Marshal` error instead of returning
early without appending. This is loud and fail-fast, consistent with the
fail-closed snapshot path, and the correct posture for a validation double. The
path remains unreachable in practice (the payloads are fixed structs of
string/int/bool/nested-struct fields that cannot fail to marshal), so it is a
belt-and-suspenders guard. Resolved.

### RESOLVED — nit: internal `attempt` vs wire `run_attempt`

`WorkflowRun.Attempt` now serializes as `run_attempt` (`types.go:136`), matching
the event body's `workflowRunEvent.RunAttempt` (`git.go:282`). The control
snapshot and the wire body no longer diverge on this field name. Resolved.

### RESOLVED (mechanism) / PERSISTS (coverage) — nit: `GIT-01` single-SHA fetch

`proveExactObjectFetch` now fetches **both** `baseSHA` (default-branch base) and
`featureSHA` (a source head) by exact SHA, each verified against `FETCH_HEAD`,
plus the unavailable-SHA and unauthorized negatives (`ref_events_test.go:43`,
`:99-116`). This satisfies the round-2 revision claim ("base and source
objects") and strengthens `GIT-01` beyond one recorded SHA. `GIT-01`'s full
enumeration (the release ref `R` and every batch source head) is still not
exhaustively fetch-proven, but the mechanism is now demonstrated on two distinct
recorded objects and generalizes trivially. Downgraded to a wording/coverage
polish nit; non-blocking.

## Regression Check On The Revisions

- **No false coupling introduced.** `race` is not the configured release ref
  (`R`), so the accepted transition correctly appends a `push` event and **no**
  workflow run (`+0` runs asserted, `:246`). The release-derivation guard
  (`git.go:174-177`, `workflow.releaseRef == ref && afterSHA != ""`) is
  unchanged and still exercised by `assertReleaseTransition`.
- **`forced` still derived from real Git.** `forcedUpdate` (`git.go:330-336`) is
  computed for all refs before the lock; the race winner is a fast-forward of
  `baseSHA`, so `forced=false` is asserted correctly (`:253`).
- **No fail-open reintroduced.** The post-receive snapshot error path
  (`git.go:96-101`) still records an observation error and returns `500`; the
  concurrency proof asserts `len(after.ObservationErrors) == 0` on the happy
  path, so a regression would surface.
- **Immutable body unchanged and still `HOOK-04`-complete** (`forced` +
  `sender`); `assertPushEvent` re-validates the frozen bytes on every transition
  including the race winner.
- Full-package and focused `-race` runs are clean; no new data race, deadlock,
  or ordering defect observed across 10 focused iterations.

No new critical, bug, or design finding. One micro-observation, not blocking:
`appendPendingEvent`'s `panic` fires while `recordRefChanges` holds `w.mu`
(`git.go:162`), so a (currently impossible) marshal failure would leave the
mutex locked as the panic unwinds through the recovered HTTP handler. Because
the payloads cannot fail to marshal, this is theoretical; noting it only for
completeness.

## Findings Summary

- **critical:** none.
- **bug:** none.
- **design:** none remaining. The round-2 residual (same-ref stale-`before`
  misattribution) is resolved by the revised concurrency proof.
- **nit:** `GIT-01` fetch coverage is now two objects (base + one source) rather
  than the full enumeration (release ref + every batch source head) — mechanism
  proven, coverage partial; the in-memory receive-pack response buffering
  remains an accepted, unbounded-but-immaterial trade for snapshot ordering; the
  unreachable marshal `panic` holds `w.mu` as it unwinds. All non-blocking.

## Verdict

**CONSENSUS: SUFFICIENT FOR PHASE 1.**

Every round-1 and round-2 design finding is now resolved. The after-snapshot
path is fail-closed and inspectable; the immutable `push` body is
`HOOK-04`-complete; and the receive-pack serialization is now guarded by a
real concurrent-contention proof that reproduces the exact stale-`before`
misattribution hazard — a pre-existing non-zero ref, two concurrent
force-with-lease pushes, exactly one accepted transition, and exactly one
correctly-attributed pending event — clean under `-race -count=10`. The
required Git transport semantics (exact-object fetch of base and source with
explicit unavailable/unauthorized failure, release-ref creation and
fast-forward, exact `--force-with-lease` guarding with stale-lease rejection,
default-branch fast-forward landing with non-fast-forward rejection, and
deletion) are proven through real Git smart HTTP, with `ls-remote` reconfirming
rejected updates move no ref. Pending `push` and `workflow_run`/`requested`
bodies plus the single queued run are derived solely from the real committed
ref diff and bound to exact before/after and head SHAs; rejected updates produce
no events, runs, observation errors, or mutations. `go build`, `go vet`, and
`-race` (full package + focused ×10) are clean. Only non-blocking coverage/style
nits remain.

This verdict is scoped strictly to phase-1 ref-transition, immutable pending
event, and queued authoritative workflow-run derivation. It asserts nothing
about later webhook signing/delivery, redelivery, ordering, or duplication,
`workflow_run` transitions beyond `queued`, CI execution, or the MergeHerder
system proof — all of which this slice correctly declares out of scope.
