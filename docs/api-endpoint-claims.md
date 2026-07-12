# P1 API Endpoint Claims

## Purpose

This document defines the claims that the P1 GitHub-shaped REST endpoints must
satisfy before they can be treated as correct. Correctness here means
compatibility with the behavior MergeHerder requires, not general GitHub API
compatibility.

These claims govern the independent proof world's GitHub-shaped server. A Go
`go-github` client provides one wire-compatibility oracle; MergeHerder remains
the real TypeScript/Octokit consumer in system proof.

The P1 REST surface is:

- `POST /app/installations/{installation_id}/access_tokens`; and
- `GET /repos/{owner}/{repo}/pulls/{pull_number}`.

Git smart HTTP and webhook delivery are separate protocol boundaries. They are
referenced where an API claim has a downstream effect, but their complete
contracts do not belong in this document.

## Shared HTTP Claims

- `HTTP-01`: Only the exact supported method and path reach each handler.
- `HTTP-02`: Requests and responses use GitHub-compatible JSON and headers.
- `HTTP-03`: Successful responses decode through the corresponding
  `go-github` client method.
- `HTTP-04`: Error responses decode as `github.ErrorResponse`.
- `HTTP-05`: Rejected requests cause no state mutation.
- `HTTP-06`: Unsupported GitHub-shaped requests fail explicitly and are
  recorded diagnostically.
- `HTTP-07`: Owner and repository path names are matched case-insensitively.
- `HTTP-08`: Authentication is evaluated before protected resource state is
  returned.

## Installation Token Endpoint

Endpoint: `POST /app/installations/{installation_id}/access_tokens`

- `TOKEN-01`: The endpoint is callable through
  `Apps.CreateInstallationToken`.
- `TOKEN-02`: Authentication requires an App JWT, not an installation or user
  token.
- `TOKEN-03`: JWT signatures are verified using the registered App public key.
- `TOKEN-04`: The JWT algorithm is `RS256`.
- `TOKEN-05`: The JWT `iss` claim identifies the registered App.
- `TOKEN-06`: The JWT `iat` claim is present and no later than the proof world's
  controllable current time.
- `TOKEN-07`: The JWT `exp` claim is present, unexpired, and no more than ten
  minutes in the future.
- `TOKEN-08`: The installation exists and belongs to the authenticated App. A
  missing installation and an installation owned by another App both return
  the same `404` status and GitHub-compatible error shape.
- `TOKEN-09`: The installation is active. A suspended or otherwise inactive
  installation is rejected using the status and error shape established by
  live conformance.
- `TOKEN-10`: An omitted request body grants the installation's existing
  repository and permission scope.
- `TOKEN-11`: `repositories` or `repository_ids` may narrow repository scope,
  but cannot expand it.
- `TOKEN-12`: `repositories` and `repository_ids` cannot both be supplied.
- `TOKEN-13`: Requested permissions may narrow the App's permissions, but
  cannot expand them.
- `TOKEN-14`: Unknown repositories, invalid permissions, and malformed
  restrictions are rejected.
- `TOKEN-15`: Success returns `201 Created`.
- `TOKEN-16`: The response contains an opaque nonempty token and its expiration
  time.
- `TOKEN-17`: The response accurately reports the token's effective permissions
  and repository scope.
- `TOKEN-18`: No consumer assumes a token length or internal format.
- `TOKEN-19`: Invalid JWTs produce `401`, and missing or foreign installations
  produce `404`. Unknown repositories, invalid permissions, and malformed
  restrictions are each independently pinned to the status and error shape
  observed in live conformance; tests must not assume that they share a status
  merely because GitHub documents a general validation-failure response. The
  inactive-installation rejection in `TOKEN-09` is likewise pinned
  independently.
- `TOKEN-20`: Multiple valid tokens may coexist without revoking or changing
  one another.

## Installation Token Behavior

These claims prove that a successful token response has the promised effect.
They are not satisfied merely because the response decodes.

- `SCOPE-01`: The minted token authenticates supported REST calls.
- `SCOPE-02`: The same token authenticates Git smart HTTP using GitHub's
  installation-token Basic-auth shape.
- `SCOPE-03`: The token cannot access a repository outside its effective
  repository scope.
