# Review: focused go-github-server adoption

## Prompt received (verbatim)

```
# Review request: focused go-github-server adoption

Review the complete diff from `origin/main` on branch `codex/dtu-veracity` in `/Users/tyler/src/merger-herder-validation/.worktrees/dtu-veracity`.

The goal is narrow: replace hand-routed GitHub REST transport with `github.com/tylergannon/go-github-server` v0.2.0 for the three implemented DTU operations (installation token creation, pull-request retrieval, and workflow-run cancellation), while preserving the existing observable contract. Git smart HTTP, webhooks, workflow execution, and the control plane remain DTU-owned. MergeHerder is read-only and must not be changed. P2 functionality is out of scope.

Inspect the implementation and tests rather than accepting the stated goal. Look broadly for correctness regressions, route or parameter binding mistakes, authentication changes, response/error envelope changes, unsupported-route behavior, dependency/type incompatibility, missing tests, and accidental scope expansion. Confirm whether the existing P1 system proof is a meaningful regression check for this diff.

Write the exact prompt you received and your findings to `ephemeral/reviews/20260713-go-github-server-adoption-sonnet.md`. Label every finding `critical`, `bug`, `design`, or `nit`. End with a clear merge recommendation. Do not modify product code or any file other than that review artifact.
```

## Scope of the diff actually reviewed

Working-tree diff against `origin/main` (branch `codex/dtu-veracity` has no commits ahead of `origin/main`; the change is uncommitted):

```
dtu/cancellation_test.go |   2 +-
dtu/public.go            | 315 +++++++++++++++++++++++++++--------------------
dtu/ref_events_test.go   |   2 +-
dtu/server_test.go       |  11 +-
go.mod                   |   5 +-
go.sum                   |  21 +++-
```

`dtu/git.go` (Git smart HTTP), webhook delivery, workflow execution, and the control plane are untouched, matching the stated scope. `go.mod` cleanly swaps `go-github v81` for `v89` and adds `go-github-server v0.2.0`; no incidental dependency creep.

Methodology: read `public.go`'s full diff, read the `go-github-server@v0.2.0` library source from the module cache (`server.go`, `zz_generated.go`) to understand routing/binding/error-writing semantics, and read MergeHerder's actual production GitHub client code (`/Users/tyler/src/merge-herder/.worktrees/p1-system-harness/src/lib/server/merge-queue/github.ts`) to determine what traffic shape the P1 system proof really exercises. Two of the findings below were also **empirically verified** by adding a throwaway test file, running it against the new code, then `git stash`-ing back to `origin/main`'s `dtu/public.go` and running the identical test again for a before/after comparison. The scratch test file was deleted afterward; the working tree is clean except for the six files above.

---

## Findings

### 1. [bug] Malformed numeric path parameters now return `400` + plain text instead of `404` + the DTU JSON envelope

All three migrated endpoints take a numeric path segment (installation ID, pull number, workflow run ID). Previously each handler parsed it manually (`strconv.ParseInt`/`Atoi`) and returned `404 Not Found` with the standard `{message, documentation_url, status}` JSON body on parse failure. That manual parsing was deleted; the numeric conversion is now done by `go-github-server`'s generic path-binding (`invokeOperation` → `bindingPath` → `parseScalar`), and a parse failure surfaces as a `*requestError`, which `serveOperationGroup` turns into a bare `http.Error(w, err.Error(), http.StatusBadRequest)` — a plain-text 400, not JSON, and never routed through DTU's `recordUnsupported`/`writeError`.

Verified directly against the running DTU:

| Request | Before (`origin/main`) | After (this diff) |
|---|---|---|
| `POST /app/installations/abc/access_tokens` | `404`, `application/json; charset=utf-8`, `{"documentation_url":"...","message":"Not Found","status":"404"}` | `400`, `text/plain; charset=utf-8`, `path parameter p0: strconv.ParseInt: parsing "abc": invalid syntax` |
| `GET /repos/Acme/widget/pulls/notanumber` | (same 404/JSON pattern, confirmed by code inspection of the deleted `strconv.Atoi` branch) | `400`, `text/plain; charset=utf-8`, `path parameter p2: strconv.ParseInt: parsing "notanumber": invalid syntax` |
| `POST /repos/Acme/widget/actions/runs/notanumber/cancel` | (same pattern) | `400`, `text/plain; charset=utf-8`, `path parameter p2: strconv.ParseInt: parsing "notanumber": invalid syntax` |

