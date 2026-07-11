# Merge Queue Domain Seams Worklog

goal: Codify the current MergeHerder product intent, queue and batch definitions, Pause/Cancel semantics, first failure-remediation seam, and regression-harness direction without prematurely expanding them into a complete business-logic or UI specification.

worktree: `/Users/tyler/src/merge-herder` root checkout. This is a design-only continuation against live dirty state; an isolated worktree would not contain that state. No product code is being changed.

skill_use: session-worklog source=pagerguild/core-tools -> the user explicitly asked that decisions, clarifications, and corrections be captured as worklog material.

context_rule: Existing code and documentation are evidence, not authority. A claimed invariant may be important or may be agent-written overreach; each must be justified against the intended software behavior.

## Original Product Intent

problem: Agent-authored PRs spend disproportionate time and inference shepherding repeated rebases and CI runs as `main` advances. With many concurrent PRs, repeated individual rebase/CI cycles produce avoidable multiplicative work and compute expense.

goal: Gate integration/deployment so only one candidate uses the repository's integration/CI position at a time. PRs that accumulate while that position is occupied become the next batch, letting their combined result pay for one authoritative CI run when the happy path succeeds.

scope: This is a small self-hosted/open-source tool for the user's own use, potentially shared under an open-source license. The target is a credible happy path plus limited retry/repair and manual escape hatches, not a fully generalized commercial merge platform.

working_happy_path:

1. Collect the PR branches assigned to the batch in a deterministic order.
2. Build one completely squashed/rebased contribution per PR on the release branch `R`.
3. Use a coding-agent integration session to assemble the chain and resolve rebase conflicts where necessary.
4. Run the repository's authoritative CI against the complete `R` head.
5. If successful and still current, fast-forward the exact tested commit chain onto the default branch.

invariant_candidate: The SHA authorized to move the default branch must be the exact release head tested successfully by CI; do not reconstruct a semantically similar merge after CI.

working_assumption: The single integration/CI position is per repository. Whether any deployment environment later requires a broader cross-repository mutex has not been established.

open_question: The exact front-door operation that submits a PR for queueing has not been designed. No GitHub label, UI action, API route, or agent command should be treated as settled merely because an older document proposed it.

open_question: The exact batch-formation trigger remains to be settled. The intended behavior is that work accumulating during an active integration/CI period can coalesce into the next batch; an explicit wait timer, cooldown after idle arrival, and batch-size cap have not been accepted.

## Automation And Human Interaction Constraints

decision: Most normal activity should be automatic: batch formation, agentic assembly, CI dispatch/correlation, successful landing, and progression to later waiting work.

correction: GitHub is infrastructure and an event/API provider, not an accepted primary product interface. Requiring users to navigate to GitHub and manipulate coded labels to operate MergeHerder was explicitly rejected.

correction: The old `queue`, `needs-agent`, and `needs-human` label vocabulary and PR-comment control plane are not accepted product decisions.

decision: The current operator controls being designed are Pause and Cancel. Their semantics are application behavior; the eventual UI/API/notification surface is secondary and intentionally not fully designed yet.

correction: "Take over a repository" was rejected as an inflated and misleading concept. Pause retains the current batch; Cancel terminates it. There is no general repository-takeover workflow.

correction: A cancelled batch does not have a generic Resume operation. Any retained or repaired PRs return through the normal submission entrance as new queue work. Unpause applies only to a batch that remained current while paused.

future_consideration: Manual handling may eventually include fixing the current batch as a whole, decomposing it into individual PRs or subsets for diagnosis, removing a single member, abandoning members, or resubmitting repaired work. Only direct fix-forward hotfixing on `R` plus whole-batch Cancel is required for the first working failure path.

notification_requirement: MergeHerder must be able to tell humans when automation cannot make safe progress. Slack is a plausible first delivery channel, but the notification transport and interaction design are not settled. Routine automatic recovery should not produce noisy alerts; human-required trouble should record concrete batch, phase, CI, PR, and SHA context before delivery.

open_question: Which conditions transition from automatic repair into human attention, how many hotfix attempts are allowed, and whether an alert merely informs or also offers controls remain unspecified.

## Current Application State Relevant To The Design

source_discovery: The application already has GitHub login, verified webhook intake, idempotent webhook-delivery storage, and database-backed GitHub installation/repository synchronization.

source_discovery: The merge coordinator itself is not implemented. `src/lib/server/workflows/push-webhook-workflow.ts` remains a stub, so historical queue behavior is design material rather than hidden existing production logic.

scope_guard: Repository and webhook listing should continue to use local database state populated by verified webhooks. Queue implementation is not an invitation to reintroduce unrelated request-time GitHub reads, owner-policy layers, or generalized SDK abstractions.

## Rejected Or Superseded Design Claims

rejected: A label-driven GitHub control plane as the main way people operate MergeHerder.

rejected: Treating the old historical handoff as a finalized architecture merely because it labels decisions as final.

rejected: A separate preservation operation when Pause or Cancel occurs; the database record already describes the batch.

rejected: Forbidding direct commits on `R`. Batch-level repair/hotfix commits are a first-class and initially preferred failure-remediation mechanism.

rejected: Requiring all batch-level fixes to be projected back into constituent PR branches before the repaired batch can land.

deferred: Automatic culprit diagnosis, binary decomposition, reconciliation PRs, conflicts-pair tracking, autonomous source-branch fixing, runner failover topology, multi-workflow aggregation, speculative trains, and rich administrative workflow UI.

not_decided: Automatically converting PRs to GitHub Draft when a batch is cancelled. Current recommendation is to leave PR state unchanged because cancellation of a combination does not prove every member is unready.

## Established Definitions And Corrections

correction: A PR cannot simultaneously be waiting in the queue and be a member of the current batch. Forming a batch claims/removes its PRs from the waiting queue for the lifetime of that batch.

correction: Withdrawing or cancelling a batch does not require a separate preservation operation. The batch and its contents already have durable database records; the state transition changes how those records may be acted upon.

decision: The operator controls under discussion are `pause` and `cancel`.

decision: A paused batch remains the current batch and continues to occupy the repository's single integration/CI position. Waiting PRs remain queued and no later batch starts.

decision: Pressing Pause makes the current batch ineligible to land and stops its CI run if one is underway. A paused CI run is not allowed to finish and later authorize landing.

decision: A cancelled batch stops being current and releases the integration/CI position, allowing the PRs already waiting in the queue to form the next batch.

decision: A batch has an ordered list of source PR branches and a derived release branch, provisionally named `R`.

decision: `R` starts from a recorded default-branch/base commit. Each source branch contributes exactly one squashed commit to `R`, in batch order.

decision: `R` may also contain zero or more batch-level repair commits above the ordered source-branch contribution commits. These commits are the direct path for fixing an integration-only failure, repairing the CI pipeline in the batch, or adding a small hotfix without first decomposing the batch.

