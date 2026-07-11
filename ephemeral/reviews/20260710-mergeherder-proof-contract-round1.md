# MergeHerder Proof-Scenario / Interaction-Contract Review — Round 1

## Prompt

> Here is my goal:
>
> Validate the correctness, completeness, and non-adventitiousness of the proposed
> MergeHerder proof-scenario API and interaction contract before a separate proof
> tool repository is designed. The contract must enumerate the external
> interactions genuinely required to prove MergeHerder's accepted behavior,
> without omitting correctness-critical GitHub/Git/CI interactions and without
> inventing APIs, product behavior, or infrastructure that the scenarios do not
> require.
>
> Review these artifacts together:
>
> - `MERGE_HERDER_PROOF_SCENARIOS.md`
> - `docs/proof-scenarios/README.md`
> - every `docs/proof-scenarios/p*.md` file
> - `GITHUB_SERVICE_CONTRACT.md`
> - the accepted/corrected product decisions in
>   `ephemeral/worklog/202607101222-merge-queue-domain-seams.md`
> - `DTU_GITHUB_SPEC.md` only as a downstream draft whose assumptions must not
>   override the scenario-first contract
>
> Review the design before implementation. Independently inspect the repository
> and authoritative GitHub/Git/act documentation where useful. Look for:
>
> - incorrect GitHub REST paths, webhook assumptions, payload/order semantics,
>   status codes, authentication behavior, or Git smart-HTTP interactions;
> - missing boundary interactions needed for the scenario to be executable and
>   for its pass conditions to be trustworthy;
> - interactions listed in the wrong actor order or on the wrong ownership side;
> - mocked or controlled behavior that must be real for the proof to mean
>   anything;
> - real behavior that can safely be controlled without weakening the proof;
> - scenario behavior that was invented, prematurely settled, or copied from
>   stale agent-authored notes rather than accepted product intent;
> - cases where the interaction contract is coupled to MergeHerder internals or
>   to one prospective proof-tool implementation;
> - contradictions among the scenario overview, per-scenario traces, GitHub
>   service contract, and product worklog; and
> - important proof scenarios or race/failure interactions missing from the
>   current set.
>
> Do not merely approve the design and do not propose a general GitHub clone.
> Prefer the smallest interaction surface that can honestly prove the scenarios.
>
> Do not edit product code or the reviewed design documents. Write the exact
> prompt you were given, your evidence-backed findings, and a concise overall
> assessment to:
>
> `ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md`
>
> Use file/line references where possible. Label every finding using exactly one
> of these definitions:
>
> - **critical:** must fix before proceeding.
> - **bug:** demonstrable incorrect behavior, broken contract, race, or
>   regression.
> - **design:** architecture, boundary, scope, maintainability, or proof issue
>   that is materially likely to cause problems.
> - **nit:** small cleanup that should not block progress.

## Method

I read all thirteen product scenarios, the interaction-contract index and every
`p*.md`, the GitHub service contract, the accepted worklog, and the downstream
DTU spec. I ground-checked "Current" claims against the live code
(`src/lib/server/github-webhooks.ts`, `src/lib/server/github-owner.ts`,
`src/routes/api/v1/github-webhook/+server.ts`) and checked the GitHub/Git-facing
assertions against my knowledge of the GitHub REST/webhook APIs, Git smart-HTTP,
and `act`. The overall design is unusually disciplined: it is scenario-first,
names transports only where they are real, and explicitly parks unresolved seams
instead of inventing them. The findings below are the correctness-critical gaps
that remain, ordered by severity.

---

## Findings

### 1. `workflow_run` has no `queued` action — the correlation seam is coded against a status name, not a webhook action

**bug**

Every scenario that observes CI describes three deliveries as "signed
`workflow_run` **queued**, then in-progress, then completed":

