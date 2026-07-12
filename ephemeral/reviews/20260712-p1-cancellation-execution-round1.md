# P1 Cancellation And Workflow Execution — Round 1 Review

## Prompt

> # Claude Review Prompt: P1 Cancellation And Workflow Execution
>
> Review the complete phase-3 change on branch
> `codex/p1-cancellation-execution` in this repository against the user's actual
> goal: build a valid, workable proof environment for MergeHerder's designated P1
> GitHub surface, including GitHub-shaped asynchronous Actions cancellation and
> real workflow execution at the exact release SHA.
>
> Inspect the full diff from `origin/main`, the surrounding implementation and
> contracts, especially:
>
> - `dtu/public.go`, `dtu/execution.go`, and their shared state/lifecycle;
> - `dtu/cancellation_test.go` and `dtu/execution_test.go`;
> - `docs/p1-cancellation-proof.md`;
> - `ephemeral/P1_ENVIRONMENT_ROADMAP.md`; and
> - prior phase behavior on which this change depends.
>
> Assess both implementation correctness and proof sufficiency. In particular,
> decide whether the runnable tests, when green with `DTU_REQUIRE_ACT=1`, really
> constitute proof of the asserted phase-3 behavior across real HTTP, `go-github`,
> Git smart HTTP, detached exact-SHA checkout, `act`, Docker, runner supervision,
> logs, conclusions, and cancellation races. Look for false-positive tests,
> unproven claims, concurrency/lifecycle defects, authentication or API-shape
> errors, environmental non-reproducibility, leaked processes/containers, and
> roadmap overclaims. Do not limit the review to these examples.
>
> Repository constraints:
>
> - Do not edit product code or existing docs.
> - Pointer fields are disallowed unless data is genuinely shared or the type is
>   noCopy; do not recommend `github.Ptr`, because Go 1.26 `new(value)` is the
>   required spelling for external pointer-heavy APIs.
> - Scripted cancellation remains the oracle for GitHub races `act` cannot
>   reproduce; `act` is the real happy-path execution engine.
>
> Write your review to
> `ephemeral/reviews/20260712-p1-cancellation-execution-round1.md`. Include this
> exact prompt, then findings labeled `critical`, `bug`, `design`, or `nit`, with
> file/line evidence and concrete fixes. State explicitly whether the proof is
> sufficient to support the phase-3 claims as written. If there are no findings
> at a severity, say so. Do not merely approve the change.

## Scope reviewed

Full working-tree diff from `origin/main`: `dtu/public.go`, `dtu/execution.go`
(new), `dtu/server.go`, `dtu/types.go`, `dtu/control.go`, `dtu/client.go`,
`dtu/cancellation_test.go`, `dtu/execution_test.go` (new), `docs/p1-cancellation-proof.md`,
`ephemeral/P1_ENVIRONMENT_ROADMAP.md`, `README.md`. Cross-read the shared
lifecycle helpers (`transitionWorkflowRun` in `dtu/delivery.go`,
`authenticateInstallationToken`/`permissionRank` in `dtu/public.go`, token
scoping in `dtu/git.go`) and confirmed `go build ./dtu`, `go vet ./dtu`, and
`go test -run TestWorkflowCancellationRaces ./dtu` are green. The two `act`
tests were read but not executed (no `DTU_REQUIRE_ACT=1` runtime here); the
findings below therefore reason about what a green `act` run would and would not
establish.

## Proof-sufficiency verdict

**Partially sufficient — the scripted half proves its claims; the execution half
proves the plumbing but not the "authoritative happy-path CI" wording.**

- The scripted cancellation surface (`TestWorkflowCancellationRaces`) is a
  genuine, well-shaped proof of the async-acceptance race semantics, the `409`
  on terminal runs, the `404` masking for missing runs and Actions-write-less
  tokens, and the go-github `AcceptedError`/`202` contract. No objection.
- The execution half genuinely proves the *mechanics*: real push → run creation,
  bare-repo clone, detached exact-SHA checkout with `HEAD` verification, `act`
  invocation, combined-log capture, conclusion mapping, and active-process
  interruption. It does **not** prove that a *representative* GitHub Actions
  workflow runs, because the "runner" is a substituted Postgres image and the
  proof workflows are deliberately trimmed to busybox `sh` one-liners (finding
  `critical-1`). The roadmap/README/doc phrase "authoritative happy-path CI"
  overreaches what the green test establishes.

