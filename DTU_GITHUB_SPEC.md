# DTU GitHub Functional Specification

> **Status: downstream infrastructure draft.** This is not the acceptance
> specification for MergeHerder. The product behavior and black-box pass
> conditions in
> [`MERGE_HERDER_PROOF_SCENARIOS.md`](./MERGE_HERDER_PROOF_SCENARIOS.md) come
> first. This document must be reduced, reshaped, or replaced based on the
> capabilities those scenarios actually require.

## What This Program Is

DTU GitHub is a test server that behaves like the small part of GitHub used by
MergeHerder.

A test must be able to:

1. start an empty DTU GitHub instance;
2. create a GitHub user, App installation, repository, branches, and pull
   requests;
3. point a real MergeHerder process at the DTU instead of `github.com`;
4. let MergeHerder perform its normal OAuth, REST, Git fetch, Git push, and CI
   cancellation operations;
5. decide exactly when GitHub would start or finish CI and deliver webhooks;
6. deliberately deliver duplicates, redeliveries, stale events, and races; and
7. inspect the final GitHub-side state to prove what MergeHerder did.

The DTU implements only the operations listed in
[`GITHUB_SERVICE_CONTRACT.md`](./GITHUB_SERVICE_CONTRACT.md). Any other
GitHub-shaped request fails explicitly.

## The Three Participants

### The test

The test creates the GitHub universe and controls external events. It uses the
DTU test-control interface.

### MergeHerder

MergeHerder is the system under test. It uses only GitHub-shaped OAuth, REST,
Git, and webhook behavior. It must not use the DTU test-control interface.

### DTU GitHub

DTU GitHub stores GitHub-side state, serves real Git repositories, creates CI
runs, and sends signed webhooks to MergeHerder when instructed by the test.

## Starting And Stopping

The DTU must provide a command that starts one isolated instance with:

- an empty temporary data directory;
- an HTTP origin for GitHub OAuth and REST calls;
- an HTTP Git origin;
- a test-control interface;
- a webhook destination URL pointing at the MergeHerder instance;
- a webhook secret;
- a controllable initial time; and
- no users, Apps, repositories, PRs, workflow runs, or deliveries.

Startup returns the GitHub-shaped origins and a handle for the test-control
interface.

Stopping the instance terminates its server and removes its temporary data
unless the test asks to retain it for diagnosis.

The test-control interface can reset the running instance to its initial empty
state.

## Test-Control Capabilities

The DTU must provide the following capabilities to its test harness. This
specification does not decide whether the control interface is an in-process
library, a typed client talking to the service, a command protocol, or private
HTTP. That choice does not affect GitHub compatibility and can be made when the
DTU repository is implemented.

### Create a user

Input:

- numeric GitHub user ID;
- login;
- display name;
- primary email and verification state;
- avatar URL or `null`; and
- organization memberships and their states.

Effect:

- creates the identity returned by the OAuth user endpoints;
- rejects a duplicate ID or login; and
- does not create a browser session in MergeHerder.

### Choose the next OAuth result

Input:

- user ID to authorize, or an instruction to deny authorization; and
- optional OAuth error for the denial case.

Effect:

- records a one-shot result consumed by the next OAuth authorization request;
- rejects an unknown user ID; and
- automatically clears the selection after that authorization request so tests
  cannot accidentally reuse a prior identity.

### Create a GitHub App

Input:

- App ID and client ID;
- OAuth client secret;
- App public key;
- allowed OAuth callback URL; and
- webhook destination and secret.

Effect:

- registers the App identity used for OAuth and App JWT verification; and
- creates no installation or repository access by itself.

### Create an installation

Input:

- installation ID;
- App ID;
- owner login and owner type;
- repository selection;
- granted permissions; and
- selected repository IDs.

Effect:

- creates an installation that can mint installation tokens;
- enqueues an `installation` webhook;
- limits later REST and Git access to the selected repositories and granted
  permissions; and
- does not deliver the webhook until the test requests delivery.

### Create a repository

Input:

- immutable repository ID;
- owner and name;
- visibility;
- default branch name;
- installation ID; and
- optional path to a local Git bundle or repository fixture.

Effect:

- creates a real bare Git repository;
- imports the supplied real Git objects and refs, if any;
- returns the authenticated smart-HTTP clone URL;
- adds the repository to the installation selection; and
- enqueues an `installation_repositories` webhook describing the addition.

