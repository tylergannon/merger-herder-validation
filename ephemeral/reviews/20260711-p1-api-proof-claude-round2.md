# P1 API Proof — Claude Round 2 Review

## Prompt Received

> # P1 API Proof Review - Claude Round 2
>
> ## Goal
>
> Re-review the complete P1 API proof implementation after the round-1 findings.
> Determine whether the current runtime and tests are now a valid, workable real
> HTTP/process/Git proof for the bounded installation-token and targeted-PR
> surface, without conflating this slice with live-GitHub conformance or the full
> MergeHerder P1 system scenario.
>
> ## Round-1 Reconciliation
>
> Read `ephemeral/reviews/20260711-p1-api-proof-claude-round1.md` and inspect the
> current implementation rather than relying on this summary.
>
> - Finding 1's semantic assertion was rejected with current primary evidence.
>   GitHub's documentation for `GET
/repos/{owner}/{repo}/pulls/{pull_number}` says a fine-grained token must have
>   at least one of Pull Requests read or Contents read. `PR-05` and
>   `canReadPulls` therefore remain. The valid proof criticism was accepted:
>   tests now mint separate tokens and successfully read the PR through each
>   permission branch in isolation.
> - Finding 2 was accepted. Git requests are rewritten to the repository's
>   canonical authorized owner/name path before `git-http-backend`, removing
>   filesystem-dependent casing. The network suite sends a mixed-case Git URL.
> - Finding 3 was accepted. A raw token-success request now independently asserts
>   `201`, token, expiry, effective permissions, and repository scope.
> - Finding 4 was accepted. PR refs are snapshotted under a read lock, resolved
>   through Git subprocesses outside the lock, then conditionally stored.
> - Finding 5 was accepted. The executable test calls both control and public
>   listeners before shutdown.
> - Finding 6 was accepted. The complete focused rejection table is bracketed by
>   mutation-counter assertions.
>
> The implementation continues to treat inactive-installation and validation
> statuses as provisional pending the separately required live-conformance layer.
>
> ## Review Instructions
>
> Adjudicate every round-1 critical, bug, and design finding and inspect the
> revisions for new bugs, races, proof gaps, or overstatement. Run relevant proof
> commands. Do not edit product code, tests, contracts, or documentation. Write
> the exact prompt and complete findings only to:
>
> `ephemeral/reviews/20260711-p1-api-proof-claude-round2.md`
>
> Label every finding using exactly one definition:
>
> - **critical:** must fix before proceeding.
> - **bug:** demonstrable incorrect behavior, broken contract, race, or
>   regression.
> - **design:** architecture, boundary, scope, maintainability, or proof issue
>   that is materially likely to cause problems.
> - **nit:** small cleanup that should not block progress.
>
> Finish with a clear consensus verdict. Do not claim that this bounded proof
> discharges the named live-GitHub or MergeHerder system layers.

## Evidence Reviewed

- Current working tree for `codex/p1-api-proof`: `dtu/{server,types,public,
git,control,client}.go`, `dtu/server_test.go`, `cmd/dtu-github/main.go`.
- `docs/api-endpoint-claims.md`, `docs/p1-api-proof.md`, `GITHUB_SERVICE_
CONTRACT.md`, and the README delta.
- Round-1 review (`ephemeral/reviews/20260711-p1-api-proof-claude-round1.md`).
- GitHub primary documentation for the fine-grained-token permission on
  `GET /repos/{owner}/{repo}/pulls/{pull_number}` (to adjudicate Finding 1).

### Commands run

- `go vet ./...` — clean.
- `gofmt -l dtu cmd` — clean.
- `go test ./...` — pass (`ok dtu 1.945s`).
- `go test -race ./dtu/` — pass, no data races (3.0s).
- `grep -rn github.Ptr dtu cmd` — none (`new(value)` convention honored).
- `grep -n` sweep of the test file for negative Git-push coverage — none found
  (see Finding R2-1).

## Adjudication of Round-1 Findings

**Finding 1 (design) — `contents:read` grants PR reads; PR-05 branches unproven —
RESOLVED (semantics upheld, coverage added).**

I independently verified GitHub's fine-grained-token requirement for
`GET /repos/{owner}/{repo}/pulls/{pull_number}`. GitHub's documentation states
the endpoint is accessible with **at least one of** "Pull requests" (read) **or**
"Contents" (read) — an OR, not the AND that round-1 asserted. Round-1's
semantic claim (#1) was therefore not supported by primary evidence and is
correctly rejected; `canReadPulls`'s `pull_requests>=read || contents>=read` is
GitHub-accurate, and keeping `PR-05` as a firm (non-provisional) claim is
justified. The coverage criticism (#2) is now satisfied:
`assertPermissionAlternatives` (server_test.go:416) mints a `contents:read`-only
token and a `pull_requests:read`-only token — each scoped to repository 100 with
the other permission absent — and reads PR 7 through REST with each, exercising
both `OR` branches in isolation. Confirmed resolved.

**Finding 2 (design) — Git transport filesystem-case dependence — RESOLVED.**
`serveGit` (git.go:64–68) now clones the request and rewrites its path to the
canonical, authorized `"/" + repository.owner + "/" + repository.name + ".git/"

