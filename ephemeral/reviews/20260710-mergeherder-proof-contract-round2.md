# MergeHerder Proof-Scenario / Interaction-Contract Review — Round 2

## Prompt

> Here is my goal:
>
> Validate the correctness, completeness, and non-adventitiousness of the revised
> MergeHerder proof-scenario API and interaction contract before a separate proof
> tool repository is designed. The contract must enumerate only the external
> interactions genuinely required to prove MergeHerder's accepted behavior while
> remaining faithful to real GitHub, Git, webhook, and Actions semantics.
>
> This is consensus review round 2. Read:
>
> - `ephemeral/reviews/20260710-mergeherder-proof-contract-round1.md`
> - `MERGE_HERDER_PROOF_SCENARIOS.md`
> - `docs/proof-scenarios/README.md`
> - every `docs/proof-scenarios/p*.md` file
> - `GITHUB_SERVICE_CONTRACT.md`
> - `DTU_GITHUB_SPEC.md` as a downstream draft only
> - `ephemeral/worklog/202607101222-merge-queue-domain-seams.md`
>
> Independently verify whether the revised artifacts resolve round-1 findings
> without adding accidental product behavior or unnecessary surfaces. Recheck the
> whole goal rather than limiting yourself to the prior findings. In particular,
> ground any conclusion about explicit
> `git push --force-with-lease=<ref>:<expected-old-SHA>` in the Git receive-pack
> protocol rather than assuming either a custom server hook or a purely local
> guard.
>
> Do not edit product code or the reviewed design documents. Write the exact
> prompt you received, findings with file/line evidence, and an overall assessment
> to:
>
> `ephemeral/reviews/20260710-mergeherder-proof-contract-round2.md`
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
> Explicitly state when a round-1 finding is resolved, rejected with adequate
> evidence, still unresolved, or replaced by a new finding. Do not merely approve
> the revision.

## Method

I re-read the round-1 review, the thirteen product scenarios plus the two added
ones (`MERGE_HERDER_PROOF_SCENARIOS.md`), the interaction-contract index and all
fourteen `p*.md` traces, `GITHUB_SERVICE_CONTRACT.md`, the downstream
`DTU_GITHUB_SPEC.md`, and the domain worklog (including its round-1 disposition
log at `:485-512`). I re-verified the two code-grounded round-1 claims against
live source: `src/lib/server/github-owner.ts:66-73` and
`src/routes/api/v1/github-webhook/+server.ts` (signature rejection). I ground the
`--force-with-lease` conclusion in the Git pack/receive-pack protocol below
rather than in a hypothetical server hook.

I did **not** merely re-confirm the round-1 fixes: I re-checked the whole goal on
the revised documents and found two new design gaps and one nit that the revision
introduced or left behind, plus one round-1 item that is only half-resolved.

---

## Adjudication Of Round-1 Findings

### R1-#1 `workflow_run queued` action — **RESOLVED**

The nonexistent `workflow_run` action `queued` is gone; the real triple is used
throughout.

- `GITHUB_SERVICE_CONTRACT.md:184` (CI-02): "Run creation emits action
  `requested` with status `queued`; later actions are `in_progress` and
  `completed`."
- `docs/proof-scenarios/p01-one-clean-pr.md:17-20`, `p02:20-22`, `p04:7-9,18-20`,
  `p08:20-22` all say action `requested` with status `queued`.
- `DTU_GITHUB_SPEC.md:420-422` ("status `queued` … action `requested`"),
  `:448-456`, `:491-497`.

No stray `action=queued` remains. The worklog records the acceptance at `:493`.

### R1-#2 PR snapshot prematurely settled as a REST GET — **RESOLVED**

The traces no longer bake in `GET .../pulls/{pull_number}`; they use a
transport-neutral `PR SNAPSHOT` interaction whose source is explicitly unsettled.

- `docs/proof-scenarios/README.md:28-30` defines `PR SNAPSHOT` with "Its source
  remains unsettled: targeted REST read, signed `pull_request` webhook, or both."
- Controlled-surface inventory `README.md:93-94`: "PR snapshots, implemented by
  the still-unsettled choice of `GET …/pulls/{pull_number}`, signed
  `pull_request` webhooks, or both."