### Change repository selection

Input:

- repository IDs to add; and
- repository IDs to remove.

Effect:

- changes which repositories an installation token may access; and
- enqueues an `installation_repositories` webhook with the exact additions and
  removals.

### Create a pull request

Input:

- PR number;
- base ref;
- head ref;
- title;
- open or closed state; and
- Draft flag.

Effect:

- verifies that the base and head refs exist in the real repository;
- derives base and head SHAs from those refs;
- creates the PR returned by GitHub's targeted PR endpoint; and
- optionally enqueues a `pull_request` webhook when requested by the caller.

The first version supports only same-repository PRs.

Creating a PR in DTU GitHub does not enqueue it in MergeHerder. A consumer test
must use MergeHerder's eventual queue-submission operation or a MergeHerder-owned
test fixture to establish queue state. The DTU does not invent the queue front
door.

### Change a PR or its source branch

The test may:

- change open/closed state;
- change the base ref;
- change the Draft flag; or
- delete the PR record.

Source-head movement is performed by pushing a real commit to the head ref or by
using the control interface to point the real ref at an existing real commit
SHA. Either path changes the real ref and enqueues the corresponding `push`
event. A later PR read derives the new head SHA from the moved ref.

### Configure authoritative CI

Input:

- workflow ID;
- workflow name;
- workflow path; and
- release-ref name or ref pattern that triggers it;
- execution mode: `scripted` or `act`; and
- for `act` mode, an explicit local-execution opt-in, a pinned runner image,
  pinned container architecture, plus explicit test variables and secrets.

Effect:

- records the one workflow treated as authoritative for this repository; and
- causes every accepted push matching the configured release ref to create a
  new queued workflow run bound to the exact pushed SHA.

`scripted` mode waits for the test to choose the run outcome. `act` mode executes
the selected repository workflow using `act` and Docker.

The DTU never chooses `act` merely because a workflow exists or is authoritative
on GitHub. A repository or test must explicitly designate the selected workflow
as safe for local execution. Without that opt-in, `act` mode is rejected and the
workflow remains scripted.

### Create an auxiliary workflow run

For workflow-identity filtering scenarios, test control may create a scripted
run for a non-authoritative workflow identity at an existing real branch and
SHA. Input is repository, workflow ID/name/path, triggering event, head branch,
and exact head SHA. The SHA must resolve in the repository.

This capability does not configure a second authoritative workflow and cannot
itself change MergeHerder. It only creates the real-shaped `workflow_run` events
needed to prove that another green workflow at the same SHA is ignored.

### Control a scripted workflow run

Input is one of:

- start the queued run;
- complete it with `success`;
- complete it with `failure`;
- complete it with `cancelled`; or
- leave it in its current state.

Effect:

- updates the run status, conclusion, and timestamps;
- retains the original head branch and exact head SHA; and
- enqueues the corresponding `workflow_run` webhook.

The test may complete a run successfully or unsuccessfully even after
MergeHerder requested cancellation. That is how cancellation races are tested.

### Execute a workflow with `act`

For an `act`-backed run, the DTU:

1. creates an isolated worktree at the run's exact head SHA;
2. generates the GitHub `push` event payload for that ref and SHA;
3. invokes a pinned `act` version for the configured workflow using the pinned
   Docker runner image and container architecture;
4. supplies only explicitly configured test variables and secrets;
5. creates the run with action `requested` and status `queued`, then moves it to
   action/status `in_progress`, enqueueing both webhooks;
6. captures stdout, stderr, exit status, and wall-clock duration for diagnosis,
   while GitHub run timestamps continue to use virtual time;
7. maps a successful `act` exit to workflow conclusion `success` and an
   unsuccessful exit to `failure`; and
8. completes the run and enqueues the corresponding `workflow_run` webhook.

The workflow executes the repository's actual YAML, actions, shell commands,
and tests against the exact candidate checkout. It receives no developer GitHub
token, host credentials, or production secrets.

The caller is responsible for selecting a workflow that does not deploy,
mutate shared infrastructure, depend on protected production environments, or
require unsupported runner features. DTU GitHub does not attempt to infer that
safety from workflow YAML.

`act` execution proves that the selected workflow can run and classify the
candidate in the configured Docker environment. It does not prove GitHub-hosted
runner parity.

### Control webhook delivery