- `MERGE_HERDER_PROOF_SCENARIOS.md:95-96`, `:100-105` (P2)
- `docs/proof-scenarios/p01-one-clean-pr.md` ("signed `workflow_run` queued, then
  in-progress, then completed/success")
- `docs/proof-scenarios/p02-three-pr-batch.md`, `p04-hotfix.md`, `p06`, `p07`
- `GITHUB_SERVICE_CONTRACT.md:178` (CI-02 "Correlate queued/in-progress/completed
  CI")
- `DTU_GITHUB_SPEC.md:442-450` (run created with "initial status `queued`") and
  `:491` ("`workflow_run` for queued, in-progress, and completed transitions")

GitHub's `workflow_run` event has exactly three **action** values: `requested`,
`in_progress`, `completed`. There is no `workflow_run` action named `queued`.
`queued` is a _status_ value (and it is a valid **action** only on the different
`workflow_job` event). The first `workflow_run` delivery arrives with
`action=requested` while `status=queued`.

Why this is a correctness problem and not a wording nit: the CI-observation seam
correlates and advances batch state from these deliveries. If MergeHerder keys
transitions off the webhook `action` and the DTU is built to emit `action=queued`
(as `DTU_GITHUB_SPEC.md:491`/`:442-450` imply), the DTU and MergeHerder will
agree with each other and both disagree with GitHub. The only thing standing
between that and a green-but-wrong suite is the live `workflow_run` conformance
canary (`GITHUB_SERVICE_CONTRACT.md:341-342`) — i.e. this is precisely a case of
"controlled behavior that must match reality" being specified against the wrong
reality.

Fix: state the contract in terms of the actual pair — action `requested`
(status `queued`) → action `in_progress` → action `completed` (with
`conclusion`) — and make the DTU emit `action=requested` for run creation. If a
literal `queued` signal is genuinely wanted, that is `workflow_job.queued`, which
is a different, un-contracted event.

### 2. The interaction traces prematurely settle `PR-01` as a targeted REST GET, contradicting the contract and worklog

**design** (contradiction)

The contract and worklog explicitly leave the PR-snapshot source **open**:

- `GITHUB_SERVICE_CONTRACT.md:176` — PR-01 status is _"Required, endpoint source
  unresolved,"_ "probably `GET /repos/{owner}/{repo}/pulls/{pull_number}` plus
  `pull_request` webhooks."
- `GITHUB_SERVICE_CONTRACT.md:186-190` — "A pull-request webhook, a targeted REST
  read after local submission, or a combination may supply the snapshot."
- Worklog `202607101222-...md:211` — open_question: "whether it is sourced from
  `pull_request` webhooks, targeted REST reads, or both is not settled."

But the per-scenario traces and the consolidated inventory hard-commit to the
REST GET as a definite, required interaction:

- `docs/proof-scenarios/README.md:88` lists
  `GET /repos/{owner}/{repo}/pulls/{pull_number}` in the "Initial Controlled-Surface
  Inventory."
- `p01-one-clean-pr.md`, `p02-three-pr-batch.md:` ("call
  `GET /repos/{owner}/{repo}/pulls/{pull_number}` once for each member"),
  `p03`, `p08`, `p13` all issue the GET as a fixed step.

This is the one place where the otherwise-careful "name the semantic interaction,
mark transport TBD" discipline slips. If the front door turns out to be a
`pull_request` webhook, the member snapshot is already in the delivered payload
and several of these GETs vanish. Recommend the traces express this as
"`MergeHerder` obtains PR snapshot (targeted REST read _or_ `pull_request`
webhook — TBD)", matching PR-01, rather than baking in the REST call.

### 3. Guarded release-ref update (`R`) is treated as a reliable server-side compare-and-swap; on GitHub `--force-with-lease` is a client-side check

**design**

- `GITHUB_SERVICE_CONTRACT.md:174` (GIT-02): "Rebuilding `R` may require a guarded
  non-fast-forward update; the coordinator must not silently overwrite a ref that
  moved outside the batch operation."
- `p08-suffix-rebuild.md`: "update `R` from recorded old head `R1` to new head
  `R2` using a guarded ref update; **reject if `R` no longer matches `R1`**."
- `DTU_GITHUB_SPEC.md:411`: "a guarded force push is rejected when the expected old
  ref no longer matches."

`git push --force-with-lease` enforces the expected-old-OID **client-side**
against the pusher's remote-tracking ref; it is not an atomic server-side
compare-and-swap over Git smart-HTTP, and GitHub's REST ref-update endpoint
(`PATCH .../git/refs`) offers only `force: true|false`, no expected-SHA
precondition. A DTU built on `git-http-backend` _can_ enforce the old-OID with a
`pre-receive` hook and thereby look stronger than production. The risk is a proof
that passes because the twin guarantees an atomicity GitHub does not.

For the default branch this is fine: a plain non-forced push is genuinely
server-rejected on non-fast-forward (P10 relies on exactly that, correctly — see
note below). For `R` the guard is real only to the extent that MergeHerder is the
sole writer; P7/P8 deliberately introduce _other_ writers (human/agent hotfix
pushes, source moves), which is when the lease matters. Recommend: (a) state that
the `R` guard is best-effort force-with-lease, not an atomic server primitive;
(b) make the DTU model the same best-effort semantics (or explicitly document the
divergence as a fidelity limit); and (c) confirm the real safety net is
exact-SHA landing correlation — only the CI-tested `R` SHA may land — rather than
the lease itself.

(P10's default-branch guard at `p10-stale-base.md` and GIT-03 is correct and
does not have this problem: non-forced fast-forward-only is server-enforced.)

### 4. P6's "distinct new run" pass condition collides with GitHub rerun semantics

**design**

P6 requires fresh CI for an unchanged `R0` and asserts:

- `MERGE_HERDER_PROOF_SCENARIOS.md:210` — "A distinct new authoritative run is
  correlated with `R0`."
- `p06-unpause-unchanged.md` — "signed ... events for new `run-R0-2`, **distinct
  from** the invalidated run" and "choose the exact GitHub REST call and
  permission required for the fresh same-SHA run."

The document correctly identifies that a no-op push cannot create a run
(`:212-214`). But the two GitHub-valid mechanisms behave differently and the pass
condition silently presumes one:

- `POST /repos/{owner}/{repo}/actions/runs/{run_id}/rerun` (or `/rerun-failed-jobs`)
  produces a **new attempt of the same `run_id`** (`run_attempt` increments), not
  a distinct run. A correlation model keyed on `run_id` would treat the rerun as
  the _same_ run and could re-admit the invalidated run, or fail the "distinct
  run" assertion.
- `workflow_dispatch` produces a distinct `run_id` but fires the
  `workflow_dispatch` event (not `push`), so the fixture workflow must declare
  that trigger, and correlation-by-`push`-SHA no longer applies.

This is a genuine unresolved product decision (correctly parked as CI-06 at
`GITHUB_SERVICE_CONTRACT.md:200`), but the P6 pass condition ("distinct new run")
should not pre-judge it. State the correlation key as `(run_id, run_attempt,
head_sha)` so whichever mechanism is chosen remains provable, and defer the
"distinct run vs. new attempt" wording until the mechanism is picked.

### 5. Pause is only proved against the `202`-then-success race; the `409` (run already terminal) race is unspecified

**design** (missing race)

P5 fixes a single ordering:

- `p05-pause-race.md`: cancel returns `202 Accepted`, _then_ a racing success is
  delivered.

But the sharper race is the other order: the run completes successfully an
instant before Pause fires, so `POST .../cancel` returns **`409`** (a terminal run
is not cancellable — correctly modeled at `DTU_GITHUB_SPEC.md:462`), and the
success `workflow_run` may already be in flight or already delivered. Accepted
Pause semantics (worklog `:89-90`: "A paused CI run is not allowed to finish and
later authorize landing") must hold in this ordering too, where MergeHerder's
cancel attempt is _rejected_ rather than accepted. Nothing in P5 or P9 exercises
"cancel returns 409 yet Pause still blocks landing." This is the ordering most
likely to leak a landing, so it deserves its own trace.

### 6. No scenario proves that a _non-authoritative_ workflow run on the same SHA is ignored

**design** (completeness)

CI-02 makes a strong claim: _"Only the configured workflow and exact current
candidate SHA may affect the batch"_ (`GITHUB_SERVICE_CONTRACT.md:178`). Real
repositories routinely have several workflows that all trigger on a push, so the
`R` push can yield multiple `workflow_run` streams sharing the exact `head_sha`
but differing in `workflow_id`/name/path. Landing must be authorized only by the
**authoritative** workflow's success, not by any green run at that SHA.

No scenario delivers a second, non-authoritative `workflow_run=completed/success`
for the current `R` SHA and asserts it does **not** authorize landing. P11 covers
duplicate/late/reordered events for the _same_ run, which is a different axis.
Recommend adding a scenario (or extending P11) with a competing workflow at the
same `head_sha` whose success is inert. This directly exercises the
`workflow_id`-filtering half of CI-02, which is currently asserted but unproven.

### 7. No negative webhook-signature scenario, despite HMAC verification being the security boundary

**design** (completeness)

`HOOK-01` (`GITHUB_SERVICE_CONTRACT.md:135`) and the interaction default
(`docs/proof-scenarios/README.md:16` "an accepted **signed** webhook returns a
`2xx`") depend on HMAC-SHA256 verification, and the live route already rejects
bad signatures with `401` (`src/routes/api/v1/github-webhook/+server.ts`). Yet no
scenario delivers a **forged/invalid-signature** body and asserts it is rejected
and mutates no state. Every trace delivers only correctly signed events. Because
the webhook route is the one unauthenticated public ingress that can drive Git
and landing state, "a body with a bad/absent `X-Hub-Signature-256` is rejected
and changes nothing" belongs in the proof set (it also validates the DTU's raw-body
signing path). Small addition, high value.

### 8. AUTH-06 org-membership check requires `read:org`, but the requested scopes are only `read:user` + `user:email`

**bug** (latent for org-owned installations; confirmed against code)

- `GITHUB_SERVICE_CONTRACT.md:72` (AUTH-06) admits "an active member of the
  configured organization at login time" via
  `orgs.getMembershipForAuthenticatedUser()` → `GET /user/memberships/orgs/{org}`.
- `GITHUB_SERVICE_CONTRACT.md:74`: "Current scopes requested by Better Auth are
  `read:user` and `user:email`." (Confirmed: no scope override in the app; the
  code path is `src/lib/server/github-owner.ts:67`
  `getMembershipForAuthenticatedUser({ org: githubAppOwner })` using the user's
  OAuth token.)

`GET /user/memberships/orgs/{org}` requires the `read:org` scope for OAuth
user-to-server tokens. With only `read:user`/`user:email`, the org path returns
`403`/`404`, and the code (`github-owner.ts:67-74`) `.catch(() => null)`s that
into `allowed: false` — i.e. **every** member of an org-owned installation is
silently denied login. This is latent today only because a user-owned
installation (`githubAppOwnerType === 'user'`) short-circuits before the call
(`github-owner.ts:59-66`). For the OAuth-login proof (DTU acceptance scenario 2 at
`DTU_GITHUB_SPEC.md:537-540` and the live canary), the org branch as specified
will fail. Either add `read:org` to the requested scopes, or switch the
org-membership check to a mechanism the current scopes support, and make the DTU
`GET /user/memberships/orgs/{org}` reject when the presented token lacks the scope
so the twin actually catches this.

### 9. CI-01's "push of `R` starts exactly the authoritative run" hides two correctness-critical external facts

**design**

`CI-01` (`GITHUB_SERVICE_CONTRACT.md:177`) and the external-service list
(`MERGE_HERDER_PROOF_SCENARIOS.md:402-403`) assume pushing `R` "causes one
identifiable authoritative workflow run." Two GitHub-specific facts make this
true only under conditions that are not called out:

1. **Pusher identity matters.** A push made with a GitHub **App installation
   token** _does_ trigger `on: push` workflows — but the Actions `GITHUB_TOKEN`
   deliberately does _not_ trigger further runs (recursion guard). The proof only
   means something if the `R` push is made with installation credentials
   (`GIT-02`/`APP-01`), and that dependency should be an explicit live-conformance
   claim, because a proof harness that pushes `R` "as itself" cannot reveal the
   difference.
2. **Trigger scoping.** "One authoritative run and no spurious runs" requires the
   fixture workflow to trigger on the release ref specifically (CI-01 says
   "release-ref name or ref pattern that triggers it"). Real workflows usually
   trigger on `pull_request`/`push: main`. P8 even pushes a source ref (`C`),
   which would start extra runs under a typical `on: push` config. The fixture's
   trigger configuration is a proof precondition and should be stated, not
   assumed.

Neither is wrong in the documents; both are load-bearing assumptions that are
currently implicit. Make them explicit contract claims so the live canary is
obligated to check them.

### 10. Minor / non-blocking

**nit** — `DTU_GITHUB_SPEC.md:433` says "A missing, **deleted**, or inaccessible
PR returns `404`." GitHub pull requests cannot be deleted; a closed PR still
returns `200` with `state: "closed"`. Drop "deleted" (or scope it to "unknown
number") so the twin does not model a non-GitHub behavior.

**nit** — `PR-01` (`GITHUB_SERVICE_CONTRACT.md:176`) claims a PR snapshot can
"distinguish an advanced head from a closed PR, **deleted source**, changed base,
or inaccessible fork." A targeted `GET .../pulls/{n}` retains the last
`head.sha`/`head.ref` after the source branch is deleted, so branch deletion is
not reliably visible from the PR object alone; detecting it needs a Git ref
check. Worth a note so the revalidation design doesn't over-trust the PR read.

**nit** — `HOOK-04` (`GITHUB_SERVICE_CONTRACT.md:138`) enumerates
`before`/`after`/deletion/creation flags but omits the `push` payload's `forced`
flag, which is directly relevant to the decision-forcing case "the release ref
`R` is moved or force-pushed by an unexpected actor"
(`MERGE_HERDER_PROOF_SCENARIOS.md:383-384`). Cheap to list now.

---

## What is correct and should not be "fixed"

To keep the smallest honest surface, these are right as written:

- REST paths and status codes: `POST /app/installations/{id}/access_tokens` →
  `201` (`README.md:15` of the scenario index); `GET .../pulls/{pull_number}` →
  `200`; `POST .../actions/runs/{run_id}/cancel` → `202` accepted / `409` on a
  terminal run (`DTU_GITHUB_SPEC.md:457-464`). All match GitHub.
- Git smart-HTTP framing (`info/refs?service=git-upload-pack` +
  `POST git-upload-pack`; receive-pack for push) and `x-access-token` Basic auth
  (`docs/proof-scenarios/README.md:29-34`, `DTU_GITHUB_SPEC.md:391-392`) are
  correct.
- P10's stale-base landing via non-forced fast-forward-only push
  (`p10-stale-base.md`, GIT-03) is the correct, server-enforced guard.
- The idempotency-by-`X-GitHub-Delivery` model (P11, `HOOK-01`) and the
  event-creation-vs-delivery split (`DTU_GITHUB_SPEC.md:493-495`) are exactly the
  right primitives for the disorder/duplicate proofs.
- Keeping Git, Postgres, HTTP envelopes, signatures, and the coordinator real
  while simulating identity/PR/run/time is the correct real-vs-controlled line,
  and the decision-forcing / open-seam sections honestly refuse to invent the
  front door, batch trigger, Cancel ratification, and restart algorithms.
- Using `act` only for candidate-behavior fidelity while keeping control-plane
  races scripted (`DTU_GITHUB_SPEC.md:470-474`, worklog `:385-391`) is the right
  division and does not overclaim `act` parity.

## Overall assessment

The scenario-first contract is fundamentally sound and close to ready to drive a
proof-tool design. It resists the two failure modes the prompt warns about: it
neither rubber-stamps nor proposes a general GitHub clone, and it repeatedly
parks unresolved product seams instead of inventing them.

The one finding that should be resolved **before** a proof tool encodes it is
**#1** (the `workflow_run` `queued` vs `requested` action): it is the sole place
where the controlled surface is specified against a GitHub behavior that does not
exist, and if the DTU and MergeHerder both adopt it they will pass in-house while
being wrong in production. **#8** is a real, code-confirmed authentication defect
for org-owned installations, though latent while the owner is user-typed.
Findings **#2–#7** and **#9** are correctness-of-proof gaps — a prematurely
settled PR transport, an over-strong `R` lease assumption, an ambiguous rerun
correlation key, and three missing-but-important interactions (the `409` Pause
race, the non-authoritative-workflow filter, and the negative-signature case) —
each of which either weakens what a green suite proves or bakes in an
unratified decision. None requires a larger interaction surface; most are
tightening or a single added trace. The remainder are nits.

No product code or reviewed design document was modified.