- `SCOPE-04`: The token cannot perform an operation exceeding its effective
  permission scope.
- `SCOPE-05`: The token stops working after expiration.
- `SCOPE-06`: For protected REST resources, a missing resource and a resource
  outside the token's repository scope return the same `404` status and
  GitHub-compatible error shape.

## Pull Request Endpoint

Endpoint: `GET /repos/{owner}/{repo}/pulls/{pull_number}`

- `PR-01`: The endpoint is callable through `PullRequests.Get`.
- `PR-02`: `owner`, `repo`, and `pull_number` identify exactly one fixture PR.
- `PR-03`: Owner and repository names are case-insensitive.
- `PR-04`: The installation token includes the requested repository.
- `PR-05`: The token has either Pull Requests read or Contents read permission.
- `PR-06`: A valid request returns `200 OK` and decodes as
  `github.PullRequest`.
- `PR-07`: The response identifies the correct repository and PR number.
- `PR-08`: The response reports the current open or closed state and Draft flag.
- `PR-09`: The response reports the current base repository, ref, and SHA.
- `PR-10`: While the source ref exists, the response reports the current head
  repository, ref, and SHA. After deletion, it retains the last PR snapshot as
  constrained by `PR-14`.
- `PR-11`: For an open same-repository PR, the reported base and head SHAs agree
  with the corresponding real Git refs.
- `PR-12`: Moving a source ref changes the next PR snapshot's head SHA.
- `PR-13`: Closing a PR changes its state without making its snapshot
  unreadable.
- `PR-14`: Deleting a source ref does not invent a new SHA. The retained PR
  snapshot and the real Git fetch result remain distinct facts.
- `PR-15`: Unknown repositories, unknown PRs, and inaccessible resources return
  `404`.
- `PR-16`: Expired or malformed credentials are rejected.
- `PR-17`: Reading a PR causes no Git, PR, workflow, or webhook mutation.
- `PR-18`: P1 asserts only PR identity, state, Draft status, and base/head
  repository, ref, and SHA. Other `github.PullRequest` response fields are
  unspecified and must not be required by MergeHerder or asserted by P1 tests.

## Live Conformance Rule

Every `HTTP-*`, `TOKEN-*`, `SCOPE-*`, and `PR-*` claim that describes GitHub
behavior is subject to the live-conformance strategy in
[`GITHUB_SERVICE_CONTRACT.md`](../GITHUB_SERVICE_CONTRACT.md). A claim is not
considered final merely because the proof world and `go-github` agree with each
other.

## Required Tests

- Every success path is exercised through a real configured `go-github`
  client.
- Every contracted rejection has a focused negative test.
- Token tests mint a token through the endpoint and then use that exact token
  against REST and Git.
- PR tests mutate the underlying real refs and verify subsequent snapshots.
- Response bodies and emitted errors are asserted independently of
  `go-github` decoding.
- Controllable time proves JWT and installation-token expiry without sleeping.

The `go-github` tests are the unit-level wire oracle. They do not replace real
Git transport tests, MergeHerder system proof, or narrow live-GitHub
conformance.

## Implementation Boundary

- Internal world state uses small concrete domain types.
- `go-github` types are confined to the REST and webhook wire boundary.
- Handlers are ordinary `net/http` handlers; P1 does not require endpoint or
  model code generation.
- Pointer-valued `go-github` fields use Go's `new(value)` form. Do not use
  `github.Ptr`.

## Explicitly Excluded From P1

- OAuth, users, browser sessions, and OAuth tokens;
- conditional requests and `304 Not Modified` behavior;
- alternate pull-request media types;
- fork and other cross-repository pull requests;
- public unauthenticated pull-request reads;
- rate limiting and abuse detection;
- pagination, listing, and search;
- general GitHub REST compatibility; and
- endpoints not required by the accepted P1 interaction contract.

## Primary References

- [GitHub App JWT generation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)
- [Create an installation access token](https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app)
- [Get a pull request](https://docs.github.com/en/rest/pulls/pulls#get-a-pull-request)
- [`go-github` installation-token client](https://github.com/google/go-github/blob/master/github/apps.go)
- [`go-github` pull-request client](https://github.com/google/go-github/blob/master/github/pulls.go)
