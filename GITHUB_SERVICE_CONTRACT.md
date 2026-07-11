# GitHub Service Contract

## Purpose

This document is the inventory of every way MergeHerder depends on GitHub. It
records why the dependency exists, the semantic claim MergeHerder makes about
it, and where that claim must be proved.

The product-level proof requirements live in
[`MERGE_HERDER_PROOF_SCENARIOS.md`](./MERGE_HERDER_PROOF_SCENARIOS.md). Those
scenarios are upstream of this inventory: they determine which GitHub contracts
are genuinely required.

The contract is deliberately narrower than the GitHub API. An endpoint, webhook,
browser URL, or Git transport behavior does not belong in the application or in
the GitHub digital twin merely because GitHub provides it. It belongs here only
when a concrete MergeHerder behavior requires it.

Each contract entry has one of these statuses:

- **Current**: used by the application today.
- **Required**: implied by an accepted MergeHerder behavior, although the
  coordinator is not implemented yet.
- **Proposed**: likely useful, but the product seam that would require it is not
  settled.
- **Deferred**: explicitly outside the first working coordinator.
- **Rejected**: considered and not part of the intended control plane.

For implemented behavior, the source code is the final description of what is
currently sent or accepted. For intended behavior, this contract and the active
domain worklog are design inputs; neither should turn an unresolved question
into a decision by accident.

## GitHub's Roles In MergeHerder

GitHub is not one dependency. It has four distinct roles:

1. **Identity provider**: authenticates a human operator.
2. **Installation and event source**: tells MergeHerder which repositories are
   installed and reports changes through signed webhooks.
3. **Git host**: stores the actual commit graph and branch references.
4. **CI control plane**: associates workflow runs with an exact commit, reports
   their state, and accepts cancellation.

The DTU may implement these roles in one process, but their contracts and proof
requirements remain separate.

## Contract Rule

Every GitHub-facing feature must make a claim of the following form:

> Given this authenticated request or event, GitHub identifies these objects,
> applies this state transition, and returns or delivers these fields and status
> semantics.

The DTU and live-GitHub conformance checks prove that claim. They do not claim
that the DTU reproduces GitHub generally.

## Current Contract: Human Authentication

MergeHerder currently uses Better Auth's GitHub provider for the web
authorization-code flow. In the installed Better Auth version, the GitHub URLs
are hard-coded rather than derived from the Octokit base URL.

| ID        | Status                                       | GitHub surface                                                                          | Why MergeHerder uses it                                                                       | Claim MergeHerder relies on                                                                                                                                                                                                                                |
| --------- | -------------------------------------------- | --------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `AUTH-01` | Current                                      | `GET https://github.com/login/oauth/authorize`                                          | Start interactive sign-in.                                                                    | GitHub validates the client and redirect URI, preserves the opaque `state`, obtains user authorization for the requested scopes, and redirects to MergeHerder with either a short-lived `code` or an OAuth error.                                          |
| `AUTH-02` | Current                                      | `POST https://github.com/login/oauth/access_token`                                      | Exchange the authorization code.                                                              | A valid client, code, and matching redirect URI yield a bearer access token; an invalid or consumed code does not.                                                                                                                                         |
| `AUTH-03` | Current                                      | `GET https://api.github.com/user`                                                       | Populate the Better Auth user and obtain the GitHub login.                                    | A valid user token identifies one stable GitHub user and supplies at least `id`, `login`, `name`, `email`, and `avatar_url`, with nullable profile fields remaining nullable.                                                                              |
| `AUTH-04` | Current                                      | `GET https://api.github.com/user/emails`                                                | Supply and verify an email when the public profile does not provide one.                      | A valid token with the requested email scope returns the user's visible email records, including `email`, `primary`, and `verified`.                                                                                                                       |
| `AUTH-05` | Current                                      | Octokit `users.getAuthenticated()` / `GET /user`                                        | Perform the application's narrow login-time owner authorization and persist the GitHub login. | The token identifies the same GitHub user seen by the OAuth provider. This currently duplicates part of `AUTH-03`; the duplication is inventory, not a recommendation.                                                                                     |
| `AUTH-06` | Current for organization-owned installations | Octokit `orgs.getMembershipForAuthenticatedUser()` / `GET /user/memberships/orgs/{org}` | Admit only an active member of the configured organization at login time.                     | For the configured organization, an authorized active member yields membership state `active`; absence, insufficient access, or another state is not accepted. User-owned installations compare the `AUTH-05` login locally and do not call this endpoint. |

