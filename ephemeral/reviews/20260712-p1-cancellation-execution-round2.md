# P1 Cancellation And Workflow Execution — Round 2 Review

## Prompt

> # Claude Re-review Prompt: P1 Cancellation And Workflow Execution
>
> Re-review the complete phase-3 change on branch
> `codex/p1-cancellation-execution` after the corrections prompted by
> `ephemeral/reviews/20260712-p1-cancellation-execution-round1.md`.
>
> The goal remains a valid, workable proof environment for MergeHerder's P1
> GitHub surface, with GitHub-shaped asynchronous cancellation and real workflow
> execution at the exact release SHA. Inspect the full current working-tree diff
> from `origin/main`, not only the most recent edits, and independently decide
> whether the runnable tests constitute sufficient proof of the phase-3 claims.
>
> Round-1 changes include:
>
> - replaced the Postgres substitute with the Actions-capable
>   `catthehacker/ubuntu:act-22.04` multi-arch manifest pinned by digest;
> - the fixture now executes commit-pinned `actions/checkout` and default bash;
> - scripted lifecycle transitions reject an execution-claimed run, and the
>   executor rechecks queued state before process start;
> - cancellation conclusions depend on a successfully delivered interrupt plus
>   nonzero process exit, preserving success-wins races;
> - every job container is labeled by DTU-instance hash and run ID, then removed
>   explicitly on cancellation and shutdown; the test proves no labeled
>   container remains;
> - cancellation negatives now cover invalid tokens and wrong-repository scope;
>   and
> - the proof document contains digest-qualified installation commands and
>   supported manifest architectures.
>
> The current proof is green under:
>
> ```sh
> DTU_REQUIRE_ACT=1 go test -race -count=1 -run 'TestActWorkflow|TestWorkflowCancellationRaces' -v -timeout=5m ./dtu
> go test -race -count=1 ./...
> go vet ./...
> govulncheck ./...
> git diff --check
> ```
>
> Assess implementation correctness, proof sufficiency, API fidelity,
> concurrency, process/container lifecycle, reproducibility, and claim wording.
> Try to falsify the tests or identify unproven assertions. Do not edit product
> code or existing docs. Pointer fields remain disallowed unless genuinely shared
> or noCopy; use Go 1.26 `new(value)`, never `github.Ptr`, for external pointer
> APIs.
>
> Write the exact prompt and findings to
> `ephemeral/reviews/20260712-p1-cancellation-execution-round2.md`. Label every
> finding `critical`, `bug`, `design`, or `nit`, with file/line evidence and a
> concrete fix. Explicitly state whether the proof is now sufficient for the
> phase-3 claims as written. If no findings exist at a severity, say so.

## Scope reviewed

Full working-tree diff from `origin/main`: `dtu/execution.go` (new),
`dtu/execution_test.go` (new), `dtu/cancellation_test.go` (new), `dtu/public.go`,
`dtu/server.go`, `dtu/types.go`, `dtu/delivery.go`, `dtu/control.go`,
`dtu/client.go`, `docs/p1-cancellation-proof.md`, `README.md`, and
`ephemeral/P1_ENVIRONMENT_ROADMAP.md`. Re-read the shared lifecycle
(`transitionWorkflowRun` `dtu/delivery.go:99-139`, `getState` `dtu/control.go:286-308`,
`authenticateInstallationToken`/`permissionRank`/`repoName` in `dtu/public.go`,
append-only `w.workflowRuns` at `dtu/git.go:229`).

Verification performed here: `go build ./...`, `go vet ./dtu`, and
`go test -race -count=1 -run TestWorkflowCancellationRaces ./dtu` are all green.
The two `act` tests were not executed (no `DTU_REQUIRE_ACT=1` Docker+image
runtime in this environment); findings about them reason about what a green run
does and does not establish. I independently confirmed the round-1
reproducibility fix: `act` v0.2.89 embeds its version via `//go:embed VERSION`
in `main.go`, and the `VERSION` file at tag `v0.2.89` contains exactly `0.2.89`,
so the documented `go install github.com/nektos/act@v0.2.89` really does produce
a binary whose `act --version` satisfies the `strings.Contains(version,
"0.2.89")` gate (`dtu/execution.go:68-72`). The locally installed `act` reports
`act version 0.2.89`.

## Proof-sufficiency verdict

**Sufficient for the phase-3 roadmap claims as written, with one unproven
sub-assertion in the proof doc (see design-1).**

Every round-1 blocker is resolved (disposition table below). The scripted
cancellation surface and the `act`-backed execution/exact-SHA/cancellation
surface are genuinely proven when green. The single remaining gap is
proof-only, not a code defect: the doc's claim that shutdown *removes* remaining
runner containers (and the force-`SIGKILL` escalation) is exercised by no test —
only the *cancellation* removal path is proven. That does not block any roadmap
checkbox, but the doc/prompt sentence "removed explicitly on cancellation **and
shutdown**; the test proves no labeled container remains" overstates what the
suite establishes.