decision: The minimum member identity is not only a branch name. Each ordered member needs its PR identity, source ref, observed source-head SHA, and the corresponding commit SHA produced on `R`; these values are necessary for reproducibility and suffix rebuilding.

decision: The batch database record is the durable description of these Git artifacts and their relationships. The Git branches and commits do not replace the database record.

decision: To distinguish the contribution chain from later batch-level repairs, the batch records the tip of the assembled contribution chain separately from the current head of `R`. The intervening Git commit range is the repair series; a second per-commit metadata model is not required merely to replay it.

## Current Assembly Model

working_definition: For ordered source branches `A, B, C, D, E`, construct `R` by applying and squashing `A` onto the base of `R`; rebasing/applying `B` onto the new tip and squashing it as the next commit; then repeating for `C`, `D`, and `E`. Original source branches are not rewritten by this assembly.

working_definition: On unpause, compare the current default-branch head and current source-branch heads with the SHAs recorded for the batch. If the base has not moved and `C` is the earliest changed source, retain the derived commits for `A` and `B`, reset/rebuild `R` from the point before `C`, and repeat assembly for `C` onward. If the base moved, the entire release chain is invalid and must be rebuilt from `A`.

working_definition: After rebuilding any invalid contribution suffix, replay the batch-level repair commits from the prior `R` onto the new contribution-chain tip. The existing agentic rebase handles conflicts during that replay. If neither the base nor a source head moved, preserve the contribution chain and repair commits as they are.

## First Supported Failure Remediation

decision: The initial implementation may support only fix-forward remediation on the current release branch `R`. After CI fails, append a repair/hotfix commit to `R` and run CI again against the new head.

decision: Fix-forward is preferred when possible because it retains the already assembled and conflict-resolved contribution chain. The batch does not need to be decomposed, its source branches do not need to be repacked, and the exact repaired chain tested by CI can land.

decision: Automatic decomposition, individual-member diagnosis, member removal, and projection of repairs back into source PR branches are not required for the first working failure path. If the batch cannot be repaired by appending commits to `R`, cancellation is the available escape hatch.

## Regression Harness Direction

problem: The coordinator is an asynchronous distributed state machine. Low-level mocks are unlikely to expose stale webhooks, duplicate completions, ref races, pause/cancel races, or mismatches between the tested and landed release head. Running every regression against GitHub would be slow and nondeterministic.

proposal: Build a deliberately partial GitHub digital-twin universe (DTU) around the exact surfaces MergeHerder consumes. Grow it scenario-by-scenario; do not attempt to reproduce GitHub generally.

decision: The GitHub DTU is a separate repository. That repository owns the partial GitHub implementation, its own unit tests, and the GitHub-versus-twin conformance suite. MergeHerder consumes a released/packageable twin and retains only MergeHerder-specific behavior scenarios.

proposal: Keep Git real. DTU scenarios should use actual local bare repositories, branches, commits, rebases, squashes, resets, and fast-forward checks. The twin supplies only GitHub's control-plane behavior around that Git repository.

proposal: Run real MergeHerder coordinator code against a real isolated Postgres database. The DTU provides deterministic pull-request records, GitHub REST responses for the endpoints actually called, workflow-run state, signed webhook deliveries, controllable virtual time, and deliberate duplicate/out-of-order/late events.

proposal: Exercise the production HTTP/webhook shapes where practical rather than replacing the coordinator's GitHub calls with method-level mocks. A test-configured GitHub API base URL plus signed callbacks to the real webhook route is sufficient; a generalized GitHub SDK abstraction is not required.

proposal: Separate deterministic orchestration regressions from agent-quality evaluation. In the deterministic DTU scenario, a scripted repair worker appends a known real Git commit to `R`. A slower real-agent evaluation may separately prove that a coding agent can discover and author such a repair.

proposal: Retain a thin real-GitHub contract/canary proof for API compatibility, App permissions, actual Actions triggering/cancellation, webhook payloads, and branch-protection behavior. The DTU is the main regression suite, not evidence that GitHub itself behaves exactly like the twin.

decision: The DTU repository's live GitHub conformance run resolves and executes with the latest available Octokit packages at run time and records the exact resolved versions. Its focused contract cases run against both a dedicated live-GitHub sandbox and the DTU, asserting the same normalized semantics for the surfaces implemented by the twin.

decision: Latest Octokit and GitHub REST API version are separate compatibility axes. Octokit endpoint methods are generated from GitHub's OpenAPI specification, while requests select a dated GitHub API version. The conformance repository targets the newest supported GitHub API version and may additionally run the pinned Octokit/API-version combination used by MergeHerder so consumer compatibility is not inferred from latest-only success.

terminology: Small tests that make live GitHub calls are contract/integration tests even when written with unit-test-sized cases. They prove only the GitHub surface modeled by the DTU, not that all of GitHub is unchanged.

first_scenario: Given a real local repository with base `M`, source branches `A`, `B`, and `C`, and assembled release head `R0`; when the twin reports CI failure for `R0`, a repair commit `H` is appended without rebuilding the contribution chain to produce `R1`, and the twin reports success for `R1`; then only `R1` may fast-forward `main`, a late or duplicate completion for `R0` can never land, and the database records the exact tested and landed release head.

companion_pause_scenario: Given CI is active for a current batch, Pause makes the batch immediately ineligible to land and requests asynchronous cancellation. Even if a racing success arrives for that run, the batch remains paused and does not land. Unpause requires a fresh authoritative run before landing is possible.

coverage_split: Pure state/invariant and generated-event tests cover transition combinations; DTU system tests cover database plus real Git plus HTTP/webhook orchestration; a small real-GitHub canary covers external contract drift; browser tests are reserved for later human-facing Pause/Cancel interactions rather than the merge engine itself.

## Cancel Semantics Under Discussion

proposal: Cancelling a batch should leave the GitHub PRs themselves unchanged rather than automatically converting them to Draft. Batch cancellation says that this integration attempt has ended; it does not necessarily mean every member PR is intrinsically unready.

proposal: Members of a cancelled batch should not automatically re-enter the waiting queue. They become eligible for later explicit resubmission through the normal entrance after the user or an agent fixes, discards, or selects them.

## Open Seams

open_question: What is the exact operation for removing one member from a paused/current batch? Removing member `C` implies rebuilding the suffix beginning at `C`, but the user-facing and state semantics are not yet decided.

open_question: What happens on unpause if a source branch was deleted, its PR was closed, or its base/identity changed rather than merely advancing to a new head?

open_question: What exact successful-landing cleanup is required for the originating PRs and their branches? Fast-forwarding synthesized commits may not make GitHub represent the PRs as merged automatically, but closing/commenting/deleting behavior is not yet accepted.

open_question: What exact repository configuration identifies the authoritative CI workflow and its timeout? Multi-workflow aggregation is deferred, but the minimal single-workflow contract is not yet frozen.

open_question: Is `R` a fixed long-lived branch per repository or a per-batch branch? Historical notes favored a fixed branch for stable preview infrastructure, but this has not been re-ratified in the current design conversation.

