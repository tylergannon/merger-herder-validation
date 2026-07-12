# P1 Environment Roadmap

## Goal

Track the executable environment required to prove
[`P1: One Clean PR Lands`](../docs/proof-scenarios/p01-one-clean-pr.md) against a
real MergeHerder process. A phase is complete only when its behavior is
implemented, exercised across its real boundary, and linked to proof evidence.

Status meanings:

- `[x]` implemented and proven within the named boundary;
- `[~]` implemented in part or inherited from a real subsystem but not yet
  proven against the complete contract; and
- `[ ]` not implemented.

## 0. API And Git Substrate

- [x] Isolated process startup and shutdown.
- [x] Separate public GitHub/Git and private control listeners.
- [x] Control creation of Apps, installations, repositories, and pull requests.
- [x] Controllable time and bounded state inspection.
- [x] `APP-01`: App JWT validation and scoped installation-token creation.
- [x] Targeted REST portion of `PR-01`, including real-ref-backed snapshots.
- [x] Installation-token authentication and scope across REST and Git.
- [x] Raw HTTP, `go-github`, Git CLI, virtual-expiry, and subprocess proof.
- [~] `GIT-01`: real smart-HTTP fetch works, but the complete required object and
  unavailable-SHA matrix is not yet recorded as proof.
- [~] `GIT-02`: real receive-pack works, but release-ref creation,
  fast-forward, and guarded force-with-lease behavior are not yet proven.
- [~] `GIT-03`: real receive-pack supplies normal Git fast-forward rules, but
  default-branch landing and stale/non-fast-forward rejection are not yet
  proven as named scenarios.

Evidence:

- [`docs/p1-api-proof.md`](../docs/p1-api-proof.md)
- [`ephemeral/reviews/20260711-p1-api-proof-reconciliation.md`](./reviews/20260711-p1-api-proof-reconciliation.md)
- [PR #1](https://github.com/tylergannon/merger-herder-validation/pull/1)

## 1. Ref Transitions And Event Creation

This is the next implementation slice. It makes Git state transitions produce
the GitHub-side facts consumed by later webhook and CI layers.

- [ ] Prove `GIT-01` fetch and unavailable/unauthorized object behavior.
- [ ] Prove `GIT-02` release-ref creation and fast-forward update.
- [ ] Prove guarded release-ref rebuild with exact force-with-lease behavior.
- [ ] Prove rejected pushes change no refs and create no events.
- [ ] Prove `GIT-03` default-branch fast-forward landing.
- [ ] Prove stale and non-fast-forward default-branch pushes fail without ref
      movement.
- [ ] Create an immutable pending `push` event from each accepted branch push
      using the actual before/after SHAs.
- [ ] Create one queued authoritative workflow run when the configured release
      ref moves, bound to its exact head SHA.
- [ ] Ensure Git state commits before event and workflow records become visible.

Completion evidence:

- named real-Git tests for `GIT-01` through `GIT-03`;
- control inspection of refs, pending events, and workflow runs; and
- negative proof that rejected updates produce neither state nor events.

## 2. Webhook Delivery And CI Lifecycle

- [ ] Separate event creation from delivery.
- [ ] List pending and attempted deliveries through control.
- [ ] Deliver signed immutable `push` and `workflow_run` payloads to a configured
      receiver.
- [ ] Preserve GUID and raw body on redelivery; create a new GUID for semantic
      duplicates.
- [ ] Allow delivery withholding, reordering, duplication, and deliberate
      signature failure.
- [ ] Move authoritative runs through queued, in-progress, and completed states.
- [ ] Emit `workflow_run` actions `requested`, `in_progress`, and `completed`
      with exact workflow, run, attempt, branch, and SHA identity.
- [ ] Prove `CI-01`, `CI-02`, and `HOOK-06` across real HTTP delivery.

## 3. Cancellation And Workflow Execution

- [ ] Implement `CI-03` Actions run cancellation with asynchronous acceptance.
- [ ] Script cancellation/completion races used by Pause scenarios.
- [ ] Execute the configured repository workflow through pinned `act` and
      Docker for authoritative happy-path CI.
- [ ] Bind checkout, run state, logs, and conclusion to the exact release SHA.
- [ ] Keep scripted mode for cancellation semantics that `act` cannot reproduce.

## 4. MergeHerder P1 System Harness

- [ ] Start real DTU GitHub and MergeHerder processes together.
- [ ] Configure MergeHerder's real GitHub/Octokit and Git origins for DTU.
- [ ] Establish the real submit-to-queue front door.
- [ ] Connect the real worker boundary and return the squash result `R0=M-SA`.
- [ ] Drive the ordered P1 REST, Git, webhook, workflow, and worker interactions.
- [ ] Observe `main==R0`, source `A==A0`, CI success for `R0`, and no current
      batch.
- [ ] Fail on every unsupported GitHub-shaped request.

## 5. Independent Compatibility

- [ ] Add the latest TypeScript/Octokit consumer proof for the implemented REST
      and webhook surfaces.
- [ ] Run narrow live-GitHub conformance for every externally asserted behavior.
- [ ] Pin inactive-installation and token-validation status/error shapes from
      independent live observations.
- [ ] Record any deliberate DTU divergence without weakening the P1 pass
      conditions.

## Explicitly Outside Current P1

- OAuth, users, browser sessions, and user tokens;
- fork and cross-repository pull requests;
- general GitHub REST compatibility;
- arbitrary Actions, checks, logs, jobs, or workflow APIs; and
- product scenarios P2 through P14 until P1 passes end to end.
