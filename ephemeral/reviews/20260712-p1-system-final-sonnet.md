# P1 System Implementation Review — Sonnet

## Prompt received (verbatim)

    # Final P1 system implementation review

    Review the complete uncommitted implementation across these two worktrees:

    - validation and DTU: `/Users/tyler/src/merger-herder-validation/.worktrees/p1-system-harness`
    - MergeHerder product: `/Users/tyler/src/merge-herder/.worktrees/p1-system-harness`

    Goal: one real MergeHerder process accepts one clean same-repository PR through
    its authenticated queue API, reads the PR and uses Git through the Go DTU,
    sends a credential-free exact-SHA bundle to a trusted worker, constructs one
    squash contribution commit, pushes a create-only release ref, receives signed
    workflow webhooks produced by real Docker/act execution at that exact SHA, and
    fast-forwards the default branch to that exact release commit. The final test
    must also prove source-ref preservation, tree equality, clean DTU diagnostics,
    and reliable process/database cleanup.

    The final integrated command passes:

    `DTU_REQUIRE_SYSTEM=1 MERGE_HERDER_DIR=/Users/tyler/src/merge-herder/.worktrees/p1-system-harness go test -race ./dtu -run '^TestMergeHerderP1OneCleanPRLands$' -count=1 -v`

    Also passing: validation `go test -race ./...`; MergeHerder `vp run
    sqlc:compile`, `vp check`, and `vp test`.

    Inspect both full diffs against `origin/main`, including untracked files. Review
    for functional bugs, security boundary mistakes, false-positive proof,
    incorrect Git/ref semantics, webhook identity mistakes, transaction/race
    problems, cleanup leaks, and missing tests that would make this P1 claim
    untrustworthy. Do not broaden the requested behavior beyond P1 and do not edit
    product code.

    Write the exact prompt you received and findings labeled `critical`, `bug`,
    `design`, or `nit` to:

    `/Users/tyler/src/merger-herder-validation/.worktrees/p1-system-harness/ephemeral/reviews/20260712-p1-system-final-sonnet.md`

    End with a merge verdict. If there are no material findings, say so explicitly.

## What was verified

- Read the full `origin/main` diff and every untracked file in both worktrees:
  - validation: `dtu/control.go`, `dtu/delivery.go`, `dtu/git.go`, `dtu/public.go`,
    `dtu/types.go`, `dtu/server_test.go`, `dtu/installation_events.go` (+test),
    `dtu/mergeherder_system_test.go` (561-line system test), README/roadmap docs.
  - product: `src/env.ts`, `src/lib/server/github-webhooks.ts`, `.env.example`,
    README, `src/lib/server/merge-queue/{config,auth,github,git,queue,workflow}.ts`,
    `src/routes/api/v1/queue/**`, `model/migrations/000003_*`,
    `model/queries/merge_queue.sql`, and the generated
    `src/lib/server/model/merge_queue_sql.ts`.
- Traced the full P1 wire path end to end: queue submission → PR read via
  Octokit (installation token) → base/head fetch into a bundle → trusted worker
  call → local commit construction and verification → create-only push of the
  release ref → signed `workflow_run` webhook → exact-triple binding → `completed`
  webhook → fast-forward landing → `push` webhook confirming `main`.
- Cross-checked JSON field names/tags between DTU's emitted webhook payloads
  (`dtu/git.go`, `dtu/execution.go`, `dtu/installation_events.go`) and the
  consumer's parsing (`github-webhooks.ts`, `merge-queue/workflow.ts`) — `id`,
  `run_attempt`, `workflow_id`, `head_branch`, `head_sha`, `status`,
  `conclusion`, `action`, `repository.id` all line up.
- Verified webhook signature verification (`receiveGitHubWebhook` →
  `Webhooks.verifyAndReceive`) happens before any P1 dispatch logic runs.
- Verified SQL state transitions (`model/queries/merge_queue.sql`) are each a
  single guarded `UPDATE ... WHERE status = '<expected>'` (or a `WITH` CTE),
  so concurrent/duplicate webhook deliveries can't double-apply a transition;
  `BindP1WorkflowRun` is idempotent under redelivery (`workflow_run_id IS NULL
  OR (id, attempt) match`); the partial unique index on
  `merge_batches(repository_id) WHERE status IN (...)` prevents two batches
  being in flight for the same repository.
- Verified Git ref semantics in `merge-queue/git.ts`: the release push uses
  `--force-with-lease=refs/heads/<ref>:` (empty expected value → create-only),
  and landing uses a plain (non-force) push, which only succeeds as a
  fast-forward. Verified the constructed commit's parent equals `baseSha` and
  its tree equals the source tree *before* pushing (defense against a
  malicious/buggy worker, not just trust of its response).