open_question: What ordering rule selects source branches within a batch, and may users or agents express dependencies or reorder members? The batch is ordered, but the ordering source is not settled.

open_question: When a coding agent assembles or repairs `R`, what minimal input/output contract and authority boundary does it receive? Deterministic regressions should not depend on live model reasoning, but production agent execution still needs a concrete carrier.

handoff: DTU-focused restart document created at `/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/handoff-XXXXXX.md.zwud5nHfpo`.

proof: Audited this worklog against the full restart handoff and current conversation. It now includes the original product problem and happy path, automation and human-intervention constraints, rejected/superseded ideas, current application implementation state, settled Git/batch/Pause/Cancel decisions, hotfix-first remediation, DTU repository and conformance direction, and intentionally unresolved application seams.

status: Application-specific considerations from the design conversation are serialized here. Fine-grained state transitions, front-door API/UI behavior, notification transport, and full failure logic intentionally remain unspecified pending seam decisions.

## GitHub Service Contract And DTU Realism Pass

goal_update: Produce an honest current-and-intended inventory of MergeHerder's GitHub dependencies before deciding how much of GitHub the separate DTU repository must implement.

skill_use: zoom-out source=pagerguild/core-tools -> mapped GitHub's four roles and the current callers before choosing a DTU boundary.

skill_use: e2e-coverage-triage source=pagerguild/core-tools -> separated deterministic coordinator system proof, live GitHub conformance, and later browser-only operator proof.

source_discovery: The current application uses GitHub as an OAuth identity provider, performs narrow user/membership REST reads during login, accepts signed GitHub App webhooks, and renders GitHub installation/configuration links. It does not yet mint installation tokens, perform Git operations as the App, read PRs, or control Actions. The push coordinator remains stubbed.

source_discovery: Better Auth `1.4.22` hard-codes the GitHub authorize, token, `/user`, and `/user/emails` URLs. A local OAuth twin therefore requires a production-and-test provider endpoint seam or must remain a live-only canary initially; DNS/TLS interception is rejected as test infrastructure.

decision: Added `GITHUB_SERVICE_CONTRACT.md` at the repository root as the living inventory of GitHub roles, current calls, required first-coordinator capabilities, proposed/deferred surfaces, semantic claims, and proof ownership.

decision: Every GitHub dependency must have a falsifiable semantic claim. The DTU and live sandbox run the same normalized contract assertion against that claim; neither endpoint presence nor broad schema imitation is sufficient.

decision: Prefer real bare Git repositories served by Git's `git-http-backend` over using Forgejo/Gitea as the DTU foundation. Git itself supplies authentic object, ancestry, fetch, push, and fast-forward behavior. A forge would still require translation of GitHub-specific App, webhook, PR, and Actions semantics and would add unrelated nondeterminism and maintenance surface.

decision: The DTU exposes a narrow GitHub-shaped plane used by MergeHerder and a separate scenario-control plane used only by tests. Test conveniences must not leak into the public simulated contract.

decision: DTU realism means real Git objects/refs, real HTTP envelopes and signatures, production MergeHerder code, and real isolated Postgres. OAuth identities, installation tokens, PR metadata, Actions state, webhook scheduling, and time are deterministic simulations constrained by the service contract.

decision: The first DTU does not execute a general Actions runner. Pushing `R` creates a run bound to the real pushed SHA; the scenario driver controls asynchronous run/cancel/completion behavior and signed delivery timing.

correction: Accepted Pause semantics require `POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel`, which requires Actions write permission. The older `docs/github_app_configuration.md` claim that Actions read-only is sufficient for v1 is now stale.

correction: Reconciled `docs/github_app_configuration.md` with the current contract: Actions write is required for Pause cancellation, label control-plane claims are rejected, and the release branch is `R` without prematurely choosing fixed versus per-batch naming.

correction: Removed old App permissions and event subscriptions that only served rejected/deferred behavior: Issues write, Pull Requests write, Checks read, Check Run/Suite, Create/Delete, Workflow Dispatch, and the fixed `mq/candidate` name. The configuration draft now keeps Metadata read, Contents write, targeted PR read, Actions write for Pause, and Installation Target/Pull Request/Push/Workflow Run events; deferred capabilities are named separately rather than silently granted.

decision: The first coordinator is expected to start CI by pushing `R`, not by `workflow_dispatch`. Workflow dispatch, rerun, force-cancel, check aggregation, PR cleanup, label workflows, and missed-webhook recovery remain proposed/deferred until application behavior requires them.

open_question: The queue front door remains unresolved. A targeted PR snapshot is required to bind and revalidate member identity/head/base/state, but whether it is sourced from `pull_request` webhooks, targeted REST reads, or both is not settled. General PR listing is not justified.

external_resource: GitHub official App authentication docs -> installation tokens are time-limited, scoped, opaque, usable for authenticated Git smart HTTP, and newly minted formats must not be assumed to be fixed-length.

external_resource: GitHub official Actions run API -> cancel is an asynchronous accepted operation and may return conflict for a run that cannot be cancelled; Actions write permission is required.

external_resource: Git official `git-http-backend` and `git-receive-pack` docs -> Git already supplies smart HTTP fetch/push and authentic fast-forward/non-fast-forward behavior suitable for the DTU's Git plane.

external_resource: Forgejo official Actions docs -> Forgejo Actions is familiar to GitHub Actions users but explicitly is not designed to be compatible, weakening it as a semantic base for a GitHub twin.

proof: Reviewed the new contract against current auth, owner authorization, webhook receiver, webhook route, GitHub App page-model code, dependency implementation, migrations, historical queue notes, and current official GitHub/Git/Forgejo documentation. No product source code was changed.

skill_use: repo-proof-policy source=pagerguild/core-tools -> selected documentation-only proof and verified that the durable contract is source-backed and linked from the relevant configuration draft.

proof: `vp fmt --write GITHUB_SERVICE_CONTRACT.md docs/github_app_configuration.md ephemeral/worklog/202607101222-merge-queue-domain-seams.md` -> formatted the three design artifacts.

proof: `vp fmt --check GITHUB_SERVICE_CONTRACT.md docs/github_app_configuration.md ephemeral/worklog/202607101222-merge-queue-domain-seams.md` -> all matched files use the correct format.

proof: `git diff --check` plus no-index whitespace checks for the two untracked design files -> no whitespace errors.

proof: Focused `rg` audit tied `AUTH-01` through `AUTH-06` to Better Auth `1.4.22` and `src/lib/server/github-owner.ts`; tied current webhook envelope/dispatch claims to `src/lib/server/github-webhooks.ts` and the webhook route; and confirmed the configuration draft links to the root contract and no longer presents labels or a fixed candidate ref as decisions.

proof_note: `vp check` is not a suitable targeted Markdown-only gate in this repository: its format phase passed, then the lint phase reported that no lintable files were found. `vp fmt --check` is the focused successful documentation gate. Application tests and real-browser proof were intentionally not run because no application behavior changed.

