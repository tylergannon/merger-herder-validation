# P1 Cancellation And Execution Session

started: 2026-07-11 22:29 CST
worktree: `/Users/tyler/src/merger-herder-validation/.worktrees/p1-cancellation-execution`
branch: `codex/p1-cancellation-execution`
base: `81b0613cd2aa9e91039cb5ad97169b4f68756198`

## Goal

Begin roadmap phase 3 with the authoritative scripted Actions cancellation
contract, then define the separate `act`-backed happy-path execution sub-slice.

## Decisions

- decision: Cancellation acceptance records intent and returns `202`; it does
  not make the run terminal.
- decision: Scripted control retains authority over whether a cancellation race
  ends cancelled, failed, or successful.
- decision: Completed runs return `409`; missing/inaccessible runs retain normal
  authentication/not-found behavior.
- decision: `act` execution will be proven separately and will not be used as an
  oracle for GitHub cancellation races.

## Activity

- merge: PR #3 merged as `81b0613 Deliver P1 webhook lifecycle (#3)`.
- environment: Docker server `29.1.5`; `act` version `0.2.89`.
- command: created this worktree directly from verified `origin/main`.
- skill_use: `session-worklog` source=pagerguild/core-tools -> record phase
  decisions, proof, review, and publication state.
- implementation: added the authenticated Actions cancellation endpoint and
  persistent cancellation-requested run state.
- proof: real `go-github` Actions requests cover queued/in-progress acceptance,
  successful and cancelled terminal races, completed conflict, missing run, and
  missing Actions permission.
- implementation: added control-driven `act` execution from an isolated,
  detached checkout of the workflow run's recorded head SHA.
- proof_failure: passing the immutable runner digest directly to `act` caused
  Docker's credential helper to block before container creation even with
  `--pull=false`; the 107-second test run was interrupted.
- correction: using the verified local tag was insufficient because `act`
  still invoked the host's configured credential store proactively.
- decision: execute the local `postgres:17-alpine` runner tag only after the
  server verifies that its repository digest is the pinned proof digest, and
  give `act` an isolated empty Docker client config. This removes host
  credential-helper behavior while preserving an immutable runner contract.
- proof_failure: the isolated runner then completed normally but failed the
  workflow because `candidate.txt` was absent from the container workspace.
- correction: pass `--bind` so the already verified detached checkout is the
  job workspace rather than relying on an unmodeled `actions/checkout` fetch.
- implementation: active execution claims are exclusive; cancellation intent
  accepted during checkout preparation is observed before process start; DTU
  shutdown interrupts active runners and escalates on shutdown timeout.
- proof: real pinned Docker/`act` happy-path and cancellation tests pass with
  `DTU_REQUIRE_ACT=1`, including exact-SHA content, retained logs, terminal
  conclusions, single-executor ownership, and no orphaned runner container.
- proof_failure: the race-enabled cancellation test exposed that cancellation
  may stop `act` before it emits child output, making nonempty child logs an
  invalid invariant.
- correction: prepend deterministic supervisor metadata containing run ID,
  exact SHA, runner tag, and verified digest before process start, then retain
  all child output that actually occurs.
- review: Claude round 1 found one critical proof overclaim, two bugs, three
  design issues, and two nits; it judged scripted cancellation sufficient but
  the Postgres-backed execution proof only partially sufficient.
- accepted: replace the substituted Postgres image with a pinned
  Actions-capable runner, execute pinned `actions/checkout` plus default bash,
  enforce mutual exclusion between scripted and real lifecycle drivers,
  recheck queued state before start, record delivered cancellation signals,
  label runner containers, and clean them explicitly.
- proof_failure: a new Docker label assertion showed `act 0.2.89` can leave its
  job container running after graceful interrupt despite `--rm`.
- correction: clean every labeled run container after signalled execution and
  during both graceful and forced DTU shutdown paths.
- review: Claude round 2 found no critical or bug issues and judged the proof
  sufficient for phase 3, with one unproven documentation subclaim: shutdown
  cleanup had implementation but no real-runtime test.
- implementation: added an instance-and-run-scoped shutdown proof that starts a
  real long job, closes DTU, waits for the execution request, and requires zero
  matching Docker containers. Recorded the pinned `act` nonzero-on-interrupt
  behavior used to distinguish cancellation from a success-wins race.
- review: Claude round 3 independently ran the required Docker/`act` proof,
  full race suite, vet, vulnerability scan, and diff check; it found no
  critical or bug issues, zero leaked containers, and explicitly reached
  consensus that phase 3 sufficiently proves every checked roadmap claim.
- correction: scope the shutdown documentation to the proven graceful path and
  correct the host-dependent test count from two to three.

## Proof And Closeout

- proof: `go vet ./...`, `go test -race -count=1 ./...`, `govulncheck ./...`,
  and `git diff --check` pass.
- proof: required host gate passes all three non-skipped real Docker/`act`
  tests plus scripted cancellation under `-race`; a final Docker label query
  confirms zero leaked runner containers.
- consensus: Claude rounds 1 through 3 and Codex reconciliation are recorded in
  `ephemeral/reviews/20260712-p1-cancellation-execution-*.md`; final consensus
  is that every checked phase-3 roadmap claim has sufficient runnable proof.
- remaining: publish the completed phase to PR #4 and merge it before beginning
  the MergeHerder P1 system harness.
- branch_pr: committed as `e43495b`, pushed, and opened as draft PR
  `https://github.com/tylergannon/merger-herder-validation/pull/4`.
