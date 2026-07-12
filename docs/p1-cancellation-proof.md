# P1 Cancellation Proof

## Boundary

This is the scripted cancellation half of roadmap phase 3. It implements
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

## Real HTTP Proof

`TestWorkflowCancellationRaces` proves:

1. `go-github` receives its normal `AcceptedError` and `202` for a queued run;
2. the queued run remains nonterminal with cancellation requested;
3. that run may still race to completed/success;
4. cancelling the completed run returns a GitHub-shaped `409`;
5. an in-progress run accepts cancellation and later completes cancelled;
6. cancellation intent remains visible on the terminal run;
7. a missing run returns `404`; and
8. a token without Actions write permission receives the same protected
   not-found result.

## Remaining Execution Slice

Docker and `act` are available locally, but real workflow execution is not
claimed here. The next phase-3 sub-slice must pin an `act` version and runner
image, check out the exact release SHA, capture logs and conclusion, and bind
process cancellation to the run supervisor. Scripted mode remains authoritative
for GitHub cancellation races that `act` cannot reproduce.