final_status: The GitHub dependency inventory, semantic contract, Git-first DTU recommendation, real-versus-simulated boundary, latest-Octokit/live conformance model, OAuth testability seam, first hotfix scenario, companion Pause race scenario, and all resulting MergeHerder-specific corrections are durable. No DTU repository or coordinator implementation was started.

## Better Auth Boundary Reconsideration

correction: The user is already dissatisfied that Better Auth introduced Kysely and `kysely-postgres-js` instead of allowing application authentication persistence to use the project's SQLc data-access layer. The DTU OAuth discovery adds a second concern: Better Auth's built-in GitHub provider hard-codes GitHub's OAuth and user-info endpoints.

source_discovery: Better Auth currently owns OAuth initiation and callback handling, users/accounts/sessions/verifications persistence, OAuth-token encryption, signed cookies, session lookup/refresh, and sign-out. Application coupling exists in `src/lib/server/auth.ts`, `src/lib/server/auth/better-auth-db.ts`, `src/hooks.server.ts`, the callback route, login/sign-out remote forms, `src/app.d.ts`, the Better Auth demo routes, the auth migration tables, and GitHub OAuth-token decryption in `src/lib/server/github-owner.ts`.

source_discovery: Official Better Auth documentation offers a custom database adapter, stateless sessions, and a generic OAuth plugin. None is a clean fit: a custom adapter would require implementing Better Auth's generic create/update/find/delete query model over SQLc; stateless sessions weaken direct revocation/persistence and do not fix the built-in provider endpoints; generic OAuth fixes endpoint configuration but retains the unwanted persistence/session abstraction.

design_lean: Replace Better Auth before building the coordinator, subject to a focused authentication contract and regression plan. This is not yet authorization to implement the replacement.

reasoning: MergeHerder needs one GitHub login provider, one persisted GitHub identity/owner verdict, and one opaque revocable browser session. It does not need account linking, passwords, multiple providers, verification flows, or a general authentication framework.

design_simplification: The human GitHub OAuth token appears unnecessary after the callback. Use it only to load/authorize the GitHub identity, then discard it. Coordinator automation uses separate GitHub App installation tokens. Eliminating persisted user OAuth tokens removes the `accounts` table, application token encryption/decryption, and a large part of the Better Auth coupling unless a later accepted user-attributed GitHub action requires retention.

proposed_replacement_boundary: Application-owned SQLc users/sessions; configured OAuth authorize/token/user/email/membership endpoints shared by production and DTU; unpredictable state bound to a short-lived secure cookie; opaque high-entropy browser session cookie; server stores only the session-token hash; explicit expiry/revocation; login-time GitHub identity and owner verdict persistence; origin-safe POST sign-out.

security_guard: Replacing Better Auth is not permission to improvise security behavior. The replacement must prove OAuth state validation and one-time code handling, secure cookie attributes, session fixation prevention, expiry/revocation, hashed token storage, callback error handling, and real GitHub login in Chrome.

external_resource: Better Auth official database/custom-adapter docs -> escaping Kysely is possible only by taking responsibility for its generic adapter contract, which is disproportionate for this single-provider application.

external_resource: Better Auth official generic OAuth/stateless-session docs -> these can separately address configurable endpoints or database removal, but do not produce the desired application-owned SQLc plus configurable-provider boundary together.

external_resource: Arctic official documentation -> a small OAuth authorization-code client is a plausible protocol helper and supports GitHub plus a generic OAuth client, but its provider/configuration behavior and release policy need a focused evaluation before selection.

decision: Updated `GITHUB_SERVICE_CONTRACT.md` to state that Better Auth is a current implementation rather than part of the GitHub contract, and to record the non-negotiable boundary any retained or replacement auth implementation must satisfy.

memory_lookup: Prior MergeHerder reconciliation confirms that the durable behavior is login-time GitHub owner authorization with a persisted local verdict and no request-time GitHub reads. Better Auth's hook was previously a useful lifecycle location, but Better Auth itself was never established as the product invariant.

skill_use: repo-proof-policy source=pagerguild/core-tools -> verified the documentation-only amendment against current auth code, migrations, dependency source, and official Better Auth/Arctic documentation.

proof: `vp fmt --check GITHUB_SERVICE_CONTRACT.md ephemeral/worklog/202607101222-merge-queue-domain-seams.md` -> both amended design files use the correct format; `git diff --check` and no-index checks report no whitespace errors.

status: Better Auth remains installed and active. Replacement is the recommended design lean, not an implemented or irrevocably settled decision.

## DTU Repository Charter

coordination: The user confirmed the Better Auth replacement is proceeding in a separate worktree. This design stream will not touch auth code, migrations, dependencies, or that worktree's implementation decisions; it consumes only the configurable OAuth/service boundary once available.

decision: Added `DTU_GITHUB_CHARTER.md` as the concrete boundary for the separate DTU repository. `GITHUB_SERVICE_CONTRACT.md` remains MergeHerder's consumer-owned inventory; the DTU repo owns the partial service, scenario controls, target-neutral conformance cases, latest-Octokit runs, and released executable/control client.

decision: Dependency direction is one-way: DTU GitHub does not import MergeHerder or encode batch states. MergeHerder consumes a released DTU for tests and keeps its own business scenarios.

decision: Each DTU release declares the GitHub service-contract IDs it implements. MergeHerder pins a release covering its required IDs. A new claim originates in MergeHerder, then gains DTU implementation/conformance before consumption; no generalized contract registry is needed.

proposal: Package the DTU as a local executable service plus typed scenario client. Each test instance uses loopback ephemeral ports and a temporary root; a container is deferred until a language-neutral consumer requires it.

decision: The DTU has three boundaries: GitHub-shaped OAuth/REST, authenticated Git smart HTTP, and a loopback-only private scenario-control interface. MergeHerder production code never calls the scenario interface.

decision: V0 uses real bare Git repositories on disk and may keep GitHub metadata in memory. DTU process-restart durability is not required to test MergeHerder restarts; persistence is added only for a consumer scenario that needs the twin itself to restart.

decision: Git smart HTTP uses GitHub's installation-token Basic-auth shape (`x-access-token` username, token password). App JWT generation/verification is cryptographically real against a test key and time claims; opaque installation-token contents remain simulated while repository/permission scope and expiry are enforced across REST and Git.

decision: Git push is the causal boundary for CI: Git updates the ref first; post-receive integration records the transition, creates the exact-SHA workflow run, and enqueues eligible webhooks. Rejected pushes produce no downstream state.

decision: PR fixture SHAs derive from real refs rather than unrelated fixture strings. Same-repository PRs are first; fork PRs wait for an explicit MergeHerder fork policy.

decision: CI cancellation is modeled as a request, not an immediate terminal transition. Scenario control may complete cancellation, leave it stuck, or race it with success/failure.