## Disposition of round-1 findings

| round-1 | status | evidence |
| --- | --- | --- |
| `critical-1` Postgres runner overclaim | **resolved** | real `catthehacker/ubuntu:act-22.04` pinned by digest (`dtu/execution.go:18-19`), fixture runs `actions/checkout` + default bash (`dtu/execution_test.go:34-43`) |
| `bug-1` terminal-state resurrection race | **resolved** | claim is atomic under one lock hold (`dtu/execution.go:27-52`); re-lock rechecks `Status != "queued"` (`dtu/execution.go:123-131`); and `transitionWorkflowRun` now 409s a claimed run (`dtu/delivery.go:117-120`) |
| `bug-2` forced-shutdown container leak | **resolved in code, partially proven** | labeled containers (`dtu/execution.go:112`), `cleanupRunContainers` on cancel (`:178-180`) and on both shutdown paths (`dtu/server.go:151-159`); shutdown removal itself untested — see design-1 |
| `design-1` conclusion keyed on exit code | **resolved** | now `active.cancellationSignalled && waitErr != nil` (`dtu/execution.go:167-173`) as recommended |
| `design-2` non-reproducible proof command | **resolved** | digest-`pull` + `tag` + `go install …@v0.2.89` setup block (`docs/p1-cancellation-proof.md`), independently verified above |
| `design-3` non-exclusive mutation paths | **resolved & tested** | claim-check enforced (`dtu/delivery.go:117-120`) and asserted (`dtu/execution_test.go:108-111`) |
| `nit-1` half-tested negative auth | **resolved** | `401` invalid-token and wrong-repo `404` now asserted (`dtu/cancellation_test.go:78-100`) |
| `nit-2` `Logs` retained/copied | **acknowledged** | still copied into `/state` only; confirmed not on any public GitHub-shaped payload (see "verified correct") |

---

## critical

None.

## bug

None. I specifically tried to re-open the round-1 races and could not:

- The claim window is closed on both ends — `activeRuns[id]` is set inside the
  same lock region that observed `Status == "queued"` (`dtu/execution.go:27-52`),
  so no transition can slip between the check and the claim; and
  `transitionWorkflowRun` rejects any claimed run (`dtu/delivery.go:117-120`).
- The post-`Wait` conclusion reads `active` after re-acquiring the lock
  (`dtu/execution.go:160-163`), and `cancelWorkflowRun` only sets
  `cancellationSignalled` when `Process.Signal` succeeds under the same lock
  (`dtu/public.go:96-101`), so a success that reaps before the signal lands
  cannot be mislabeled `cancelled` — the "success wins" race is preserved.
- Concurrent `command.Wait()` (handler goroutine) vs `Signal`/`Kill`
  (`cancelWorkflowRun`, `stopActiveRuns`) is safe, and the shared
  `&logs` writer is only read after `Wait` returns.

## design

### design-1 — shutdown container-removal and the force-`SIGKILL` escalation are asserted but unproven

`docs/p1-cancellation-proof.md` (Boundary) states: "DTU shutdown interrupts and
removes remaining active runners, escalating the process signal if graceful
server shutdown expires," and the prompt's own summary claims containers are
"removed explicitly on cancellation **and shutdown**; the test proves no labeled
container remains."

The code path exists — `stopActiveRuns(false)` on close and `stopActiveRuns(true)`
on control-shutdown-timeout, each calling `cleanupRunContainers`
(`dtu/server.go:122,128-130,142-159`) — but **no test drives it**. The only
container-removal assertion, `assertNoRunContainers` (`dtu/execution_test.go:129`),
runs after a *cancellation*, not after an instance shutdown, and nothing
exercises the `controlErr != nil` force branch (`dtu/server.go:128-131`) at all.
So the round-1 `bug-2` fix's shutdown half is unverified, and the "and shutdown"
clause of the claim is not backed by the suite.

Why it matters: this is exactly the leaked-container class the prompt asks about,
and the wording promises a proof that does not exist.

Fix (test-only, allowed): add a test that starts a long-running run, waits for
`in_progress` + a labeled container, closes the instance while the run is active,
and asserts `assertNoRunContainers(runID)` afterward. To also cover the
escalation branch, close with an already-expired `context` so
`controlServer.Shutdown` returns immediately and `stopActiveRuns(true)` runs.
Alternatively, soften the doc to say shutdown removal is implemented but proven
only indirectly.

### design-2 — `cancelled` label still depends on `act` exiting nonzero on `SIGINT` (adequately pinned, worth stating)