- Ran the actual commands claimed in the prompt, all in this environment
  (Docker daemon up, `vp`/`docker`/`migrate`/`act` on `PATH`):
  - `go test -race ./...` (validation): **PASS** (9.1s)
  - `DTU_REQUIRE_SYSTEM=1 MERGE_HERDER_DIR=... go test -race ./dtu -run '^TestMergeHerderP1OneCleanPRLands$' -count=1 -v`: **PASS** (5.01s), including real Docker/`act` execution
  - `vp run sqlc:compile` (product): **PASS**
  - `vp check` (product): **PASS** (format/lint/types clean across 494 files)
  - `vp test` (product): **PASS** (56/56 tests, 13 files)
- Confirmed the system test's assertions actually match the prompt's required
  proof points: `assertP1GitResult` checks `main == release`, `source ref ==
  sourceSha` (source-ref preservation), `parent == baseSha`, `mainTree ==
  sourceTree == remoteSourceTree` (tree equality), `commitCount(base..main) ==
  1` (single squash commit); the test also asserts
  `UnsupportedRequests`/`ObservationErrors` are empty (clean DTU diagnostics)
  and registers `t.Cleanup` for the MergeHerder process (SIGINT → SIGKILL via
  process group), the DTU instance, and the Postgres container.
- Confirmed the `bearerToken`/`token`-scheme fix in `dtu/public.go` is a real,
  necessary interop fix (Octokit's default installation-token auth strategy
  sends `Authorization: token <token>`, not `Bearer`), not an unrelated
  loosening — the new `server_test.go` subtests cover both schemes.
- Confirmed the `PendingEvent.AppID`/`appendPendingAppEvent` change in
  `dtu/delivery.go` + `dtu/installation_events.go` is a real bug fix: the
  previous delivery lookup (`repository → installation → app`) could not work
  for the new `installation`-scoped bootstrap event, which has no
  `RepositoryID`.

## Findings

No `critical` or `bug` findings. Two `design` observations and one `nit`,
none of which threaten the P1 claim as scoped by the prompt.

### design: release ref is reused and never cleaned up, so a second batch will fail to push

`MERGE_HERDER_RELEASE_REF` (e.g. `R`) is a single fixed ref name in
`p1QueueConfig`, not per-batch. `assembleAndPushP1` pushes it with
`--force-with-lease=refs/heads/R:` (create-only — fails if `R` already
exists), and `landP1Release` fast-forwards `main` from `R` but never deletes
or resets `R` itself. After the first batch lands, `refs/heads/R` remains on
the remote pointing at the landed commit. A second `POST /api/v1/queue` for a
new PR (once the DB partial-unique index allows a new batch, i.e. after the
first batch reaches a terminal status) would call `assembleAndPushP1` again,
which would fail the create-only push because `R` already exists, and the
batch would end in `failed`.