decision: Webhook event creation and delivery are separate. Scenario controls can select, delay, fail, redeliver with the same GUID/body, duplicate semantically, reorder, or deliver stale events without wall-clock sleeps.

decision: The private control vocabulary expresses GitHub facts/events, never `createBatch`, `pauseBatch`, `cancelBatch`, or `landBatch`. Those remain MergeHerder operations.

decision: Unknown GitHub-shaped endpoints fail explicitly and are recorded. The DTU does not silently accept contract growth or pad responses with cosmetically realistic unused data.

decision: Live conformance setup may use extra GitHub administration/observation APIs without adding them to MergeHerder's contract or requiring the DTU to implement them.

decision: Latest-Octokit conformance is a separate runtime-resolved compatibility job alongside reproducible pinned-client tests. It records exact package and dated API versions and only claims agreement for named contract IDs.

decision: First released scope covers real Git/App token scope, signed webhooks, installation/repository/push intake, same-repo PR snapshots, release/default ref updates, one push-triggered workflow, workflow-run lifecycle/cancel, and adversarial delivery scheduling. OAuth joins once the separate replacement worktree supplies configurable endpoints.

scenario_priority: Hotfix and Pause-race scenarios gate the first release. Source-head movement, stale default-branch landing, and delivery-idempotency scenarios are the next fidelity targets rather than prerequisites for implementing every coordinator seam.

correction: Late and duplicate `R0` completions remain part of the first hotfix scenario. The later delivery-idempotency scenario broadens that proof to installation, repository, push, and workflow events rather than postponing exact-SHA duplicate protection.

open_question: Final repo/package name, implementation language, scenario transport, in-memory versus embedded v0 metadata, `R` naming, `PR-01` endpoint source, live webhook capture, and exact OAuth release timing remain deliberately open.

skill_use: repo-proof-policy source=pagerguild/core-tools -> selected documentation-only proof for the new root charter and its bidirectional routing from the GitHub service contract.

proof: `vp fmt --check DTU_GITHUB_CHARTER.md GITHUB_SERVICE_CONTRACT.md ephemeral/worklog/202607101222-merge-queue-domain-seams.md` -> all charter/contract/worklog files use the repository format.

proof: `git diff --check` plus no-index whitespace checks for untracked design files -> no whitespace errors.

proof: Focused semantic audit confirmed separate-repo/one-way dependency language, three runtime boundaries, real `git-http-backend`, `x-access-token` Git auth, asynchronous cancellation, latest-Octokit matrix, prioritized scenarios, open decisions, and bidirectional charter/contract links.

proof_note: An initial negative `rg` check for forbidden batch-control operations produced a false positive because the charter explicitly says those operations do not exist and the formatted sentence wraps across lines. Manual inspection confirmed the names appear only in that prohibition.

status: DTU repository scaffolding and implementation remain unstarted. The charter is ready for review of its open decisions before a separate repository is created.

## DTU Charter Rejected And Replaced

correction: The user rejected `DTU_GITHUB_CHARTER.md` because it described ownership, packaging, and fidelity philosophy without concretely stating what the program must do. That criticism is correct. The source material was concrete GitHub behavior, but the generated output was an architecture charter.

skill_use: customer-feedback-triage source=pagerguild/core-tools -> compared the complaint to the conversation's concrete source truth and fixed the generated design artifact rather than defending or merely rewording the charter.

source_truth: DTU GitHub must be a controllable fake GitHub server: tests create users/installations/repos/PRs, MergeHerder uses normal OAuth/REST/Git/cancel calls, tests decide CI/webhook timing and races, and final GitHub-side state is inspectable.

decision: Deleted `DTU_GITHUB_CHARTER.md` and replaced it with `DTU_GITHUB_SPEC.md`, organized around startup, exact private control operations, GitHub-facing endpoints, automatic state effects, webhook delivery controls, unsupported behavior, and executable acceptance scenarios.

decision: The private scenario surface is now concretely specified as loopback JSON HTTP under `/_dtu`, with reset, user/App/installation/repository/PR/workflow setup, run transitions, delivery/redelivery/duplicate/failure controls, virtual time, and state inspection.

decision: OAuth identity/denial selection is a one-shot `POST /_dtu/oauth/next-authorization` operation consumed and cleared by the next authorize request; the DTU no longer relies on an undefined "configured user."

decision: The spec now states the exact GitHub-facing behavior required for OAuth authorize/token/user/email/membership, App installation-token minting, authenticated Git smart HTTP, targeted PR reads, Actions cancellation, and signed installation/repository/PR/push/workflow-run webhooks.

decision: The spec now defines causal effects: accepted Git pushes update real refs before enqueueing push events; release pushes create distinct exact-SHA workflow runs; cancellation records a request without forcing a terminal result; and webhook creation is independent of delivery.

decision: Seven concrete acceptance scenarios define what the DTU must enable MergeHerder to prove: installation intake, OAuth login, hotfix, Pause race, source movement while paused, stale-default landing, and webhook disorder/duplication.

skill_use: e2e-coverage-triage source=pagerguild/core-tools -> assigned concrete behaviors to DTU endpoint tests, MergeHerder-plus-DTU system tests, deterministic browser OAuth regression, and thin live-GitHub conformance instead of treating every scenario as the same kind of E2E test.

decision: Coordinator scenarios use real MergeHerder, Postgres, Git, HTTP, and signed webhooks against DTU. Browser-plus-DTU covers deterministic login integration but never replaces the repository's required real-Chrome/real-GitHub login proof.

correction: Repository packaging, implementation language, embedded-state choice, and generalized capability-registry discussion were removed from the primary DTU specification because they did not answer what the software must do.

boundary: Creating a PR in DTU GitHub does not enqueue it in MergeHerder. Until the product front door is defined, MergeHerder system tests establish queue state through a MergeHerder-owned fixture. The DTU implements no submit/batch/Pause/Cancel/hotfix/land operations.

skill_use: repo-proof-policy source=pagerguild/core-tools -> verified the replacement functional spec is source-backed, formatted, linked from the service contract, and contains no active reference to the rejected charter.

proof: `vp fmt --check DTU_GITHUB_SPEC.md GITHUB_SERVICE_CONTRACT.md ephemeral/worklog/202607101222-merge-queue-domain-seams.md` -> all changed design artifacts use the correct format.

proof: `git diff --check` plus no-index checks for untracked design files -> no whitespace errors.

proof: Focused operation audit found every required private control operation, OAuth endpoint, installation-token endpoint, targeted PR read, Actions cancel endpoint, and all seven acceptance scenarios in `DTU_GITHUB_SPEC.md`; `DTU_GITHUB_CHARTER.md` no longer exists or appears in active contract/spec files.

status: The DTU artifact now specifies concrete commands, inputs, effects, GitHub-facing behavior, event causality, failure behavior, and acceptance scenarios. No DTU implementation or MergeHerder application code was changed.

## Invented `/_dtu` Endpoints Removed

