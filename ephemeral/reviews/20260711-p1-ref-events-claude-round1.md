# P1 Ref Events Review - Claude Round 1

## Exact Prompt

> # P1 Ref Events Review - Claude Round 1
>
> ## Goal
>
> Review roadmap phase 1 as a complete vertical slice. The runtime must prove the
> required Git fetch, release-ref, force-with-lease, and default-branch semantics
> through real Git smart HTTP. Accepted branch changes must create immutable
> pending push-event bodies from the actual committed before/after refs. A
> configured release-ref change must additionally create one distinct queued
> authoritative workflow run and pending `workflow_run` `requested` body bound to
> the exact resulting SHA. Rejected updates must move no refs and create no GitHub
> state.
>
> The slice does not deliver webhooks, transition runs beyond queued, execute
> workflows, or run MergeHerder. Those remain later roadmap phases. Internal data
> defaults to concrete values; pointers are limited to shared state, lifecycle or
> no-copy types, and external APIs requiring them. Do not use `github.Ptr`.
>
> ## Review Instructions
>
> If you have a code review skill, use it. Review the complete current diff,
> contracts, roadmap, implementation, and runnable proof against the goal. Run
> the proof and inspect real Git behavior where useful. Prioritize incorrect Git
> semantics, races between receive-pack and observation, false event creation,
> wrong before/after identities, mutable event bodies, workflow correlation
> errors, missing rejection proof, lifecycle problems, scope expansion, and
> documentation overstatement.
>
> Do not edit product code, tests, contracts, roadmap, or documentation. Write
> the exact prompt and findings only to:
>
> `ephemeral/reviews/20260711-p1-ref-events-claude-round1.md`
>
> Label every finding using exactly one definition:
>
> - **critical:** must fix before proceeding.
> - **bug:** demonstrable incorrect behavior, broken contract, race, or
>   regression.
> - **design:** architecture, boundary, scope, maintainability, or proof issue
>   that is materially likely to cause problems.
> - **nit:** small cleanup that should not block progress.
>
> Finish with a verdict on whether the implementation and proof are sufficient
> for phase 1. Do not equate phase-1 sufficiency with later webhook delivery, CI
> execution, or MergeHerder system proof.

## Method

- Read the complete working-tree diff (`README.md`, `dtu/client.go`,
  `dtu/control.go`, `dtu/git.go`, `dtu/server.go`, `dtu/types.go`, roadmap),
  the new proof (`dtu/ref_events_test.go`, `docs/p1-ref-events-proof.md`), and
  the prior codex review.
- Cross-checked the roadmap `[x]` claims against `GITHUB_SERVICE_CONTRACT.md`
  (`GIT-01/02/03`, `HOOK-04`, `CI-02`).
- Ran the proof under the race detector, repeated:
  `go test ./dtu/ -run TestGITRefTransitionsAndEventCreation -race -count=3`.
  Result: PASS on all three iterations (the printed
  `fatal: git upload-pack: not our ref 111...` line is the expected negative
  unavailable-SHA fetch). `go build ./...` and `go vet ./...` are clean.

## What The Slice Gets Right

- **Real Git semantics, observed not simulated.** Before/after branch state is
  read from the actual bare repo via `git for-each-ref refs/heads`
  (`branchRefs`) straddling the real `git-http-backend` CGI invocation. Events
  are derived purely from the committed ref diff, so accept/reject is decided by
  Git itself, not reimplemented.
- **Correct before/after identities.** Each changed branch yields exactly one
  `push` body carrying the real `before`/`after` object IDs, with zero-OID
  substitution and `created`/`deleted` flags computed from that substitution.
  The proof asserts create (`0..→feature`), fast-forward, force-with-lease
  rebuild, default-branch landing, and delete (`feature→0..`) transitions
  against real SHAs pulled from the fixture.
- **Guarded release semantics proven live.** `--force-with-lease=refs/heads/R:<old>`
  rebuild succeeds; a stale-lease retry and a non-fast-forward `main` push both
  fail, and the proof reconfirms via `ls-remote` that the server ref did **not**
  move and that no pending event / workflow run / mutation was produced. This
  matches `GIT-02`/`GIT-03` including the "fails without moving the ref" clause.
- **Rejection produces no state.** Read-only-token push (403 before receive-pack
  runs), stale lease, and non-fast-forward all leave `PendingEvents`,
  `WorkflowRuns`, and `Mutations` unchanged (`assertNoGitHubStateChange`).
- **Workflow correlation is exact.** A configured release-ref move appends one
  queued `WorkflowRun` with a unique id, attempt 1, and the exact head branch/SHA,
  plus a `workflow_run`/`requested` body whose `workflow_run.id` equals the run
  id and whose `head_sha` equals the moved SHA. Delete of the release ref is
  correctly guarded (`afterSHA != ""`) so no run is queued on deletion.
- **Race between concurrent receive-packs addressed.** A per-repository no-copy
  `*sync.Mutex` (`receiveLocks`) serializes the snapshot→update→snapshot→record
  sequence so overlapping pushes to one repo cannot cross-attribute a stale
  `before`. Distinct repos stay independent. Bodies are marshaled once at
  creation and retained as `json.RawMessage`, so over-the-wire event bodies are
  immutable.
