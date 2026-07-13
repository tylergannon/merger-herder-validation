# Re-review: go-github-server adoption fixes (round 2)

## Prompt received (verbatim)

```
# Re-review request: go-github-server adoption fixes

Re-review the complete current diff from `origin/main` on branch `codex/dtu-veracity` in `/Users/tyler/src/merger-herder-validation/.worktrees/dtu-veracity`.

Your first review is recorded in `ephemeral/reviews/20260713-go-github-server-adoption-sonnet.md`. It found two bugs and missing migration-specific coverage. The implementation now prevalidates malformed numeric parameters for all three generated routes, normalizes generated service errors through the DTU's established GitHub JSON envelope, and adds `TestGeneratedRESTContractCompatibility` covering the three malformed paths and both installation-token `422` rejection paths. The complete Go suite, race suite, vet, and unchanged-MergeHerder P1 system proof pass.

Verify the fixes directly and review the entire resulting diff for regressions, incomplete fixes, brittle tests, authentication/route/error behavior changes, and accidental scope expansion. Do not assume the stated fixes are correct. Write the exact prompt received and findings to `ephemeral/reviews/20260713-go-github-server-adoption-sonnet-round2.md`, labeling findings `critical`, `bug`, `design`, or `nit`, and end with a clear merge recommendation. Do not modify any other file.
```

## Scope of the diff reviewed

Full working-tree diff against `origin/main` (still uncommitted on `codex/dtu-veracity`, which has no commits ahead of `origin/main`):

```
dtu/cancellation_test.go |   2 +-
dtu/public.go            | 341 +++++++++++++++++++++++++++++------------------
dtu/ref_events_test.go   |   2 +-
dtu/server_test.go       |  62 +++++++--
go.mod                   |   5 +-
go.sum                   |  21 ++-
6 files changed, 284 insertions(+), 149 deletions(-)
```

Methodology: read the full current `dtu/public.go`, re-read `go-github-server@v0.2.0`'s `server.go` and `zz_generated.go` from the module cache, ran `go build`, `go vet`, `go test ./...`, and `go test -race ./...` (all green). Then, rather than trusting the worklog's account, added a throwaway test file (`dtu/zzz_scratch_verify_test.go`, deleted afterward — working tree is clean except the six files above) that starts a live DTU instance and fires raw HTTP requests to empirically re-derive both of round 1's original repro tables, plus several new probes aimed at the fix's edges. All scratch output is summarized inline below.

---

## Findings

### 1. [confirmed fixed] Malformed numeric path parameters — round-1 bug #1

Verified live against all three routes, with and without an `/api/v3` prefix:

| Request | Result |
|---|---|
| `POST /app/installations/abc/access_tokens` | `404`, `application/json; charset=utf-8`, `{"documentation_url":"https://docs.github.com/rest","message":"Not Found","status":"404"}` |
| `GET /repos/Acme/widget/pulls/notanumber` | same shape |
| `POST /repos/Acme/widget/actions/runs/notanumber/cancel` | same shape |
| `GET /api/v3/repos/Acme/widget/pulls/notanumber` | same shape |

`hasMalformedNumericPath` (new, in `public.go`) pre-parses the numeric segment for all three routes before the request ever reaches `api.ServeHTTP`, and rejects with the legacy `writeError(..., 404, "Not Found")` — exactly the fix direction round 1 suggested. `TestGeneratedRESTContractCompatibility` in `server_test.go` now asserts this for all three routes including content-type. This bug is genuinely fixed, not just test-shaped around.

One residual gap in the fix's own path-matching, found by inspection rather than repro: `restPathParts` strips `/api/v3` only when followed by `/` or when the path is exactly `/api/v3`, whereas the library's own `route.match` does an unconditional `strings.TrimPrefix(r.URL.Path, "/api/v3")` (and also strips `/api/uploads`, irrelevant here). The two disagree only on pathological inputs like `/api/v3xyz/...`, which no real client sends and which isn't one of the three routes' concern (no `/api/uploads` traffic exists here). Not worth blocking on — noted for completeness, not filed as a finding.

### 2. [confirmed fixed] Divergent error envelopes — round-1 bug #2

