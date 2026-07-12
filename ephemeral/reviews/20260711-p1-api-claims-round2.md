# P1 GitHub API Claims Consensus Review - Round 2

## Prompt Received

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

---

## Evidence Reviewed

- `docs/api-endpoint-claims.md` (current revised version, untracked)
- `ephemeral/reviews/20260711-p1-api-claims-round1.md` and its prompt
- `GITHUB_SERVICE_CONTRACT.md`, including its working diff (adds one paragraph
  linking `docs/api-endpoint-claims.md` from the `APP-01`/`PR-01` section; no
  other change)
- `README.md` diff (adds one pointer sentence to `docs/api-endpoint-claims.md`)
- `DTU_GITHUB_SPEC.md`, in full
- `docs/proof-scenarios/p01-one-clean-pr.md` (the P1 interaction trace)
- `git status` / `git diff` for the current uncommitted change set
- Live GitHub REST documentation for "Create an installation access token for
  an app" (current status-code list: `201`, `401`, `403`, `404`, `422`, with no
  further documented breakdown of which failure condition maps to which of
  `403`/`404`/`422`)
- Live GitHub documentation for "Generating a JSON Web Token (JWT) for a GitHub
  App" (confirms the 60-second backdating recommendation for `iat` to absorb
  clock drift; does not itself state whether a future `iat` is rejected)
- `google/go-github` `apps.go` current `master`
  (`CreateInstallationToken`, `InstallationTokenOptions`, `InstallationToken`):
  confirms the client performs no client-side validation of
  `Repositories`/`RepositoryIDs`/`Permissions` — any rejection of an invalid
  combination is entirely server-side, so the claims document is the only
  place this behavior can be pinned down for the DTU

## Adjudication Of Round-1 Findings

### D1 — `TOKEN-18`'s `403`/`404` split was underspecified

**Resolved.** `TOKEN-08` now isolates "installation exists and belongs to the
authenticated App" and states plainly that a missing installation and one
owned by another App both return `404`. `TOKEN-09` isolates the "installation
is active" precondition and explicitly defers its status/error shape to live
conformance rather than asserting an unverified `403`. `TOKEN-19` restates the
same split at the top level (`401` for invalid JWTs, `404` for missing/foreign
installations) and explicitly funnels everything else through live
conformance. This is exactly the fix D1 recommended, and it matches the
community-reported real-GitHub behavior already cited in round 1. See D7 below
for a related, newly introduced gap in `TOKEN-19`'s coverage.

### D2 — `SCOPE-06`'s non-disclosure claim was not independently testable

**Resolved.** `SCOPE-06` now reads: "For protected REST resources, a missing
resource and a resource outside the token's repository scope return the same
`404` status and GitHub-compatible error shape." This is precise and
directly mirrors `PR-15`'s already-testable pattern, exactly as recommended.

### D3 — Fork/cross-repository PRs were never explicitly excluded

**Resolved.** "Explicitly Excluded From P1" now lists "fork and other
cross-repository pull requests," bringing `docs/api-endpoint-claims.md` in
line with `DTU_GITHUB_SPEC.md`'s "The first version supports only
same-repository PRs."

### D4 — The PR endpoint's response-field boundary was implicit

**Resolved.** `PR-18` now states the claim explicitly: "P1 asserts only PR
identity, state, Draft status, and base/head repository, ref, and SHA. Other
`github.PullRequest` response fields are unspecified and must not be required
by MergeHerder or asserted by P1 tests." This carries `DTU_GITHUB_SPEC.md`'s
equivalent sentence into the document a test author actually consults.

### D5 — Only `TOKEN-18` was flagged for live conformance

**Resolved.** A new "Live Conformance Rule" section states the blanket rule
directly: "Every `HTTP-*`, `TOKEN-*`, `SCOPE-*`, and `PR-*` claim that
describes GitHub behavior is subject to the live-conformance strategy in
`GITHUB_SERVICE_CONTRACT.md`. A claim is not considered final merely because
the proof world and `go-github` agree with each other." This removes the
misleading implication that only one claim needed independent verification.

### D6 — `TOKEN-06`'s JWT `iat` tolerance was non-operational

