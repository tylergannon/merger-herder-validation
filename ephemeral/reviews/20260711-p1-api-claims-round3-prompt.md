# P1 GitHub API Claims Consensus Review - Round 3

## Goal

Make the final consensus determination whether the claims in
`docs/api-endpoint-claims.md`, if rendered into sufficient tests and satisfied
by the implementation, constitute a valid and workable replication of the
designated P1 subset of GitHub's API surface for MergeHerder.

The bounded REST subset remains:

- `POST /app/installations/{installation_id}/access_tokens`; and
- `GET /repos/{owner}/{repo}/pulls/{pull_number}`.

## Prior Reviews And Revision

Read both prior review artifacts and prompts under `ephemeral/reviews/`.
Round 1's six design findings were adjudicated as resolved in round 2. Round 2
found one remaining design issue, D7: `TOKEN-19` ambiguously mapped three
distinct request-validation failures to `422` under the collective phrase
"invalid restrictions."

The contract now states separately that invalid JWTs produce `401` and missing
or foreign installations produce `404`. It requires unknown repositories,
invalid permissions, and malformed restrictions to be pinned independently to
live-conformance observations, and forbids tests from assuming a shared status
merely from GitHub's generic validation-failure documentation.

The contract also resolves round 2's N6 by making the current-head rule apply
while the source ref exists and tying the retained post-deletion snapshot to
`PR-14`. N5 and N7 are deliberate reinforcement across distinct proof layers,
not contradictions.

## Review Instructions

Inspect the current claims document, the prior reviews, and the relevant
contracts and P1 trace. Explicitly adjudicate D7 and verify that the revision
does not leave a placeholder that prevents deterministic tests: a claim may
use a recorded live-conformance result as test input, but the resulting tests
must pin each condition separately. Recheck whether any critical, bug, or
design issue remains at the level of the complete bounded-replication goal.

Do not edit product code or the contract under review. Write the exact prompt
you received and your complete findings to:

`ephemeral/reviews/20260711-p1-api-claims-round3.md`

The review artifact is the only file you should create or modify.

Label every finding using exactly one definition:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Finish with a binary consensus verdict. State whether the claims, subject to
their named proof layers and live-conformance inputs, are sufficient for this
bounded API replication. Do not equate consensus on the contract with proof
that an implementation already exists or passes.