correction: The user challenged the `/_dtu` endpoint namespace. It had no source in GitHub or the product design; it was an invented private HTTP transport and URL convention introduced while trying to make the test controls concrete.

skill_use: customer-feedback-triage source=pagerguild/core-tools -> the complaint correctly identified an output-generation error: required control capabilities had been conflated with an unjustified transport and endpoint design.

decision: Removed every `/_dtu` path and the assumption that scenario control is JSON HTTP. `DTU_GITHUB_SPEC.md` now specifies only the required test-control capabilities and their inputs/effects. In-process library, typed service client, command protocol, or private HTTP remains an implementation choice for the separate repository.

boundary: GitHub-shaped OAuth/REST/Git endpoints remain concrete because compatibility with GitHub requires them. Test-control transport is not GitHub-facing and therefore is not prematurely specified.

correction: Removed leftover transport wording (`control API`, inspection `endpoint`) and split App creation from installation creation so each test-control capability has one clear input/effect boundary.

proof: `vp fmt --check` and whitespace checks pass; focused search confirms no `/_dtu`, private-control-API, JSON-transport, or administrative-token assumption remains in active DTU contract/spec files.

## `act`-Backed Workflow Execution

correction: The earlier categorical statement that the DTU should not execute workflow YAML was too strong. The user correctly challenged whether a Docker-based `act` runtime would materially improve fidelity.

source_discovery: Current `act` reads real `.github/workflows` files, accepts event payloads, resolves job dependency paths, and uses Docker runner containers. It can therefore execute repository workflow code against an exact candidate checkout and produce a real process result.

source_discovery: Official `act` documentation explicitly says it is not completely compatible with GitHub runners. Unsupported behavior includes run-step cancellation, concurrency, job permissions, job timeouts, complete GitHub context, OIDC, environment scoping, annotations, and exact hosted-runner images. Default runner images are intentionally incomplete.

design_correction: DTU CI has two modes behind the same GitHub workflow-run model. `scripted` mode remains required for deterministic outcomes, late events, and cancellation races. `act` mode runs the actual selected workflow against the exact SHA in a pinned Docker runner image and maps the supervised process result into the workflow-run conclusion.

decision: `act`-backed execution is required for a focused workflow-fidelity scenario, not for every coordinator regression. Scripted mode remains the correctness oracle for GitHub control-plane races that `act` cannot represent.

decision: An `act` run records workflow, SHA, pinned `act` version, pinned runner image, exit status, logs, and duration. It receives only explicit test variables/secrets and no developer GitHub token, host credentials, or production secrets.

decision: Cancelling an `act` run asks the runner supervisor to stop the `act` process/job containers, but this is not treated as GitHub cancellation parity. The Pause race remains scripted and live-GitHub conformance covers real cancellation semantics.

scenario_added: A real-workflow scenario makes `R0` fail and repaired `R1` pass by executing the repository's actual workflow through `act`; a small live workflow using the same candidate content measures the known fidelity gap.

external_resource: Official `act` usage/runner docs -> real workflow/event/Docker execution is useful, while runner images and runtime behavior intentionally differ from GitHub-hosted Actions.

external_resource: Official `act` unsupported-functionality docs -> control-plane/cancellation/concurrency/permission fidelity cannot be delegated to `act`.

skill_use: e2e-coverage-triage source=pagerguild/core-tools -> separated actual safe workflow execution from deterministic coordinator/control-plane proof and environment-sensitive live CI.

source_discovery: Read-only target-repository scan found mixed `act` viability. `attractor-go` has a conventional Ubuntu CI workflow; PlainTerms has some simple Ubuntu workflows but its preview E2E path depends on Vercel repository dispatch, custom runner label `playwright-testing-16vcpu`, Supabase/Vercel APIs, Doppler, artifacts, and concurrency; PDX's workflow deploys to Fly.io.

decision: `act` is explicit opt-in per selected workflow. DTU GitHub never infers local safety or automatically runs an authoritative workflow through `act`. Without a local-safe designation and explicit test-only inputs, the run remains scripted.

security_boundary: `act` workflows receive no developer GitHub token, host credentials, production secrets, or implicit access to shared infrastructure. Deploying or environment-mutating workflows are not eligible merely because `act` can parse them.

correction: The DTU functional spec now has eight acceptance scenarios rather than seven; the eighth is the explicit local-safe `act` workflow execution case.

tool_error: A double-quoted `rg` verification pattern contained Markdown backticks, so zsh executed the installed `act` binary while expanding the command. `act` inspected the Docker host, then exited immediately because `/Users/tyler/src/merge-herder/.github/workflows` does not exist. No workflow or job container started. Re-run uses single-quoted patterns.

source_discovery: The accidental no-workflow invocation emitted `act`'s Apple M-series warning recommending an explicit `--container-architecture linux/amd64`. The DTU runner configuration and evidence must therefore pin and report container architecture as well as `act` version and runner image.

memory_lookup: Prior MergeHerder guidance reinforces keeping this addition tied to concrete required behavior. The `act` mode is therefore a narrowly selected workflow executor, not a generalized runner platform or an excuse to add unrelated GitHub abstractions.

proof: `vp fmt --check` passed for `DTU_GITHUB_SPEC.md`, `GITHUB_SERVICE_CONTRACT.md`, and this worklog; `git diff --check` and no-index whitespace checks for the untracked design files passed.

proof: Focused content audit confirms that the functional spec requires both execution modes, explicit local-safe opt-in, pinned `act` version/runner image/container architecture, supervised cancellation, captured execution evidence, and acceptance scenario 8. The service contract points to the official `act` usage and unsupported-functionality documentation.

status: The design now says yes to real workflow execution through `act` and Docker where it is safe and useful, while retaining scripted workflow runs for deterministic GitHub control-plane behavior. No application or DTU implementation code was changed.

## Scenario-First Proof Refocus

correction: The user rejected "DTU GitHub" as the organizing concept before the actual MergeHerder proof cases are understood. Infrastructure must be derived from concrete proof scenarios, not the other way around.

decision: Added `MERGE_HERDER_PROOF_SCENARIOS.md` as a black-box proof contract. It deliberately does not assume a GitHub twin, `act`, mocks, or live GitHub as the implementation.

decision: The primary behavioral proof suite should live outside the MergeHerder repository so the application is not the sole author of its own acceptance standard. Scenario observations use Git SHAs, trees, CI identities, and product-visible state rather than coupling the oracle to MergeHerder's internal database schema.

scenario_inventory: Defined concrete setup, actions, and pass conditions for K=1 landing, clean multi-PR batching, single-flight queueing, hotfix repair, Pause race, unchanged Unpause, paused direct hotfix, suffix rebuild plus repair replay, Cancel, stale-base landing, event disorder/idempotency, crash recovery, and agentic conflict resolution.

design_discovery: Unpause with no Git changes still requires a distinct authoritative CI run for the same SHA. A no-op ref push cannot be assumed to create a push-triggered workflow run, so the product must deliberately choose a valid rerun or dispatch mechanism.

