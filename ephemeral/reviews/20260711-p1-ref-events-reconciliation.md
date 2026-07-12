# P1 Ref Events Consensus Decision

Decision: Roadmap phase 1 is complete. Real Git transport proves `GIT-01`
through `GIT-03`; accepted ref changes derive immutable pending push events and
configured release changes derive exact queued workflow state. Delivery and
later run transitions remain phase 2.

Evidence: `dtu/ref_events_test.go`, `docs/p1-ref-events-proof.md`, the updated
roadmap, race-enabled repeated real-Git proof, Codex self-review, and three
Claude Opus review rounds.

Accepted findings: serialize receive-pack observation per repository; fail
loudly and diagnostically when post-receive observation fails; prove the
same-existing-ref stale-before race; include `forced` and App `sender` in the
immutable body; align `run_attempt`; cover multiple exact reachable SHAs.

Rejected findings: none. Findings characterized as non-blocking were also
addressed when they materially improved the phase proof.

Review artifacts:

- `ephemeral/reviews/20260711-p1-ref-events-codex.md`
- `ephemeral/reviews/20260711-p1-ref-events-claude-round1.md`
- `ephemeral/reviews/20260711-p1-ref-events-claude-round2.md`
- `ephemeral/reviews/20260711-p1-ref-events-claude-round3.md`

Proof still required: final vet, race-enabled suite, vulnerability and diff
checks, plus PR state. Roadmap phase 2 still owns signing, delivery attempts,
redelivery, ordering, duplication, and workflow transitions after `queued`.
