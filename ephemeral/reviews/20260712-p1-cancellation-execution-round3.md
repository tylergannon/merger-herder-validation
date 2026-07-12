# P1 Cancellation And Workflow Execution — Round 3 (Final Convergence) Review

## Prompt

> # Claude Final Review Prompt: P1 Cancellation And Workflow Execution
>
> Perform the final convergence review of the complete phase-3 diff on
> `codex/p1-cancellation-execution`. Read both prior reviews, especially
> `ephemeral/reviews/20260712-p1-cancellation-execution-round2.md`, then inspect
> the full current diff from `origin/main` and the runnable proof.
>
> Round two judged the phase proof sufficient but identified one unproven doc
> subclaim: active runner cleanup during DTU shutdown. The current diff adds
> `TestActWorkflowShutdownStopsSupervisor`, which starts a real long-running
> Docker/`act` job, captures its DTU-instance and run-ID container labels, closes
> the instance, requires the execution request to unblock, and requires zero
> matching containers afterward. The proof doc also records the pinned
> `act 0.2.89` nonzero-on-interrupt behavior used with a successfully delivered
> signal to distinguish cancellation from a success-wins race.
>
> The local required proof is green under:
>
> ```sh
> DTU_REQUIRE_ACT=1 go test -race -count=1 -run 'TestActWorkflow|TestWorkflowCancellationRaces' -v -timeout=5m ./dtu
> go test -race -count=1 ./...
> go vet ./...
> govulncheck ./...
> git diff --check
> ```
>
> Decide independently whether the phase-3 implementation and tests now
> constitute sufficient proof for every checked roadmap claim as written. Look
> for remaining false positives, concurrency defects, process/container leaks,
> API fidelity gaps, environmental overclaims, and mismatches between code,
> tests, docs, and roadmap. Do not edit product code or existing docs.
>
> Write this exact prompt and findings to
> `ephemeral/reviews/20260712-p1-cancellation-execution-round3.md`. Label findings
> `critical`, `bug`, `design`, or `nit`, with file/line evidence and concrete
> fixes. Explicitly state whether consensus is reached that phase 3 is sufficient
> proof. If a severity has no findings, say so.

## Scope reviewed

Full working-tree diff from `origin/main`: `dtu/execution.go` (new),
`dtu/execution_test.go` (new, now three `TestActWorkflow*` tests),
`dtu/cancellation_test.go`, `dtu/public.go`, `dtu/server.go`, `dtu/types.go`,
`dtu/delivery.go`, `dtu/control.go`, `dtu/client.go`,
`docs/p1-cancellation-proof.md`, `README.md`,
`ephemeral/P1_ENVIRONMENT_ROADMAP.md`, and the worklog. Re-read the shared
lifecycle (`transitionWorkflowRun` `dtu/delivery.go:99-139`, `cancelWorkflowRun`
`dtu/public.go:52-106`, `stopActiveRuns`/`close` `dtu/server.go:120-160`,
append-only `w.workflowRuns`).

**Verification performed here — the `act` runtime was genuinely available**, so
unlike rounds 1–2 the Docker/`act` assertions were exercised, not merely reasoned
about. This host has `act version 0.2.89` (`/opt/homebrew/bin/act`), Docker
server `29.1.5`, and the pinned `catthehacker/ubuntu@sha256:93b433d1…` image. The
full required proof is green:

- `DTU_REQUIRE_ACT=1 go test -race -count=1 -run 'TestActWorkflow|TestWorkflowCancellationRaces' -v -timeout=5m ./dtu` → PASS (all three `act` tests **ran**, not skipped: Execution 1.08s, Cancellation 0.73s, Shutdown 0.75s).
- `go test -race -count=1 ./...` → PASS.
- `go vet ./...` → clean; `govulncheck ./...` → no vulnerabilities; `git diff --check` → clean.
- `docker ps -aq --filter label=dtu.run_id` after the suite → `0` (no leaked containers).

Because `waitForRunContainer` (`dtu/execution_test.go:198-215`) `t.Fatal`s if no
labeled container ever appears, the green shutdown run proves a real labeled
container was created **and** removed across `instance.Close`. Round-2 `design-1`
(shutdown removal implemented but unexercised) is therefore genuinely closed for
the graceful path.

## Proof-sufficiency verdict

**Consensus reached: phase 3 is sufficient proof for every *checked* roadmap
claim as written.** All five checkboxes in
`ephemeral/P1_ENVIRONMENT_ROADMAP.md:87-91` (async-acceptance cancellation,
scripted Pause races, pinned-`act`/Docker happy-path CI, exact-SHA
checkout/state/logs/conclusion binding, retained scripted mode) are backed by
green, non-false-positive tests that I ran here against a real runtime.

