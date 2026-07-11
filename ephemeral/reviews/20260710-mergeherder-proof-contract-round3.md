# MergeHerder Proof-Scenario / Interaction-Contract Review — Round 3

## Prompt

> Here is my goal:
>
> Perform the final independent validation of the MergeHerder proof-scenario API
> and interaction contract before a separate proof-tool repository is designed.
> The contract must be correct, complete for accepted behavior, faithful to real
> GitHub/Git/Actions semantics, and free of invented product behavior or
> unnecessary surfaces.
>
> Read both prior reviews and all current artifacts:
>
> - `ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md`
> - `ephemeral/reviews/20260710-mergeherder-proof-contract-round2.md`
> - `MERGE_HERDER_PROOF_SCENARIOS.md`
> - `docs/proof-scenarios/README.md`
> - every `docs/proof-scenarios/p*.md`
> - `GITHUB_SERVICE_CONTRACT.md`
> - downstream `DTU_GITHUB_SPEC.md`
> - relevant decisions in
>   `ephemeral/worklog/202607101222-merge-queue-domain-seams.md`
>
> Verify the whole goal again, including round-2 resolutions: auxiliary
> non-authoritative workflow runs, invalid-signature delivery, OAuth scope
> enforcement, P6's blocked/unresolved fresh-run mechanism, and P10's
> fast-forward rejection wording. Find new issues if they exist rather than merely
> checking boxes.
>
> Do not edit product code or reviewed design documents. Write the exact prompt,
> findings with evidence, and final assessment to:
>
> `ephemeral/reviews/20260710-mergeherder-proof-contract-round3.md`
>
> Label every finding exactly:
>
> - **critical:** must fix before proceeding.
> - **bug:** demonstrable incorrect behavior, broken contract, race, or
>   regression.
> - **design:** architecture, boundary, scope, maintainability, or proof issue
>   that is materially likely to cause problems.
> - **nit:** small cleanup that should not block progress.
>
> Explicitly say whether any critical, bug, or design finding remains. Do not
> claim consensus merely because earlier findings were edited.

## Method

I re-read both prior reviews, the fourteen product scenarios in
`MERGE_HERDER_PROOF_SCENARIOS.md`, the interaction-contract index and all
fourteen `p*.md` traces, `GITHUB_SERVICE_CONTRACT.md`, the downstream
`DTU_GITHUB_SPEC.md`, and the domain worklog (including its round-1 and round-2
disposition logs at `:485-523`). I re-verified the two code-grounded claims
against live source — `src/lib/server/github-owner.ts:66-73` and the webhook
ingress path `src/routes/api/v1/github-webhook/+server.ts:8-33` plus
`src/lib/server/github-webhooks.ts:77-101,301-321` — and I ground-checked every
GitHub/Git/Actions assertion against the REST, webhook, and Actions run
lifecycle.

I did not merely re-confirm the round-2 dispositions. I re-ran the whole goal on
the current documents. The five round-2 resolutions the prompt names are all
present and correct (adjudicated below). One new proof-fidelity defect survived
that pass — a GitHub-impossible run transition baked into the P4 hotfix
scenario and its DTU acceptance scenario — plus two nits.

---

## Adjudication Of The Five Named Round-2 Resolutions

### Auxiliary non-authoritative workflow runs (was R2-N1, part 1) — **RESOLVED**

