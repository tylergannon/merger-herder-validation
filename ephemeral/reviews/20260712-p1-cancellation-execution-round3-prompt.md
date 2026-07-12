# Claude Final Review Prompt: P1 Cancellation And Workflow Execution

Perform the final convergence review of the complete phase-3 diff on
`codex/p1-cancellation-execution`. Read both prior reviews, especially
`ephemeral/reviews/20260712-p1-cancellation-execution-round2.md`, then inspect
the full current diff from `origin/main` and the runnable proof.

Round two judged the phase proof sufficient but identified one unproven doc
subclaim: active runner cleanup during DTU shutdown. The current diff adds
`TestActWorkflowShutdownStopsSupervisor`, which starts a real long-running
Docker/`act` job, captures its DTU-instance and run-ID container labels, closes
the instance, requires the execution request to unblock, and requires zero
matching containers afterward. The proof doc also records the pinned
`act 0.2.89` nonzero-on-interrupt behavior used with a successfully delivered
signal to distinguish cancellation from a success-wins race.

The local required proof is green under:

```sh
DTU_REQUIRE_ACT=1 go test -race -count=1 -run 'TestActWorkflow|TestWorkflowCancellationRaces' -v -timeout=5m ./dtu
go test -race -count=1 ./...
go vet ./...
govulncheck ./...
git diff --check
```

Decide independently whether the phase-3 implementation and tests now
constitute sufficient proof for every checked roadmap claim as written. Look
for remaining false positives, concurrency defects, process/container leaks,
API fidelity gaps, environmental overclaims, and mismatches between code,
tests, docs, and roadmap. Do not edit product code or existing docs.

Write this exact prompt and findings to
`ephemeral/reviews/20260712-p1-cancellation-execution-round3.md`. Label findings
`critical`, `bug`, `design`, or `nit`, with file/line evidence and concrete
fixes. Explicitly state whether consensus is reached that phase 3 is sufficient
proof. If a severity has no findings, say so.