- parts[2:]`before handing it to`git-http-backend`, so the on-disk lookup no
longer depends on request casing. `repository`is captured under the read lock
(git.go:25–30) as a value copy;`parts[2:]`is constrained by`isGitPath`/
`gitService`to fixed service tokens (no traversal).`RawQuery`is preserved
through`canonicalURL := \*request.URL`, so `?service=`survives. server_test.go:197
now drives`git ls-remote`against`aCmE/WIDGET`, so the mixed-case Git path is
  proven, not merely masked by a case-insensitive dev FS. Confirmed resolved.

**Finding 3 (design) — token success body only graded by go-github — RESOLVED.**
`assertRawTokenResponse` (server_test.go:476) issues a raw POST with `appJWT`
and independently asserts `201`, nonempty `token`, `expires_at ==
proofTime+1h`, `permissions.contents == "read"`, and the single scoped
repository (id 100, name `widget`) — closing the place where the oracle graded
its own homework. Confirmed resolved.

**Finding 4 (nit) — subprocess spawn under the world write lock — RESOLVED.**
`refreshPullSnapshots` (git.go:94) now snapshots pull keys/refs under `RLock`,
resolves refs via `git` outside any lock, then re-acquires `Lock` and stores
only when the snapshot's refs still match (guards against a racing state
change). Correct and no longer holds the lock across subprocess spawns.

**Finding 5 (nit) — executable never drove its public listener — RESOLVED
(minimally).** `TestExecutableStartup` (server_test.go:294) now issues a real
HTTP GET to the subprocess's public listener and asserts the `404` +
`X-DTU-Unsupported: true` unsupported-request behavior end-to-end. See R2-2:
this exercises the public listener but not a full token→PR journey through the
built binary; that remains proven only in-process. Accepted as a nit.

**Finding 6 (nit) — HTTP-05 mutation-invariance only on the PUT path —
RESOLVED.** The full 14-row rejection table (server_test.go:148–176), spanning
401/403/404/422 auth and validation failures, is now bracketed by
`rejectionsBefore`/`rejectionsAfter` mutation-counter assertions
(server_test.go:146,177–180). HTTP-05 now holds by test across every contracted
rejection class, not only unsupported-PUT. Confirmed resolved.

All six round-1 findings are adequately addressed. No round-1 finding was a
`critical` or a demonstrable `bug`, and none regressed.

## New Findings

### R2-1 — Git `git-receive-pack` write-permission rejection is asserted only by inspection — **nit**

`serveGit` gates pushes on `contents:write` (git.go:43) and reads on
`contents:read` (git.go:47). The **read** rejection branch is covered
positively-negatively: `pullOnly` (a `pull_requests:read`-only token with no
`contents`) fails `git ls-remote` (server_test.go:219), and an out-of-scope repo
fails (server_test.go:209). But **no test pushes with an insufficient-permission
token**, so the `git-receive-pack && contents < write → 403` branch — the write
half of `SCOPE-04` over Git transport — is never exercised. Every `runFails`
Git assertion in the suite uses `ls-remote` (upload-pack/read side). The
implementation is correct by reading, but one `git push` with a `contents:read`
token asserting failure would move this from inspection to proof, mirroring the
symmetry the round-1 coverage findings valued. Coverage thinness, not a defect.

### R2-2 — The built binary still never serves a full token→PR journey — **nit**

`TestExecutableStartup` now proves the subprocess boots, emits a readiness
record, serves control, serves an unsupported public request, and shuts down on
signal — a genuine improvement over round 1. It still does not mint a token and
read a PR through the _built binary's_ public listener; the load-bearing P1
surface (token creation, PR read, Git transport, expiry) is proven only against
the in-process `dtu.Start` instance. `docs/p1-api-proof.md:42` describes the
test accurately ("consumes its readiness record, calls its control listener"),
so this is not overstatement — only a residual opportunity to make the
executable proof fully load-bearing. Non-blocking.

### R2-3 — Token-body decoding is stricter than GitHub (`DisallowUnknownFields`) — **nit**

`decodeOptionalJSON` (public.go:249) sets `DisallowUnknownFields()`, so an
installation-token request carrying any field outside
`github.InstallationTokenOptions` is rejected `422`. Real GitHub tolerates
unknown body fields. This never bites the go-github client (it sends exactly
`InstallationTokenOptions`) and the `422` validation statuses are already marked
provisional-pending-live-conformance, so it sits inside the acknowledged
provisional bucket. Worth noting only so the eventual live-conformance pass
pins whether strict rejection is intended. Non-blocking.

### R2-4 — Claim-coverage table still credits `HTTP-07` to REST only — **nit**

With Finding 2 fixed, Git smart-HTTP is now case-folded and directly tested
(server_test.go:197), yet the `docs/p1-api-proof.md:52` evidence cell still reads
"case-insensitive REST lookup." This _understates_ current coverage (the
opposite of overstatement) and is harmless, but the evidence description is now
stale relative to the code. A one-line doc touch would keep the coverage table
honest about what the Git path proves. Non-blocking.

### R2-5 — `selectRepositories` name match ignores owner — **nit**

When a token request narrows by `repositories: [name]`, `selectRepositories`
(public.go:274–287) matches any in-scope repository whose `name` equals the
requested string via `EqualFold`, without checking owner. Within a single
installation two repositories can share a name under different owners; this
would select the first match. Not reachable by the current single-owner
fixtures and not a correctness issue for the proven scope, but a latent
ambiguity if multi-owner installations are ever seeded. Non-blocking.

## Re-Verified Sound (spot-checks beyond the round-1 list)

- **JWT path unchanged and correct:** `RS256`-only (`WithValidMethods` +
  explicit `alg` check), signature keyed by `iss`→registered App public key,
  `iat`/`exp` presence enforced, `iat<=now`, `exp>now`, `exp<=now+10m`, all on
  virtual time. Negative matrix (server_test.go:155–168) covers each.
- **Auth-before-state ordering (HTTP-08):** both handlers parse id → authenticate
  → decode/scope → touch locked state; no protected state precedes auth.
- **Concurrency of the revised paths:** `getPullRequest` copies `repository`/
  `pull` under `RLock`, then resolves refs and marshals outside the lock — safe.
  `refreshPullSnapshots` re-validates refs before storing. `-race` clean.
- **Lifecycle:** `Start` removes the temp data dir on every early-return path and
  transfers ownership on success (`keepDataDir=false`); `close` is `sync.Once`,
  drains both `serve` goroutines, force-`Close`s on `Shutdown` error.
- **Boundary/conventions:** all product-shaped calls cross real HTTP/Git;
  `dtu` is imported only for control types; no `github.Ptr`; `new(value)`
  throughout; `go-github` confined to the wire boundary; `vet`/`gofmt`/`-race`
  clean.
- **No new overstatement:** `docs/p1-api-proof.md` and the claims doc continue to
  disclaim live-GitHub parity, the full MergeHerder scenario, and the
  Octokit/webhook/workflow/worker layers, and keep `403`/`422`/inactive statuses
  provisional.

## Consensus Verdict

**Implementation:** sound for the declared P1 boundary. All six round-1 findings
are adequately resolved; I found **no new critical issues, no demonstrable bugs,
and no races**. The disputed round-1 semantic claim (Finding 1) is settled in
the implementation's favor by GitHub's own documentation: `contents:read` OR
`pull_requests:read` is the correct rule for the PR-get endpoint, and both
branches now have isolated positive REST coverage.

**Proof:** real and load-bearing where it matters — token minting over loopback
HTTP via go-github, that exact token used against both REST and real Git smart
HTTP (including a proven mixed-case Git path), independent raw-body assertions on
both success and error shapes, virtual-time expiry without sleeping, and a
subprocess boot/shutdown check. The remaining findings (R2-1 through R2-5) are
`nit`-level coverage and hygiene items — most notably the untested Git
write-permission rejection branch — and none blocks progress.

**This bounded proof does not discharge, and does not claim to discharge, the
narrow live-GitHub conformance layer or the full MergeHerder P1 system
scenario.** The inactive-installation and validation-failure statuses remain
provisional pending that separate live-conformance job. Within its declared
scope, and with round-1's conditions met, I consider the P1 API proof
**complete and consensus-passed**; addressing R2-1 (a negative Git push) would
be the single highest-value optional follow-up.