decision: Deterministic coordinator proof and coding-agent quality evaluation are separate. A controlled integration worker proves coordinator contracts; repeated real-agent conflict fixtures measure agent resolution quality without making the core regression suite nondeterministic.

open_scenarios: Source movement while actively running, source closure/deletion, unexpected release-ref movement, CI non-completion, unchanged-SHA retry policy, unresolvable repair conflicts, successful PR cleanup, and batch formation/order/cap remain explicit decision-forcing cases rather than silently invented behavior.

skill_use: e2e-coverage-triage source=pagerguild/core-tools -> scenarios are stated in product behavior while test mechanics and fixture control remain outside scenario assertions; real cross-boundary orchestration is separated from pure state tests and agent evaluations.

decision: Routed the scenario-first hierarchy from `README.md`, linked the scenario contract as upstream of `GITHUB_SERVICE_CONTRACT.md`, and marked `DTU_GITHUB_SPEC.md` as a downstream infrastructure draft that must be reduced, reshaped, or replaced after the scenarios are accepted.

boundary: The scenario document is explicitly a discussion draft. In particular, the proposed effects of Cancel on member queue state and GitHub PR Draft state are presented as requiring ratification rather than silently promoted from the worklog into settled behavior.

skill_use: repo-proof-policy source=pagerguild/core-tools -> treated the change as documentation-only, routed the new proof contract from the repository README, checked the durable diff against the accepted conversation decisions, and ran targeted formatting plus whitespace gates.

proof: `vp fmt --check README.md MERGE_HERDER_PROOF_SCENARIOS.md GITHUB_SERVICE_CONTRACT.md DTU_GITHUB_SPEC.md ephemeral/worklog/202607101222-merge-queue-domain-seams.md` passed; `git diff --check` passed; a focused heading audit found all thirteen proposed scenarios, the decision-forcing section, the inferred test-world capabilities, and the DTU downstream-draft warning.

status: The design is now oriented around concrete independent proof cases. Test infrastructure has intentionally not been selected from those requirements yet.

## Real Workflow Execution Required By Batch Proof

decision: The user established that the first three-PR rebase/batch proof must run the actual workflow using `act` or a similar Actions-compatible executor. A test that directly injects CI success would not prove that the assembled candidate works.

decision: P2 now checks out exact release SHA `R0`, executes the proof repository's real `.github/workflows` CI definition, and feeds the observed run identity, SHA, status, logs, and conclusion through MergeHerder's production CI-observation seam.

decision: The hotfix scenario likewise obtains failure for `R0` and success for `R1` by executing the same real workflow against both exact candidate SHAs. The fixture includes a genuine integration assertion rather than a hard-coded requested outcome.

boundary: Real workflow execution and controllable GitHub run delivery are complementary. `act` or its equivalent proves candidate behavior; a controllable run/event mechanism is still required for cancellation races, delayed or late completions, duplicates, and reordering.

skill_use: session-worklog source=pagerguild/core-tools -> recorded the user's correction as a durable proof requirement and updated the scenario contract rather than leaving it only in conversation.

## Per-Scenario Interaction Contracts

goal: Expand every product-level proof scenario into a concise ordered trace with one bullet per external interaction so the independent proof repository can determine exactly what must be real, controlled, or doubled.

decision: Added `docs/proof-scenarios/README.md` plus one interaction-contract file for each of P1 through P13. The vocabulary distinguishes product operations, GitHub REST, GitHub webhooks, real Git smart HTTP, real workflow execution, integration-worker calls, harness controls, and black-box observations.

decision: All known GitHub calls are named at their protocol boundary: targeted PR GET, App installation-token POST, Actions cancellation POST, Git upload-pack/receive-pack requests, and signed webhook POSTs to the real MergeHerder route. Conditional token reuse is allowed without changing scenario semantics.

boundary: The contract does not invent URLs for MergeHerder Submit/Pause/Unpause/Cancel/state operations or for proof-harness controls. It names the semantic interaction and marks transport as TBD.

design_discovery: A hotfix push while paused will normally trigger the push-configured workflow immediately. The product must decide whether that run is cancelled, allowed but invalidated, or retained after Unpause before P7 can freeze its interaction trace.

design_discovery: Restart scenarios expose a possible workflow-run inspection dependency. Each crash-boundary recovery algorithm must be accepted before the proof contract can decide whether Git refs plus durable webhook receipts suffice or a GitHub Actions run-read API is required.

decision: The interaction-contract index contains a consolidated initial controlled-surface inventory. Request and immediate response count as one interaction; default status/response semantics are stated for token minting, targeted PR reads, accepted webhooks, asynchronous cancellation, and real Git protocol outcomes.

skill_use: repo-proof-policy source=pagerguild/core-tools -> routed the interaction directory from `README.md` and the parent proof contract, then checked document format, whitespace, scenario-file count, headings, bullets, links, and required controlled-surface terms.

proof: `vp fmt --check` passed for all nineteen active design/worklog files; `git diff --check` passed; the interaction audit confirmed thirteen `p*.md` contracts, one ordered-bullet trace per file, valid scenario-index targets, and coverage of token minting, PR reads, Actions cancellation, Git smart HTTP, webhook envelopes, real workflow execution, and unresolved P6/P7/P12 interactions.

unproved: These documents stipulate the desired contracts but do not execute application behavior. Product-operation transports and the three explicitly unresolved interaction policies remain open by design.

status: The repository now contains a concise per-scenario interaction contract sufficient to begin designing the separate proof harness from required boundary behavior rather than from a generalized GitHub clone.

## Claude/Codex Consensus Review Of Proof Contracts

skill_use: claude-codex-consensus source=pagerguild/core-tools -> the user explicitly requested an independent Opus review using `agent opus`, exact-prompt capture, severity labels, revision, and repeated review until material findings are resolved.

tool_error: The first `agent opus` invocation failed before review because an invalid `ANTHROPIC_API_KEY` overrode the working Claude login. The unchanged prompt was rerun successfully with that environment variable removed and Doppler autoload disabled.

review_round_1: Opus wrote `ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md` after reviewing all product scenarios, per-scenario traces, GitHub contract, worklog decisions, DTU draft, current webhook/auth code, and external protocol semantics.

accepted_bug: Replaced nonexistent `workflow_run` action `queued` with GitHub's real sequence: action `requested` with status `queued`, then actions `in_progress` and `completed`.

accepted_bug: Recorded the existing `AUTH-06` defect: current `read:user` plus `user:email` scopes do not authorize private organization-membership reads. The auth implementation is being replaced in a separate worktree, so this design task corrects the service contract without editing auth code.

accepted_design: Restored PR snapshot source to an unsettled semantic interaction rather than prescribing targeted REST GETs; changed rerun correlation to `(run_id, run_attempt, head_sha)`; added Pause's terminal-run/`409` ordering; added rejection of non-authoritative workflows at the same SHA; added forged-signature scenario P14; and made App-token push identity plus release-ref-scoped workflow configuration explicit.