- `p01:6-7`, `p02:8-10`, `p03:6-7,14-15`, `p08:9-11`, `p13:6-7` all use
  `PR SNAPSHOT … through the selected, still-TBD source`.

This is consistent with `GITHUB_SERVICE_CONTRACT.md:182` (PR-01 "endpoint source
unresolved") and worklog `:211`.

### R1-#3 `--force-with-lease` server-side compare-and-swap — **REJECTED with adequate evidence (the rejection is correct)**

Round 1 claimed the guarded `R` update is "a client-side check … not an atomic
server-side compare-and-swap." Grounded in the receive-pack protocol, that
claim is **wrong for the explicit-SHA form**, and the revision's rejection (see
worklog `:501-509`) is sound.

Protocol grounding (not a custom hook, not a local-only guard):

1. Git's push protocol sends each ref update as a command
   `<old-oid> SP <new-oid> SP <ref>` in the receive-pack command list
   (`Documentation/technical/pack-protocol.txt`, `update = old-id SP new-id SP
name`).
2. `git push --force-with-lease=refs/heads/R:<expected-old-SHA>` transmits
   `<expected-old-SHA>` as the command's `<old-oid>`, and uses the force flag
   only to bypass the fast-forward/ancestry check — not to bypass the old-oid.
3. On the server, `git receive-pack` applies each update through a ref
   transaction that takes a ref lock, reads the ref's current value under that
   lock, and performs an atomic compare-and-swap against the transmitted
   `<old-oid>`. If the current value ≠ `<expected-old-SHA>`, the transaction
   fails ("cannot lock ref … but expected …") and the update is rejected. This
   CAS is independent of the force flag.
4. `git-http-backend` execs the same `git receive-pack`, so this holds over Git
   smart HTTP. Because smart-HTTP advertisement and the push POST are separate
   requests, the explicit-SHA lease is in fact evaluated against the ref's real
   value at apply time, which is _stronger_, not weaker, than trusting the
   advertisement.

The distinction from a plain `--force` (which sends the currently advertised
value as `<old-oid>`, so its CAS passes trivially) is exactly what makes the
explicit lease meaningful. No `pre-receive` hook and no local-only guard is
required.

The revised documents state this correctly and honestly:

- `GITHUB_SERVICE_CONTRACT.md:180` (GIT-02): "Rebuilding `R` may use explicit
  `--force-with-lease=refs/heads/R:<expected-old-SHA>`; receive-pack rejects the
  update if the server ref no longer matches that old object ID."
- `docs/proof-scenarios/p08-suffix-rebuild.md:15-17`: "push `R2` using explicit
  `--force-with-lease=refs/heads/R:R1`; normal receive-pack processing rejects
  the update if the server ref no longer matches `R1`."
- `DTU_GITHUB_SPEC.md:412-416`: rejection "by normal receive-pack processing …
  no custom strengthening hook" (paraphrase of `:415`).

The one residual risk round 1 raised — the twin becoming _stronger_ than
production via a hook — is explicitly foreclosed by the "normal receive-pack
processing" wording. Rejection stands.

### R1-#4 P6 rerun correlation key — **RESOLVED**

The correlation key is now the GitHub-faithful triple, and the two rerun/dispatch
mechanisms are no longer pre-judged.