Verified live: the two 422 causes on installation-token creation (unknown JSON field, pre-check; both `repositories`+`repository_ids`, inside the migrated service) now produce **byte-identical** bodies:
```json
{"documentation_url":"https://docs.github.com/rest","message":"Validation Failed","status":"422"}
```
This is a stronger fix than the round-1 repro alone would prove out: I additionally minted a real installation token and hit the **404** paths that originate from inside the migrated services (`githubAPIError(404, "Not Found")` for cancel-workflow-run on an unknown run ID, and for pull-request retrieval on an unknown repo) and confirmed those also now come back as the identical legacy envelope, not the `github.ErrorResponse`-shaped body (`errors:null`, no `status` field) round 1 caught. The fix is a general one: `publicHandler`'s wrapper now buffers every response from `api.ServeHTTP`, and for any `status >= 400` re-derives the message and re-emits it through the same `writeError` path used by the pre-checks — so it isn't limited to the one 422 case the new test happens to assert. `TestGeneratedRESTContractCompatibility` only directly exercises the 422/422 pair, but the underlying mechanism is unconditional, and I verified it generalizes.

### 3. [bug] The unsupported-route detection contract silently narrowed to a fraction of previously-covered paths, and this isn't tested for

Before this migration, **every** request that didn't match one of the three hand-routed cases fell into a single `default: w.handleUnsupported(...)` branch: `404` + the DTU JSON envelope + `X-DTU-Unsupported: true` + an entry appended to `w.unsupported`, which is exposed via the control plane as `UnsupportedRequests` and used by two existing tests (`server_test.go:134`, `mergeherder_system_test.go:137`) as a "no unexpected traffic occurred" completeness assertion.

`githubserver.New` is constructed with `Services{Actions: actionsService{...}, Apps: appsService{...}, PullRequests: pullRequestsService{...}}`. Per the library's doc comment ("nil service fields simply leave their routes unregistered"), a **non-nil** service field causes **every method of that entire interface** to be registered as a live route — and `ActionsService`, `AppsService`, and `PullRequestsService` are each dozens-to-hundreds of methods (e.g. `PullRequests.Create`, `PullRequests.CreateComment`, `Apps.CompleteAppManifest`, `Apps.AddRepository`, and every Actions permissions/runner/secret/variable endpoint), none of which DTU implements beyond the three target operations. `actionsService`/`appsService`/`pullRequestsService` embed the corresponding `Unimplemented*Service`, so any of those sibling routes resolves to `ErrNotImplemented`, and `serveOperationGroup` handles that case *before* it ever reaches `writeErrorResponse` or the `notFoundCallback`:

```go
if errors.Is(invokeErr, ErrNotImplemented) {
    http.Error(w, invokeErr.Error(), http.StatusNotImplemented)
    return
}
```