The DTU now enumerates the exact control capability P11 needs:
`DTU_GITHUB_SPEC.md:242-249` ("Create an auxiliary workflow run … create a
scripted run for a non-authoritative workflow identity at an existing real
branch and SHA"), driven to completion via the scripted-run controls
(`:251-269`). This matches `p11-event-disorder.md:22-23` and
`MERGE_HERDER_PROOF_SCENARIOS.md:335-336`, and the controlled-surface inventory
now lists it (`docs/proof-scenarios/README.md:104`, "non-authoritative workflow
identity at an existing SHA"). The capability explicitly "cannot itself change
MergeHerder" (`:248-249`), so it stays on the scenario-control plane. Faithful:
in real GitHub a second `on: push` workflow at the same release-ref push
produces a distinct `workflow_id` run at the same `head_sha`, which is exactly
what this models.

### Invalid-signature delivery (was R2-N1, part 2) — **RESOLVED**

`DTU_GITHUB_SPEC.md:312-313` adds "send it with a missing or deliberately
invalid `X-Hub-Signature-256`, recording the response without marking the
delivery validly accepted," matching `p14-forged-webhook.md:5-8` and
`README.md:102`. The `401` and the "must not poison deduplication" claim are
code-accurate: `receiveGitHubWebhook` throws before any persistence when the
signature is absent (`github-webhooks.ts:83-85`), and when the signature is
present-but-invalid `verifyAndReceive` rejects before the `onAny`
`recordGitHubWebhookEvent` handler runs, so `storeDelivery` /
`upsertGithubWebhookDelivery` (`github-webhooks.ts:303-307,223-242`) is never
reached and no dedup row is written. The route maps that error to `401`
(`+server.ts:25-27`). P14's ordering (reject before durable dedup, then accept
the same GUID with a valid signature) is therefore honest against the real code.

### OAuth scope enforcement (was R2-N2 / the unresolved half of R1-#8) — **RESOLVED**

The DTU OAuth model now tracks scopes end to end: authorize "records the
requested OAuth scopes on the code" (`DTU_GITHUB_SPEC.md:356`), token exchange
issues a token "carrying the code's granted scopes" (`:368`), and the membership
endpoint "rejects a token that lacks organization-membership read scope;
`read:user` plus `user:email` is not sufficient" (`:381-383`). This satisfies the
contract directive at `GITHUB_SERVICE_CONTRACT.md:82` ("The proof tool must not
model `AUTH-06` as succeeding with the currently documented scopes"). The
underlying code defect is still present and still confirms the finding (review
only, no edit): `github-owner.ts:66-68` calls
`getMembershipForAuthenticatedUser({ org: githubAppOwner }).catch(() => null)`
and returns `allowed: membership?.data.state === 'active'`, so a scope rejection
collapses to `allowed:false`. Acceptable to leave, because auth is being replaced
in a separate worktree (worklog `:237-269`) and the twin now refuses to make the
defect look green. (One residual is a nit, below: the rejection status code is
unspecified.)

### P6 blocked/unresolved fresh-run mechanism (was R2-N3) — **RESOLVED**

P6 now carries an explicit `Status:` line — "required by the accepted
Pause/Unpause behavior but blocked until the product chooses GitHub rerun or
workflow dispatch … The proof tool must not manufacture a no-op-push substitute"
(`MERGE_HERDER_PROOF_SCENARIOS.md:195-197`). It is consistent across all layers:
CI-06 is "Required mechanism unresolved" (`GITHUB_SERVICE_CONTRACT.md:206`), the
DTU lists rerun/dispatch under Unsupported Behavior and states P6 is blocked
(`DTU_GITHUB_SPEC.md:532,538-541`), and the trace parks the interaction
(`p06-unpause-unchanged.md:11-13,24-25`). The rerun-vs-dispatch fidelity is
correctly pre-stated — "A rerun may retain `run-R0` and increment `run_attempt`
without emitting action `requested`; a dispatch may create a new run ID"
(`p06:16-19`, `MERGE_HERDER_PROOF_SCENARIOS.md:215-217`) — which matches GitHub
(reruns do not emit `requested`). No no-op-push substitute is invented anywhere.

### P10 fast-forward rejection wording (was R2-N4) — **RESOLVED**

`p10-stale-base.md:8-9` now reads "attempt a non-forced push of `R0` to `main`;
receive-pack sees current head `M1` and rejects `R0` by the fast-forward
ancestry check," removing the round-2 "from expected `M`" lease implication. This
is protocol-accurate: a non-forced push carries the freshly advertised `M1` as
`<old-oid>`, and the update is refused because `M1` is not an ancestor of `R0`
(non-fast-forward), not by any old-oid lease. GIT-03
(`GITHUB_SERVICE_CONTRACT.md:181`) and the pass conditions
(`MERGE_HERDER_PROOF_SCENARIOS.md:313-315`) are consistent.

I also re-confirmed the round-1/round-2 items the prompt did not name: the
`workflow_run` triple `requested(queued)/in_progress/completed` (R1-#1),
transport-neutral `PR SNAPSHOT` (R1-#2), the receive-pack CAS grounding of
explicit `--force-with-lease` (R1-#3, correctly rejected), the
`(run_id, run_attempt, head_sha)` correlation key (R1-#4), the `409` terminal-run
Pause ordering (R1-#5), and the CI-01 pusher-identity/trigger-scoping claims
(R1-#9) all remain correct and unregressed in the current documents.

---

## New Findings (fresh pass over the whole goal)

### N1. P4's "late successful completion for the failed `R0` run" is a transition GitHub cannot emit

**design** (fidelity; also forces the twin to model a non-GitHub run transition)

In the hotfix scenario, `run-R0` reaches a terminal state with conclusion
**`failure`**:

- `p04-hotfix.md:7-9`: "`workflow_run` … `completed` with conclusion `failure`
  for `run-R0`, attempt 1, `head_sha=R0`."
- `MERGE_HERDER_PROOF_SCENARIOS.md:145-147`: the workflow "fails when executed
  against exact SHA `R0`."

The scenario then delivers a **success** for that same run:

- `MERGE_HERDER_PROOF_SCENARIOS.md:154`: "deliver a late successful completion
  for the old `R0` run"; pass `:162`: "The late `R0` success changes neither Git
  nor batch state."
- `p04-hotfix.md:23-24`: "deliver a late completed/success event for `run-R0`
  using either its original delivery ID or a new delivery ID."
- Downstream, `DTU_GITHUB_SPEC.md:578-579` (acceptance scenario 3): "A late or
  duplicate successful completion for `R0` changes nothing."

A GitHub `workflow_run` sets its `conclusion` once when it reaches `completed`.
A run that completed as `failure` does not later emit a second `completed` with
`success`; the only way the same `run_id` produces a success is a **rerun**,
which is a new `run_attempt` (and P4 performs no rerun of `R0`). So "a late
successful completion for `run-R0`" has no faithful origin. Two concrete
problems follow:

1. **The twin must model an impossible transition.** To deliver this event the
   DTU has to flip an already-terminal run's conclusion `failure → success`,
   which contradicts its own run lifecycle: "A later release-ref push creates a
   different run … It does not mutate or reuse the earlier run"
   (`DTU_GITHUB_SPEC.md:475-476`), and the scripted-run controls
   (`:251-269`) describe completing an _active_ run, not re-completing a
   terminal one. Either the DTU quietly grows a non-GitHub capability, or the
   acceptance scenario cannot be executed as written.
2. **The live canary can never reproduce it.** The property "a real late/stale
   event for the superseded head cannot land" is then proven only against a
   synthetic event GitHub does not produce — the same class of "green in-house,
   untestable against reality" gap that round-1 #1 corrected for the `queued`
   action.

The phrasing "late **or duplicate**" (`p04:23`, `DTU:578`) only compounds it: the
sole event `run-R0` actually produced is the **failure** completion, so a
faithful "duplicate" is a duplicate of the failure, not a success. The
"original delivery ID" branch (`p04:24`) is likewise contradictory — GitHub
redelivery is immutable body+GUID, so `run-R0`'s original GUID carries the
failure body, never a success body.

This does not weaken any _accepted product_ invariant: MergeHerder ignores the
event because `run-R0` is not the current run, and the genuinely faithful
version of "a superseded _green_ run cannot land" is already proved correctly by
P11, where `run-0` for `R0` was never failed — `p11-event-disorder.md:11-12`
("send late completed/success for `run-0`, `head_sha=R0`") pairs a real success
with a real superseded run. Fix P4 by making its late event a late/duplicate
**redelivery of `run-R0`'s actual `failure` completion** (or a stale
`in_progress`) — which still proves the superseded-run / idempotency guard — and
align `DTU_GITHUB_SPEC.md:578` and its scenario-3 wording the same way. Leave the
"late _success_ supersedes a green run" proof to P11, where it is GitHub-faithful.

### N2. The DTU does not pin the status code for an insufficient-scope membership read

**nit**

`DTU_GITHUB_SPEC.md:381-383` says `GET /user/memberships/orgs/{org}` "rejects a
token that lacks organization-membership read scope" but does not state the
status code, while it does pin `401` for missing/invalid/expired tokens and
`404` for a genuinely missing membership. Real GitHub returns `403` (with the
`X-Accepted-OAuth-Scopes`/`X-OAuth-Scopes` mismatch) for an OAuth token lacking
`read:org`, distinct from the `404` used for an absent membership. This does not
affect MergeHerder — `github-owner.ts:66-68` catches every non-success to
`allowed:false` — but the shared live-vs-DTU conformance assertion for `AUTH-06`
cannot be normalized until the scope-rejection code is fixed. Pin it (`403`) so
the twin and the live sandbox agree.

### N3. Foreign-owner installation-event filtering has no negative proof

**nit**

The current webhook receiver drops installation/repository mutations whose owner
does not match the configured owner
(`github-webhooks.ts:210-221,247`, `installationMatchesOwnerPolicy`) — accepted,
security-relevant current behavior (HOOK-02,
`GITHUB_SERVICE_CONTRACT.md:142`, "Events for another configured owner are
recorded but do not mutate installation/repository state"). DTU acceptance
scenario 1 (`DTU_GITHUB_SPEC.md:558-561`) proves only the matching-owner intake
path; nothing delivers a correctly signed event for a _different_ owner and
asserts no installation/repository row is mutated. This is outside the
merge-coordinator surface and low priority under the minimalism policy, but if
intake fidelity is claimed, one negative-owner delivery in scenario 1 would
close it cheaply. Not blocking.

---

## What Is Correct And Should Not Be "Fixed"

- The five named round-2 resolutions are present and faithful (adjudicated
  above); none regressed the round-1 fixes.
- The controlled-surface inventory (`README.md:90-105`) remains a small honest
  set: token minting, PR snapshot (transport unsettled), Actions cancel, the
  unresolved P6 rerun/dispatch, the possible P12 run-inspection read, real Git
  smart HTTP, the four signed webhook event types, invalid-signature rejection,
  and harness-only control including non-authoritative workflow identity. No new
  GitHub endpoint or product surface was invented since round 2.
- REST/Git/Actions semantics remain accurate: `POST …/access_tokens` → `201`;
  cancel → `202`/`409`; membership `401`/`404` (plus the N2 scope code); the
  `requested(queued)/in_progress/completed` triple; explicit
  `--force-with-lease=refs/heads/R:<old-SHA>` enforced by ordinary receive-pack
  CAS; non-forced default-branch landing rejected by the fast-forward ancestry
  check; delivery/creation split and GUID idempotency.
- The known holes stay parked rather than invented: Submit/Pause/Unpause/Cancel
  transports, batch trigger/ordering, the P6 fresh-CI mechanism, the P7
  paused-run policy, and the P12 recovery/run-read dependency
  (`README.md:111-120`, `p06:24-25`, `p07:21`, `p12:25-26`).

## Overall Assessment

**A design finding remains.** N1 (P4 / DTU acceptance scenario 3) specifies a
`workflow_run` transition GitHub cannot emit — a `completed(failure)` run later
delivering `completed(success)` without a rerun — which would force the twin to
model a non-GitHub run mutation and would prove the superseded-run guard only
against an event the live canary can never reproduce. It is not a running-code
bug and does not break an accepted product invariant (P11 already proves the
faithful superseded-green-run case), so it is design-level, not critical; but it
should be corrected before the proof tool encodes it, by making P4's late event a
late/duplicate of `run-R0`'s real `failure` completion and aligning the DTU
scenario-3 wording. N2 and N3 are nits.

No **critical** or **bug** finding was identified. The five round-2 resolutions
the prompt named are all correctly landed, and the earlier-round fixes are intact
— but this is stated on independent re-verification, not as consensus-by-edit:
the fresh pass surfaced N1, which the prior rounds did not catch. Apart from N1,
the scenario-first contract is correct, complete for the accepted behavior,
faithful to GitHub/Git/Actions semantics, and free of invented surfaces, and is
ready to drive the separate proof-tool design once N1 is folded in.

No product code or reviewed design document was modified.