Current scopes requested by Better Auth are `read:user` and `user:email`.
Ordinary authenticated page requests use the persisted local authorization
verdict and do not call GitHub.

Those current scopes do not authorize the organization-membership read required
by `AUTH-06` for private membership. Organization-owned login is therefore a
known broken contract until the in-progress authentication replacement either
requests `read:org` or adopts a different membership check. The proof tool must
not model `AUTH-06` as succeeding with the currently documented scopes.

### DTU treatment of OAuth

The OAuth flow is cheap to model but is not deeply coupled to batching. The DTU
OAuth provider needs real HTTP redirects, state/code handling, one-time code
exchange, bearer-token checks, and configurable user/membership records. It does
not need a realistic GitHub login page, password handling, MFA, SAML, account
recovery, or GitHub session cookies.

There is a real testability seam to resolve: the current Better Auth GitHub
provider hard-codes GitHub's authorization, token, and user-info URLs. We should
not route `github.com` through local DNS or a TLS interception proxy merely to
test OAuth. Either:

1. keep OAuth itself in a thin live-browser canary at first; or
2. make MergeHerder's provider configuration use the same configurable OAuth
   endpoints in production and DTU, with GitHub's real URLs as production
   configuration.

The second option gives better deterministic coverage, but it must exercise the
same application auth path in both environments. A test-only alternate auth
implementation would provide false confidence.

### Authentication implementation is not part of the GitHub contract

Better Auth is the current implementation, not an accepted product dependency.
Its replacement is under active consideration because it currently brings a
second database/query layer into an application whose data access otherwise
belongs to SQLc, and its built-in GitHub provider prevents straightforward DTU
endpoint configuration.

Any retained or replacement authentication implementation must preserve
`AUTH-01` through `AUTH-06` while satisfying these application boundaries:

- the application owns its user/session schema, migrations, and SQLc queries;
- production GitHub and the DTU use the same OAuth code path with configured
  provider endpoints;
- OAuth state is unpredictable, short-lived, bound to the initiating browser,
  and compared safely on callback;
- the browser session uses an opaque high-entropy token in a `Secure`,
  `HttpOnly`, `SameSite=Lax` cookie, with expiry and server-side revocation;
- only a hash of the browser session token is stored server-side;
- GitHub identity and the owner-authorization verdict are persisted at login;
  and
- the human's GitHub OAuth token is not retained after login unless a later
  accepted behavior specifically requires user-attributed GitHub API access.

The coordinator authenticates as the GitHub App installation through `APP-01`;
it does not reuse the human operator's OAuth token.

## Current Contract: Installation And Repository Discovery

MergeHerder deliberately does not list installations or repositories through
the GitHub REST API during ordinary application use. Local database state is
populated from verified GitHub App webhooks.

| ID        | Status  | GitHub surface                                | Why MergeHerder uses it                                                       | Claim MergeHerder relies on                                                                                                                                                                                                                                                                                    |
| --------- | ------- | --------------------------------------------- | ----------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `HOOK-01` | Current | Webhook HTTP envelope                         | Authenticate and deduplicate deliveries.                                      | A delivery has `X-GitHub-Delivery`, `X-GitHub-Event`, and `X-Hub-Signature-256`; the signature is HMAC-SHA256 over the exact raw body using the configured webhook secret. A requested redelivery retains the delivery GUID.                                                                                   |
| `HOOK-02` | Current | `installation` webhook                        | Create, update, suspend, or remove the locally known GitHub App installation. | The payload identifies the installation, account, target type, repository-selection mode, permissions, and suspension state. `action=deleted` means the installation no longer authorizes repository access. Events for another configured owner are recorded but do not mutate installation/repository state. |
| `HOOK-03` | Current | `installation_repositories` webhook           | Add or disable repositories selected for the installation.                    | The payload identifies the installation and the repositories added or removed. Repository identity is the immutable repository ID; owner/name are mutable attributes. Removed repositories become disabled locally.                                                                                            |
| `HOOK-04` | Current | `push` webhook                                | Update the locally observed repository head and wake the workflow boundary.   | The payload identifies the repository, installation, pushed ref, `before`, `after`, deletion/creation/forced flags, and sender. The current code records `after` and a push timestamp, then calls a stub dispatcher.                                                                                           |
| `HOOK-05` | Current | Other subscribed webhook events               | Retain an observable receipt without inventing behavior.                      | A correctly signed event may be stored even when MergeHerder has no workflow for it. The current receiver marks non-push events ignored after installation/repository synchronization where applicable.                                                                                                        |
| `NAV-01`  | Current | GitHub App install/configuration browser URLs | Let a human install the App or change repository selection.                   | These are external browser destinations, not REST API calls and not an application control plane. The DTU does not need to reproduce GitHub's installation settings UI for coordinator regression tests.                                                                                                       |

