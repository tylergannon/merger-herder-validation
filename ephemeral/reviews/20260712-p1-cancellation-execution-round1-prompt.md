# Claude Review Prompt: P1 Cancellation And Workflow Execution

Review the complete phase-3 change on branch
`codex/p1-cancellation-execution` in this repository against the user's actual
goal: build a valid, workable proof environment for MergeHerder's designated P1
GitHub surface, including GitHub-shaped asynchronous Actions cancellation and
real workflow execution at the exact release SHA.

Inspect the full diff from `origin/main`, the surrounding implementation and
contracts, especially:

- `dtu/public.go`, `dtu/execution.go`, and their shared state/lifecycle;
- `dtu/cancellation_test.go` and `dtu/execution_test.go`;
- `docs/p1-cancellation-proof.md`;
- `ephemeral/P1_ENVIRONMENT_ROADMAP.md`; and
- prior phase behavior on which this change depends.

Assess both implementation correctness and proof sufficiency. In particular,
decide whether the runnable tests, when green with `DTU_REQUIRE_ACT=1`, really
constitute proof of the asserted phase-3 behavior across real HTTP, `go-github`,
Git smart HTTP, detached exact-SHA checkout, `act`, Docker, runner supervision,
logs, conclusions, and cancellation races. Look for false-positive tests,
unproven claims, concurrency/lifecycle defects, authentication or API-shape
errors, environmental non-reproducibility, leaked processes/containers, and
roadmap overclaims. Do not limit the review to these examples.

Repository constraints:

- Do not edit product code or existing docs.
- Pointer fields are disallowed unless data is genuinely shared or the type is
  noCopy; do not recommend `github.Ptr`, because Go 1.26 `new(value)` is the
  required spelling for external pointer-heavy APIs.
- Scripted cancellation remains the oracle for GitHub races `act` cannot
  reproduce; `act` is the real happy-path execution engine.

Write your review to
`ephemeral/reviews/20260712-p1-cancellation-execution-round1.md`. Include this
exact prompt, then findings labeled `critical`, `bug`, `design`, or `nit`, with
file/line evidence and concrete fixes. State explicitly whether the proof is
sufficient to support the phase-3 claims as written. If there are no findings
at a severity, say so. Do not merely approve the change.
