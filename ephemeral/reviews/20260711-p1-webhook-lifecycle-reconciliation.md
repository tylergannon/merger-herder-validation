# P1 Webhook Lifecycle Consensus Decision

Decision: Roadmap phase 2 is complete and sufficiently proven. Real push and
workflow events cross real HTTP with exact-byte HMAC, controlled order,
attempts, redelivery, semantic duplication, and exact run lifecycle identity.

Evidence: `dtu/delivery_test.go`, `docs/p1-webhook-lifecycle-proof.md`, the
roadmap, race-enabled suite, and three Claude Opus rounds.

Accepted findings: deliver and parse a real-push event in-slice; return `502`
for transport/non-2xx delivery while retaining the attempt; reassert requested
queued identity; connect the delivered push ref/after fields to the actual Git
push SHA.

Rejected findings: none. Remaining nits do not affect phase-2 correctness.

Proof still required: final repository gates and PR state. Phase 3 owns Actions
cancellation and real workflow execution.
