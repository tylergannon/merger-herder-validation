# P1 GitHub API Claims Consensus Review - Round 1

## Prompt Received

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

---

## Evidence Reviewed

- `docs/api-endpoint-claims.md` (the document under review, untracked/new)
- `MERGE_HERDER_PROOF_SCENARIOS.md`
- `docs/proof-scenarios/p01-one-clean-pr.md` and `docs/proof-scenarios/README.md`
- `GITHUB_SERVICE_CONTRACT.md` (current working copy, including the two-line
  diff that links `docs/api-endpoint-claims.md` from the `APP-01`/`PR-01`
  section)
- `DTU_GITHUB_SPEC.md`
- `ephemeral/reviews/20260710-mergeherder-proof-contract-reconciliation.md`
- `README.md` diff (adds a pointer to `docs/api-endpoint-claims.md`)
- `git diff` and `git status` for the current uncommitted change set
- `/Users/tyler/src/merge-herder` checkout: confirmed the application itself is
  a TypeScript/SvelteKit codebase (`src/lib/server/github-app.ts`,
  `github-webhooks.ts`) with no Go or `go-github` usage, and read
  `docs/github_app_configuration.md` for the actual configured App permission
  set (`Pull requests: Read-only`, `Contents: Read and write`, `Actions: Read
and write`)
- Live GitHub REST documentation for "Create an installation access token for
  an app" and "Get a pull request" (fetched current pages)
- `google/go-github` `apps.go` (`CreateInstallationToken`,
  `InstallationTokenOptions`, `InstallationToken`) and `pulls.go`
  (`PullRequestsService.Get`) source, current `master`
- Web search corroboration for GitHub App JWT `iat`/clock-drift convention and
  for community-reported behavior of installation-token minting against an
  installation ID the requesting App does not own

## Findings

### Critical

None.

### Bug

None. No claim was found to be flatly, demonstrably incorrect as GitHub
behavior. The strongest candidate (the `403`/`404` split in `TOKEN-18`) is
downgraded to **design** below because the document already flags it as
unresolved and defers to live conformance rather than asserting it with false
confidence.

### Design

**D1. `TOKEN-18`'s `403` mapping for "forbidden installations" is probably
wrong for the case it most needs to cover, and the document doesn't say which
failure condition maps to which code.**

`TOKEN-08` bundles two distinct preconditions into one bullet: "the
installation exists, belongs to the authenticated App, and is active."
`TOKEN-18` then says "forbidden installations produce `403`; missing resources
produce `404`," without saying which of those two failure modes ("doesn't
belong to this App" vs. "exists but is suspended/inactive") is "forbidden" and
which is "missing."

