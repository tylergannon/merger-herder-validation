# P1 API Proof Review - Claude Round 2

## Goal

Re-review the complete P1 API proof implementation after the round-1 findings.
Determine whether the current runtime and tests are now a valid, workable real
HTTP/process/Git proof for the bounded installation-token and targeted-PR
surface, without conflating this slice with live-GitHub conformance or the full
MergeHerder P1 system scenario.

## Round-1 Reconciliation

Read `ephemeral/reviews/20260711-p1-api-proof-claude-round1.md` and inspect the
current implementation rather than relying on this summary.

- Finding 1's semantic assertion was rejected with current primary evidence.
  GitHub's documentation for `GET
/repos/{owner}/{repo}/pulls/{pull_number}` says a fine-grained token must have
  at least one of Pull Requests read or Contents read. `PR-05` and
  `canReadPulls` therefore remain. The valid proof criticism was accepted:
  tests now mint separate tokens and successfully read the PR through each
  permission branch in isolation.
- Finding 2 was accepted. Git requests are rewritten to the repository's
  canonical authorized owner/name path before `git-http-backend`, removing
  filesystem-dependent casing. The network suite sends a mixed-case Git URL.
- Finding 3 was accepted. A raw token-success request now independently asserts
  `201`, token, expiry, effective permissions, and repository scope.
- Finding 4 was accepted. PR refs are snapshotted under a read lock, resolved
  through Git subprocesses outside the lock, then conditionally stored.
- Finding 5 was accepted. The executable test calls both control and public
  listeners before shutdown.
- Finding 6 was accepted. The complete focused rejection table is bracketed by
  mutation-counter assertions.

The implementation continues to treat inactive-installation and validation
statuses as provisional pending the separately required live-conformance layer.

## Review Instructions

Adjudicate every round-1 critical, bug, and design finding and inspect the
revisions for new bugs, races, proof gaps, or overstatement. Run relevant proof
commands. Do not edit product code, tests, contracts, or documentation. Write
the exact prompt and complete findings only to:

`ephemeral/reviews/20260711-p1-api-proof-claude-round2.md`

Label every finding using exactly one definition:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Finish with a clear consensus verdict. Do not claim that this bounded proof
discharges the named live-GitHub or MergeHerder system layers.
