# go-github-server adoption

worktree: `/Users/tyler/src/merger-herder-validation/.worktrees/dtu-veracity`
branch: `codex/dtu-veracity`

goal: Replace the DTU's hand-routed GitHub REST transport with `go-github-server` v0.2.0 while preserving the existing validation behavior.

correction: MergeHerder is the read-only system under test. This change must not add product features or modify the MergeHerder repository.
decision: Ship only generated routing for installation-token creation, pull-request retrieval, and workflow-run cancellation, plus the shared `go-github` v89 upgrade.
decision: Keep Git smart HTTP, webhooks, workflow execution, and the control plane owned by the DTU.
decision: Unused generated service methods may return their embedded unimplemented response; no endpoint allowlist is required.
excluded: P2 three-PR scenario changes, MergeHerder changes, and P2 review artifacts.
external_resource: `github.com/tylergannon/go-github-server` v0.2.0 resolves to commit `7ec969b2b313ee14ec3d3d47b1e8a2ac2d8e53e3`.
proof: `go test ./...`, `go test -race ./...`, `go vet ./...`, and `git diff --check` passed.
proof: `DTU_REQUIRE_SYSTEM=1 MERGE_HERDER_DIR=/Users/tyler/src/merge-herder/.worktrees/p1-system-harness go test -race ./dtu -run '^TestMergeHerderP1OneCleanPRLands$' -count=1 -v` passed; that MergeHerder tree has no content diff from its current `origin/main`.
review: Sonnet found two bugs: malformed numeric route parameters changed from JSON `404` to plain-text `400`, and generated service errors used a different JSON envelope from outer-adapter errors. It also requested migration-specific tests and noted that the P1 system proof covers token creation and pull retrieval but not cancellation.
fix: Prevalidate numeric parameters for all three generated routes, normalize generated GitHub errors through the existing DTU error writer, and add focused coverage for malformed paths and equal `422` envelopes. Existing cancellation race coverage exercises `CancelWorkflowRunByID` through `go-github` v89.
proof: After the fixes, focused compatibility tests, `go test ./...`, `go test -race ./...`, `go vet ./...`, `git diff --check`, and the unchanged-MergeHerder P1 system proof all passed again.
review: Sonnet round 2 independently reproduced the fixes, ran the full Go gates, and recommended the change as mergeable.
decision: Accept that unused methods embedded from the three generated services return the library's plain `501` and are not added to `UnsupportedRequests`. The user explicitly rejected an allowlist or extra protection for endpoints MergeHerder does not call.
remaining: Cancellation has real HTTP and `go-github` v89 coverage, but not yet an unchanged-MergeHerder/Octokit consumer proof. That belongs to the separate DTU-versus-real-GitHub veracity work and is not claimed by this adoption PR.