accepted_nits: Clarified that closed PRs remain readable, source-ref deletion requires real Git inspection, and `push` webhook inventory includes the `forced` flag.

rejected_finding: Round 1 characterized explicit force-with-lease as merely client-side and warned that receive-pack enforcement would make the proof server stronger than GitHub. Git's pack protocol sends `old-id new-id ref`, and the receive-pack server validates that the ref still equals `old-id` before update. The contract now requires explicit `--force-with-lease=refs/heads/R:<expected-old-SHA>` and ordinary receive-pack semantics, with no custom strengthening hook.

external_resource: GitHub Actions trigger documentation -> `workflow_run` actions are `requested`, `in_progress`, and `completed`; reruns do not emit `requested`.

external_resource: GitHub workflow-run REST documentation -> rerun is `POST /repos/{owner}/{repo}/actions/runs/{run_id}/rerun`, returns `201`, uses Actions write permission, and represents a fresh attempt of the run.

external_resource: GitHub OAuth scope documentation -> `read:org` is the read-only scope for organization membership.

external_resource: Git receive-pack protocol documentation -> update commands carry expected `old-id`, and the server validates the ref has not changed before applying the update.

review_round_2: Prepared an unchanged-goal follow-up prompt requiring Opus to verify the complete revised contract and explicitly adjudicate every round-1 finding.

review_round_2_result: Opus wrote `ephemeral/reviews/20260710-mergeherder-proof-contract-round2.md`, confirmed every round-1 bug/design finding was resolved or correctly rejected, and independently confirmed the explicit force-with-lease receive-pack semantics.

accepted_design: Added test control for auxiliary non-authoritative workflow runs at an existing SHA and for missing/invalid webhook signatures, making P11 and P14 executable without general multi-workflow support.

accepted_design: Added OAuth granted-scope tracking and required the membership endpoint to reject tokens without organization-membership read scope, preventing the proof server from making the known AUTH-06 defect look green.

accepted_design: Corrected CI-06 from Deferred to Required mechanism unresolved. P6 is explicitly blocked until the product chooses rerun or dispatch; the proof tool and downstream draft may not substitute a no-op push.

accepted_nit: Corrected P10 to state that the non-forced landing push is rejected by receive-pack's fast-forward ancestry check against current `M1`, not by a lease on expected `M`.

review_round_3: Required because round 2 found material downstream design gaps. The final prompt asks Opus to verify the complete goal after those corrections and report any remaining critical, bug, or design findings.

review_round_3_result: Opus wrote `ephemeral/reviews/20260710-mergeherder-proof-contract-round3.md`, verified every round-2 correction, and found one remaining design defect: P4 asked a terminal failed run to later emit success without a new attempt, which GitHub cannot do.

accepted_design: P4 now redelivers the immutable completed/failure body with the same GUID and sends a semantic failure duplicate with a new GUID. P11 remains the separate GitHub-faithful proof that a superseded run's real late success cannot land.

accepted_nits: Pinned insufficient OAuth organization scope to `403` and added the already-contracted negative foreign-owner installation intake proof.

consensus_status: Three review rounds exhausted the skill's loop limit. The final round prescribed one narrow correction, which Codex independently accepted and applied exactly. No critical, bug, or design disagreement remains; intentionally unresolved product seams remain explicit. Final reconciliation is in `ephemeral/reviews/20260710-mergeherder-proof-contract-reconciliation.md`.

## Proof Tool Language And Generation Direction

recommendation: Implement the proof runtime in Go using `net/http`, not Huma. The hard runtime responsibilities are Git smart HTTP, `git-http-backend`, Docker/`act` subprocess supervision, controllable scheduling, and process lifecycle; Go's standard library owns these directly.

recommendation: Use `libopenapi` to load and mechanically slice GitHub's upstream bundled OpenAPI description to only the accepted operation set and transitive schema references. GitHub's document remains the route/parameter/status/schema authority rather than regenerating a new API description from Go handlers.

recommendation: Use `github.com/google/go-github` types directly as the implementation request/response and webhook models. Pointer fields and `omitempty` are useful wire-presence semantics, not a reason to generate parallel model structs. Add only the smallest local extension when a required field/type is genuinely absent.

boundary: `go-github` is the practical implementation model library, while the selected upstream OpenAPI and live conformance remain the external contract. This does not imply translation away from `go-github`; it means generated bindings and tests detect rather than silently inherit SDK drift.

recommendation: Generate `net/http` method/path bindings from the libopenapi operation model and a small explicit operation-to-`go-github` type mapping. Investigate reusing `go-github`'s own `openapi_operations.yaml` rather than independently guessing method/type associations.

recommendation: Keep latest-Octokit conformance as a tiny Node test client rather than a second service. It resolves the latest Octokit at run time, calls the narrow server/live contract, records exact versions, and has no role in Git, workflow execution, or scenario orchestration.

proof: `vp fmt --check` passed across twenty-seven current design, interaction, review, reconciliation, and worklog files; `git diff --check` passed; all fourteen indexed scenario files exist and contain ordered interaction bullets.

proof: Focused semantic audit found no remaining impossible P4 late-success wording or nonexistent `workflow_run` queued action. It confirmed failure redelivery, requested/queued distinction, run-attempt correlation, non-authoritative workflow control, invalid-signature rejection, OAuth `403`, explicit receive-pack lease, blocked P6 status, and P10 ancestry rejection.

final_status: Three independent Opus review rounds and Codex reconciliation validate the prescribed API/interaction contract. All critical/bug/design findings were corrected or rejected with protocol evidence; intentionally unresolved product decisions are named and block only their dependent scenarios. No application code was changed.

## Publication To Independent Repository

user_goal: Put every validation/design artifact created during this session into a separate repository and publish it as `tylergannon/merger-herder-validation`.

decision: Created a clean sibling repository at `/Users/tyler/src/merger-herder-validation` without modifying or removing the source artifacts from MergeHerder. It contains the three root contracts, fourteen per-scenario traces, all three consensus prompts/reviews, final reconciliation, this worklog, a focused README, and an MIT license.

skill_use: repo-proof-policy source=pagerguild/core-tools -> verified root routing, twenty-seven Markdown files, all relative links, fourteen indexed scenarios, three review rounds plus prompts, source-copy identity, formatter compliance, and staged whitespace before publication.

tool_note: `vp fmt --check` could not run with the new docs-only repository as its working directory because it is not a Vite+ workspace. Running MergeHerder's configured Vite+ formatter against the new repository's absolute file paths passed all twenty-seven Markdown files.

proof: GitHub repository creation and initial push succeeded; the repository is public with default branch `main`, and GitHub reported all fourteen scenario files. The initial local and remote heads both resolved to `854b9e1a9d1edc28b2585da49bcd6b1640d069ac` before this publication-closeout update.

status: Independent validation repository published at `https://github.com/tylergannon/merger-herder-validation`. A final closeout commit will include this publication record.
