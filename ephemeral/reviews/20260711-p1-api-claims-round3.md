# P1 GitHub API Claims Consensus Review - Round 3

## Prompt Received

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

---

## Evidence Reviewed

- `docs/api-endpoint-claims.md` (current version, untracked, full text)
- `ephemeral/reviews/20260711-p1-api-claims-round1.md` and its prompt
- `ephemeral/reviews/20260711-p1-api-claims-round2.md` and its prompt
- `GITHUB_SERVICE_CONTRACT.md`, in full, including its working diff (adds one
  paragraph linking `docs/api-endpoint-claims.md` from the `APP-01`/`PR-01`
  section — unchanged since round 1/2)
- `README.md` diff (adds one pointer sentence — unchanged since round 1/2)
- `DTU_GITHUB_SPEC.md`, in full, with particular attention to "Mint an
  installation token," "What should be simulated," "Live GitHub conformance,"
  and "GitHub Conformance Tests"
- `docs/proof-scenarios/p01-one-clean-pr.md` (the P1 interaction trace)
- `git status` / `git diff` for the current uncommitted change set: confirms
  `docs/api-endpoint-claims.md` is the only substantively new content since
  round 2; `GITHUB_SERVICE_CONTRACT.md` and `README.md` carry only the same
  two pointer-sentence diffs already reviewed in rounds 1 and 2

## Adjudication Of D7

**D7 — `TOKEN-19`'s `422` mapping did not cover all `TOKEN-14` failure
conditions.**

**Resolved.** `TOKEN-19` now reads:

> Invalid JWTs produce `401`, and missing or foreign installations produce
> `404`. Unknown repositories, invalid permissions, and malformed restrictions
> are each independently pinned to the status and error shape observed in live
> conformance; tests must not assume that they share a status merely because
> GitHub documents a general validation-failure response.

