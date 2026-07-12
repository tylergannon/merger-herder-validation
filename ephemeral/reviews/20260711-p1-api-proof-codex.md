# P1 API Proof Codex Review

## Prompt Received

Review the complete current implementation and proof against the user's goal:
build a real HTTP-server proof environment for the agreed P1 GitHub API subset,
using `go-github` as a wire oracle without treating it as the whole correctness
proof. Identify correctness, security, lifecycle, scope, and proof gaps. Check
the concrete-value convention, real network and Git boundaries, virtual time,
claim coverage, and whether the documented limits are honest. Label findings
critical, bug, design, or nit. Do not equate passing tests with proof outside
the stated boundary.

## Evidence Reviewed

- Complete working diff from `origin/main`.
- `docs/api-endpoint-claims.md` and `docs/p1-api-proof.md`.
- `dtu` runtime, control client, public handlers, Git CGI, and tests.
- `cmd/dtu-github` lifecycle and readiness behavior.
- `go vet ./...`, `go test -race -count=1 ./...`, and `git diff --check`.

## Findings

### Bug: trailing slash reached an exact-path handler

Accepted and fixed. The router used `strings.Trim(path, "/")`, allowing a
trailing slash to reach a supported endpoint despite `HTTP-01`. It now removes
only the single leading slash, and the negative network proof asserts that a
trailing slash is unsupported.

### Bug: control repository owner allowed parent traversal

Accepted and fixed. A trusted control caller could supply owner `..`, placing a
bare repository outside the configured repository root. Control path
components now reject empty, dot, dot-dot, slash, and backslash values.

### Design: live and MergeHerder system proof remain outstanding

Accepted as an explicit boundary, not represented as discharged. The runtime
uses provisional `403`/`422` results for the conditions whose exact status is
deferred to live GitHub. The documentation also says the real MergeHerder,
Octokit, webhook, workflow, worker, and final P1 observation layers remain.

### Critical

None remaining.

### Bug

None remaining after the two fixes above.

### Nit

None.

## Codex Verdict

Ready for independent Claude review. The implementation supplies a real
network/process/Git proof for the bounded P1 API runtime. It does not yet prove
live-GitHub parity or the complete MergeHerder P1 scenario, and does not claim
to do so.
