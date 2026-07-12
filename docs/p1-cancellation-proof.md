# P1 Cancellation And Execution Proof

## Boundary

The scripted half of roadmap phase 3 implements
`POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel` through the real
`go-github` Actions client.

An active queued or in-progress run returns `202 Accepted` and records
`cancellation_requested=true`. Acceptance does not choose or synthesize the
terminal result. The private control plane may subsequently complete that run
as cancelled, failed, or successful, preserving the Pause race in which success
arrives after cancellation was accepted.

Completed runs return `409`. Missing runs, inaccessible repositories, invalid
tokens, and tokens without Actions write permission do not reveal protected run
state.

The execution half claims each queued authoritative run exactly once, clones
the real bare repository into an isolated checkout, detaches at the run's
recorded `head_sha`, verifies `HEAD`, and executes the configured workflow with
`act 0.2.89`. The runner is the Actions-capable
`catthehacker/ubuntu:act-22.04` image only after its multi-architecture manifest
digest is verified as
`catthehacker/ubuntu@sha256:93b433d1c736e9c4361edf3bd4ea47434fa6323c4e70fdf34f826280584bab2d`.
The fixture runs `actions/checkout` pinned at
`34e114876b0b11c390a56381ad16ebd13914f8d5` and a default-bash verification
step. `act` receives an empty Docker client credential config while still using
the normal host daemon, the verified checkout is bound as its workspace,
pulling is disabled, and the local image architecture is passed explicitly.

Runner startup emits the authoritative `in_progress` event. Process completion
captures combined logs and emits `completed` with `success`, `failure`, or
`cancelled` on the same run ID, attempt, branch, and SHA. A public cancellation
request interrupts an active `act` process. Every job container is labeled by
run ID and explicitly removed after interruption. DTU shutdown interrupts and
removes remaining active runners. The real proof covers graceful shutdown;
timeout escalation remains defensive cleanup rather than a claimed behavior.

An active execution is concluded `cancelled` only when its interrupt was
successfully delivered and pinned `act 0.2.89` exits nonzero. A job that reaches
successful process exit first remains successful even if cancellation intent
was recorded, matching the separately scripted success-wins race.

## Real HTTP Proof

`TestWorkflowCancellationRaces` proves:

1. `go-github` receives its normal `AcceptedError` and `202` for a queued run;
2. the queued run remains nonterminal with cancellation requested;
3. that run may still race to completed/success;
4. cancelling the completed run returns a GitHub-shaped `409`;
5. an in-progress run accepts cancellation and later completes cancelled;
6. cancellation intent remains visible on the terminal run;
7. a missing run returns `404`;
8. an invalid token returns `401`;
9. a token scoped to another repository receives the protected `404`; and
10. a token without Actions write permission receives the same protected
    not-found result.

## Real Execution Proof

`TestActWorkflowExecutionAtExactSHA` proves a real Git push creates the run,
the run's exact detached checkout supplies the workflow workspace, Docker and
`act` execute the configured workflow, and the recorded logs and successful
conclusion remain bound to its release SHA.

`TestActWorkflowCancellationStopsSupervisor` starts a real long-running
workflow, waits for `in_progress`, cancels it through `go-github`, observes
`act` stop, and proves the same run completes cancelled with cancellation intent
and retained runner logs. It also proves the runner container received the
run-ID label and no labeled container remains after cancellation.

`TestActWorkflowShutdownStopsSupervisor` starts another real long-running job,
captures both its DTU-instance and run-ID labels, closes the DTU, and proves the
execution request unblocks and no matching container remains.

Install the pinned prerequisites and tag the digest-qualified image for local,
no-pull execution:

```sh
go install github.com/nektos/act@v0.2.89
docker pull catthehacker/ubuntu@sha256:93b433d1c736e9c4361edf3bd4ea47434fa6323c4e70fdf34f826280584bab2d
docker tag catthehacker/ubuntu@sha256:93b433d1c736e9c4361edf3bd4ea47434fa6323c4e70fdf34f826280584bab2d catthehacker/ubuntu:act-22.04
```

The pinned manifest contains `linux/amd64`, `linux/arm64/v8`, and `linux/arm/v7`;
the proof accepts and passes the local `amd64` or `arm64` image architecture to
Docker explicitly. Then run the required host proof command:

```sh
DTU_REQUIRE_ACT=1 go test -count=1 -run 'TestActWorkflow' -v ./dtu
```

Without `DTU_REQUIRE_ACT=1`, these three host-dependent tests skip when the pinned
runtime is unavailable. Scripted mode remains authoritative for GitHub
cancellation races that `act` cannot reproduce.