**Resolved.** `TOKEN-06` now reads: "The JWT `iat` claim is present and no
later than the proof world's controllable current time." This is a concrete,
directly testable predicate (compare `iat` against the DTU's own virtual
clock), and it tracks GitHub's own stated rationale for backdating `iat` by 60
seconds — that rationale exists specifically to keep `iat` from appearing to
be in the future relative to GitHub's clock, i.e., GitHub's real tolerance for
a future `iat` is effectively the thing being guarded against, not a
documented grace window. Any residual uncertainty about exact real-GitHub
tolerance is now explicitly covered by the D5 blanket live-conformance rule, so
this claim can neither silently pass nor silently diverge undetected.

### N1 — "Recorded" wording contradicted the live-derivation claim

**Resolved.** `PR-09` and `PR-10` now say "current base repository, ref, and
SHA" and "current head repository, ref, and SHA," matching `PR-12`'s
move-the-ref behavior and `DTU_GITHUB_SPEC.md`'s "read from the real refs at
request time." See D8 below for one residual precision question this wording
change surfaces.

### N2 — `TOKEN-20` mislabeled a test-methodology requirement as a GitHub claim

**Resolved.** The test-methodology sentence now lives only in "Required
Tests": "Controllable time proves JWT and installation-token expiry without
sleeping." `TOKEN-20` itself was rewritten into a genuine GitHub-behavior
claim: "Multiple valid tokens may coexist without revoking or changing one
another." No duplication remains.

### N3 — The `github.Ptr` style rule is a coding-standard preference

**Not resolved, and correctly so.** The rule still sits in "Implementation
Boundary." Round 1 explicitly said this needed no action if the authors were
comfortable keeping a single implementation-boundary section; nothing about
this changed, and nothing forces a change now.

### N4 — The document didn't say whose implementation the claims govern

**Resolved.** The document now opens with: "These claims govern the
independent proof world's GitHub-shaped server. A Go `go-github` client
provides one wire-compatibility oracle; MergeHerder remains the real
TypeScript/Octokit consumer in system proof." This removes the ambiguity a
reader would otherwise only resolve by cross-referencing `README.md` and
`GITHUB_SERVICE_CONTRACT.md`.

## New Findings

### Design

**D7. `TOKEN-19`'s `422` mapping does not cover all the failure conditions
`TOKEN-14` enumerates, reintroducing D1's exact problem in a new spot.**

`TOKEN-14` lists three distinct request-validation failures: "Unknown
repositories, invalid permissions, and malformed restrictions are rejected."
`TOKEN-19` assigns a status code using only one of those three terms: "...and
invalid restrictions produce `422`." It never says what status "unknown
repositories" or "invalid permissions" produce. A reader cannot tell whether
"invalid restrictions" in `TOKEN-19` is shorthand meant to cover all three
`TOKEN-14` failure modes, or whether it narrowly means only the third
("malformed restrictions"), leaving the other two to fall through to the
catch-all "every remaining rejection status and error shape is pinned by live
conformance."

This is materially the same shape of problem D1 identified and fixed for the
installation-lookup claims (two distinct preconditions bundled under one
ambiguous label with no stated mapping) — it just reappears here because the
`TOKEN-19` rewrite that fixed D1 narrowed its own scope to the lookup claims
and didn't revisit the validation claims sitting right next to them in the
same bullet.

Live confirmation: GitHub's current REST documentation for this endpoint lists
`422` as "Validation failed, or the endpoint has been spammed" — a single
generic validation bucket with no further public breakdown by failure
condition, and `go-github`'s `InstallationTokenOptions` performs no
client-side validation of `Repositories`/`RepositoryIDs`/`Permissions` at all
(confirmed by reading current `apps.go`), so the DTU is the only place this
mapping gets pinned down; it will not get discovered "for free" from a client
library.

It is plausible that all three `TOKEN-14` conditions do map to `422` (GitHub's
single generic validation-failure code), in which case the fix is purely
editorial: say "unknown repositories, invalid permissions, and malformed
restrictions all produce `422`" instead of the narrower "invalid
restrictions." But that is exactly the kind of resolvable-now ambiguity D1
flagged, and it should be resolved in the document rather than left for
implementation-time discovery, using the same live-conformance-checked
per-condition treatment D1 established for `TOKEN-08`/`TOKEN-09`.

### Nit

**N5. `SCOPE-06` and `PR-15` now assert the same non-disclosure behavior from
two different sections without cross-referencing each other.**

