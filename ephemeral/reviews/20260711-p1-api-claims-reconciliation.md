# P1 GitHub API Claims Consensus Decision

## Decision

Consensus is **sufficient**. If every claim in
`docs/api-endpoint-claims.md` is rendered into the tests required by that
document and those tests pass, the result constitutes a valid and workable
replication of the designated P1 GitHub REST subset for MergeHerder:

- `POST /app/installations/{installation_id}/access_tokens`; and
- `GET /repos/{owner}/{repo}/pulls/{pull_number}`.

This is a bounded compatibility decision, not a claim of general GitHub API
compatibility and not evidence that the implementation or tests exist or pass.

## Accepted Findings

- D1: Separate missing/foreign installations from inactive installations and
  pin uncertain behavior through live conformance.
- D2: Express resource non-disclosure as equal `404` status and error shape.
- D3: Explicitly exclude fork and cross-repository pull requests.
- D4: Bound the asserted pull-request response fields.
- D5: Apply live conformance to every GitHub-behavior claim.
- D6: Make the JWT `iat` predicate operational under controllable time.
- D7: Pin unknown repositories, invalid permissions, and malformed
  restrictions independently instead of guessing a shared `422` mapping.
- N6: Define head snapshot behavior before and after source-ref deletion.
- N8: Cross-reference the independently pinned inactive-installation failure.

N3 was retained as an intentional implementation convention. N5 and N7 were
accepted as deliberate reinforcement across distinct proof layers.

## Evidence

- `docs/api-endpoint-claims.md`
- `GITHUB_SERVICE_CONTRACT.md`
- `DTU_GITHUB_SPEC.md`
- `docs/proof-scenarios/p01-one-clean-pr.md`
- `ephemeral/reviews/20260711-p1-api-claims-round1.md`
- `ephemeral/reviews/20260711-p1-api-claims-round2.md`
- `ephemeral/reviews/20260711-p1-api-claims-round3.md`

Round 3 contains the final independent Sonnet adjudication: no critical, bug,
or design finding remains open.

## Remaining Proof Work

Consensus on the claims does not discharge them. Correctness still requires:

- `go-github` client tests for both success paths and contracted failures;
- independent assertions on raw response bodies, headers, and error shapes;
- token scope, permission, expiry, coexistence, REST, and Git transport tests;
- real-ref mutation tests for pull-request snapshots;
- per-condition live-GitHub conformance inputs for deferred behavior; and
- MergeHerder system proof through its real TypeScript/Octokit consumer.