Verified live:
```
POST /repos/Acme/widget/pulls  (PullRequests.Create; valid, well-formed body)
  -> 501, text/plain; charset=utf-8, "github service method is not implemented"

POST /app-manifests/somecode/conversions  (Apps.CompleteAppManifest)
  -> 501, text/plain; charset=utf-8, "github service method is not implemented"
```
Neither response carries `X-DTU-Unsupported`, neither is JSON, and neither is recorded in `w.unsupported` — I confirmed the only writer of that slice is `recordUnsupported`, which is wired solely to `WithNotFoundCallback`, itself only invoked when a request matches **no** registered route at all (`registerOperations`'s `if notFoundCallback != nil { notFoundCallback(r) }` after the route-group loop fails). A genuinely-unrecognized path (e.g. `DELETE /repos/Acme/widget/pulls/1`) still correctly gets the old 404+JSON+header+audit-trail treatment, verified.

Net effect: the "did anything unexpected happen" audit trail the harness's own tests rely on (`if len(finalState.UnsupportedRequests) != 0 { t.Fatal(...) }`) now has a blind spot covering every unimplemented method of three large GitHub REST interfaces — not just the handful of routes that predate this migration. This is a real behavior change from "preserving the existing observable contract," and it's a documented, intentional tradeoff (the worklog states: "decision: Unused generated service methods may return their embedded unimplemented response; no endpoint allowlist is required"), not an accidental regression. I'm flagging it as `bug` rather than `design` because the consequence is concrete and currently untested in either direction — there is no test asserting sibling-route behavior at all, so a future change that narrows or widens this surface would go unnoticed. If the decision to accept this tradeoff stands, it's worth at least one test pinning the current 501/plain-text/untracked behavior so it can't silently regress further (e.g., to something that panics, or to 200).

Practical risk for *this specific* migration is low: none of the three P1-relevant MergeHerder code paths (mint token, read pull, cancel run) call any sibling method, so the P1 system proof passing is unaffected. But it does mean the DTU's own "clean run" diagnostic is weaker than it was, which matters if this DTU is ever relied on as a trustworthy negative-space signal ("nothing unexpected happened") for future scope.

### 4. [design, carried over — round 1's finding #4 is not actually resolved]

Round 1 noted the P1 system proof is meaningful for `CreateInstallationToken` and `PullRequests.Get` (both called by MergeHerder's real Octokit-based production code) but not for `CancelWorkflowRunByID` (no caller in MergeHerder's product code at all; coverage is unit-test-only). The worklog's response to this is: *"Existing cancellation race coverage exercises `CancelWorkflowRunByID` through `go-github` v89."* — referring to `dtu/cancellation_test.go`, whose diff in this round is a one-line import bump (`v81` → `v89`), i.e. unchanged test logic.

That test does genuinely exercise the migrated route end-to-end over real HTTP (`client.Actions.CancelWorkflowRunByID(...)` via a real `go-github` v89 client) — it is not nothing. But it does not address what round 1 actually flagged: MergeHerder's production code is a JS/TS Octokit consumer, not a Go `go-github` consumer. A DTU unit test driving the route through Go's `go-github` client says the route works for *that* client's request shape; it says nothing about whether it would behave identically for the Octokit request shape MergeHerder would actually send if it ever grew a caller for this endpoint. This is exactly the class of gap that let round 1's bugs #1 and #2 slip past a fully-green `go test ./...` in the first place. Restating the pre-existing unit test as the answer to a system-level coverage gap doesn't close it — this finding stands as originally raised. Not blocking (P2/out-of-scope for adding a MergeHerder-side caller), but the worklog's framing overstates what was actually done here.

### 5. [nit, carried over, unaddressed] `randomToken()` prefixes tokens with `"ghs_"`

Unchanged from round 1. Still harmless (no `Authenticator` is wired into `githubserver.New`, so the prefix-sniffing dispatch in the library's `authenticate()` is unreachable here), still technically outside the stated "preserve the observable contract" scope, still plausibly an authenticity improvement. Not re-verified in detail since nothing about it changed.

### 6. [nit, carried over, unaddressed] `strings.Fields`-based Authorization parsing

Unchanged from round 1 (`authorizationCredential` in the current diff is the same logic as round 1's `authorizationCredential`/`bearerCredential`). Non-issue for any real token.

### 7. [nit] New test coverage for finding #2 is narrower than the bug it fixes

`TestGeneratedRESTContractCompatibility` asserts envelope-shape equality only for the two 422-on-installation-token-creation causes (matching the exact repro from round 1's finding #2 write-up). It does not assert equality between, say, a pre-check 404 and a `githubAPIError`-sourced 404 (e.g. cancel-on-unknown-run vs. malformed-path), which is the broader claim the fix actually makes true (verified manually above, finding #2). The test as shipped would not catch a future change that special-cased 422 normalization without fixing the general case. Cheap to extend; not blocking.

---

## Merge recommendation

**Mergeable**, with one open item worth a decision (not necessarily a blocking fix) before merge.

Round 1's two verified bugs (#1: malformed-path 400/plain-text; #2: divergent error envelopes) are genuinely fixed — I re-derived both original repro tables live and additionally verified the fix generalizes beyond the specific cases the new test covers (404s sourced from inside the migrated services now match the legacy envelope shape too, not just the one 422 pair that's actually tested). The new `TestGeneratedRESTContractCompatibility` is well-targeted and not brittle. Full suite, race suite, and vet all pass, confirmed independently.

The new item (finding #3) is a real, verified behavior change that the stated goal ("preserving the existing observable contract") would nominally prohibit, but it's a documented decision rather than an oversight, it doesn't affect the three P1-relevant operations, and it doesn't regress anything this branch's own tests currently check. I'd treat it as a decision to ratify explicitly (and ideally pin with one test asserting the 501/untracked shape) rather than a hard blocker — but whoever owns this tradeoff should see it stated plainly, since "no endpoint allowlist is required" quietly also means "no audit trail for calls to unimplemented sibling endpoints," which is a bigger surface than the phrase suggests on its own. Finding #4 is a documentation/framing issue in the worklog, not a code defect — the underlying gap round 1 raised is still open, just restated as closed.

Recommend: merge after either (a) accepting finding #3 as-is with a one-line note in the worklog acknowledging the audit-trail gap, or (b) adding a `WithNotFoundCallback`-equivalent hook for `ErrNotImplemented` responses so sibling-route calls are still tracked/enveloped consistently. Neither is required to unblock the three actually-migrated operations, which are solid.