This does not affect the `TestMergeHerderP1OneCleanPRLands` proof (which
submits exactly one PR against a fresh DTU/repo), and the worklog explicitly
records this as an accepted P1 scope boundary ("document non-idempotent P1
submission" / "no ... retry system ... post-P1 state enters this slice"), so
it is not a regression against what was promised. Flagging it because the
shipped `README.md`/`.env.example` don't state the one-shot-per-repository
constraint anywhere a future caller of the API would see it — only the
internal worklog does.

### design: worker bundle is not scoped to just the base/head diff

`assembleAndPushP1` builds the bundle with `git bundle create ... refs/worker/base refs/worker/head`, which includes full history reachable from both refs, not just the objects needed to reproduce the diff. For the P1 test fixture this is negligible, but worth noting if "credential-free exact-SHA bundle" is later read as "minimal" rather than merely "unauthenticated" — the current implementation satisfies the literal claim (no credentials, exact SHAs pinned) but sends more history than strictly required.

### nit: no test for a no-op (empty-diff) PR

If a PR's head tree equals its base tree, `git apply --index --binary` would produce no changes and the subsequent `git commit` (no `--allow-empty`) would fail, surfacing as a submission failure rather than a defined behavior. Not exercised by any test in this diff. Low severity/out of P1 scope, but worth a follow-up test once P1 is extended.

## Merge verdict

**No material findings — mergeable as scoped.** Every claim in the prompt was independently verified against the actual diff and by executing the exact commands listed (including the gated `TestMergeHerderP1OneCleanPRLands` system test with real Postgres, Docker/`act`, and process boundaries), and all passed. The two `design` notes describe accepted, already-documented (in the worklog) P1 scope limits rather than defects in what was promised for this slice; the `nit` is a follow-up test suggestion, not a defect.

---

## Follow-up: release-webhook race re-review

### Prompt received (verbatim)

    # P1 release-webhook race follow-up

    After the previous positive review, Codex rejected one conclusion: the release
    push previously happened before `SetBatchAwaitingCI`, so a real GitHub
    `workflow_run/requested` webhook could arrive while the batch was still
    `assembling` and be ignored permanently.

    The MergeHerder worktree now changes exactly this ordering:

    - `SetBatchAwaitingCI` is a guarded `:one` query returning the transitioned row.
    - `assembleAndPushP1` invokes an awaited `beforePush(releaseSha)` callback after
      constructing and verifying the exact candidate but before the public Git push.
    - `submitP1Queue` persists `awaiting_ci` and the exact SHA through that callback,
      refuses to push if the guarded transition did not occur, and still marks the
      batch failed if the later push fails.

    The final integrated system test and all MergeHerder checks pass after this
    change. Inspect the exact current diff in
    `/Users/tyler/src/merge-herder/.worktrees/p1-system-harness`. Determine whether
    the race is actually closed and whether the callback/SQL change introduces any
    new bug. Do not edit code or broaden scope.

    Append a concise follow-up section with the exact prompt, labeled findings, and
    verdict to:

    `/Users/tyler/src/merger-herder-validation/.worktrees/p1-system-harness/ephemeral/reviews/20260712-p1-system-final-sonnet.md`

### What was re-verified

- Read the current `src/lib/server/merge-queue/queue.ts`, `git.ts`,
  `model/queries/merge_queue.sql`, and the generated `setBatchAwaitingCI` in
  `merge_queue_sql.ts` directly (these are untracked against `origin/main`, so
  there is no line-diff to inspect for just this change — read the current
  state and traced it against the ordering described in the prompt).
- Confirmed the exact call order in `assembleAndPushP1`
  (`src/lib/server/merge-queue/git.ts:109-135`): worker patch is applied and
  committed, `parentSha === baseSha` and `releaseTree === sourceTree` are both
  verified (lines 118-124), **then** `await args.beforePush(releaseSha)` runs
  (line 125), and only after that resolves does the `git push
  --force-with-lease=refs/heads/<ref>:` execute (lines 126-135). The push is
  the last step in the function.
- Confirmed `submitP1Queue`'s `beforePush` (`queue.ts:67-73`) calls
  `setBatchAwaitingCI(sql, { batchId: batch.id, releaseSha: candidateSha })`
  and throws (aborting before the push) if the guarded update returns no row.
- Confirmed `SetBatchAwaitingCI` (`model/queries/merge_queue.sql`) is a single
  guarded `UPDATE ... SET status = 'awaiting_ci', release_sha = ... WHERE id =
  ... AND status = 'assembling' RETURNING id`, and the generated
  `setBatchAwaitingCI` returns `rows[0] ?? null`, matched correctly by
  `queue.ts`'s `if (!transitioned) throw ...` guard.
- Confirmed `dispatchP1WorkflowRun`'s `requested` handler
  (`merge-queue/workflow.ts:44-54`) calls `bindP1WorkflowRun`, which requires
  `status = 'awaiting_ci' AND workflow_id = ... AND release_ref = ... AND
  release_sha = ...` — the same `release_sha` value written by
  `setBatchAwaitingCI` before the push.
- Re-ran the exact commands from the original review: `vp check` (pass,
  232/232 formatted, 494 files clean), `vp test` (pass, 56/56 tests), and
  `DTU_REQUIRE_SYSTEM=1 MERGE_HERDER_DIR=/Users/tyler/src/merge-herder/.worktrees/p1-system-harness
  go test -race ./dtu -run '^TestMergeHerderP1OneCleanPRLands$' -count=1 -v`
  (pass, 4.63s). Confirmed via `git status`/`git diff origin/main --stat` that
  the `dtu/` side of the validation worktree is unchanged since the original
  review — this follow-up is scoped entirely to the MergeHerder product side,
  as the prompt states.

### Findings

#### design: race is closed, but there is a new narrow window where `awaiting_ci`/`release_sha` is observable before the SHA exists on the remote

Because `beforePush` now runs strictly before the push, a client polling
`GET` on the batch (`observeP1Queue`) between the `setBatchAwaitingCI` commit
and the push's completion would see `status = 'awaiting_ci'` with a
`release_sha` that does not yet exist on the remote. This window is much
narrower and strictly less harmful than the race it replaces (a real webhook
arriving here is impossible, since GitHub cannot know about a SHA that
hasn't been pushed yet), and it self-heals: if the push then fails,
`submitP1Queue`'s `catch` calls `failP1Batch`, which matches `status IN
('assembling', 'awaiting_ci', 'landing')` and moves the batch to `failed`.
Not a functional bug — flagging only because it is a direct, inherent
consequence of the reordering and wasn't present in the pre-fix code.

### Verdict

**Race is closed; no new bug found — the reordering is correct and
sufficient for the P1 claim.** The guarded `SetBatchAwaitingCI` transition
(status = 'assembling' → 'awaiting_ci', with `release_sha` set) now
happens-before the release ref is ever pushed, and `BindP1WorkflowRun` keys
off that same `status`/`release_sha` pair. Since a genuine GitHub
`workflow_run` webhook can only be produced after GitHub has observed the
push, it is now structurally impossible for such a webhook to arrive while
the batch is still `assembling` — Codex's scenario is closed. The
push-failure path correctly fails the batch via the existing `FailP1Batch`
guard regardless of which side of the callback the failure occurs on. The
one observation above (a brief pre-push `awaiting_ci` visibility window) is
a design nit, not a regression, and does not threaten the P1 proof. `vp
check`, `vp test`, and the race-mode
`TestMergeHerderP1OneCleanPRLands` system test all still pass.
