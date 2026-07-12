# P1 API Proof — Claude Round 1 Review

## Prompt Received

> # P1 API Proof Review - Claude Round 1
>
> ## Goal
>
> Build a valid, workable first executable proof environment for the designated
> P1 subset of GitHub used by MergeHerder. It must start real HTTP servers and
> prove behavior by making real requests rather than relying only on handler unit
> tests. The bounded public surface is installation-token creation, targeted PR
> retrieval, and the Git smart-HTTP effect of the minted token. Setup, time
> control, and inspection belong to a distinct private control listener.
>
> The proof must use `google/go-github` as a wire-level client oracle while also
> asserting raw responses, real Git behavior, virtual expiry, and executable
> startup. It must not claim that this alone proves live-GitHub parity or the full
> MergeHerder P1 system scenario. Internal data should use concrete values;
> pointers are appropriate only for shared state, lifecycle/no-copy types, or
> external APIs that require them. Do not use `github.Ptr`; Go 1.26 `new(value)`
> is the convention.
>
> ## Review Instructions
>
> If you have a code review skill, use it. Otherwise review directly. Give a
> complete review of the current design, implementation, documentation, and
> proof against the goal. Inspect the full branch diff and repository contracts,
> not only the obvious Go files. Run relevant proof commands and explore source
> or dependency behavior where useful.
>
> Prioritize demonstrable bugs, races, security problems, incorrect Git or GitHub
> semantics, missing claim coverage, proof that bypasses the intended boundary,
> over-engineering, unrequested layers, lifecycle failures, wrong ownership
> boundaries, and documentation that overstates what passed. Prefer real findings
> over style preferences.
>
> Do not edit product code, tests, contracts, or documentation. Write the exact
> prompt you received and your complete findings only to:
>
> `ephemeral/reviews/20260711-p1-api-proof-claude-round1.md`
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
> Finish with a clear verdict on the implementation and the proof. Passing tests
> alone is not consensus; identify any remaining conditions precisely.

## Evidence Reviewed

- Full working tree for branch `codex/p1-api-proof` (the P1 runtime is in
  untracked `cmd/`, `dtu/`, `go.mod`, `go.sum`, `docs/p1-api-proof.md`; the
  committed diff from `origin/main` carries the contract/claims docs).
- `dtu/{server,types,public,git,control,client}.go`, `dtu/server_test.go`,
  `cmd/dtu-github/main.go`.
- `docs/api-endpoint-claims.md`, `docs/p1-api-proof.md`, README delta, and the
  `GITHUB_SERVICE_CONTRACT.md` addition.
- Prior `ephemeral/reviews/20260711-p1-api-proof-codex.md`.
- `go-github/v81` `InstallationPermissions` wire tags in the module cache.

### Commands run

- `go vet ./...` — clean.
- `go test ./...` — pass (`ok dtu 2.2s`).
- `go test -race ./dtu/` — pass, no data races (3.2s).
- `gofmt -l .` — clean.
- `grep -rn github.Ptr dtu cmd` — none (convention honored; `new(value)` used).
- Filesystem case-sensitivity probe on the dev machine — **case-insensitive**
  (macOS), relevant to Finding 2.

The two defects from the Codex round (trailing-slash reaching an exact-path
handler; control owner `..` traversal) are present as fixed:
`public.go` trims only the single leading slash, and `validPathComponent`
rejects empty/`.`/`..`/slash/backslash. Both confirmed.

## Summary Judgment

This is a genuinely real proof, not a handler-unit-test in disguise. Product-
shaped calls cross real loopback HTTP (`go-github` with a rewritten `BaseURL`,
raw `http.DefaultClient`, the Git CLI over `git-http-backend`), setup/inspection
is confined to a separate control listener, virtual time drives both JWT and
installation-token expiry without sleeping, and `cmd/dtu-github` is built and
launched as an actual subprocess. Internal state uses concrete domain types and
confines `go-github` types to the wire boundary. The documentation is honest
about what is _not_ proven (live-GitHub parity, the full MergeHerder scenario,
the Octokit/webhook/workflow/worker layers) and correctly marks the `403`/`422`
statuses as provisional pending live conformance.

I found **no new critical issues and no demonstrable bugs or races** beyond the
two already fixed. The remaining findings are design/scope and coverage items —
most importantly a GitHub-semantics divergence baked into a firm (non-
provisional) claim whose two branches are never independently exercised.

## Findings

### Finding 1 — `contents:read` grants PR reads; PR-05's two branches are unproven — **design**

`dtu/public.go`:

```go
func canReadPulls(permissions map[string]string) bool {
    return permissionRank(permissions["pull_requests"]) >= permissionRank("read") ||
        permissionRank(permissions["contents"]) >= permissionRank("read")
}
```

Claim `PR-05` states the token must have "either Pull Requests read or Contents
read permission," and the implementation faithfully encodes that `OR`. Two
problems:

1. **GitHub-semantics divergence.** On real GitHub, `GET
/repos/{owner}/{repo}/pulls/{pull_number}` with an installation token
   requires the **Pull requests: read** repository permission. `contents:read`
   alone does not grant pull-request reads. Unlike the `403`/`422` statuses, the
   claims document does **not** mark `PR-05` as provisional — it is stated as a
   firm claim under "Pull Request Endpoint," and `GITHUB_SERVICE_CONTRACT.md`
   line 158 separately lists "read pull requests through REST" as a needed
   capability. This means a non-provisional claim asserts behavior GitHub most
   likely does not have, and live conformance will contradict it. Either scope
   `canReadPulls` to `pull_requests:read` (the GitHub-accurate rule), or move
   `PR-05` into the same explicitly-provisional bucket as `403`/`422` with a
   note that the `contents`-read allowance is a MergeHerder convenience awaiting
   live pinning.

2. **The `OR` is never exercised.** No test reads a PR through a
   `contents:read`-only token, and no test reads a PR through a
   `pull_requests:read`-only token. The golden-journey token carries _both_
   `contents:write` and `pull_requests:read` (seeded installation scope), so a
   successful read proves neither branch in isolation. The negative test's
   `contents:read`-only token (`server_test.go:193`) is used only for `git
ls-remote`; the `pull_requests:read`-only token (`:206`) is used only against
   Git (where it correctly fails). So the exact permission gate that `PR-05`
   asserts has zero direct REST coverage. Add one PR read with a
   `pull_requests:read`-only token (positive) and, if the `contents` allowance
   is kept, one with a `contents:read`-only token.

### Finding 2 — HTTP-07 case-insensitivity is proven for REST only; Git transport is filesystem-case-dependent — **design**

`HTTP-07` ("Owner and repository path names are matched case-insensitively") is
listed under **Shared HTTP** claims. For REST it holds: `repoName()` lowercases
both sides and the golden journey reads `PullRequests.Get(ctx, "aCmE",
"WIDGET", 7)`. But Git smart-HTTP goes over the same HTTP surface and does **not**
behave case-insensitively:

`dtu/git.go` authorizes using the case-insensitive `repoName` lookup, then hands
the **literal, case-sensitive** `request.URL.Path` to `git-http-backend` with
`GIT_PROJECT_ROOT` pointed at `.../repositories`. The repository lives on disk at
`repositories/{Owner}/{Name}.git` with the original case. A request to
`/AcMe/widget.git/...` would pass the DTU authorization check (lowercased match)
but then fail inside `git-http-backend` on a case-sensitive filesystem, because
`repositories/AcMe/widget.git` does not exist there.

I confirmed the dev machine's filesystem is **case-insensitive** (macOS), which
masks this entirely — the same proof on a case-sensitive Linux CI runner would
behave differently for mixed-case Git URLs. The coverage table maps `HTTP-07`
evidence to "case-insensitive REST lookup" only, so this is arguably scoped, but
the claim's placement under _Shared HTTP_ invites the reader to assume it covers
Git too. Recommend either (a) explicitly scoping `HTTP-07` to REST and stating
Git paths are case-sensitive, or (b) normalizing the on-disk repo path / lookup
so Git matches the claim — and pinning whatever is decided against live GitHub.
No functional change is required for the current tests, but this is a latent
portability gap that CI on Linux could expose.

### Finding 3 — Token-creation success body is asserted only through `go-github`; no raw independent check — **design**

The claims doc's Required Tests say "Response bodies and emitted errors are
asserted independently of `go-github` decoding." That is satisfied for:

- the PR success body (`assertRawPullResponse`, raw `number`/`head.sha`), and
- token/PR **error** shapes (`assertRawTokenFailure`, `rawError`).

But the **successful** installation-token response
(`201`, `token`, `expires_at`, `permissions`, `repositories`) is validated only
through `Apps.CreateInstallationToken`'s decoding (`server_test.go:38–53`).
Because `permissionsToGitHub` marshals a `map[string]string` back through
`github.InstallationPermissions`, the wire mapping for the success body is
proven only by the same library that would also decode a subtly wrong shape. A
short raw `GET`-style assertion on the token-create body (mirroring
`assertRawPullResponse`) would close the one place where the oracle grades its
own homework. Coverage gap, not a defect.

### Finding 4 — `refreshPullSnapshots` spawns `git` subprocesses while holding the world write lock — **nit**

`dtu/git.go`:

```go
func (w *world) refreshPullSnapshots(repository repository) {
    w.mu.Lock()
    defer w.mu.Unlock()
    for key, pull := range w.pulls {
        ...
        if sha := resolveRef(repository.gitDir, pull.baseRef); sha != "" { ... }
        if sha := resolveRef(repository.gitDir, pull.headRef); sha != "" { ... }
    }
}
```

Each `resolveRef` forks `git rev-parse`. Doing this under `w.mu.Lock()` blocks
every public and control handler (all of which take `w.mu`) for the duration of
N subprocess spawns after every `git-receive-pack`. Harmless for a single-
threaded proof, but it's the kind of "subprocess under a global lock" pattern
that becomes a latency/liveness surprise if the runtime is ever exercised
concurrently. Consider snapshotting the pull keys/refs under the lock, resolving
outside it, then re-acquiring to store.