The only residual is proof-doc narrative, not code and not a roadmap checkbox:
the `## Boundary` sentence about **shutdown-escalation** (`stopActiveRuns(true)`)
still describes behavior no test drives, and the doc's test count is stale. Both
are `design`/`nit`, neither blocks the checked claims.

## Disposition of round-2 findings

| round-2 | status | evidence |
| --- | --- | --- |
| `design-1` shutdown/force removal asserted but unproven | **half-resolved** | graceful shutdown removal now proven by `TestActWorkflowShutdownStopsSupervisor` (`dtu/execution_test.go:132-179`), verified live; force-escalation branch still unexercised — see `design-1` below |
| `design-2` `cancelled` label coupled to `act` nonzero-on-`SIGINT` | **acknowledged/pinned** | `active.cancellationSignalled && waitErr != nil` (`dtu/execution.go:167-173`); `0.2.89` pin (`:17,68-72`) load-bearing and empirically confirmed by the green cancellation run |
| `nit-1` container helper filtered `run_id` only | **resolved** | helper now scopes by instance **and** run ID via the `runContainer{runID,instance}` struct captured from the live label (`dtu/execution_test.go:193-235`), matching production `cleanupRunContainers` (`dtu/execution.go:200-205`) |
| `nit-2` graceful path force-removes container after `SIGINT` | **unchanged** | `stopActiveRuns(false)` still `cleanupRunContainers` immediately after `SIGINT` (`dtu/server.go:157-158`); harmless, see `nit-2` below |

---

## critical

None.

## bug

None. I re-checked every concurrency and lifecycle edge and ran the suite under
`-race`:

- **No lock defect in the refactored `cancelWorkflowRun`.** The round-2/3 diff
  replaced `defer w.mu.Unlock()` with explicit unlocks; every one of the five
  early returns and the success path unlocks exactly once
  (`dtu/public.go:66,71,83,88,103`) — no missing or double unlock. `-race` clean.
- **Claim/exclusion is airtight.** `activeRuns[id]` is set inside the same lock
  region that observed `Status=="queued"` (`dtu/execution.go:27-52`), re-checked
  after the unlocked setup window (`:123-131`), and `transitionWorkflowRun` 409s
  any claimed run under `w.mu` (`dtu/delivery.go:117-120`). Proven live: the
  scripted transition during execution returns `409 "workflow run execution is
  claimed"` and a second executor gets `409 "not queued"`
  (`dtu/execution_test.go:108-115`).
- **Success-wins race preserved.** `cancellationSignalled` is set only when
  `Process.Signal` succeeds under `w.mu` (`dtu/public.go:96-101`); shutdown's
  `stopActiveRuns` never sets it, so a shutdown-interrupted run concludes
  `failure`, and a job that exits 0 before the signal lands stays `success`.
- **No container leak.** Verified empirically: `0` `dtu.run_id`-labeled
  containers remain after the full suite. Cancellation and graceful-shutdown both
  remove the labeled container; the happy path relies on `act --rm`.

## design

### design-1 — proof-doc "escalating the process signal if graceful server shutdown expires" is still exercised by no test

`docs/p1-cancellation-proof.md:36-38` (Boundary): "DTU shutdown interrupts and
removes remaining active runners, **escalating the process signal if graceful
server shutdown expires**." The escalation is the force branch: `close` runs
`stopActiveRuns(true)` (which `Process.Kill()` + `cleanupRunContainers`) only when
`controlServer.Shutdown(ctx)` returns non-nil (`dtu/server.go:128-131,152-155`).

The newly added `TestActWorkflowShutdownStopsSupervisor` does **not** reach that
branch. It closes with a healthy 5s context (`dtu/execution_test.go:170-172`), and
— decisively — asserts `must(t, instance.Close(shutdownContext))`
(`:172`), which requires `close` to return `nil`. A `nil` return is only possible
when both `Shutdown` calls succeed, i.e. the *graceful* `SIGINT` path
(`stopActiveRuns(false)`). If graceful shutdown ever expired, `controlErr` would
be non-nil, `close` would return a joined error, and the test would fail at
`must` rather than exercise escalation. So `stopActiveRuns(true)` /
`Process.Kill()` and the round-1 `bug-2` force-path leak fix remain unproven —
exactly the residual round-2 `design-1` flagged and asked to close "with an
already-expired context." This is a doc-vs-suite overclaim only; no roadmap
checkbox asserts shutdown behavior, so it does not block consensus.

Fix (test-only, allowed): add a shutdown test that forces the branch — start a
run, `waitForRunContainer`, then `instance.Close` with an **already-expired**
context (`ctx, cancel := context.WithCancel(t.Context()); cancel()`) so
`controlServer.Shutdown` returns immediately, `stopActiveRuns(true)` runs, and
`assertNoRunContainers` still holds. Tolerate the non-nil `close` error in that
test. Alternatively, soften the Boundary sentence to state the escalation path is
implemented but proven only indirectly.