`SCOPE-06` ("a missing resource and a resource outside the token's repository
scope return the same `404` status and GitHub-compatible error shape") and
`PR-15` ("Unknown repositories, unknown PRs, and inaccessible resources return
`404`") are two instances of the same principle applied to the same endpoint
(a PR read using a scope-limited token). This is harmless — it is the general
principle (`SCOPE-06`) plus its concrete PR-endpoint instantiation (`PR-15`),
similar in kind to `HTTP-07`/`PR-03`'s existing case-insensitivity
duplication — but a one-line cross-reference between them would make it clear
this is intentional reinforcement rather than two independently-drifting
claims that a future edit could accidentally decorrelate.

**N6. `PR-09`/`PR-10`'s "current" wording and `PR-14`'s "retained snapshot"
language describe the same mechanism without being tied together explicitly.**

The N1 fix correctly changed "recorded" to "current" to avoid implying a
value frozen at PR-creation time. `PR-14` then separately establishes that
"Deleting a source ref does not invent a new SHA. The retained PR snapshot and
the real Git fetch result remain distinct facts" — i.e., once the underlying
ref is gone, "current" cannot mean freshly re-derived from a live ref, it must
mean the last value GitHub itself continues to report on the PR object. Read
together the two claims are consistent and correct (this is in fact real
GitHub behavior: a PR's `head.sha` persists after branch deletion even though
fetching that ref then fails), but nothing in `PR-09`/`PR-10` flags that
"current" degrades to "last known" once the ref backing it is gone. A one-clause
addition to `PR-09`/`PR-10` — e.g., "current base/head ... SHA, or the
retained value if the underlying ref no longer exists" — would make the
already-correct design explicit rather than requiring a reader to reconcile it
against `PR-14` unaided.

**N7. `TOKEN-18`'s opacity requirement is stated once from the server's side
(`TOKEN-16`) and once from the consumer's side (`TOKEN-18`), with nothing
connecting them.**

`TOKEN-16` establishes the property ("opaque nonempty token"); `TOKEN-18`
restates the same constraint as a consumer-facing rule ("No consumer assumes a
token length or internal format"). This mirrors the same wording already used
in `GITHUB_SERVICE_CONTRACT.md`'s `APP-01` entry, so it is deliberate
cross-document consistency rather than accidental duplication, and it costs
nothing to keep. Noted only; no action needed.

## Stronger-Than-GitHub, Placeholders, And Contradictions — Explicit Check

- **Stronger-than-GitHub behavior:** none found. `TOKEN-06`'s tightened `iat`
  predicate (D6) tracks GitHub's own documented rationale rather than
  inventing a stricter rule, and is in any case covered by the blanket live-
  conformance rule if it turns out to diverge. `TOKEN-12` ("`repositories` and
  `repository_ids` cannot both be supplied") matches GitHub's documented
  constraint, not an invented one.
- **Unresolved placeholders:** `TOKEN-09`'s deferral of the inactive-
  installation status/error shape to live conformance is a deliberate,
  explained deferral (matching the round-2 prompt's own summary of the
  intended revision), not an accidental gap. It is the correct way to avoid
  asserting an unverified guess as settled fact.
- **Contradictions:** none found beyond the D7 mapping ambiguity and the N6
  wording gap above, both of which are reconcilable-in-place rather than
  actual conflicts between claims.
- **Claims that still cannot become deterministic tests:** none found. Both
  claims round 1 flagged as non-operational (`SCOPE-06`, `TOKEN-06`) are now
  concrete. `TOKEN-09` is the one remaining claim without a committed status
  code, but it is explicitly and correctly deferred to live conformance rather
  than asserted as untestable prose.

## Verdict

**Sufficient, with one remaining condition.** All six round-1 design findings
are resolved as described above, and none of the four round-1 nits require
further action (N3 was correctly left alone; N1, N2, N4 are resolved). No
critical or bug-level finding exists in either round. The revisions did not
introduce stronger-than-GitHub behavior, and the one deliberately deferred
claim (`TOKEN-09`) is deferred for the right reason rather than left vague by
oversight.

One new design-level gap (D7) should be closed before treating this document
as final: `TOKEN-19` should state a status code (or explicit live-conformance
deferral) for each of the three `TOKEN-14` failure conditions individually —
"unknown repositories," "invalid permissions," and "malformed restrictions" —
rather than assigning `422` under the single term "invalid restrictions" and
leaving the other two conditions to an implicit catch-all. This is the same
fix, applied the same way, that already resolved D1; it does not require new
investigation beyond deciding whether all three conditions share `422` or
whether any of them needs its own live-conformance-pinned code.

The nits (N5-N7) are worth a pass but do not gate proceeding, consistent with
how round 1's nits were treated.
