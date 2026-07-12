# P1 API Proof Review - Claude Round 1

## Goal

Build a valid, workable first executable proof environment for the designated
P1 subset of GitHub used by MergeHerder. It must start real HTTP servers and
prove behavior by making real requests rather than relying only on handler unit
tests. The bounded public surface is installation-token creation, targeted PR
retrieval, and the Git smart-HTTP effect of the minted token. Setup, time
control, and inspection belong to a distinct private control listener.

The proof must use `google/go-github` as a wire-level client oracle while also
asserting raw responses, real Git behavior, virtual expiry, and executable
startup. It must not claim that this alone proves live-GitHub parity or the full
MergeHerder P1 system scenario. Internal data should use concrete values;
pointers are appropriate only for shared state, lifecycle/no-copy types, or
external APIs that require them. Do not use `github.Ptr`; Go 1.26 `new(value)`
is the convention.

## Review Instructions

If you have a code review skill, use it. Otherwise review directly. Give a
complete review of the current design, implementation, documentation, and
proof against the goal. Inspect the full branch diff and repository contracts,
not only the obvious Go files. Run relevant proof commands and explore source
or dependency behavior where useful.

Prioritize demonstrable bugs, races, security problems, incorrect Git or GitHub
semantics, missing claim coverage, proof that bypasses the intended boundary,
over-engineering, unrequested layers, lifecycle failures, wrong ownership
boundaries, and documentation that overstates what passed. Prefer real findings
over style preferences.

Do not edit product code, tests, contracts, or documentation. Write the exact
prompt you received and your complete findings only to:

`ephemeral/reviews/20260711-p1-api-proof-claude-round1.md`

Label every finding using exactly one definition:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Finish with a clear verdict on the implementation and the proof. Passing tests
alone is not consensus; identify any remaining conditions precisely.