GitHub's own docs list both `403` and `404` as possible responses for this
endpoint, but multiple first-party community reports (GitHub Support
engineers responding in `github/community` discussions) describe the
practically observed behavior for "installation ID doesn't belong to your App"
as a `404` ("Integration not found" / "installation ID is incorrect, or your
app doesn't have access to that installation") — not a `403`. If `TOKEN-18` is
implemented literally as written, the most common real "forbidden" case (an
App JWT presented for an installation ID it doesn't own) would very likely
diverge from live GitHub's actual `404`, and the DTU/live-conformance suite
would catch this only if someone thinks to test that specific sub-case.

The claim already hedges ("Live conformance must verify these mappings"), so
this isn't presented as settled fact — but given the ambiguity is resolvable
now with public evidence, it should be resolved in the document rather than
left for implementation-time discovery. Recommend splitting `TOKEN-08` into
"installation exists and belongs to the App" (candidate: `404` on failure,
matching GitHub's non-disclosure pattern used elsewhere in this same document,
see `PR-15`) and "installation is active" (candidate: `403`), and stating that
split explicitly in `TOKEN-18`.

**D2. `SCOPE-06`'s non-disclosure claim is not independently testable as
written.**

"Authentication failures do not unnecessarily reveal whether a protected
resource exists" has no operational definition — a test cannot assert
"unnecessarily." Contrast this with `PR-15` on the PR endpoint: "Unknown
repositories, unknown PRs, and inaccessible resources return `404`," which is
a precise, directly testable instantiation of the same non-disclosure
principle (uniform status code regardless of whether the resource is absent or
merely out of scope). `SCOPE-06` should be rewritten to the same standard,
e.g. stating exactly which token-endpoint failure conditions must be
indistinguishable by status code and body shape. This also feeds directly into
D1: once D1's split is made, `SCOPE-06` is the natural place to pin down which
of those codes must not leak "installation exists but isn't yours" versus
"installation doesn't exist."

**D3. Fork/cross-repository pull requests are never explicitly excluded, even
though a sibling document already commits to that boundary.**

`DTU_GITHUB_SPEC.md` states plainly: "The first version supports only
same-repository PRs." `docs/api-endpoint-claims.md`'s `PR-11` quietly inherits
this scope by only promising SHA agreement "for an open same-repository PR,"
but the "Explicitly Excluded From P1" list never says fork PRs are out of
scope, and `PR-10` ("the response reports the recorded head repository, ref,
and SHA") reads as if head-repository identity is a first-class, fully general
concern. Since `MERGE_HERDER_PROOF_SCENARIOS.md`'s common fixture only ever
uses same-repository source branches (`A`, `B`, `C`, ...), this narrowing is
almost certainly correct — but it should be stated in the exclusions list,
both for consistency with `DTU_GITHUB_SPEC.md` and so an implementer doesn't
either over-build fork handling or leave `PR-10`'s "head repository" claim
ambiguous for the one case (same-repo) that actually matters.

**D4. The PR endpoint's response-field boundary is left implicit, unlike the
token endpoint's.**

`TOKEN-16`/`TOKEN-17` give the installation-token endpoint a precise content
contract: the response "accurately reports the token's effective permissions
and repository scope," and "no consumer assumes a token length or internal
format." `PR-06` through `PR-10` commit to only five field groups (identity,
open/closed + Draft, base, head) out of GitHub's much larger `PullRequest`
payload (`mergeable`, `merged`, `merged_by`, comment/review counts, labels,
requested reviewers, milestone, etc.). Nothing states whether the rest of the
payload must be present-but-inert, omitted, or simply unspecified.
`DTU_GITHUB_SPEC.md` addresses this at the DTU-design level ("Optional
response fields may be omitted or filled with inert values unless MergeHerder
reads them"), but that sentence isn't carried into this claims document, where
it's the thing a test author would actually need. Recommend adding an
equivalent sentence to the Pull Request Endpoint section: the five listed
field groups are asserted; all other `PullRequest` fields are unspecified and
must not be relied upon by MergeHerder or asserted by P1 tests.

**D5. Only one claim (`TOKEN-18`) is explicitly flagged as requiring live
conformance, even though the same risk applies more broadly.**

`GITHUB_SERVICE_CONTRACT.md`'s Conformance Strategy commits _every_ contract
ID to the same "run the same normalized assertion against the DTU and a live
sandbox" pattern — it is not scoped to error-code mappings only. Within
`docs/api-endpoint-claims.md`, claims with just as much real-GitHub-dependent
content as `TOKEN-18` — `TOKEN-06`/`TOKEN-07`'s JWT time tolerances,
`PR-15`'s `404` unification, `SCOPE-05`'s exact expiry cutoff — carry the same
conformance risk but aren't flagged the same way. Either state a blanket rule
("every `TOKEN-*`, `SCOPE-*`, `PR-*`, and `HTTP-*` claim in this document is
subject to the live-conformance strategy in `GITHUB_SERVICE_CONTRACT.md`") or
remove the singular `TOKEN-18` flag as redundant with that already-standing
project-wide commitment. As written, the lone flag on `TOKEN-18` reads as if
the other 50 claims are considered DTU-only and settled, which is not the
intent expressed elsewhere in the document set.

**D6. `TOKEN-06`'s JWT `iat` tolerance is vague where `TOKEN-07`'s is precise.**

`TOKEN-07` gives `exp` a concrete, testable bound: "present, unexpired, and no
more than ten minutes in the future," matching GitHub's documented maximum.
`TOKEN-06` gives `iat` no comparable bound: "not unacceptably in the future."
GitHub's own recommended practice (backdating `iat` by ~60 seconds to absorb
clock drift) implies there is a concrete, small tolerance window GitHub
itself expects implementers to reason about, and the DTU will need one
specific number to implement and test against regardless of what the claims
document says. Recommend giving `TOKEN-06` the same numeric-bound treatment as
`TOKEN-07` (e.g., "the `iat` claim is not more than N seconds in the future,"
with N chosen and justified against GitHub's own guidance) rather than leaving
it as a subjective, untestable predicate.

### Nit

**N1. `PR-09`/`PR-10`'s "recorded" wording contradicts the document's own
live-derivation claim.**

Both bullets say the response reports the "recorded" base/head repository,
ref, and SHA. "Recorded" reads as "frozen at PR-creation time," but `PR-12`
("Moving a source ref changes the next PR snapshot's head SHA") and
`DTU_GITHUB_SPEC.md` ("The SHAs are read from the real refs at request time")
both establish that these values are derived live on every read, not cached.
Suggest "current" or "live-derived" in place of "recorded" in `PR-09`/`PR-10`
to avoid nudging an implementer toward caching/freezing SHAs at PR-creation
time.

**N2. `TOKEN-20` is a test-methodology requirement mislabeled as a
GitHub-behavior claim, and it duplicates the Required Tests section.**

"Expiration is evaluated against controllable world time rather than
wall-clock sleeps" is not a claim about what GitHub does; it's a requirement
on how the proof harness must be built. It sits inside the `TOKEN-*` list,
which is otherwise entirely about GitHub-facing semantics, and it restates
almost verbatim the Required Tests section's closing line ("Controllable time
proves JWT and installation-token expiry without sleeping"). This is a small,
concrete instance of the test-control/GitHub-behavior conflation the review
was asked to check for. Move it into Required Tests and drop the duplicate.

**N3. The "Implementation Boundary" section's `github.Ptr` style rule is a
coding-standard preference, not a correctness claim.**

It's harmless where it sits, and low priority to move, but it is a different
kind of statement than everything else in the document (a house style rule
rather than a testable GitHub-compatibility claim). Worth a note only; no
action required if the authors are comfortable keeping a single
"implementation boundary" catch-all section here.

**N4. The document doesn't say, on its own, whose implementation these claims
govern.**

`docs/api-endpoint-claims.md` requires `go-github`, `net/http` handlers, and
`github.Ptr`-avoidance — all Go-specific. That only makes sense once you know
(from `README.md`'s "Proposed Implementation Direction," not from this file)
that these are the **DTU's** server-side claims, verified by an independent Go
client, and not a description of MergeHerder's own client stack (which is
TypeScript/Octokit — confirmed by inspecting `/Users/tyler/src/merge-herder`).
`GITHUB_SERVICE_CONTRACT.md` says "Exact Octokit method names remain
implementation choices" immediately before linking this document, which is the
closest existing disambiguation, but it's still indirect. One sentence at the
top of `docs/api-endpoint-claims.md` — "these claims describe the DTU's
GitHub-shaped server behavior, proved with an independent Go/`go-github`
client; they say nothing about MergeHerder's own Octokit-based
implementation" — would remove the ambiguity for a reader who opens only this
file.

## Answers To The Review Questions

**Is every behavior needed by P1's two REST endpoints claimed?** Yes. Cross-checked against `docs/proof-scenarios/p01-one-clean-pr.md`'s interaction trace and `GITHUB_SERVICE_CONTRACT.md`'s `APP-01`/`PR-01` entries, nothing required for the one-clean-PR path, or for the later scenarios that reuse the same two endpoints (`P8` suffix rebuild, `P10` stale base), is missing from the claim list. The gaps found (D3, D4) are boundary-precision gaps on already-covered behavior, not missing behavior.

**Is any claim wrong, contradictory, unimplementable, or broader than P1 requires?** One claim (`TOKEN-18`'s `403` assignment, D1) is probably factually wrong for its main real-world case. Two claims (`SCOPE-06`, D2; `TOKEN-06`, D6) are effectively unimplementable as literally worded because they use non-operational language ("unnecessarily," "unacceptably") where the sibling claims in the same document (`PR-15`, `TOKEN-07`) show the concrete alternative is achievable. No claim was found to be broader than P1 requires — scope discipline is otherwise good throughout, and the "Explicitly Excluded From P1" list is doing real work.

**Are authentication, JWT validation, token issuance, scope, expiration, error behavior, and non-disclosure sufficiently specified?** Mostly. Algorithm (`TOKEN-04`), issuer (`TOKEN-05`), expiry bound (`TOKEN-07`), scope-narrowing rules (`TOKEN-09`–`TOKEN-12`), and coexistence (`TOKEN-19`) are all precise and match documented/observed GitHub behavior. The two weak points are `TOKEN-06` (D6) and `SCOPE-06`/the `TOKEN-18` split (D1, D2) — both fixable by borrowing the precision already used elsewhere in the same document.

**Does the targeted PR snapshot remain correct as PR state and real Git refs change?** Yes, and this is one of the stronger parts of the document. `PR-12` (moving source ref), `PR-13` (closing), and `PR-14` (deleting source ref, snapshot vs. real-fetch distinction) form a coherent, testable model that matches `DTU_GITHUB_SPEC.md`'s live-read design. The base-ref (default-branch) staleness case (`P10`) is correctly _not_ re-derived through this endpoint — the prior reconciliation review already attributed that guard to `receive-pack`'s fast-forward ancestry check rather than the PR endpoint, and this document is consistent with that decision. The only defect found here is the "recorded" wording (N1), which is cosmetic but points the wrong direction.

**Do `go-github` client tests plus the behavioral/Git/system/live-conformance layers form a sound oracle?** Yes. The layered design is sound: `go-github`-mediated tests prove wire compatibility with an independent, well-maintained client; independently-asserted raw response bodies (Required Tests) catch anything `go-github`'s tolerant, pointer-based decoding would silently accept; the DTU-plus-MergeHerder system tests described in `DTU_GITHUB_SPEC.md` exercise the _actual_ MergeHerder client (Octokit/TypeScript), which is the layer that would catch any residual `go-github`-vs-Octokit interpretation gap that the Go-only oracle can't see; and live-GitHub conformance closes the loop against real GitHub. D5 is the only real gap: the document should say this layering applies uniformly rather than flagging only `TOKEN-18` for live verification.

**Do the explicit exclusions leave a hole that would let an incompatible implementation pass?** No outright hole, but two boundary gaps (D3 fork PRs, D4 PR field scope) mean an implementation could satisfy every literal claim while making undocumented choices in areas MergeHerder doesn't need, or leave a test author guessing what's safe to assert. Neither is a compatibility risk given P1's actual fixture, but both are worth closing for the reason stated in each finding.

**Do response/error field requirements need to be more concrete?** Yes for the PR endpoint's non-listed fields (D4) and for the two non-operational predicates (D2, D6). The token endpoint's success-path fields (`TOKEN-15`/`16`/`17`) and the PR endpoint's error-path fields (`PR-15`/`16`) are already at the right level of concreteness and can serve as the template for the fixes above.

**Are test controls being confused with GitHub-facing behavior?** Only in one small, well-contained place: `TOKEN-20` (N2). `DTU_GITHUB_SPEC.md`'s separation of the "GitHub-shaped plane" from the "scenario-control plane," and this document's own separation of numbered claims from the "Required Tests" section, are otherwise consistently maintained.

**Would the resulting implementation be maintainable and realistically achievable without becoming a generalized GitHub clone?** Yes. Fifty-one claims across two endpoints, with an explicit and actively-used exclusion list, is proportionate. Nothing in the document asks for GitHub fidelity beyond what P1 needs; if anything the risk runs the other way (a few claims are stated more loosely than they need to be, per D2/D6), which is the safer direction for a bounded first implementation.

## Verdict

**Sufficient, with conditions.** The claims in `docs/api-endpoint-claims.md`, if implemented and tested as described, would constitute a valid and workable bounded replication of the P1 REST subset for MergeHerder. There are no critical or bug-level findings: no claim is broader than P1 requires, no required behavior is missing, and the layered `go-github`/behavioral/system/live-conformance oracle is sound in design.

Before treating this document as final, the following should be resolved (all **design**-level, none blocking exploratory implementation work, but all worth fixing before the DTU's error-mapping and PR-field code is written against this document as a spec):

1. Resolve D1: split `TOKEN-08`/`TOKEN-18`'s installation-lookup failure into "doesn't belong to the App" (likely `404`) versus "belongs to the App but inactive/suspended" (candidate `403`), checked against live GitHub rather than left to implementation-time discovery.
2. Resolve D2 and D6: replace `SCOPE-06` and `TOKEN-06`'s non-operational language ("unnecessarily," "unacceptably") with concrete, testable predicates, following the precedent already set by `PR-15` and `TOKEN-07` in the same document.
3. Resolve D3 and D4: add fork/cross-repository PRs to "Explicitly Excluded From P1," and add an explicit statement that PR response fields beyond the five listed groups are unspecified/inert.
4. Resolve D5: state that the live-conformance strategy applies to the whole document, not only `TOKEN-18`.

The nits (N1–N4) are worth a pass but do not gate proceeding.