The control interface lists pending and attempted deliveries, including event,
action, GUID, repository, raw body, and current delivery status.

For a selected delivery, the test can:

- send the stored raw body to MergeHerder's configured webhook URL with GitHub's
  headers and a valid signature, recording the HTTP response;
- send it with a missing or deliberately invalid `X-Hub-Signature-256`,
  recording the response without marking the delivery validly accepted;
- redeliver the same immutable raw body with the same delivery GUID;
- create a new GUID containing the same semantic event and make it available for
  delivery; or
- record an attempted failed delivery without calling MergeHerder.

The test chooses delivery order by selecting GUIDs. No event is delivered merely
because it was created.

### Control time

Input is a duration.

Effect:

- advances OAuth-code, access-token, installation-token, and workflow
  timestamps without sleeping; and
- makes expired credentials fail subsequent requests.

### Inspect the universe

Returns enough state for assertions and diagnosis:

- users and memberships;
- Apps, installations, permissions, and token expirations;
- repository metadata and current refs;
- PR metadata and currently derived head/base SHAs;
- workflow runs, attempts, cancellation requests, statuses, and conclusions;
- pending and attempted webhook deliveries; and
- every unsupported GitHub-shaped request received.

This inspection capability is for tests and diagnosis only.

## GitHub-Shaped OAuth Behavior

### Start authorization

`GET /login/oauth/authorize`

The request includes client ID, callback URL, scopes, and opaque state. The DTU:

1. validates the registered client and callback URL;
2. consumes the one-shot user or denial configured through test control;
3. creates a short-lived, one-time authorization code;
4. records the requested OAuth scopes on the code; and
5. redirects to the callback with the code and the exact input state.

If the next result is denial, the DTU redirects with the configured OAuth error
and the exact input state and creates no code.

No fake GitHub login screen, password, MFA, or GitHub browser session is needed.

### Exchange the code

`POST /login/oauth/access_token`

A valid client, client secret, unused code, and matching callback produce a
bearer user token carrying the code's granted scopes. A code is accepted only
once and expires according to the virtual clock. Invalid requests return an
OAuth error and create no token.

### Read the authorized user

The user token authorizes:

- `GET /user`;
- `GET /user/emails`; and
- `GET /user/memberships/orgs/{org}`.

These endpoints return the configured user, email, and membership state using
the fields MergeHerder consumes. Missing, invalid, or expired tokens return
`401`. `GET /user/memberships/orgs/{org}` returns `403` for a token that lacks
organization-membership read scope; `read:user` plus `user:email` is not
sufficient. A missing organization membership returns `404`.

## GitHub App Authentication

### Mint an installation token

`POST /app/installations/{installation_id}/access_tokens`

The DTU:

1. verifies the App JWT signature against the configured public key;
2. verifies App identity and JWT time claims using virtual time;
3. verifies that the installation belongs to the App;
4. rejects requested repositories or permissions outside the installation;
5. creates an opaque installation token with an expiration; and
6. returns the token, expiration, granted permissions, and selected repository
   information expected by the client.

The token string is deliberately opaque and has no promised length or prefix.

The same token and scope rules apply to REST requests and Git smart HTTP.

## Git Smart-HTTP Behavior

Every repository is a real bare Git repository served over smart HTTP.

MergeHerder authenticates using Basic auth with username `x-access-token` and an
installation token as the password.

The DTU must support the normal Git client operations required for:

- clone/fetch of the default, source, and release refs;
- fetching real commits by reachable SHA;
- pushing a new release ref;
- fast-forwarding the release ref;
- rebuilding the release ref with a guarded force push;
- appending a repair commit to the release ref; and
- fast-forwarding the default branch to the tested release SHA.

Observable rules:

- invalid or expired credentials reject the Git request;
- a token cannot access a repository outside its installation selection;
- the Git server, not fixture metadata, determines whether objects and refs
  exist;
- a non-forced non-fast-forward update is rejected;
- an explicit `--force-with-lease=refs/heads/R:<expected-old-SHA>` push is
  rejected by normal receive-pack processing when the server ref no longer
  matches that old object ID;
- a rejected push changes no refs and creates no GitHub events; and
- an accepted push updates the real ref before it creates downstream events.

For every accepted branch push, the DTU enqueues a signed `push` webhook using
the actual old and new SHAs. A push matching the configured CI release ref also
creates a workflow run with status `queued` and enqueues its `workflow_run`
event with action `requested`.