Details and concrete fixes follow.

---

## critical

### critical-1 — `postgres:17-alpine` runner substitution makes "real / authoritative CI" an overclaim

`dtu/execution.go:14-18` pins the `act` runner platform image to
`postgres:17-alpine` and maps `ubuntu-latest` to it at `dtu/execution.go:107`
(`--platform ubuntu-latest=postgres:17-alpine`). Both proof workflows are then
written to fit that image's limits: pure `run:` steps with an explicit
`shell: sh` and no `uses:` actions (`dtu/execution_test.go:33-42, 83-92`).

Why this undercuts the claim:

- `postgres:17-alpine` is a musl/busybox image with no `bash`, no `node`, and
  none of the GitHub-hosted-runner toolchain. Any realistic MergeHerder CI
  workflow — `actions/checkout`, `setup-*`, `bash` default shell, node-based
  actions — would fail on it. The test avoids every one of those by construction.
- What a green `DTU_REQUIRE_ACT=1` run therefore proves is: "`act` can execute a
  trivial POSIX-`sh` command inside a Postgres container at the right SHA." That
  is real and useful for the *plumbing*, but it is not "authoritative happy-path
  CI" (`ephemeral/P1_ENVIRONMENT_ROADMAP.md:88-89`) nor "real workflow
  execution" in the sense the goal implies.

The code is correct; the **claim** is the defect. Two acceptable resolutions:

1. Preferred: pin a real runner image (e.g. a `catthehacker/ubuntu:act-*`
   digest) so the proof exercises the actual GitHub-shaped runner surface
   (bash, node, `actions/checkout`), and let the fixture workflow use a
   `uses:`-based step. This makes the "authoritative happy-path CI" wording true.
