# P1 Ref Events Codex Review

## Goal Reviewed

Complete roadmap phase 1 using real Git behavior: prove required fetch and ref
transition semantics, then derive immutable pending push events and queued
workflow state only from accepted ref changes.

## Findings

### Bug: overlapping receive-pack snapshots could misattribute transitions

Accepted and fixed. Independent receive-pack requests could snapshot the same
old refs and later report transitions with stale `before` values. Each
repository now owns a shared no-copy mutex that serializes the receive-pack
snapshot, Git update, post-snapshot, and event-recording sequence. Different
repositories remain independent.

### Design: delivery is intentionally absent

Accepted as the declared phase boundary. Pending raw event bodies are created
and inspected, but not signed or delivered. Phase 2 owns delivery attempts,
redelivery, ordering, duplication, and later workflow transitions.

### Critical

None remaining.

### Bug

None remaining after receive-pack serialization.

## Verdict

Ready for independent Claude review. The proof uses real Git smart HTTP and
checks both accepted and rejected updates against refs, pending events, workflow
runs, and mutation state. It does not claim webhook delivery or workflow
execution.
