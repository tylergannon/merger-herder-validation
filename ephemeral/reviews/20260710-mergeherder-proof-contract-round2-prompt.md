Here is my goal:

Validate the correctness, completeness, and non-adventitiousness of the revised
MergeHerder proof-scenario API and interaction contract before a separate proof
tool repository is designed. The contract must enumerate only the external
interactions genuinely required to prove MergeHerder's accepted behavior while
remaining faithful to real GitHub, Git, webhook, and Actions semantics.

This is consensus review round 2. Read:

- `ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md`
- `MERGE_HERDER_PROOF_SCENARIOS.md`
- `docs/proof-scenarios/README.md`
- every `docs/proof-scenarios/p*.md` file
- `GITHUB_SERVICE_CONTRACT.md`
- `DTU_GITHUB_SPEC.md` as a downstream draft only
- `ephemeral/worklog/202607101222-merge-queue-domain-seams.md`

Independently verify whether the revised artifacts resolve round-1 findings
without adding accidental product behavior or unnecessary surfaces. Recheck the
whole goal rather than limiting yourself to the prior findings. In particular,
ground any conclusion about explicit
`git push --force-with-lease=<ref>:<expected-old-SHA>` in the Git receive-pack
protocol rather than assuming either a custom server hook or a purely local
guard.

Do not edit product code or the reviewed design documents. Write the exact
prompt you received, findings with file/line evidence, and an overall assessment
to:

`ephemeral/reviews/20260710-mergeherder-proof-contract-round2.md`

Label every finding using exactly one definition:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Explicitly state when a round-1 finding is resolved, rejected with adequate
evidence, still unresolved, or replaced by a new finding. Do not merely approve
the revision.