The round-1 recommendation was adopted: `dtu/execution.go:167-173` now requires
`cancellationSignalled && waitErr != nil` for `cancelled`. This robustly protects
the *success-wins* direction. The residual is the other direction — a genuinely
interrupted run is labeled `cancelled` only because `act 0.2.89` exits nonzero on
`SIGINT`; if a future `act` exits `0` on graceful interrupt,
`TestActWorkflowCancellationStopsSupervisor` (`dtu/execution_test.go:126`) would
observe `success` and fail. The `0.2.89` pin (`dtu/execution.go:17,68-72`) makes
this deterministic and the green run empirically confirms the exit behavior, so
this is not a defect today — only a version-coupling to record. No change
required; noting so the pin is understood as load-bearing for this assertion.

## nit

### nit-1 — test container helper filters on `run_id` only, unlike production cleanup

`runContainerIDs` (`dtu/execution_test.go:163-172`), used by both
`waitForRunContainer` and `assertNoRunContainers`, filters on
`label=dtu.run_id=<id>` alone, whereas production `cleanupRunContainers`
correctly scopes by `dtu.instance` **and** `dtu.run_id` (`dtu/execution.go:200-205`).
Because each fresh instance's `nextRunID` starts at the same base, two `act`
tests in the same binary can mint the same run ID; the tests only stay correct
because they run sequentially and each container is torn down (`--rm` / cleanup)
before the next. Tightening the helper to also match
`label=dtu.instance=<hash>` (exposing the instance label to the test, e.g. via a
control/state field) would remove the cross-instance fragility and make the
assertion match what the product actually guarantees.

### nit-2 — graceful `stopActiveRuns(false)` force-removes the container immediately after `SIGINT`

On a normal instance close, `stopActiveRuns(false)` sends `SIGINT` and then
*immediately* `docker rm -f`s the container (`dtu/server.go:157-158`), before
`act` can react. This collapses the "graceful vs. forced" distinction the doc
implies ("escalating … if graceful server shutdown expires") — the container is
force-removed on the graceful path too. It is harmless (and actually helps `act`
exit promptly), but a reader of the doc would expect force-removal only on
escalation. Either drop the `cleanupRunContainers` call from the non-force branch
(relying on `act --rm` after `SIGINT`, with the force branch as the net) or
adjust the doc wording.

## Things verified correct (no finding)

- **No public API-shape leak.** `WorkflowRun` (now carrying
  `cancellation_requested` and `logs`) is serialized only on the control `/state`
  path (`dtu/control.go:291,304`); the public webhook payload is the separate
  GitHub-shaped struct at `dtu/git.go:246`. The non-`omitempty`
  `cancellation_requested` tag never reaches a GitHub-shaped response.
- **Async-acceptance contract** is faithful: `202` + `X-GitHub-Request-Id`, no
  terminal synthesis (`dtu/public.go:104-107`), `go-github` `AcceptedError`
  asserted (`dtu/cancellation_test.go:88-93`); `409` on completed
  (`dtu/public.go:85-88`); protected `404` for missing run / wrong repo / no
  actions:write (`dtu/public.go:64-77`), `401` for bad credentials.
- **Exact-SHA binding** is real: detached checkout + `resolveCheckoutHEAD`
  equality gate (`dtu/execution.go:97-104`) plus the test's `HeadSHA ==
  releaseSHA`, `head_sha=<sha>`, `actions/checkout`, and `Verify exact candidate`
  log assertions (`dtu/execution_test.go:50-57`).
- **Claim/exclusion** is enforced and tested end to end: second executor gets
  `409 already claimed` / `not queued`, scripted transition gets `409 claimed`
  (`dtu/execution_test.go:108-115`).
- **Pre-start cancellation window** concludes `cancelled` without starting a
  process (`dtu/execution.go:132-143`); `defer` releases the claim on every early
  error return (`dtu/execution.go:54-61`), leaving the run re-executable.
- **Runner-image gate** matches the multi-arch index digest via `RepoDigests`
  and passes the local `amd64`/`arm64` architecture explicitly
  (`dtu/execution.go:233-267`); `docker tag` preserves `RepoDigests`, so the
  documented tag-then-`--pull=false` flow resolves the pinned image offline.
- **Reproducibility** of the `act` install verified independently (see Scope).
- `index` stability holds (`w.workflowRuns` append-only, `dtu/git.go:229`);
  `getState` copies under `RLock` while mutators hold the write lock.

## Summary

Round 2 closes every round-1 blocker: the runner is now a real Actions-capable
image exercising `actions/checkout` and bash, the terminal-state resurrection
race is doubly guarded, cancellation-vs-success is decided by delivered-signal
rather than exit code, and the documented setup is genuinely reproducible.
`critical` and `bug`: none. The one substantive item is `design-1` — the
shutdown/force-escalation container removal is implemented but not exercised by
any test, so that portion of the claim is unproven; add a shutdown-with-active-run
test (allowed, test-only) or soften the wording. With that caveat noted, the
proof is sufficient for the phase-3 claims as written.
