# P1 MergeHerder System Harness Session

started: 2026-07-12 05:00 CST
validation_worktree: `/Users/tyler/src/merger-herder-validation/.worktrees/p1-system-harness`
validation_branch: `codex/p1-system-harness`
validation_base: `656918afdb495c6de57067d4af884f6256c6e22c`
product_worktree: `/Users/tyler/src/merge-herder/.worktrees/p1-system-harness`
product_branch: `codex/p1-system-harness`
product_base: `db472df`

## Goal

Complete roadmap phase 4 by implementing the minimal MergeHerder P1 queue slice
and proving it against real DTU, Postgres, worker, Git, webhook, Docker, and
`act` boundaries.

## Activity

- merge: validation PR #4 merged as `656918a Add scripted P1 workflow cancellation (#4)`.
- source_discovery: current MergeHerder `origin/main` has login, verified
  webhook persistence, and repository views, but no queue engine, submission
  operation, worker boundary, release-ref logic, or landing logic.
- decision: phase 4 therefore requires a minimal real product implementation,
  not only a harness wrapper.
- decision: choose an authenticated HTTP queue API as the unsettled product
  front door; GitHub labels/comments remain rejected.
- decision: choose a trusted HTTP worker returning one binary patch and commit
  message so MergeHerder itself owns exact release construction and Git pushes.
- skill_use: `session-worklog` source=pagerguild/core-tools -> preserve product
  boundary decisions, proof failures, review, and cross-repository publication.
- command: `vp install` completed in the isolated MergeHerder worktree; SQLc
  `1.31.1` and golang-migrate are available.
- review: Claude plan review found two critical, two bug, four design, and three
  nit issues; it agreed with the boundary choices but found the plan
  unexecutable/circular without corrections.
- accepted: extend DTU bootstrap events, align DTU and JWT clocks, prove landed
  tree equality, bind the exact workflow run triple, persist the batch before
  worker/push, use a create-only release guard, explicitly assert DTU diagnostic
  state, and document non-idempotent P1 submission.
- correction: the worker receives a base/head Git bundle rather than an
  installation token; it returns `git diff --binary` and MergeHerder retains all
  authenticated fetch, commit, and push authority.
- correction: user requires the full specified DTU, with minimalist
  implementation and proof; P1 is the first completed system slice, not the
  final product.
- correction: remove the uncommitted Node attestation framework; retain only
  Go DTU and system-harness behavior.
- command: `go test -race ./...` passed for the validation repository.
- command: `vp check` and `vp test` passed for the MergeHerder worktree.
- failure: the first system run exposed missing Octokit `token` authorization
  support, bigint decoding assumptions, an early Postgres migration race, and
  dev-server process-group cleanup.
- fix: accept GitHub-compatible `Bearer` and `token` installation auth, retry
  migrations until the forwarded database is ready, decode database IDs as
  strings, capture product logs on failures, and terminate the full dev-server
  process group.
- proof: `DTU_REQUIRE_SYSTEM=1 MERGE_HERDER_DIR=/Users/tyler/src/merge-herder/.worktrees/p1-system-harness go test -race ./dtu -run '^TestMergeHerderP1OneCleanPRLands$' -count=1 -v` passed in 4.76 seconds.
- review: Claude Sonnet independently inspected both complete diffs and reran
  every native and integrated check; it found no critical or bug-level issue.
- codex_finding: the original product ordering persisted `awaiting_ci` after
  pushing the release ref, so an immediate requested webhook could arrive while
  the batch was still `assembling` and be ignored.
- fix: persist the guarded `awaiting_ci` transition and exact release SHA after
  candidate verification but before the public push; refuse to push when that
  transition does not occur.
- review: Claude's focused follow-up confirmed the webhook race is structurally
  closed and the reordered failure path remains correct.
- proof: the final post-review system run passed in 4.69 seconds; product
  formatting, SQL compilation, type checks, and all 56 unit tests also passed.