This is a direct, verified violation of "preserving the existing observable contract": both the HTTP status code and the body shape (JSON → plain text) changed. No test in the diff or the pre-existing suite exercises malformed path parameters for any of the three endpoints, so `go test ./...` (which fully passes) does not catch it.

Fix direction: mirror the existing `isInstallationTokenRequest` / `validateInstallationTokenBody` pattern — validate/parse the numeric path segments in the outer wrapper before delegating to `api.ServeHTTP`, and reject with the old `writeError(..., http.StatusNotFound, "Not Found")` shape on failure, exactly as before.

### 2. [bug] Same endpoint, same nominal error, two different JSON envelopes

`POST /app/installations/{id}/access_tokens` can reject a body with `422 Validation Failed` for two different reasons: an unknown JSON field (checked in the new pre-library `validateInstallationTokenBody`) or supplying both `repositories` and `repository_ids` (checked inside `appsService.CreateInstallationToken`, after dispatch into the library). Both paths return the exact same status code and message text, but now produce **different wire bodies**:

- Unknown field (pre-check, still uses the legacy `writeError`/`writeJSON` helpers):
  `{"documentation_url":"https://docs.github.com/rest","message":"Validation Failed","status":"422"}`
- Both repo fields (inside the migrated service, returned via the new `githubAPIError` helper → library's `writeErrorResponse` → `json.Marshal(*github.ErrorResponse)`):
  `{"message":"Validation Failed","errors":null,"documentation_url":"https://docs.github.com/rest"}`

Both bodies were captured from a live request against the running server (verified). The difference isn't cosmetic — one has a `status` field and no `errors` field, the other has `errors: null` (because `github.ErrorResponse.Errors` has no `omitempty` tag) and no `status` field. This is not limited to installation-token creation: **every** error returned via `githubAPIError()` from any of the three migrated services (401 bad-credentials, 403 suspended, 404 not-found, 409 conflict-on-cancel, 422, 500) now uses the new shape, while every other DTU-emitted error (unsupported-route 404s, the pre-check 422, git errors) still uses the old shape. Confirmed for the cancel-workflow-run 404 case too: `{"message":"Not Found","errors":null,"documentation_url":"..."}`.

The new shape is, incidentally, closer to what `go-github`'s own `ErrorResponse` struct actually looks like (and thus what any Octokit/go-github consumer would parse), and the old `status` field was never part of real GitHub's error body in the first place — so this isn't a fidelity regression relative to real GitHub. It is, however, a **new internal inconsistency**: the same DTU process now emits two different error envelope shapes for indistinguishable classes of error, depending on which code path (pre-check vs. inside a migrated service) rejected the request. Any consumer or future contract test asserting envelope shape by field presence (rather than just `message`) will observe non-deterministic-looking behavior across otherwise-equivalent 422s from the same endpoint.

Fix direction: pick one shape and make the legacy `writeError` helper and `githubAPIError` produce it consistently (e.g., drop the `status` field project-wide, since it isn't real GitHub behavior anyway, and let `writeError` build a `github.ErrorResponse`-shaped body directly).

### 3. [design] The migration shipped with zero migration-specific tests

The only test changes in the diff are import-path bumps (`v81` → `v89`) and swapping `client.BaseURL = ...` for `github.WithEnterpriseURLs(...)` in the shared `githubClient` test helper (forced by `go-github` v89 making `Client.baseURL` a private field with no setter — this part is a necessary consequence of the version bump, not scope creep). No new test asserts anything about the new routing/binding/error-writing behavior introduced by adopting the library. Both bugs above exist entirely within `go test ./...`'s blind spot: the full suite passes unmodified. For a migration whose explicit goal is contract preservation, the diff would benefit from at least one test per endpoint covering a malformed path parameter and one asserting envelope shape equality between at least two different error causes on the same endpoint.

### 4. [design] The P1 system proof is a meaningful check for two of the three endpoints, but not the third

- `CreateInstallationToken` and `PullRequests.Get` **are** meaningfully covered: MergeHerder's real production code (`src/lib/server/merge-queue/github.ts`, functions `mintInstallationToken` and `readP1Pull`) calls both through an `Octokit` client whose `baseUrl` is the raw `GITHUB_API_URL` with no `/api/v3` prefix — exactly the unprefixed routing path that `TestMergeHerderP1OneCleanPRLands` exercises end-to-end through the real product. That test passing (per the worklog) is real evidence these two endpoints still work for actual production traffic shape.
- `CancelWorkflowRunByID` is **not exercised by the system proof at all**. `TestMergeHerderP1OneCleanPRLands` is the "one clean PR lands" happy path; grepping MergeHerder's merge-queue source finds no caller of the cancel-workflow-run endpoint anywhere in the P1 product code. Coverage for this endpoint is exclusively `dtu/cancellation_test.go`'s unit tests, run through the same `githubClient` helper that was switched to `github.WithEnterpriseURLs`, which prefixes every request path with `/api/v3/`. `go-github-server`'s route matcher strips that prefix identically to no-prefix requests (`server.go`'s `route.match`, confirmed by reading it), so this doesn't hide a routing bug today — but it does mean the DTU's own unit test for this endpoint exercises a URL shape that no real consumer (MergeHerder's Octokit client) ever actually sends, and the endpoint has no system-level (real-product) regression coverage at all.

Net: the system proof is meaningful for 2 of the 3 migrated operations; it provides no signal on the third. This is worth knowing when deciding how much weight the "proof: ... passed" line in the worklog should carry for this specific diff — it does not, by itself, validate the cancellation migration.

### 5. [nit] `randomToken()` now prefixes minted installation tokens with `"ghs_"`

Unrelated to the stated migration. `go-github-server`'s own prefix-based auth dispatch (`authenticate()` sniffing `ghp_`/`ghs_`) is only reachable when an `Authenticator` is supplied to `githubserver.New`; DTU passes `nil`, so that code path is entirely bypassed and the prefix has no functional effect on routing or auth in this codebase. It changes the literal shape of a value DTU emits (installation tokens), which is technically outside "preserving the existing observable contract," though harmless in practice (it's an opaque bearer credential either way, and it's a plausible authenticity improvement — real GitHub App tokens do start with `ghs_`). Not called out in the worklog's decision list. Low-impact scope creep; flagging rather than blocking on it.

### 6. [nit] Authorization header parsing tightened from `strings.Cut` to `strings.Fields`

`authorizationCredential` (new) requires exactly two whitespace-separated fields; the old `authenticateInstallationToken`/`bearerToken` used `strings.Cut(header, " ")`, splitting only on the first literal space. The practical difference only matters for pathological headers with irregular whitespace (extra spaces, tabs) around the credential, which no real token ever contains. This also brings DTU's own parsing in line with the library's internal `bearerCredential`, which already uses `strings.Fields`. Effectively a non-issue, noted for completeness.

---

## Merge recommendation

**Do not merge as-is.** The migration is well-scoped (no drift into git/webhooks/workflow/control-plane territory, clean dependency bump, and the two core endpoints covered by the P1 proof do work end-to-end), but findings #1 and #2 are verified, concrete regressions against the explicitly stated goal of preserving the existing observable contract, and they ship with no test coverage that would have caught them. Both are narrow and cheap to fix (pre-validate numeric path segments the same way installation-token bodies are already pre-validated; unify the error envelope between `writeError` and `githubAPIError`). Fix those two, add tests for malformed path parameters and for envelope-shape equality across error causes on the same endpoint, and this is a clean, mergeable, appropriately narrow migration.
