# P1 Cancellation And Workflow Execution Reconciliation

## Goal

Determine whether roadmap phase 3 implements and sufficiently proves the
GitHub-shaped cancellation contract plus real exact-SHA workflow execution
needed by the MergeHerder P1 environment.

## Codex Assessment

The initial implementation established the intended boundaries but did not yet
justify its "authoritative happy-path CI" wording: its Postgres runner could
only execute deliberately reduced shell steps. It also needed mutual exclusion
between scripted and real run drivers and deterministic runner cleanup.

Codex accepted every material Claude finding and revised both implementation
and proof rather than weakening the checked roadmap claims.

## Claude Findings And Resolution

Round 1 judged scripted cancellation sufficient and execution only partially
sufficient. Accepted corrections:

- replaced the substituted Postgres image with an Actions-capable runner pinned
  by multi-architecture manifest digest;
- ran commit-pinned `actions/checkout` and default bash in the fixture;
- made scripted and real run mutation paths mutually exclusive and rechecked
  terminal state before runner start;
- distinguished delivered cancellation from intent and preserved success-wins
  races;
- labeled containers by DTU instance and run ID, then cleaned them explicitly;
- documented reproducible `act` and image installation; and
- added invalid-token and wrong-repository cancellation negatives.

Round 2 found no critical or bug issues and judged the phase proof sufficient,
but noted shutdown cleanup was documented without a real-runtime test. The
accepted correction added `TestActWorkflowShutdownStopsSupervisor`.

Round 3 ran the required proof on the real host runtime. It found no critical or
bug issues, observed zero leaked containers, and explicitly reached consensus
that every checked phase-3 roadmap claim has sufficient runnable proof. Its
remaining documentation residuals were resolved by scoping shutdown prose to
the tested graceful path and correcting the test count.

## Final Proof Shape

- real `go-github` cancellation requests across loopback HTTP;
- real installation-token-authenticated Git push into the DTU bare repository;
- immutable workflow run creation at the pushed release SHA;
- isolated detached checkout with exact `HEAD` verification;
- pinned `act 0.2.89`, digest-verified Actions runner, pinned checkout action,
  default bash, and the normal host Docker daemon;
- authoritative run state, webhook events, logs, and conclusion bound to the
  same run ID, attempt, branch, and SHA;
- scripted success-wins and cancelled races;
- active process interruption and instance-scoped zero-container cleanup; and
- race, vet, vulnerability, and diff-integrity gates.

## Consensus

Consensus reached: phase 3 is implemented and the tests constitute sufficient
proof for every checked phase-3 roadmap claim. No unresolved critical, bug, or
design dissent remains.
