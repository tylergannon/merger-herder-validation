Here is my goal:

Validate the correctness, completeness, and non-adventitiousness of the proposed
MergeHerder proof-scenario API and interaction contract before a separate proof
tool repository is designed. The contract must enumerate the external
interactions genuinely required to prove MergeHerder's accepted behavior,
without omitting correctness-critical GitHub/Git/CI interactions and without
inventing APIs, product behavior, or infrastructure that the scenarios do not
require.

Review these artifacts together:

- `MERGE_HERDER_PROOF_SCENARIOS.md`
- `docs/proof-scenarios/README.md`
- every `docs/proof-scenarios/p*.md` file
- `GITHUB_SERVICE_CONTRACT.md`
- the accepted/corrected product decisions in
  `ephemeral/worklog/202607101222-merge-queue-domain-seams.md`
- `DTU_GITHUB_SPEC.md` only as a downstream draft whose assumptions must not
  override the scenario-first contract

Review the design before implementation. Independently inspect the repository
and authoritative GitHub/Git/act documentation where useful. Look for:

- incorrect GitHub REST paths, webhook assumptions, payload/order semantics,
  status codes, authentication behavior, or Git smart-HTTP interactions;
- missing boundary interactions needed for the scenario to be executable and
  for its pass conditions to be trustworthy;
- interactions listed in the wrong actor order or on the wrong ownership side;
- mocked or controlled behavior that must be real for the proof to mean
  anything;
- real behavior that can safely be controlled without weakening the proof;
- scenario behavior that was invented, prematurely settled, or copied from
  stale agent-authored notes rather than accepted product intent;
- cases where the interaction contract is coupled to MergeHerder internals or
  to one prospective proof-tool implementation;
- contradictions among the scenario overview, per-scenario traces, GitHub
  service contract, and product worklog; and
- important proof scenarios or race/failure interactions missing from the
  current set.

Do not merely approve the design and do not propose a general GitHub clone.
Prefer the smallest interaction surface that can honestly prove the scenarios.

Do not edit product code or the reviewed design documents. Write the exact
prompt you were given, your evidence-backed findings, and a concise overall
assessment to:

`ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md`

Use file/line references where possible. Label every finding using exactly one
of these definitions:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.
