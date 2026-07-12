# P1 Ref Events Review - Claude Round 1

## Goal

Review roadmap phase 1 as a complete vertical slice. The runtime must prove the
required Git fetch, release-ref, force-with-lease, and default-branch semantics
through real Git smart HTTP. Accepted branch changes must create immutable
pending push-event bodies from the actual committed before/after refs. A
configured release-ref change must additionally create one distinct queued
authoritative workflow run and pending `workflow_run` `requested` body bound to
the exact resulting SHA. Rejected updates must move no refs and create no GitHub
state.

The slice does not deliver webhooks, transition runs beyond queued, execute
workflows, or run MergeHerder. Those remain later roadmap phases. Internal data
defaults to concrete values; pointers are limited to shared state, lifecycle or
no-copy types, and external APIs requiring them. Do not use `github.Ptr`.

## Review Instructions

If you have a code review skill, use it. Review the complete current diff,
contracts, roadmap, implementation, and runnable proof against the goal. Run
the proof and inspect real Git behavior where useful. Prioritize incorrect Git
semantics, races between receive-pack and observation, false event creation,
wrong before/after identities, mutable event bodies, workflow correlation
errors, missing rejection proof, lifecycle problems, scope expansion, and
documentation overstatement.

Do not edit product code, tests, contracts, roadmap, or documentation. Write
the exact prompt and findings only to:

`ephemeral/reviews/20260711-p1-ref-events-claude-round1.md`

Label every finding using exactly one definition:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  that is materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Finish with a verdict on whether the implementation and proof are sufficient
for phase 1. Do not equate phase-1 sufficiency with later webhook delivery, CI
execution, or MergeHerder system proof.