This names all three `TOKEN-14` conditions ("unknown repositories, invalid
permissions, and malformed restrictions") individually, word-for-word matching
`TOKEN-14`'s own enumeration, and states a single explicit rule for all three:
each is pinned independently by live conformance, and no shared status may be
assumed. This is a stronger and more precise fix than round 1's analogous D1
fix (which assigned a specific candidate status per condition); here the
document declines to guess a code that public GitHub documentation does not
actually break down by condition (confirmed again this round: GitHub's REST
reference for this endpoint still lists only a single generic `422`
"Validation failed" bucket, and `go-github`'s `InstallationTokenOptions`
performs no client-side validation of `Repositories`/`RepositoryIDs`/
`Permissions`, so no client-library evidence can substitute for the deferral).

This removes the exact defect D7 identified: there is no longer a term
("invalid restrictions") standing ambiguously for a subset of `TOKEN-14`'s
conditions, and no condition is left silently uncovered by both a stated code
and an explicit deferral.

## Deterministic-Test Check (Live-Conformance Placeholders)

The document now contains exactly two claims that defer a status/error-shape
decision to live conformance rather than stating it outright:

1. `TOKEN-09` (installation exists but is inactive) — a single condition,
   deferred as a whole. There is no ambiguity about which condition the
   deferral covers, because the bullet states only one condition.
2. `TOKEN-19`'s validation-failure sentence (three `TOKEN-14` conditions) —
   the deferral explicitly requires each of the three to be "independently
   pinned," and explicitly forbids treating them as sharing one status. This
   is exactly the operational form the round-3 prompt asks for: "a claim may
   use a recorded live-conformance result as test input, but the resulting
   tests must pin each condition separately."

Both deferrals are backed by a concrete, already-specified mechanism, not bare
hand-waving: `DTU_GITHUB_SPEC.md`'s "GitHub Conformance Tests" section states
that the compatibility job "runs the same normalized assertions against DTU
and live GitHub" and "reports exactly which contract IDs passed," and the
"Required Tests" section of `docs/api-endpoint-claims.md` itself requires that
"every contracted rejection has a focused negative test." Read together, the
intended process is: run each condition against live GitHub in isolation,
record its actual status and error shape, and then pin the DTU's negative test
for that specific condition to the recorded value — which is deterministic
once recorded, and does not require GitHub's documentation to already state
the breakdown. This is the same pattern round 2 already accepted for `TOKEN-09`
as "a deliberate, explained deferral... not left vague by oversight," and
`TOKEN-19`'s revision extends it correctly to the three validation conditions.

No placeholder remains that would block a test author from writing
deterministic tests: every status-code question in the document is now either
(a) stated outright (`TOKEN-08`'s `404`, `TOKEN-19`'s `401`/`404`, `PR-15`'s
`404`, `SCOPE-06`'s `404`, `TOKEN-15`'s `201`, `PR-06`'s `200`), or (b)
explicitly and individually deferred to a named, mechanized live-conformance
process (`TOKEN-09`, `TOKEN-19`'s three validation conditions).

## Recheck Of Round 2's N6

**N6 — `PR-09`/`PR-10`'s "current" wording didn't flag that it degrades to
"last known" once the underlying ref is deleted.**

**Resolved**, and resolved more precisely than round 2's finding suggested was
strictly necessary. `PR-10` now reads: "While the source ref exists, the
response reports the current head repository, ref, and SHA. After deletion,
it retains the last PR snapshot as constrained by `PR-14`." This states the
conditional directly in `PR-10` itself (rather than requiring a reader to
infer it by cross-referencing `PR-14` unaided, as round 2 found) and points
explicitly at `PR-14` for the deletion-retention rule, so the two claims are
now self-declaring as one mechanism rather than two claims a reader must
reconcile. `PR-09` is unaffected, correctly — the base ref's staleness
behavior is a distinct, already-settled concern (`P10`'s base-staleness case
is handled by `receive-pack`'s fast-forward check, not by this endpoint, per
the round-1 evidence trail), and `PR-09` does not need the same conditional.

## Recheck Of Round 2's N5 And N7 (Confirmed, No Action)

Both are re-verified against the current document text and remain correctly
characterized as intentional, non-contradictory reinforcement rather than new
findings:

- **N5** (`SCOPE-06` vs. `PR-15`): `SCOPE-06` states the general
  installation-token-behavior principle ("a missing resource and a resource
  outside the token's repository scope return the same `404`"); `PR-15`
  states the same principle's concrete instantiation for the one endpoint
  P1 actually reads through a scoped token ("unknown repositories, unknown
  PRs, and inaccessible resources return `404`"). The two are stated in
  different sections for different audiences (general token-scope behavior
  vs. one endpoint's error contract) and agree on the same status and
  non-disclosure shape. No drift risk beyond ordinary document-maintenance
  discipline, same as `HTTP-07`/`PR-03`'s existing case-insensitivity
  duplication.
- **N7** (`TOKEN-16` vs. `TOKEN-18`): `TOKEN-16` states the server-side
  property ("opaque nonempty token"); `TOKEN-18` restates it as a
  consumer-facing rule ("no consumer assumes a token length or internal
  format"), mirroring `GITHUB_SERVICE_CONTRACT.md`'s `APP-01` entry, which
  states both sides of the same property for the same reason. Deliberate
  cross-document consistency, not accidental duplication.

Neither requires any change to the document.

## New Findings

### Critical

None.

### Bug

None. No claim in the current document is demonstrably incorrect against
GitHub's documented or observed behavior. As in rounds 1 and 2, the only
claims with real external-behavior risk (`TOKEN-09`'s and `TOKEN-19`'s
deferred conditions) are explicitly hedged to live conformance rather than
asserted as settled fact, so they cannot be "wrong" in the document itself —
only unverified until the live-conformance job runs, which is the correct
posture for a claim GitHub does not publicly document at that granularity.

### Design

None. All six round-1 design findings and round 2's one design finding (D7)
are resolved as described above, with no regressions introduced by the
revision and no newly discovered design-level gap at the level of the
complete bounded-replication goal.

### Nit

**N8. `TOKEN-19` reads as a consolidated status-code summary for the
installation-token endpoint's rejections but omits `TOKEN-09`'s
deferral.**

`TOKEN-19` now enumerates every `TOKEN-*` rejection condition except one:
JWT invalidity (`401`), missing/foreign installation (`404`), and the three
`TOKEN-14` validation failures (deferred, independently) are all present, but
the "installation exists but is inactive" condition (`TOKEN-09`) is not
mentioned at all — neither with a stated code nor with an explicit
cross-reference to `TOKEN-09`'s own deferral. This is not the same shape of
defect as D1/D7 (there is no ambiguous shared term standing in for `TOKEN-09`,
and `TOKEN-09`'s own bullet is unambiguous and already correctly deferred on
its own), so it does not block a deterministic test from being written. It is
only a minor completeness/consistency gap in a bullet that otherwise reads as
"here is the full status-code map for this endpoint's rejections": a reader
who treats `TOKEN-19` as that exhaustive map, without also reading `TOKEN-09`
in isolation, could momentarily miss that inactive-installation rejection is
also live-conformance-deferred rather than assuming it is somehow implied by
the `404` given to missing/foreign installations. A one-clause addition to
`TOKEN-19` (e.g., "...; the installation-inactive case in `TOKEN-09` is
likewise independently pinned by live conformance") would close this, but it
does not gate proceeding.

## Full-Goal Recheck

Re-examined the document as a whole, not only the D7/N6 deltas, against the
bounded-replication goal:

- **Every behavior needed by P1's two REST endpoints is still claimed.**
  Unchanged from rounds 1–2's finding; the D7/N6 revisions only sharpened
  existing claims, they did not remove or narrow any required behavior.
- **No claim is broader than P1 requires.** The exclusion list, `PR-18`'s
  field boundary, and the same-repository-only scope (D3) remain intact and
  unchanged.
- **The layered oracle (`go-github` client tests, independently-asserted raw
  bodies, DTU/MergeHerder system tests, live conformance) remains sound** and
  is now the explicit mechanism that makes `TOKEN-09`'s and `TOKEN-19`'s
  deferred conditions deterministic rather than speculative, per the
  Deterministic-Test Check above.
- **No new contradiction was found** between `TOKEN-14`/`TOKEN-19` and any
  other claim, or between `PR-09`/`PR-10`/`PR-14` and any other claim.
- **No stronger-than-GitHub behavior was introduced** by the D7 or N6
  revisions: both changes narrow the document's own precision without
  inventing new GitHub-facing constraints, and both remain subject to the
  blanket live-conformance rule if reality diverges.

## Verdict

**Consensus: sufficient.** The claims in `docs/api-endpoint-claims.md`,
rendered into tests as the document itself requires (every stated status
exercised directly, every deferred status pinned per-condition from a
recorded live-conformance observation, negative tests focused per contracted
rejection, controllable time for expiry, real Git refs for PR snapshot
behavior), constitute a valid and workable bounded replication of the
designated P1 REST subset for MergeHerder. D7 is fully resolved and does not
recur elsewhere in the document; no critical, bug, or design finding remains
open across all three rounds. The single new item (N8) is a nit and does not
gate proceeding.

This verdict is a determination about the contract's soundness and
testability, not about implementation status. `docs/api-endpoint-claims.md`
defines what must be true; it is not evidence that a DTU server, `go-github`
test suite, or live-conformance job implementing these claims yet exists or
passes. That remains to be built and separately verified against this
document.
