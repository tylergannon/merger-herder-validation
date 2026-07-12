# P1 GitHub API Claims Consensus Review - Round 1

## Goal

Determine whether the claims in `docs/api-endpoint-claims.md`, if rendered into
sufficient tests and satisfied by the implementation, constitute a valid and
workable replication of the designated P1 subset of GitHub's API surface for
MergeHerder.

The designated REST subset is:

- `POST /app/installations/{installation_id}/access_tokens`; and
- `GET /repos/{owner}/{repo}/pulls/{pull_number}`.

The intended claim is deliberately bounded: this should reproduce the REST and
token behavior MergeHerder needs for P1. It must not imply general GitHub API
compatibility. Git smart HTTP, webhook delivery, workflow execution, and
MergeHerder's coordinator behavior are adjacent proof boundaries rather than
additional REST endpoints.

## Evidence To Review

Read at minimum:

- `docs/api-endpoint-claims.md`;
- `MERGE_HERDER_PROOF_SCENARIOS.md`;
- `docs/proof-scenarios/p01-one-clean-pr.md`;
- `docs/proof-scenarios/README.md`;
- `GITHUB_SERVICE_CONTRACT.md`;
- `DTU_GITHUB_SPEC.md`;
- `ephemeral/reviews/20260710-mergeherder-proof-contract-reconciliation.md`;
- the current uncommitted diff; and
- current primary GitHub documentation and `google/go-github` source where
  useful.

MergeHerder is available at `/Users/tyler/src/merge-herder`. Its root checkout
contains unrelated user changes, so inspect it without modifying or cleaning
that worktree. Use current `origin/main` when repository state matters.

## Review Questions

Review the design at the level of the stated goal, not just document wording.
In particular, determine:

- whether every behavior needed by P1's two REST endpoints is claimed;
- whether any listed claim is wrong, contradictory, unimplementable, or broader
  than P1 requires;
- whether authentication, JWT validation, token issuance, token scope,
  expiration, error behavior, and non-disclosure are sufficiently specified;
- whether the targeted PR snapshot remains correct as PR state and real Git refs
  change;
- whether `go-github` client tests plus the stated behavioral, Git, system, and
  live-conformance layers form a sound oracle;
- whether the explicit exclusions leave any hole that would allow an
  implementation to pass while being incompatible with MergeHerder;
- whether response/error field requirements need to be more concrete;
- whether test controls are accidentally being confused with GitHub-facing
  behavior; and
- whether the resulting implementation would be maintainable and realistically
  achievable without becoming a generalized GitHub clone.

Do not demand unrelated GitHub fidelity or speculative future-scenario support.
Prefer a smaller complete contract over generalized completeness.

## Output

Write the exact prompt you received and your complete findings to:

`ephemeral/reviews/20260711-p1-api-claims-round1.md`

Do not edit product code or the contract under review. The review artifact is
the only file you should create or modify.

Label every finding using exactly one of these definitions:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

If there are no findings at a severity, state that explicitly. Finish with a
clear verdict on whether the current claims are sufficient for the bounded
replication claim, and list any conditions attached to that verdict.