## Pull Request API Behavior

`GET /repos/{owner}/{repo}/pulls/{number}` returns the configured PR with:

- repository and PR number;
- open/closed and Draft state;
- base ref and current base SHA;
- head ref and current head SHA; and
- the fields required by the current Octokit response type and consumed by
  MergeHerder.

The SHAs are read from the real refs at request time. Moving a source branch
therefore changes a later response without rewriting the PR fixture.

An unknown or inaccessible PR returns `404`. A closed PR remains readable with
state `closed`. Source-branch deletion is determined from the real Git ref, not
inferred solely from the retained PR head fields.

The first version does not implement general PR listing, search, comments,
labels, merge, close, or Draft mutation through GitHub's public API.

## Actions Behavior

### Create runs from release pushes

Each accepted push to the configured release ref creates one workflow run with:

- a unique run ID;
- attempt `1`;
- configured workflow identity;
- triggering event `push`;
- exact head branch and head SHA;
- initial status `queued`; and
- null conclusion.

A later release-ref push creates a different run bound to the new SHA. It does
not mutate or reuse the earlier run.

### Cancel a run

`POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel`

- An active queued or in-progress run returns `202` and records that
  cancellation was requested.
- The call does not complete the run automatically.
- A completed run returns `409`.
- A missing or unauthorized run returns the corresponding authentication or
  not-found error.

After `202`, the test decides whether the run becomes cancelled, remains active,
fails, or succeeds in scripted mode. This behavior is required to reproduce
Pause races.

For an `act`-backed run, cancellation records the request and asks the DTU's
runner supervisor to stop the `act` process and its job containers. If they stop,
the run completes as `cancelled`. Because `act` does not implement GitHub's
run-step cancellation protocol, cancellation-parity and race tests use scripted
mode and remain covered by live GitHub conformance.

## Webhook Behavior

Every webhook delivery contains:

- `X-GitHub-Delivery`;
- `X-GitHub-Event`;
- `X-Hub-Signature-256` over the exact raw body; and
- a JSON payload containing the contracted GitHub fields.

The DTU must create these event types:

- `installation`;
- `installation_repositories`;
- `pull_request` when explicitly requested by setup;
- `push`; and
- `workflow_run` with actions `requested`, `in_progress`, and `completed`.

Delivery and event creation are separate. This is non-negotiable: tests must be
able to move GitHub state forward while withholding or reordering its webhook
notifications.

Redelivery preserves raw body and GUID. A semantic duplicate uses a new GUID.

## Unsupported Behavior

The first DTU does not provide:

- a GitHub web UI;
- a general-purpose forge;
- a fully compatible GitHub-hosted Actions runtime;
- arbitrary repository or PR listing/search;
- labels, comments, Draft updates, PR merging, or branch deletion APIs;
- check runs or multi-workflow aggregation;
- workflow dispatch, rerun, logs, or jobs APIs;
- fork PRs;
- branch-protection rules beyond guarded default-branch fast-forward behavior;
- rate limiting or GitHub-scale pagination; or
- GitHub Enterprise variants.

Because same-SHA Unpause requires a fresh CI execution, proof scenario P6 is
blocked until the product selects rerun or dispatch and this downstream draft
implements that selected interaction. Its absence is explicit, not simulated by
a no-op push.

The `act`-backed mode specifically does not claim support for GitHub behaviors
that `act` itself does not implement, including runner-side step cancellation,
concurrency enforcement, job permissions, job timeouts, complete GitHub context,
OIDC, environment protection, annotations, or exact GitHub-hosted runner images.

It also does not provide MergeHerder operations such as submit-to-queue, create
a batch, Pause, unpause, Cancel, hotfix, or land. Tests invoke those through
MergeHerder; the DTU supplies only the GitHub environment in which they occur.

An unsupported GitHub-shaped request returns an explicit unsupported error and
is visible through test-state inspection.

## Required Acceptance Scenarios

### 1. Installation and repository intake

Given an App, installation, and selected repository exist, when their queued
installation webhooks are delivered, MergeHerder stores the installation and
repository exactly once. Redelivery with the same GUID does not duplicate them.
A correctly signed installation event for a different configured owner is
recorded but does not mutate installation or repository state.

### 2. OAuth login