- **Scope discipline.** No delivery, signing, run transitions beyond `queued`,
  execution, or MergeHerder logic leaked in. Internals default to concrete
  values; the only pointers added are the per-repo mutex (no-copy shared state)
  and `Conclusion any`/nil solely to render the wire `null`. No `github.Ptr`.

## Findings

### design: after-snapshot Git failure silently drops a real transition

`dtu/git.go` (`serveGit`, post-CGI block):

```go
afterRefs, err := branchRefs(repository.gitDir)
if err == nil {
    w.recordRefChanges(repository, beforeRefs, afterRefs)
}
w.refreshPullSnapshots(repository)
```

The pre-CGI snapshot fails closed (500, push blocked). The **post**-CGI snapshot
fails open: if `branchRefs` errors after `git-http-backend` has already committed
a ref update, the accepted change produces no `push` event, no workflow run, and
no mutation — and the client still sees push success. That silently violates the
slice's core invariant ("each accepted branch push creates a pending event"). It
is low-probability (`for-each-ref` rarely fails) and untriggered by the proof, so
not blocking, but the observation side should not swallow the error — record a
failure marker, retry, or surface it rather than dropping the event.

### design: the receive-pack serialization fix has no regression proof

The codex round added `receiveLocks` specifically to close a
receive-pack/observation race, but `TestGITRefTransitionsAndEventCreation` is
entirely sequential. The property "overlapping pushes to one repo never report a
stale `before`, and distinct repos stay independent" is reasoned, not exercised.
Given the prompt explicitly prioritizes "races between receive-pack and
observation," the fix that addresses exactly that risk is currently unverified.
A concurrent-push proof (even two goroutines pushing different branches to one
repo, asserting two disjoint correct before/after pairs) would convert the
reasoning into evidence. Non-blocking for phase 1 mechanics, but it is a real
proof gap on the highest-risk axis.

### design: immutable push body omits spec-named fields (`forced`, `sender`)

`HOOK-04` states the `push` payload "identifies the repository, installation,
pushed ref, `before`, `after`, deletion/creation/forced flags, and sender." The
stored body (`appendPushEvent`) carries ref/before/after/created/deleted/
repository/installation but **not** `forced` or `sender`/`pusher`. This matters
because the bodies are marshaled once and retained as immutable raw JSON _now_
for a later delivery layer: the force-with-lease rebuild (before=`advancedSHA`,
after=`repairSHA`, a genuine non-fast-forward) currently serializes to a body
byte-for-byte shaped like a fast-forward — a consumer cannot recover `forced`
from the body alone. Phase 1 does not deliver webhooks, so this is not a
blocker, but because the bytes are frozen at creation, phase 2 cannot backfill
them without regenerating; worth deciding now whether the immutable body should
already be `HOOK-04`-complete. (`workflow_run` bodies are correspondingly
minimal but cover the `CI-02`-named fields for the `requested` action.)

### nit: `GIT-01` fetch coverage is one recorded SHA, roadmap says "objects"

The roadmap now reads "`GIT-01` … proves exact reachable objects." The proof
fetches a single recorded SHA (the feature head) and checks `FETCH_HEAD`, plus an
unavailable SHA and an unauthorized attempt — which does establish the mechanism
and satisfies `GIT-01`'s "fetching a recorded SHA yields the same object graph …
or fails explicitly." But `GIT-01`'s scope enumerates the default-branch base,
each batch source head, and the release branch; only one of these is
SHA-fetch-proven. The mechanism generalizes trivially, so this is wording/coverage
polish, not a correctness gap.

### nit: internal `WorkflowRun.attempt` vs wire `run_attempt`

`WorkflowRun` serializes `attempt` in the control snapshot while the event body
uses `run_attempt` (the real GitHub field). Both consumers are internal and the
proof pins both, so this is harmless, but the divergent names invite confusion.

## Verdict

**Sufficient for phase 1.** The required Git transport semantics — fetch of a
recorded object with explicit unavailable/unauthorized failure, release-ref
creation and fast-forward, exact `--force-with-lease` guarding with stale-lease
rejection, and default-branch fast-forward landing with non-fast-forward
rejection — are proven through real Git smart HTTP, and the ref-verification via
`ls-remote` confirms rejected updates move no ref. Pending `push` and
`workflow_run`/`requested` bodies plus the queued run are derived solely from the
real committed ref diff, bound to exact before/after and head SHAs, and rejected
updates demonstrably create no events, runs, or mutations. The test is clean
under `-race` across repeated runs.

The findings above are non-blocking: the after-snapshot fail-open and the missing
concurrency regression are latent/proof gaps rather than demonstrated defects, and
the payload-completeness question belongs to the delivery layer whose absence this
slice correctly declares. This verdict covers phase-1 ref/event/run derivation
only and asserts nothing about later webhook delivery, `workflow_run` transitions
beyond `queued`, CI execution, or the MergeHerder system proof.