The delivery handler is idempotent by delivery GUID. GitHub can redeliver a
delivery, so duplicate receipt is an ordinary condition rather than an
exceptional test case.

## Current Negative Inventory

The application does **not** currently do any of the following:

- mint GitHub App installation tokens;
- clone, fetch, or push a repository as the App;
- read pull requests through REST or GraphQL;
- create or update Git references through REST;
- dispatch, inspect, rerun, or cancel Actions workflow runs;
- read check runs for a commit;
- close pull requests, comment on them, change Draft state, or delete branches;
- use labels as the MergeHerder control plane; or
- list repositories from GitHub to populate the application dashboard.

The coordinator in `src/lib/server/workflows/push-webhook-workflow.ts` is still a
stub. Historical documents that describe the negative-inventory operations as
implemented are stale or aspirational.

## Required Contract: First Working Batch Coordinator

These are the smallest GitHub capabilities implied by the accepted happy path,
Pause semantics, and fix-forward-on-`R` failure path. Exact Octokit method names
remain implementation choices; the semantic claims are the contract.

| ID        | Status                               | GitHub surface                                                                                               | Why MergeHerder needs it                                                                                     | Claim MergeHerder relies on                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| --------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `APP-01`  | Required                             | `POST /app/installations/{installation_id}/access_tokens`                                                    | Obtain short-lived App credentials for Git transport and installation API calls.                             | A valid App JWT for the recorded installation yields a time-limited token whose repository and permission scope cannot exceed the installation. Expired or out-of-scope tokens are rejected. The token is opaque; MergeHerder must not assume a length or format.                                                                                                                                                                                              |
| `GIT-01`  | Required                             | Git smart-HTTP fetch using an installation token                                                             | Read the recorded default-branch base, each batch member's source head, and the release branch if it exists. | Advertised refs resolve to real Git commit objects. Fetching a recorded SHA yields the same object graph GitHub stores, or fails explicitly if it is unavailable or unauthorized.                                                                                                                                                                                                                                                                              |
| `GIT-02`  | Required                             | Git smart-HTTP push of the release ref `R`                                                                   | Publish the assembled contribution chain and later repair commits so CI can run on the exact candidate.      | The accepted push makes `R` point to the pushed real commit. Rebuilding `R` may use explicit `--force-with-lease=refs/heads/R:<expected-old-SHA>`; receive-pack rejects the update if the server ref no longer matches that old object ID.                                                                                                                                                                                                                     |
| `GIT-03`  | Required                             | Git smart-HTTP push of the default branch                                                                    | Land the exact successfully tested release head.                                                             | A non-forced update succeeds only when it is a fast-forward allowed by repository policy. The new default-branch head is exactly the successfully tested `R` SHA, not a reconstructed merge. A stale or non-fast-forward update fails without moving the ref.                                                                                                                                                                                                  |
| `PR-01`   | Required, endpoint source unresolved | Pull-request snapshot, probably `GET /repos/{owner}/{repo}/pulls/{pull_number}` plus `pull_request` webhooks | Record and later revalidate each member's PR identity, source ref/head, base ref, and open state.            | The snapshot ties one PR number to a repository, base branch/SHA, head repository/ref/SHA, and lifecycle state. It can identify an advanced head, closed PR, changed base, or inaccessible fork. Source-ref deletion requires a real Git ref check because a PR snapshot may retain the last head SHA/ref.                                                                                                                                                     |
| `CI-01`   | Required                             | CI start caused by pushing `R`                                                                               | Start the repository's authoritative CI without reconstructing the candidate.                                | An App installation-token push of `R` causes an identifiable authoritative workflow run for the pushed head SHA. The configured workflow explicitly matches the release branch/ref pattern. The proof does not substitute an Actions `GITHUB_TOKEN`, whose triggered events ordinarily do not create another workflow run. No explicit `workflow_dispatch` call is assumed for the first design.                                                               |
| `CI-02`   | Required                             | `workflow_run` webhook                                                                                       | Correlate requested/in-progress/completed CI with the current batch and exact candidate SHA.                 | Run creation emits action `requested` with status `queued`; later actions are `in_progress` and `completed`. The event identifies repository, installation, workflow identity, run ID/attempt, triggering event, head branch, head SHA, status, and conclusion. Only the configured workflow and exact current `(run_id, run_attempt, head_sha)` may affect the batch. Duplicate, late, superseded-attempt, or other-workflow events do not authorize landing. |
| `CI-03`   | Required                             | `POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel`                                                    | Implement Pause: stop an underway run while keeping the batch current.                                       | A cancellable run accepts the request asynchronously. Acceptance does not itself make the run terminal, and a later completion event still may arrive. A completed or otherwise uncancellable run may return conflict. Regardless of the response race, Pause makes that run ineligible to authorize landing.                                                                                                                                                  |
| `HOOK-06` | Required                             | `push` webhook for `R` and the default branch                                                                | Observe published candidate and landing ref movement.                                                        | The payload's ref and `after` SHA describe the ref state produced by the push. Webhook observation is not a substitute for the guarded Git push result or for the database's exact tested-SHA invariant.                                                                                                                                                                                                                                                       |

