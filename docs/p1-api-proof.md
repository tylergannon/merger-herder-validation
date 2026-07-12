# P1 API Proof

## Boundary

This runtime implements and tests the first GitHub-shaped surface needed by the
P1 scenario:

- `POST /app/installations/{installation_id}/access_tokens`;
- `GET /repos/{owner}/{repo}/pulls/{pull_number}`; and
- Git smart HTTP for the repositories authorized by those tokens.

The public GitHub/Git listener and private control listener are distinct. Tests
create and inspect the universe only through control HTTP. Product-shaped calls
use real HTTP clients and Git subprocesses; they do not call handlers or stores
directly.

## Golden Journey

`TestP1APIGoldenJourney` performs one connected proof:

1. Start an empty server with temporary storage and fixed virtual time.
2. Create an RSA-backed App, active installation, and real bare repository
   through control HTTP.
3. Sign a real RS256 App JWT and mint through
   `go-github`'s `Apps.CreateInstallationToken`.
4. Use that exact token to push real `main` and `feature` refs over Git smart
   HTTP.
5. Create a PR through control HTTP and read it with
   `go-github`'s `PullRequests.Get`.
6. Assert the raw HTTP representation independently of `go-github` decoding.
7. Move the real feature ref and observe the new PR head SHA.
8. Clone the feature ref with the same installation token.
9. Close the PR, delete its source ref, and observe its retained last head SHA.
10. Advance virtual time without sleeping and prove the token fails for both
    REST and Git.
11. Verify the journey made no unsupported GitHub-shaped requests.

`TestP1APINegativeClaims` exercises the contracted JWT, installation,
restriction, scope, permission, error-shape, unsupported-request, and
non-disclosure failures. Its subtests include the relevant claim IDs.

`TestExecutableStartup` builds and launches `cmd/dtu-github`, consumes its
machine-readable readiness record, calls its control listener, and verifies a
clean signal-driven shutdown.

## Claim Coverage

The current proof covers the P1 endpoint claims in these groups:

| Claims                        | Evidence                                                                                                                                                                                                               |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `HTTP-01` through `HTTP-08`   | Exact routing, GitHub JSON and headers, `go-github` success/error decoding, mutation counters, unsupported-request inspection, case-insensitive REST and Git lookup, and authentication-first failures                 |
| `TOKEN-01` through `TOKEN-20` | Real App JWT and `go-github` minting, negative JWT matrix, installation ownership/activity, repository and permission narrowing, malformed restrictions, opaque independent tokens, response scope, and virtual expiry |
| `SCOPE-01` through `SCOPE-06` | The minted token is used against REST and real Git; repository/permission exclusions, expiry, and indistinguishable missing/out-of-scope REST errors are asserted                                                      |
| `PR-01` through `PR-18`       | Real `PullRequests.Get`, raw response checks, identity/state/Draft and base/head fields, case folding, real-ref movement, closure, source deletion, expiry, no-read-mutation, and deliberately bounded fields          |

The inactive-installation response and individual validation-failure statuses
currently use the implementation's provisional `403` and `422` behavior. The
claims document intentionally requires those values to be checked independently
against live GitHub before they are treated as final compatibility facts.

## Deliberate Limits

This proof does not claim that the full P1 MergeHerder scenario passes. That
requires starting the real MergeHerder process and adding its submission front
door, webhooks, workflow execution, worker interaction, and final ref-state
observation. It also does not replace the required narrow live-GitHub
conformance job or a TypeScript/Octokit consumer proof.
