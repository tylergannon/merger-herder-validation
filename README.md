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

## Executable P1 API Proof

The repository now contains the first executable DTU GitHub slice:

- `cmd/dtu-github` starts isolated public and test-control HTTP listeners;
- `dtu` implements App JWT validation, installation-token creation, targeted PR
  reads, controllable time, and explicit unsupported-request diagnostics;
- real bare repositories are served through `git-http-backend`; and
- the proof suite uses `google/go-github`, raw HTTP, the Git CLI, and an actual
  server subprocess across real loopback listeners.

Run the proof with:

```sh
go test -race ./...
```

See [`docs/p1-api-proof.md`](./docs/p1-api-proof.md) for the tested journey,
claim coverage, and remaining proof boundaries. Live-GitHub conformance and the
full MergeHerder P1 system scenario remain separate required layers.

The next completed slice, [`docs/p1-ref-events-proof.md`](./docs/p1-ref-events-proof.md),
proves required ref transitions and derives pending push/workflow state from
real receive-pack results.

[`docs/p1-webhook-lifecycle-proof.md`](./docs/p1-webhook-lifecycle-proof.md)
proves signed delivery, redelivery/duplication, caller-controlled order, and
workflow lifecycle through completion.

Progress toward that complete environment is tracked in
[`ephemeral/P1_ENVIRONMENT_ROADMAP.md`](./ephemeral/P1_ENVIRONMENT_ROADMAP.md).

## License

MIT. See [`LICENSE`](./LICENSE).