`CI-03` means the App will need **Actions: read and write**, not the read-only
permission stated in the older GitHub App configuration document. This is a
concrete correction produced by the accepted Pause behavior.

The exact queue front door remains unresolved. `PR-01` is therefore a required
capability but not yet a decision that MergeHerder must poll or list pull
requests. A pull-request webhook, a targeted REST read after local submission,
or a combination may supply the snapshot. There is no current requirement for
`GET /repos/{owner}/{repo}/pulls` as a general listing API.

## Proposed Or Deferred GitHub Surfaces

These must not be added to the DTU until application behavior requires them.

| ID           | Status                        | Surface                                                                       | What decision would require it                                                                                                                                                               |
| ------------ | ----------------------------- | ----------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CI-04`      | Proposed                      | `GET /repos/{owner}/{repo}/actions/runs/{run_id}`                             | Reconciliation after process restart or a suspected missing webhook. It is not needed merely to process a valid `workflow_run` payload.                                                      |
| `CI-05`      | Proposed                      | Check-runs or combined-status reads for the candidate SHA                     | A decision that the authoritative CI result is an aggregate of checks rather than one configured Actions workflow. Multi-workflow/check aggregation is deferred.                             |
| `CI-06`      | Required mechanism unresolved | `POST /repos/{owner}/{repo}/actions/runs/{run_id}/rerun` or workflow dispatch | Accepted Unpause semantics require a fresh authoritative execution for unchanged `R`. The product must choose rerun or dispatch before P6 and its proof-tool interaction can be implemented. |
| `CI-07`      | Deferred                      | Force-cancel, logs, and jobs APIs                                             | Stuck-cancellation recovery or automated failure-log retrieval. The first fix-forward loop may receive failure context by a narrower mechanism still to be designed.                         |
| `PR-02`      | Deferred                      | Update/close PR, comments, Draft conversion, branch deletion                  | A settled post-landing or Cancel cleanup policy. Current design does not convert cancelled PRs to Draft and does not yet specify successful PR cleanup.                                      |
| `REF-01`     | Proposed alternative          | REST Git-reference endpoints                                                  | A deliberate choice to move refs via REST instead of Git smart HTTP. The DTU should not implement both paths preemptively.                                                                   |
| `INSTALL-01` | Proposed                      | Installation/repository listing REST APIs                                     | A settled recovery mechanism for missed installation webhooks. Normal repository listing remains database-backed.                                                                            |
| `HOOK-07`    | Proposed                      | Webhook delivery listing/redelivery APIs                                      | An automatic failed-delivery recovery job. GitHub does not automatically redeliver failed deliveries; v1 has not accepted such a job.                                                        |
| `LABEL-01`   | Rejected                      | Labels as queue state or operator commands                                    | This was rejected as the primary product interface. No DTU label subsystem is justified by MergeHerder v1.                                                                                   |

## DTU Realism Model

The DTU is realistic when it preserves the semantics that can make
MergeHerder's decisions right or wrong. It does not need GitHub's implementation
architecture, database schema, user interface, or breadth.

### What must be real

1. **Git objects and refs.** Repositories are actual bare Git repositories.
   Commits, trees, ancestry, conflicts, squash commits, force-with-lease, and
   fast-forward rejection are evaluated by Git itself.
2. **Application boundaries.** MergeHerder runs its production coordinator,
   webhook route, Git client, and database code against the DTU's HTTP and Git
   endpoints. Method-level mocks are not the main system proof.
3. **HTTP envelopes.** Request methods, paths, headers, authentication failures,
   status codes, JSON shapes, redirects, raw webhook bodies, and webhook
   signatures are exercised over HTTP.
4. **Persistent application state.** MergeHerder uses a real isolated Postgres
   database. Batch correctness is asserted from both the Git graph and database
   rows.

### What should be simulated

1. OAuth identities, organization membership, authorization codes, and tokens.
2. GitHub App installation-token issuance, expiry, repository scope, and the
   small permission set MergeHerder exercises.
3. Pull-request metadata and lifecycle state around real source refs.
4. Actions workflow-run creation and state transitions. Deterministic scenarios
   use scripted outcomes; selected workflow-fidelity scenarios execute the
   repository's actual workflow with `act` and Docker against the exact SHA.
5. Webhook delivery scheduling, including delay, redelivery, duplicate delivery,
   delivery failure, and late events.
6. Time. Token expiry, run delay, retry windows, and cancellation races use a
   controllable clock rather than wall-clock sleeps.

### What remains live-GitHub proof

- GitHub's real authorization/consent UI and registered callback behavior;
- actual GitHub App permission configuration and installation UI;
- exact GitHub branch-protection and App-bypass behavior;
- actual Actions runner scheduling, workflow parsing, and hosted-runner behavior;
- the production webhook payloads and API behavior represented by each contract
  claim; and
- compatibility with current GitHub and current Octokit releases.

These live checks are thin conformance canaries, not the primary MergeHerder
regression suite.

## Recommended DTU Architecture

The DTU's required observable behavior is defined in
[`DTU_GITHUB_SPEC.md`](./DTU_GITHUB_SPEC.md).

### Use Git, not an off-the-shelf forge

The first DTU should use bare repositories served by `git-http-backend`, Git's
own server-side implementation of smart HTTP. It supports real fetch and push
without requiring the DTU to implement Git's wire protocol. A small HTTP wrapper
can validate DTU installation tokens before delegating to Git and can use Git
receive hooks to apply the narrow branch rules being modeled.

Starting from Forgejo or Gitea would initially make the twin less trustworthy,
not more:

- their pull-request, App-auth, webhook, and Actions models are not GitHub's;
- an adapter would have to translate one forge's semantics into another's;
- their own internal asynchronous behavior would make deterministic race tests
  harder;
- the DTU would inherit a large upgrade and configuration surface unrelated to
  the contract; and
- the most valuable thing they supply for this project—real Git storage and
  transport—is already available directly from Git.

Forgejo becomes worth reconsidering only if the contract grows to require a
real forge UI, real generic pull-request workflows, or actual local execution of
many GitHub Actions workflows. The current contract does not.

### One public GitHub-shaped plane, one private scenario plane

The DTU should expose two intentionally different interfaces:

1. **GitHub-shaped plane**: only the OAuth, REST, webhook, and Git behavior in
   this contract. MergeHerder talks only to this plane.
2. **Scenario-control plane**: a test API or library used to create users,
   installations, repositories, PR records, CI outcomes, time advances, and
   delivery schedules. Production MergeHerder code must never call it.

This separation prevents test conveniences from leaking into the simulated
GitHub contract.

### State transition fidelity matters more than response breadth

For every implemented contract entry, the DTU should model:

- stable object identity and actual cross-object references;
- the smallest required success response;
- the error states MergeHerder makes decisions from;
- authorization and repository-scope rejection;
- whether the operation is synchronous or merely accepted;
- the resulting Git/ref/PR/run state;
- which webhook deliveries become eligible as a result; and
- configurable delivery timing and ordering.

Optional response fields may be omitted or filled with inert values unless
MergeHerder reads them. The DTU should fail loudly when MergeHerder calls an
uncontracted endpoint or begins reading an unmodeled field.

## Conformance Strategy

The DTU repository owns a shared contract suite. Each case names one of the IDs
in this document and runs the same normalized assertion against:

1. the DTU;
2. a dedicated live-GitHub sandbox repository using the latest available
   Octokit packages where Octokit is the client; and
3. where useful, the Octokit and dated GitHub API version pinned by
   MergeHerder.

OAuth redirect/code cases use an HTTP client or real browser rather than
pretending they are Octokit tests. Webhook shape cases use the latest applicable
Octokit webhook packages in addition to real captured deliveries.

Examples of normalized assertions:

- `AUTH-03`: token identifies the configured user and exposes the fields the
  application consumes;
- `APP-01`: minted token accesses the selected repository, rejects another
  repository, and expires;
- `GIT-03`: a fast-forward default-branch update succeeds and a non-fast-forward
  update is rejected without movement;
- `CI-03`: cancelling an active run is accepted asynchronously and cancelling a
  terminal run reports conflict;
- `HOOK-01`: a redelivery retains its GUID and verifies with the same secret;
  and
- `CI-02`: the completion payload correlates the run to the exact pushed head
  SHA.

"Latest Octokit" and "latest GitHub REST API version" are separate axes. The
live job must record the exact resolved package versions and API version. A
latest-Octokit success proves only the named contract cases; it cannot prove
that GitHub has not changed any API anywhere.

Small live cases may look like unit tests, but semantically they are destructive
contract/integration tests. They need an isolated sandbox owner/repository,
unique refs, cleanup, and serialization where GitHub state would otherwise
overlap.

## First DTU Scenario Target

The first end-to-end scenario should exercise the smallest behavior that
justifies the twin:

1. Create a real bare repository with default head `M` and source PR refs `A`,
   `B`, and `C`.
2. Let real MergeHerder code obtain a DTU installation token, fetch the refs,
   assemble real squash commits, and push release head `R0`.
3. Let the DTU create a workflow run bound to the exact `R0` SHA and deliver
   signed requested/in-progress/completed events ending in failure.
4. Append a real repair commit `H`, producing `R1`, and push it without
   rebuilding the contribution chain.
5. Complete the new run successfully for exact head `R1`.
6. Fast-forward the real default branch to `R1`.
7. Deliver a late or duplicate completion for `R0` and prove that neither Git
   nor the database changes.

This scenario makes Git, HTTP, signatures, Postgres, exact-SHA correlation,
and stale-event rejection real enough to find the coordinator bugs we care
about. A companion Pause scenario cancels an active run asynchronously, delivers
a racing success anyway, proves that the paused batch cannot land, and requires
a fresh run after unpause. Neither scenario requires a general GitHub clone.

## Sources

- [GitHub App authentication and installation tokens](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation)
- [GitHub REST API endpoints for Apps](https://docs.github.com/en/rest/apps)
- [GitHub REST API endpoints for workflow runs](https://docs.github.com/en/rest/actions/workflow-runs?apiVersion=2026-03-10)
- [GitHub REST API endpoints for Git references](https://docs.github.com/en/rest/git/refs?apiVersion=2026-03-10)
- [GitHub pull-request endpoint](https://docs.github.com/en/rest/pulls/pulls?apiVersion=2026-03-10)
- [GitHub webhook events and payloads](https://docs.github.com/en/webhooks/webhook-events-and-payloads)
- [GitHub webhook signature validation](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries)
- [GitHub webhook redelivery](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks)
- [Git smart-HTTP backend](https://git-scm.com/docs/git-http-backend)
- [Git receive-pack and non-fast-forward behavior](https://git-scm.com/docs/git-receive-pack)
- [Forgejo Actions compatibility statement](https://forgejo.org/docs/latest/user/actions/overview/)
- [`act` execution model and usage](https://nektosact.com/usage/)
- [`act` unsupported GitHub Actions behavior](https://nektosact.com/not_supported.html)