- `MERGE_HERDER_PROOF_SCENARIOS.md:211-213`: "correlated by `(run_id,
run_attempt, head_sha)`. A GitHub rerun may increment `run_attempt` while
  retaining the original `run_id`; a dispatch may create a new run ID."
- `docs/proof-scenarios/p06-unpause-unchanged.md:16-19`; `CI-02`
  (`GITHUB_SERVICE_CONTRACT.md:184`) uses the same triple.

### R1-#5 `409` (terminal-run) Pause race — **RESOLVED**

The sharper ordering is now an explicit second run.

- `MERGE_HERDER_PROOF_SCENARIOS.md:178-181` (P5 second run: terminal/success,
  withheld webhook, Pause, cancel returns `409`, then deliver the withheld
  success).
- `docs/proof-scenarios/p05-pause-race.md:16-25`.
- Modeled server-side at `DTU_GITHUB_SPEC.md:468` ("A completed run returns
  `409`").

### R1-#6 non-authoritative workflow at the same SHA — **RESOLVED at the scenario level; see New Finding N1 for the downstream gap**

The proof now exists:

- `MERGE_HERDER_PROOF_SCENARIOS.md:331-332` (P11 action 5) and `:339` (only the
  configured authoritative workflow may authorize landing).
- `docs/proof-scenarios/p11-event-disorder.md:20-27`.
- Backed by CI-02's `workflow_id` filter (`GITHUB_SERVICE_CONTRACT.md:184`).

The scenario is correct; the downstream twin cannot yet emit the event it
requires (New Finding N1).

### R1-#7 negative webhook-signature scenario — **RESOLVED at the scenario level; see New Finding N1 for the downstream gap**

- New scenario P14 (`MERGE_HERDER_PROOF_SCENARIOS.md:383-390`) and
  `docs/proof-scenarios/p14-forged-webhook.md`.
- The `401` rejection is code-confirmed:
  `src/routes/api/v1/github-webhook/+server.ts` returns
  `{ status: 401 }` on `InvalidGithubWebhookSignatureError`, and `400` for
  missing delivery headers.
- The controlled-surface inventory lists the need at `README.md:102`
  ("missing/invalid-signature webhook rejection before durable deduplication").

Scenario resolved; DTU control-operation list has not absorbed it (New Finding
N1).

### R1-#8 AUTH-06 requires `read:org`, but only `read:user`/`user:email` are requested — **contract half RESOLVED; DTU/twin half STILL UNRESOLVED (folded into New Finding N2)**

The contract now records the defect exactly and forbids modeling it as working:

- `GITHUB_SERVICE_CONTRACT.md:78-82`: "Those current scopes do not authorize the
  organization-membership read required by `AUTH-06` … Organization-owned login
  is therefore a known broken contract … The proof tool must not model `AUTH-06`
  as succeeding with the currently documented scopes."

The underlying code defect is still present and still confirms the finding
(review only, no edit): `src/lib/server/github-owner.ts:66-73` calls
`getMembershipForAuthenticatedUser({ org: githubAppOwner }).catch(() => null)`
and returns `allowed: membership?.data.state === 'active'`, so a `403`/`404`
from the missing scope collapses to `allowed: false` for every org member. This
is acceptable to leave as-is here because auth is being replaced in a separate
worktree (worklog `:237-269`), and the contract records the constraint.

However, round-1 #8's second half — "make the DTU reject when the presented
token lacks the scope so the twin actually catches this" — is **not** reflected
in the downstream `DTU_GITHUB_SPEC.md`. See New Finding N2.

### R1-#9 CI-01 hidden facts (pusher identity, trigger scoping) — **RESOLVED**

Both load-bearing facts are now explicit contract claims:

- `GITHUB_SERVICE_CONTRACT.md:183` (CI-01): "An App installation-token push of
  `R` causes an identifiable authoritative workflow run … The configured
  workflow explicitly matches the release branch/ref pattern. The proof does not
  substitute an Actions `GITHUB_TOKEN`, whose triggered events ordinarily do not
  create another workflow run."
- `docs/proof-scenarios/p01:11-13`, `p02:14-16` ("the fixture workflow explicitly
  matches this release branch/ref pattern").

### R1-#10 nits — **RESOLVED**

- "deleted PR returns 404" removed: `DTU_GITHUB_SPEC.md:437-439` now says an
  unknown/inaccessible PR is `404`, a closed PR remains readable with state
  `closed`, and source-branch deletion is a real Git-ref check.
- PR-01 deleted-source caveat added: `GITHUB_SERVICE_CONTRACT.md:182`
  ("Source-ref deletion requires a real Git ref check because a PR snapshot may
  retain the last head SHA/ref").
- `push` `forced` flag added to HOOK-04: `GITHUB_SERVICE_CONTRACT.md:144`.

---

## New Findings (fresh pass over the whole goal)

### N1. The downstream DTU control surface does not yet support the two scenarios added to close R1-#6 and R1-#7

**design** (downstream draft; executability/completeness)

The two new proofs require test-control capabilities that `DTU_GITHUB_SPEC.md`
does not enumerate, so a proof tool built from that spec as written could not run
them:

1. **Non-authoritative `workflow_run` (P11):** the twin models exactly one
   workflow — `DTU_GITHUB_SPEC.md:228` ("records the one workflow treated as
   authoritative for this repository") — and creates runs _only_ from release-ref
   pushes (`:229-230`, `:448`). The webhook-control operations
   (`DTU_GITHUB_SPEC.md:290-301`) can only send/redeliver/duplicate a _stored_
   delivery that originated from a modeled event. There is no capability to emit
   a `workflow_run` for a second, non-authoritative `workflow_id` at the current
   SHA, which is exactly what `p11-event-disorder.md:22-23` and
   `MERGE_HERDER_PROOF_SCENARIOS.md:331-332` require. The controlled-surface
   inventory (`docs/proof-scenarios/README.md:101`) also lists only generic
   `workflow_run` deliveries and does not call out the non-authoritative case, so
   this gap exists in both the inventory and the downstream spec.
2. **Missing/invalid-signature delivery (P14):** every webhook-control operation
   in `DTU_GITHUB_SPEC.md:290-301` signs with "a valid signature." There is no
   operation to deliver a raw body with a missing or invalid
   `X-Hub-Signature-256`, which `p14-forged-webhook.md:5-8` requires. Here the
   inventory is ahead of the spec — `README.md:102` already lists the need — so
   this half is purely a downstream-draft lag.

Neither is a correctness error in the accepted scenarios; both are places where
the downstream twin draft must grow one control capability each before P11's
`workflow_id`-filter proof and P14 can execute. Because the prompt scopes
`DTU_GITHUB_SPEC.md` as a downstream draft, this is design-level rather than
blocking, but it should be recorded so the two scenarios are not silently
dropped when the proof tool is built.

### N2. The downstream DTU spec models no OAuth scopes, so the twin cannot enforce the AUTH-06 "must not succeed with current scopes" directive

**design** (downstream draft; this is the unresolved half of R1-#8)

`GITHUB_SERVICE_CONTRACT.md:82` mandates: "The proof tool must not model
`AUTH-06` as succeeding with the currently documented scopes." But the DTU
OAuth model tracks no scopes at all:

- authorize (`DTU_GITHUB_SPEC.md:334-341`) validates only client and callback,
  not requested scopes;
- token exchange (`:348-354`) issues a bearer token with no recorded scope;
- `GET /user/memberships/orgs/{org}` (`:358-366`) returns membership state or
  `404` regardless of token scope.

Consequently a twin built to this spec would let an org-membership read succeed
even under `read:user`/`user:email`, which is precisely the "green-but-wrong"
condition the contract forbids and which round-1 #8 asked the twin to catch. The
risk is latent while installations are user-owned (`github-owner.ts:59-64`
short-circuits before the call) and while OAuth/DTU integration is deferred
(worklog `:305`), but the downstream spec currently contradicts the contract
directive rather than deferring it. Fix by either modeling the scope rejection on
the membership endpoint or explicitly marking that endpoint non-conformant for
AUTH-06 until scopes are modeled.

### N3. P6 is classified MVP but its enabling mechanism is Deferred and unbuildable on the current twin

**design** (scope/executability tension, honestly parked but mislabeled)

P6 sits under "## MVP Release-Gating Scenarios"
(`MERGE_HERDER_PROOF_SCENARIOS.md:51`) and requires a _fresh authoritative CI
run for an unchanged `R0`_ (`:203-214`) explicitly _without_ a no-op push. The
only GitHub-valid mechanisms — rerun or `workflow_dispatch` — are classified
**Deferred** at `GITHUB_SERVICE_CONTRACT.md:206` (CI-06) and are listed under
DTU **Unsupported Behavior** (`DTU_GITHUB_SPEC.md:515`). The twin creates runs
only from release-ref pushes (`:230`, `:448`), so with the no-op push forbidden
it has no way to birth the fresh run P6 needs. P6 is therefore an MVP scenario
whose mechanism is simultaneously deferred and unbuildable.

This is disclosed as a contract hole (`p06:24-25`, `README.md:115`), and the DTU
acceptance list correctly omits P6 (its eight scenarios at
`DTU_GITHUB_SPEC.md:533-594` cover intake, OAuth, hotfix, pause race,
source-move, stale-base, disorder, and `act`), so the artifacts are internally
consistent — but the "MVP" label on P6 is not, since an MVP proof cannot be run
until CI-06 is un-deferred and the twin gains a rerun/dispatch surface. Recommend
either promoting the chosen fresh-CI mechanism out of CI-06's Deferred bucket (it
is a genuine MVP dependency of accepted Pause/Unpause semantics, worklog
`:89-90`) or marking P6 as blocked-pending-CI-06 rather than MVP-ready.

### N4. P10's "non-forced push from expected M" conflates the receive-pack rejection mechanism

**nit**

`docs/proof-scenarios/p10-stale-base.md:8` says MergeHerder will "attempt
non-forced push of `main` from expected `M` to `R0`; Git rejects it as
non-fast-forward/stale." A _non-forced_ push does not carry a lease on `M`: over
smart HTTP the client sends the freshly advertised value (`M1`) as `<old-oid>`,
and the rejection is produced by the fast-forward/ancestry check (`R0` is not a
descendant of `M1`), not by a compare-and-swap against `M`. The observable
outcome in the pass conditions (`main` stays at `M1`, no forced update) is
correct and GIT-03 (`GITHUB_SERVICE_CONTRACT.md:181`) is right; only the
"from expected `M`" phrasing implies a lease the non-forced landing path does
not use. Either drop "from expected `M`" or state that stale-base landing relies
on the ancestry check rather than an old-oid lease.

---

## What Is Correct And Should Not Be "Fixed"

- The `--force-with-lease` treatment (GIT-02, `p08`, `DTU:412-416`) is now
  protocol-accurate and explicitly avoids a strengthening hook — see the R1-#3
  grounding above.
- The `(run_id, run_attempt, head_sha)` correlation key, the `409` terminal-run
  Pause ordering, and the non-authoritative-`workflow_id` filter are all faithful
  to GitHub's Actions/webhook semantics.
- Keeping the `PR SNAPSHOT` transport unsettled, and marking the P6 fresh-CI
  mechanism, P7 paused-run policy, and P12 run-inspection endpoint as explicit
  contract holes, correctly refuses to invent surfaces.
- The added P14 forged-webhook proof, the CI-01 pusher-identity/trigger-scoping
  claims, and the closed-PR/deleted-source PR-01 caveats are minimal and
  code/protocol-faithful (webhook `401` confirmed in the route;
  `github-owner.ts` confirms the AUTH-06 collapse-to-denied path).
- No adventitious product behavior or new GitHub endpoint was introduced by the
  revision; the controlled-surface inventory (`README.md:90-104`) remains a small
  honest set and the DTU still fails loudly on uncontracted requests
  (`DTU_GITHUB_SPEC.md:317-318`, `:530-531`).

---

## Overall Assessment

The revision resolves every substantive round-1 finding and correctly rejects the
one that was wrong. R1-#1, #2, #4, #5, #9, and #10 are cleanly resolved; #6 and
#7 are resolved at the scenario level; #8 is resolved at the contract level. R1-#3
is properly rejected: grounded in the receive-pack command format and the ref
transaction's atomic compare-and-swap, an explicit
`--force-with-lease=refs/heads/R:<expected-old-SHA>` is enforced server-side by
ordinary `git receive-pack` (and by `git-http-backend` over smart HTTP), so the
guarded `R` rebuild neither depends on a local-only guard nor requires a custom
server hook.

The remaining gaps are downstream and non-blocking for the _scenario/interaction
contract_, which is the artifact the prompt asks to validate before a proof tool
is designed. Two of them (N1, N2) are lags in the explicitly-downstream
`DTU_GITHUB_SPEC.md`: the twin cannot yet emit a non-authoritative `workflow_run`
(P11) or an invalid-signature delivery (P14), and it models no OAuth scopes, so it
cannot enforce the contract's own "AUTH-06 must not succeed with current scopes"
rule. N3 is a real MVP-versus-Deferred inconsistency around P6's fresh-CI
mechanism that should be reclassified rather than left labeled MVP. N4 is a
one-line wording nit about the stale-base rejection mechanism.

None of N1–N4 requires a larger interaction surface or reopens a resolved
scenario; each is either a downstream twin capability to add before that scenario
can execute, a classification to correct, or a phrasing to tighten. The
scenario-first contract itself is correct, complete for the accepted behavior,
and free of invented surfaces, and is ready to drive the separate proof-tool
design once N1–N3 are folded into that tool's charter.

No product code or reviewed design document was modified.