### design-2 — `cancelled` conclusion remains coupled to pinned `act 0.2.89` nonzero-on-`SIGINT` (adequate, restated for the record)

Unchanged from round 2 and confirmed correct here: `dtu/execution.go:167-173`
requires `cancellationSignalled && waitErr != nil`, which protects the
success-wins direction robustly. The other direction — a genuinely interrupted
run labeled `cancelled` — depends on `act 0.2.89` exiting nonzero on `SIGINT`. The
green `TestActWorkflowCancellationStopsSupervisor` run I executed empirically
confirms that exit behavior, and the `0.2.89` pin (`dtu/execution.go:17,68-72`)
makes it deterministic. No change required; the pin is load-bearing for this
assertion and should stay pinned.

## nit

### nit-1 — proof doc says "these two host-dependent tests" but there are now three

`docs/p1-cancellation-proof.md:95`: "Without `DTU_REQUIRE_ACT=1`, these **two**
host-dependent tests skip …". The suite now has three `TestActWorkflow*` tests
(Execution, Cancellation, Shutdown, `dtu/execution_test.go:16,60,132`), all gated
by `requireActRuntime`. Stale count. Fix: change "two" to "three" (or "these
host-dependent tests").

### nit-2 — graceful `stopActiveRuns(false)` force-removes the container immediately after `SIGINT`, and the shutdown test cannot attribute removal to DTU vs `act --rm`

On a normal close, `stopActiveRuns(false)` sends `SIGINT` and *immediately*
`docker rm -f`s the container (`dtu/server.go:157-158`) — concurrently with
`act`'s own `--rm` teardown. Consequence: `TestActWorkflowShutdownStopsSupervisor`
proves the observable ("no labeled container remains after shutdown") but cannot
prove *DTU* did the removal rather than `act --rm`, and the "graceful vs. forced"
distinction the Boundary implies is collapsed on the graceful path. Harmless
(both are idempotent and it helps `act` exit promptly). If the doc's escalation
narrative matters, either drop `cleanupRunContainers` from the non-force branch
(relying on `act --rm`, with the force branch as the net) or adjust the wording.

### nit-3 — shutdown test is timing-coupled to `act`'s `SIGINT` reaction within 5s

Because `must(t, instance.Close(shutdownContext))` (5s) demands a graceful
`nil` return (see `design-1`), the test fails rather than escalates if `act` is
slow to react to `SIGINT` under load. It passed comfortably here (0.75s), but the
assertion couples correctness to interrupt latency. Minor; a larger context or
tolerating a non-nil `close` error would remove the coupling.

## Things verified correct here (no finding)

- **All three `act` tests genuinely execute Docker/`act`** — not skipped — and
  their strong assertions (`Conclusion=="success"`/`"cancelled"`, logs contain
  `head_sha=<sha>`, `actions/checkout`, `Verify exact candidate`) can only pass if
  `act` really ran the container at the exact SHA. `dtu/execution_test.go:55,126`.
- **Exact-SHA binding** real: detached checkout + `resolveCheckoutHEAD` equality
  gate (`dtu/execution.go:97-104`) plus test SHA/log assertions.
- **Negative auth fully covered**: `401` invalid token, cross-repo `404`,
  no-`actions:write` `404`, missing-run `404`, completed `409`
  (`dtu/cancellation_test.go:65-100`); `new("write")` used (not `github.Ptr`).
- **No public API-shape leak**: `Logs`/`cancellation_requested` serialize only on
  the control `/state` path (`dtu/types.go:164-167`, `dtu/control.go`), never on
  the GitHub-shaped webhook payload.
- **Reproducible setup**: `go install …@v0.2.89` + digest `pull`/`tag`
  (`docs/p1-cancellation-proof.md:81-85`) matches the runtime gates
  (`dtu/execution.go:68-72`, `verifyRunnerImage` `:233-267`); confirmed against the
  installed `0.2.89` and pinned digest.
- **No leaked containers/processes** after the full `-race` suite (measured `0`).

## Summary

With a real `act`/Docker runtime, I ran the full required proof and it is green,
non-false-positive, and leak-free. Round-2's one open item — shutdown container
removal — is now genuinely proven for the graceful path by
`TestActWorkflowShutdownStopsSupervisor`, and round-2 `nit-1` (loose test filter)
is fixed. `critical`: none. `bug`: none. The remaining items are proof-doc
narrative: the shutdown **force-escalation** branch is still unexercised
(`design-1`) and the doc's "two tests" count is stale (`nit-1`), plus two minor
notes (`nit-2`, `nit-3`). None of these touch a checked roadmap claim.

**Consensus: phase 3 constitutes sufficient proof for every checked roadmap claim
as written.** Recommended before calling the *proof document* fully honest:
address `design-1` (add the forced-escalation test or soften the Boundary
sentence) and fix the `nit-1` count.
