Here is my goal:

Perform the final independent validation of the MergeHerder proof-scenario API
and interaction contract before a separate proof-tool repository is designed.
The contract must be correct, complete for accepted behavior, faithful to real
GitHub/Git/Actions semantics, and free of invented product behavior or
unnecessary surfaces.

Read both prior reviews and all current artifacts:

- `ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md`
- `ephemeral/reviews/20260710-mergeherder-proof-contract-round2.md`
- `MERGE_HERDER_PROOF_SCENARIOS.md`
- `docs/proof-scenarios/README.md`
- every `docs/proof-scenarios/p*.md`
- `GITHUB_SERVICE_CONTRACT.md`
- downstream `DTU_GITHUB_SPEC.md`
- relevant decisions in
  `ephemeral/worklog/202607101222-merge-queue-domain-seams.md`

Verify the whole goal again, including round-2 resolutions: auxiliary
non-authoritative workflow runs, invalid-signature delivery, OAuth scope
enforcement, P6's blocked/unresolved fresh-run mechanism, and P10's
fast-forward rejection wording. Find new issues if they exist rather than merely
checking boxes.

Do not edit product code or reviewed design documents. Write the exact prompt,
findings with evidence, and final assessment to:

`ephemeral/reviews/20260710-mergeherder-proof-contract-round3.md`

Label every finding exactly:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Explicitly say whether any critical, bug, or design finding remains. Do not
claim consensus merely because earlier findings were edited.