Given an authorized configured user, when a real browser follows MergeHerder's
login flow through DTU OAuth, MergeHerder validates state, records the GitHub
identity and owner verdict, creates its own browser session, and does not retain
the user OAuth token after callback processing.

### 3. Hotfix after CI failure

Given real source branches `A`, `B`, and `C`, MergeHerder assembles and pushes
release head `R0`. The DTU creates a run for exact SHA `R0` and the test completes
it with failure. A repair commit `H` is appended without rebuilding the
contribution chain, producing `R1`. The DTU creates a different run for exact SHA
`R1`. When that run succeeds, MergeHerder fast-forwards the default branch to
exactly `R1`. A redelivery and semantic duplicate of `R0`'s actual failure
completion change nothing; the separate disorder scenario proves that a real
superseded run's late success is also inert.

### 4. Pause races with CI completion

Given CI is active for `R0`, when the operator pauses the batch, MergeHerder
calls the cancel endpoint and immediately makes the run unable to authorize
landing. The DTU returns `202` but then delivers a racing successful completion.
The default branch does not move. When the batch is unpaused, a fresh run is
required before landing.

### 5. Source branch changes while paused

Given PRs `A`, `B`, `C`, `D`, and `E` are assembled and the batch is paused, the
test moves the real source ref for `C`. A later PR read reports the new SHA.
MergeHerder retains contributions `A` and `B`, rebuilds from `C` onward, replays
the repair commits, pushes the new release head, and runs fresh CI.

### 6. Default branch moves before landing

Given the release head is green, the test advances the real default branch
before MergeHerder lands. MergeHerder's guarded fast-forward fails. The newer
default-branch commit is not overwritten and the previously green release head
is not treated as landed.

### 7. Webhook disorder and duplication

Given multiple run and push events exist, the test delivers them out of order,
redelivers one GUID, and sends a semantic duplicate with a new GUID. MergeHerder
does not perform a transition twice and only the exact current release/run may
authorize landing.

### 8. Execute the real workflow

Given an explicitly local-safe repository workflow that tests the candidate
contents, `R0` contains a real failing condition and `R1` contains its repair. In
`act` mode, the DTU runs the actual workflow at exact SHA `R0` and reports
failure, then runs the same workflow at exact SHA `R1` and reports success. The
captured run evidence names the workflow, SHA, runner image, `act` version, exit
status, container architecture, and logs.

## Where These Behaviors Are Tested

### DTU repository tests

Prove each private control operation, GitHub-shaped endpoint, Git ref effect,
workflow transition, signature, credential failure, and delivery-control
operation in isolation against the DTU process.

### MergeHerder plus DTU system tests

Run scenarios 1 and 3 through 8 using real MergeHerder server/coordinator code,
real Postgres, real Git clients and repositories, DTU HTTP endpoints, and signed
webhook delivery. Scenarios 3 through 7 use scripted CI where deterministic
control is required; scenario 8 uses `act` and Docker. These are the primary
coordinator regressions.

### Browser regression against DTU

Run scenario 2 through the real MergeHerder login and callback routes using a
browser and the DTU OAuth endpoints. This proves deterministic application OAuth
and session behavior, but it does not replace the required live-GitHub login
proof.

### Live GitHub conformance

Prove that the contracted GitHub endpoint, permission, Git transport, Actions,
and webhook semantics still match the DTU. Real interactive OAuth remains a
real-Chrome MergeHerder proof.

## GitHub Conformance Tests

The DTU repository must contain contract tests for every GitHub-shaped endpoint
and webhook it implements.

Each applicable case runs against:

1. DTU GitHub; and
2. a dedicated live GitHub sandbox.

The live test may use additional setup/inspection APIs without requiring the DTU
to implement those APIs. Setup behavior is not part of MergeHerder's contract.

The compatibility job resolves the latest applicable Octokit packages at run
time, records their exact versions and the dated GitHub API version, and runs the
same normalized assertions against DTU and live GitHub. It reports exactly which
contract IDs passed.

The valid claim is “these named GitHub behaviors still agree,” not “GitHub has
not changed.”

Interactive live OAuth remains a separate real-Chrome MergeHerder proof unless
a safe automated live user is deliberately designed.

The DTU's `act` mode is also compared with a deliberately small live workflow
case using the same workflow and candidate content. Differences caused by
unsupported `act` behavior are recorded as fidelity limits rather than hidden by
the DTU.
