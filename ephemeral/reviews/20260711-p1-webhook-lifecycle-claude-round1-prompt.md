# P1 Webhook Lifecycle Review - Claude Round 1

## Goal

Review roadmap phase 2 as proof, not merely code. Immutable pending push and
workflow bodies must be deliverable over real HTTP with GitHub delivery/event
headers and HMAC-SHA256 over exact bytes. Control must support withholding,
arbitrary order, invalid signatures, attempts, same-GUID/body redelivery, and
new-GUID/same-body semantic duplication. Authoritative runs must move through
queued, in-progress, and completed while emitting exactly correlated immutable
events. Delivery and transition failures must remain observable.

Phase 3 cancellation and workflow execution, and later MergeHerder/live proof,
are explicitly outside this slice.

## Instructions

Review the complete diff, roadmap, proof doc, implementation, and real HTTP
test. Run proof commands. Find protocol errors, races, mutable-body mistakes,
incorrect signature or identity behavior, invalid state transitions, swallowed
delivery failures, and proof overstatement. Do not edit implementation or docs.
Write the exact prompt and findings only to:

`ephemeral/reviews/20260711-p1-webhook-lifecycle-claude-round1.md`

Label findings critical, bug, design, or nit using the established consensus
definitions. Finish with an explicit verdict on proof sufficiency for phase 2.