### Finding 5 — `TestExecutableStartup` never drives the subprocess's public listener — **nit**

`TestExecutableStartup` builds and launches the binary, reads the readiness
line, calls the **control** listener (`/state`), and verifies signal-driven
shutdown. It does not issue a single request to the subprocess's _public_
GitHub/Git listener. So "executable startup" proves the process boots, emits a
machine-readable readiness record, serves control, and shuts down cleanly — but
not that the built binary actually serves the P1 surface end-to-end (that is
only ever proven against the in-process `dtu.Start` instance). `docs/p1-api-
proof.md` describes the test accurately, so this is not overstatement; it is a
modest opportunity to make the executable proof load-bearing by minting a token
and reading a PR through the subprocess.

### Finding 6 — HTTP-05 mutation-invariance is asserted only on the unsupported-PUT path — **nit**

`HTTP-05` ("Rejected requests cause no state mutation") is directly asserted via
the mutation counter only for the unsupported `PUT` case (`server_test.go:245`).
The validation-failure (`422`) and auth-failure (`401`/`404`) rejection paths
rely on reading the implementation (which increments `w.mutations` only on the
success path) rather than on a before/after mutation assertion. The
implementation is correct, so this is coverage thinness, not a bug; one extra
`state`-diff around a `422` token request would make `HTTP-05` hold by test
rather than by inspection.

## What I Explicitly Checked and Found Sound

- **Auth ordering (HTTP-08):** installation id parse → App-JWT auth → body
  decode → state lock. No protected state is touched before authentication.
- **JWT matrix (TOKEN-02..07):** `RS256`-only via `WithValidMethods` + explicit
  `alg` check; signature verified against the registered App key selected by
  `iss`; `iat`/`exp` presence enforced; `iat <= now`, `exp > now`, `exp <=
now+10m` all evaluated against **virtual** time. Negative subtests cover each.
- **Expiry (SCOPE-05/PR-16):** `now.Before(expiresAt)` gate for both REST and
  Git; `AdvanceTime("1h1ns")` then both a `go-github` `401` and a failing `git
ls-remote`. Genuinely time-driven, no sleeps.
- **Scope narrowing (TOKEN-11/13):** repository/permission narrowing cannot
  expand; `repositories`+`repository_ids` mutual exclusion; permission rank
  monotonicity. Response reports effective scope.
- **PR ref semantics (PR-10/12/14):** read prefers the live ref and falls back
  to the retained snapshot; `refreshPullSnapshots` guards on non-empty resolve,
  so a deleted source ref retains the last real SHA rather than inventing one.
  Cross-checked against the local fixture's `rev-parse` output.
- **No-mutation-on-read (PR-17):** `getPullRequest` takes `RLock`, never
  increments `mutations`; asserted by before/after state diff.
- **Boundary:** all product-shaped calls are real HTTP/Git; `dtu` is imported
  only for control-client types and inputs (legitimate setup/inspection). No
  handler is called directly.
- **Lifecycle:** `Start` cleans the temp data dir on every early-return path and
  hands ownership to the runtime on success; `Close` is `sync.Once`-guarded,
  drains both `serve` goroutines, and force-`Close`s on `Shutdown` error.
- **Conventions:** no `github.Ptr`; `new(value)` throughout; concrete internal
  types; `go-github` confined to the wire boundary. `go vet`, `-race`, and
  `gofmt` all clean.

## Verdict

**Implementation:** sound for the stated P1 boundary. No critical issues, no
demonstrable bugs, no races. The one substantive correctness concern is
Finding 1 — `contents:read` granting PR reads is a real divergence from GitHub
semantics that is currently asserted as a firm (non-provisional) claim and never
independently exercised.

**Proof:** real and load-bearing where it counts (network, Git transport, minted
token used against both REST and Git, virtual expiry, subprocess startup), and
honest about its limits. It does **not** prove live-GitHub parity or the full
MergeHerder P1 scenario, and does not claim to.

**Remaining conditions before this should be treated as consensus-passed
(precise):**

1. Resolve Finding 1: either restrict `canReadPulls` to `pull_requests:read`, or
   reclassify `PR-05`'s `contents`-read allowance as provisional-pending-live-
   conformance — and add a positive PR read through a `pull_requests:read`-only
   token (plus a `contents:read`-only read if that branch is retained).
2. Resolve Finding 2: explicitly scope `HTTP-07` to REST (documenting Git path
   case-sensitivity) or normalize Git repo resolution, so the claim's meaning is
   OS-independent — the current pass is partly an artifact of a case-insensitive
   dev filesystem.

Findings 3–6 are recommended coverage/robustness improvements and need not block
progress. With Findings 1 and 2 addressed, I would consider the bounded P1 API
proof complete for its declared scope.
