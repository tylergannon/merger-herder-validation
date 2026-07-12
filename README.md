# MergeHerder Validation

Independent behavioral contracts and proof-harness design for
[MergeHerder](https://github.com/tylergannon/merge-herder).

This repository defines what MergeHerder must prove before it defines the
implementation that will prove it. It is intentionally separate from the
application so the acceptance standard cannot fit itself to MergeHerder's
internal implementation.

## Start Here

- [`MERGE_HERDER_PROOF_SCENARIOS.md`](./MERGE_HERDER_PROOF_SCENARIOS.md)
  defines the black-box product scenarios and pass conditions.
- [`docs/proof-scenarios/`](./docs/proof-scenarios/) expands every scenario into
  its ordered product, GitHub REST, webhook, Git, workflow, worker, harness, and
  observation interactions.
- [`GITHUB_SERVICE_CONTRACT.md`](./GITHUB_SERVICE_CONTRACT.md) inventories the
  exact GitHub behavior on which those scenarios depend.
- [`docs/api-endpoint-claims.md`](./docs/api-endpoint-claims.md) defines the
  executable correctness claims for the P1 REST endpoints.
- [`DTU_GITHUB_SPEC.md`](./DTU_GITHUB_SPEC.md) is a downstream infrastructure
  draft. It is subordinate to the scenarios and may be reduced or replaced as
  implementation begins.

## Independent Review

The contract received three adversarial Claude Opus review rounds followed by a
Codex reconciliation:

- [Round 1](./ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md)
- [Round 2](./ephemeral/reviews/20260710-mergeherder-proof-contract-round2.md)
- [Round 3](./ephemeral/reviews/20260710-mergeherder-proof-contract-round3.md)
- [Final reconciliation](./ephemeral/reviews/20260710-mergeherder-proof-contract-reconciliation.md)

The full product-design history is retained in
[`ephemeral/worklog/202607101222-merge-queue-domain-seams.md`](./ephemeral/worklog/202607101222-merge-queue-domain-seams.md).

## Proposed Implementation Direction

The current recommendation for the future proof runtime is:

- Go with `net/http`;
- `libopenapi` over a mechanically selected subset of GitHub's upstream OpenAPI
  document;
- `google/go-github` request, response, and webhook types;
- real Git smart HTTP through `git-http-backend`;
- pinned `act` and Docker for real workflow execution; and
- a tiny latest-Octokit conformance client for the named live-GitHub contracts.

This repository currently contains specifications and review evidence, not the
proof-tool implementation.

## License

MIT. See [`LICENSE`](./LICENSE).
