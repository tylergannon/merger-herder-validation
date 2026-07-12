# P1 Ref Events Proof

## Boundary

This slice completes the named Git transport contracts and creates the
GitHub-side state needed by the later delivery and CI lifecycle layers. It does
not send webhooks or execute workflows.

For an authenticated `git-receive-pack`, DTU snapshots the repository's real
branch refs before and after `git-http-backend` returns. Only actual ref
differences create state:

- one immutable pending `push` event per changed branch, containing the actual
  `before` and `after` object IDs, creation/deletion/forced flags, and App
  sender; and
- when the configured release ref moves to an object, one new queued workflow
  run plus an immutable pending `workflow_run` `requested` event bound to that
  exact ref and SHA.

Rejected pushes leave the ref maps equal, so they create neither events nor
workflow runs. Event bodies are serialized once at creation and retained as raw
JSON for the later signing and delivery layer.

Receive-pack observation is serialized per repository so overlapping pushes
cannot cross-attribute a stale `before` SHA. The CGI response is buffered until
the post-receive ref snapshot and event transaction complete. A snapshot failure
is recorded diagnostically and returned as a server failure rather than silently
reporting success without an event.

## Real-Git Scenario

`TestGITRefTransitionsAndEventCreation` proves the behavior through real Git
smart HTTP:

1. Fetch a reachable commit by exact SHA.
2. Reject an unavailable SHA and invalid installation token.
3. Reject a push made with a read-only token without creating state.
4. Create release ref `R` and observe its push event and queued run.
5. Fast-forward `R` and observe a distinct event and run for the new SHA.
6. Rebuild `R` with the exact current force-with-lease value.
7. Reject a stale lease without moving `R` or creating state.
8. Fast-forward `main` to the rebuilt release commit and create only its push
   event.
9. Reject a non-fast-forward `main` update without moving the ref or creating
   state.
10. Delete a source branch and record the actual SHA-to-zero transition.
11. Race two exact-lease updates against the same pre-existing ref; exactly one
    succeeds and exactly one event records the real old SHA and winning SHA.

Each accepted release movement creates a unique run ID with attempt `1`, event
`push`, status `queued`, null wire conclusion, configured workflow identity,
and the exact release branch and head SHA.

## Remaining Layers

Pending events are inspectable only through the control API. Roadmap phase 2
must add signed HTTP delivery, immutable redelivery, semantic duplication,
withholding/reordering, delivery attempts, and later workflow-run transitions.
Roadmap phases 3 through 5 still own cancellation, real workflow execution,
MergeHerder system proof, Octokit proof, and live-GitHub conformance.
