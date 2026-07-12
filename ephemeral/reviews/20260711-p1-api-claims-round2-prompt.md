# P1 GitHub API Claims Consensus Review - Round 2

## Goal

Re-review the revised `docs/api-endpoint-claims.md` and determine whether its
claims, if rendered into sufficient tests and satisfied by the implementation,
constitute a valid and workable replication of the designated P1 subset of
GitHub's API surface for MergeHerder.

The bounded REST subset remains:

- `POST /app/installations/{installation_id}/access_tokens`; and
- `GET /repos/{owner}/{repo}/pulls/{pull_number}`.

## Prior Review

Read:

- `ephemeral/reviews/20260711-p1-api-claims-round1.md`; and
- `ephemeral/reviews/20260711-p1-api-claims-round1-prompt.md`.

Round 1 found no critical or bug issues and six design issues. The contract was
revised to:

- distinguish missing or foreign installations (`404`) from inactive
  installations whose exact rejection remains pinned by live conformance;
- replace subjective JWT timing and non-disclosure language with operational
  predicates;
- exclude fork and cross-repository PRs;
- define the asserted PR response-field boundary;
- apply live conformance to every GitHub-behavior claim;
- describe base and head values as current rather than recorded;
- keep controllable time in test requirements rather than GitHub-facing token
  claims; and
- identify the Go server and `go-github` client as proof-world concerns, while
  MergeHerder remains the TypeScript/Octokit system-test consumer.

## Review Instructions

Inspect the current diff and all evidence needed to evaluate the goal. At
minimum, re-read `docs/api-endpoint-claims.md`, the P1 interaction trace,
`GITHUB_SERVICE_CONTRACT.md`, `DTU_GITHUB_SPEC.md`, and the round-1 findings.
Use current primary GitHub documentation or `google/go-github` source where
useful.

Explicitly adjudicate every round-1 critical, bug, and design finding. Check
whether the revisions introduced stronger-than-GitHub behavior, unresolved
placeholders, contradictions, or claims that still cannot become deterministic
tests. Look for new findings at the level of the complete bounded-replication
goal, not merely wording.

Do not edit product code or the contract under review. Write the exact prompt
you received and your complete findings to:

`ephemeral/reviews/20260711-p1-api-claims-round2.md`

The review artifact is the only file you should create or modify.

Label every finding using exactly one definition:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Finish with a clear consensus verdict and any remaining conditions. If a prior
finding is resolved, say why. If it is not, state the precise remaining gap.