2. If the Postgres image is retained for size/offline reasons, downgrade the
   prose in `docs/p1-cancellation-proof.md`, `README.md:68-72`, and roadmap
   item 3 to state precisely what is proven ("`act` executes the configured
   `run:`-only workflow against a pinned local container at the exact release
   SHA"), and record the runner-fidelity gap explicitly rather than implying
   parity with a GitHub runner.

Note this is a doc/claim change, which the "do not edit existing docs" constraint
touches — flagging for the author to decide, since the alternative (a real runner
image) is a product-code change.

---

## bug

### bug-1 — execution re-lock path never re-validates `Status == "queued"`; a racing scripted completion is silently resurrected

In `dtu/execution.go`, the run is claimed (`w.activeRuns[run.ID] = nil`,
line 49), the lock is released (line 50), and then all of the slow work — `act`
version probe, image digest verify, clone, detached checkout, HEAD verify
(lines 61-118) — runs **with no lock held**. The lock is re-acquired at line 120,
but the only recheck performed is `run.CancellationRequested` (line 122). It
does **not** re-check `run.Status`.

`transitionWorkflowRun` (`dtu/delivery.go:99-135`) can legally move that same
run `queued → completed` during the unlocked window (it takes `w.mu`, finds the
run by ID, and writes a terminal status). When execution resumes it reads the
now-`completed` run (line 121) and unconditionally calls `command.Start()`, sets
`run.Status = "in_progress"` (line 141), and later overwrites the conclusion
(lines 150-164) — clobbering an authoritative terminal state and emitting a
second `in_progress`/`completed` event pair for a run that was already done.

The two modes are "used separately" by policy, but nothing enforces that, and
the claim block (`dtu/execution.go:37-41`) already advertises "claims each
queued authoritative run exactly once" — which this path violates under the race.

Fix: after re-acquiring the lock at line 120, re-read and re-validate before
starting:

```go
w.mu.Lock()
run = w.workflowRuns[index]
if run.Status != "queued" {           // lost the race to a scripted transition
    delete(w.activeRuns, run.ID)
    claimed = false
    w.mu.Unlock()
    writeControlError(response, http.StatusConflict, "workflow run is no longer queued")
    return
}
if run.CancellationRequested { ... }   // existing branch
```

### bug-2 — forced shutdown `SIGKILL`s `act`, leaking the `--rm` job container and bind mount

`dtu/server.go` `stopActiveRuns(true)` calls `command.Process.Kill()` (SIGKILL)
on the `act` process, and `close` invokes it on the control-shutdown-timeout
escalation path. `act` is launched with `--rm` and `--bind`
(`dtu/execution.go:109-111`), so container teardown is `act`'s responsibility on
exit. SIGKILL gives `act` no chance to run that teardown, so the Docker daemon
keeps the job container (and the bound checkout dir) alive after DTU is gone —
a leaked container/mount, exactly the class the prompt asks about.

The graceful path (`SIGINT`, `stopActiveRuns(false)`) is fine because `act`
traps it and cleans up; only the forced escalation leaks.

Fix options: record the container name via `act --container-name dtu-run-<id>`
(or a deterministic `--container-options --name=...`) and `docker rm -f` it in
the force branch after the kill; or, at minimum, document that a hard
shutdown-timeout can orphan a container and log the run IDs that were force-killed
so the leak is observable rather than silent.

---

## design

### design-1 — cancelled-vs-success depends on `act`'s undocumented exit code

The conclusion switch (`dtu/execution.go:155-162`) checks `waitErr == nil`
*before* `run.CancellationRequested`:

```go
case waitErr == nil:            run.Conclusion = "success"
case run.CancellationRequested: run.Conclusion = "cancelled"
default:                        run.Conclusion = "failure"
```

For an actively interrupted run, "cancelled" is only reported because `act`
happens to exit non-zero on `SIGINT`. That is not a documented `act` contract and
can change across versions; if a future `act` exits `0` on graceful interrupt,
`TestActWorkflowCancellationStopsSupervisor` (`dtu/execution_test.go:120`) would
flip to a false `success` and fail. The `0.2.89` pin masks the fragility but
does not remove it.

The tension is real: "cancel requested but the job finished first" should stay
`success` (this is the Pause race the scripted oracle models), whereas "we
signalled a running process and it stopped" should be `cancelled`. Distinguish
them by whether the signal was actually delivered rather than by exit code —
e.g. record a `signalled bool` when `cancelWorkflowRun`/`stopActiveRuns` signals
this command, and prefer `cancelled` only when `signalled && waitErr != nil`.
This makes the proof robust rather than dependent on an `act` implementation
detail.

### design-2 — documented proof command is under-specified for a fresh host (non-reproducible as written)

`docs/p1-cancellation-proof.md` gives the proof command as:

```sh
DTU_REQUIRE_ACT=1 go test -count=1 -run 'TestActWorkflow' -v ./dtu
```

but the host prerequisites it depends on are only described in prose, not as
runnable setup. `requireActRuntime` (`dtu/execution_test.go:137-149`) hard-gates
on (a) `act` on `PATH` whose `--version` contains exactly `0.2.89`
(`dtu/execution.go:66-70`) and (b) `postgres:17-alpine` present locally with
RepoDigest exactly `postgres@sha256:742f40ea…`. Because `postgres:17-alpine` is
a moving tag, a fresh `docker pull postgres:17-alpine` will generally resolve to
a *newer* digest, so under `DTU_REQUIRE_ACT=1` the tests `t.Fatal` at
`actRuntimeUnavailable` (`dtu/execution_test.go:151-156`) rather than proving
anything. The doc names the digest but never tells the operator to
`docker pull postgres@sha256:742f40ea…` or how to obtain `act 0.2.89`.

Also unverified here: the pinned value is treated as valid on both `amd64` and
`arm64` (`verifyRunnerImage` accepts either, `dtu/execution.go:230`), which only
holds if `742f40ea…` is the multi-arch **index** digest recorded identically on
both arches; if it is a per-arch manifest digest the proof is single-arch only.
Worth stating explicitly.

Fix: add an explicit setup block to the doc — install `act 0.2.89`, then
`docker pull postgres@sha256:742f40ea…` (by digest, not tag) — and note the
architecture(s) the digest is known to cover.

### design-3 — two authoritative mutation paths are not mutually exclusive

`transitionWorkflowRun` and `executeWorkflowRun` can both drive the same run's
lifecycle and both emit `in_progress`/`completed` events. Beyond the concrete
race in bug-1, this is a structural sharp edge: the "scripted oracle" and the
"real engine" write the same fields with no guard preventing a run from being
driven by both. Consider marking a run as execution-claimed in a way that
`transitionWorkflowRun` rejects (e.g. `409` if `activeRuns[id]` is present), so
the separation the prompt asserts is enforced by the code rather than by
convention.

---

## nit

### nit-1 — cancellation auth failure returns `404`, and the negative auth cases are only half-tested

`cancelWorkflowRun` returns `404` for a token lacking Actions-write or repo
scope (`dtu/public.go:70-73`). Returning `404` to hide protected state is a
defensible, documented choice (`docs/p1-cancellation-proof.md` "do not reveal
protected run state") and matches the existing `getPullRequest` pattern
(`dtu/public.go:208`), so no change requested — but note it diverges from real
GitHub, which returns `403` for a visible repo with insufficient permission.
Tests cover the missing-actions-permission `404` (`dtu/cancellation_test.go:68-75`)
but not (a) an entirely invalid/absent token (`401`, `dtu/public.go` authOK
branch) or (b) a token scoped to a *different* repo. The doc claims "invalid
tokens … do not reveal protected run state"; add a `401` assertion so that
sentence is actually proven.

### nit-2 — `run.Logs` is retained in-world and copied into every `/state` snapshot

Captured `act` output is stored on the `WorkflowRun` (`dtu/execution.go:154`,
`dtu/types.go` `Logs` field) and `getState` deep-copies all runs including their
logs (`dtu/control.go:291`). For the trimmed proof workflows this is negligible,
but it means log volume scales into every control-plane state read and lives for
the instance lifetime. Fine for a proof harness; worth a comment so it is not
mistaken for a durable/streamed log store. (Confirmed the `Logs`/
`cancellation_requested` fields are *not* exposed on any public GitHub-shaped
response — `WorkflowRun` is only serialized on the control `/state` path — so
there is no public API-shape leak.)

---

## Things checked that are correct (no finding)

- Claim/placeholder concurrency: the `activeRuns[id] = nil` sentinel correctly
  blocks a second executor with `409 "already claimed"` (`dtu/execution.go:37-41`)
  and the pre-start cancellation window is handled (a cancel during setup sets
  `CancellationRequested`, sees the `nil` process so does not mis-signal, and the
  run is concluded `cancelled` at `dtu/execution.go:122-133`).
- `defer` claim cleanup (`dtu/execution.go:52-59`) releases the sentinel on every
  early error return, so a failed setup leaves the run re-executable rather than
  wedged.
- Signalling an already-reaped process (cancel arriving after `Wait`) is safe:
  `os.Process.Signal` returns "process already finished" and is ignored
  (`dtu/public.go` post-unlock signal, `dtu/server.go` stopActiveRuns).
- `command.Stdout == command.Stderr == &logs` is not a data race: `os/exec`
  detects the shared writer and uses a single copy goroutine; the buffer is only
  read after `command.Wait()` returns.
- `index` stability: `w.workflowRuns` is append-only (only `append` at
  `dtu/git.go:229`), so the index captured before the unlocked window remains
  valid.
- Exact-SHA binding is genuinely proven: detached checkout + `resolveCheckoutHEAD`
  equality gate (`dtu/execution.go:95-102`) plus the test's
  `HeadSHA == releaseSHA` and `head_sha=<sha>` log assertions
  (`dtu/execution_test.go:49-54`).
- Scripted race semantics are faithfully proven, including the deliberate
  "success wins after acceptance" case (`dtu/cancellation_test.go:45-48`) and the
  `409` on a completed run.

## Summary

The change is well-built and the scripted cancellation proof is solid. Blocking
items before this can be called phase-3 "done as written": align the
"authoritative happy-path CI" claim with the Postgres-runner reality
(`critical-1`), close the terminal-state resurrection race (`bug-1`), handle the
forced-shutdown container leak (`bug-2`), and make the documented proof command
actually reproducible on a fresh host (`design-2`). The remaining `design`/`nit`
items harden the proof and its claims.
