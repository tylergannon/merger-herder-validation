# MergeHerder Proof-Contract Consensus Reconciliation

Decision: The scenario-first API/interaction contract is ready to drive design
of the separate proof repository, subject to the explicitly unresolved product
choices already marked in the scenarios. No generalized GitHub clone is
authorized.

Evidence:

- `ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md`
- `ephemeral/reviews/20260710-mergeherder-proof-contract-round2.md`
- `ephemeral/reviews/20260710-mergeherder-proof-contract-round3.md`
- `MERGE_HERDER_PROOF_SCENARIOS.md`
- `docs/proof-scenarios/`
- `GITHUB_SERVICE_CONTRACT.md`
- `DTU_GITHUB_SPEC.md`

Accepted findings:

- Use real `workflow_run` actions `requested`, `in_progress`, and `completed`;
  `queued` is the initial status, not an action.
- Keep PR snapshot transport unresolved until the product front door chooses
  REST, `pull_request` webhook, or both.
- Correlate CI by workflow identity plus `(run_id, run_attempt, head_sha)`.
- Prove both Pause cancellation orderings (`202` then success and terminal
  `409` then delayed success).
- Prove that another green workflow at the same SHA is inert.
- Prove invalid webhook signatures are rejected before deduplication.
- Require App installation-token pushes and a release-ref-scoped workflow.
- Record and model the current OAuth organization-scope defect.
- Mark P6 blocked until rerun versus dispatch is selected.
- Attribute default-branch stale landing rejection to receive-pack's
  fast-forward ancestry check.
- Keep a terminal failed run's later events immutable; P4 redelivers/duplicates
  its failure, while P11 separately proves a real stale success is inert.

Rejected finding:

- Explicit `--force-with-lease=refs/heads/R:<expected-old-SHA>` is not merely a
  local check. The expected old object ID is transmitted in the receive-pack
  update command and validated by the server's ref transaction. The proof server
  should use ordinary receive-pack semantics without a custom strengthening
  hook.

Resolved nits:

- Scope rejection is `403`.
- Foreign-owner installation intake has a negative proof.
- Closed PR/source-ref deletion and push `forced` semantics are stated
  accurately.

Unresolved product decisions, intentionally not invented:

- Submit/Pause/Unpause/Cancel transports.
- Batch trigger and ordering.
- Rerun versus dispatch for unchanged-SHA Unpause.
- Treatment of CI triggered while a batch is paused.
- Restart reconciliation/run inspection.

Proof still required: implementation of these contracts in the separate proof
repository and live-GitHub conformance for every GitHub-facing claim.
