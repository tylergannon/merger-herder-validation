# Claude Re-review Prompt: P1 Cancellation And Workflow Execution

Re-review the complete phase-3 change on branch
`codex/p1-cancellation-execution` after the corrections prompted by
`ephemeral/reviews/20260712-p1-cancellation-execution-round1.md`.

The goal remains a valid, workable proof environment for MergeHerder's P1
GitHub surface, with GitHub-shaped asynchronous cancellation and real workflow
execution at the exact release SHA. Inspect the full current working-tree diff
from `origin/main`, not only the most recent edits, and independently decide
whether the runnable tests constitute sufficient proof of the phase-3 claims.

Round-1 changes include:

- replaced the Postgres substitute with the Actions-capable
  `catthehacker/ubuntu:act-22.04` multi-arch manifest pinned by digest;
- the fixture now executes commit-pinned `actions/checkout` and default bash;
- scripted lifecycle transitions reject an execution-claimed run, and the
  executor rechecks queued state before process start;
- cancellation conclusions depend on a successfully delivered interrupt plus
  nonzero process exit, preserving success-wins races;
- every job container is labeled by DTU-instance hash and run ID, then removed
  explicitly on cancellation and shutdown; the test proves no labeled
  container remains;
- cancellation negatives now cover invalid tokens and wrong-repository scope;
  and
- the proof document contains digest-qualified installation commands and
  supported manifest architectures.

The current proof is green under:

```sh
DTU_REQUIRE_ACT=1 go test -race -count=1 -run 'TestActWorkflow|TestWorkflowCancellationRaces' -v -timeout=5m ./dtu
go test -race -count=1 ./...
go vet ./...
govulncheck ./...
git diff --check
```

Assess implementation correctness, proof sufficiency, API fidelity,
concurrency, process/container lifecycle, reproducibility, and claim wording.
Try to falsify the tests or identify unproven assertions. Do not edit product
code or existing docs. Pointer fields remain disallowed unless genuinely shared
or noCopy; use Go 1.26 `new(value)`, never `github.Ptr`, for external pointer
APIs.

Write the exact prompt and findings to
`ephemeral/reviews/20260712-p1-cancellation-execution-round2.md`. Label every
finding `critical`, `bug`, `design`, or `nit`, with file/line evidence and a
concrete fix. Explicitly state whether the proof is now sufficient for the
phase-3 claims as written. If no findings exist at a severity, say so.
