# P1 Ref Events Review - Claude Round 3

## Goal

Make the final consensus determination for roadmap phase 1 after resolving
round 2's remaining design proof gap.

## Revision

The concurrency proof no longer creates two unrelated refs from zero. It first
creates one real `race` ref at a known SHA, then launches two concurrent pushes
with the same exact force-with-lease expectation and divergent candidate SHAs.
Exactly one may succeed. The proof requires exactly one new pending event whose
`before` is the known original SHA and whose `after` is the actual winning
remote SHA, with no workflow run or observation error. The focused scenario
passes under `-race -count=10`.

The round-2 nits were also tightened: pending-event JSON marshal failure is loud
rather than silently dropped, control snapshots use `run_attempt`, and
exact-SHA fetch covers both base and source objects.

## Review Instructions

Inspect the current revisions and prior reviews. Adjudicate the remaining
design finding and check for regressions. Run the focused proof. Do not edit
implementation, tests, contracts, roadmap, or docs. Write the exact prompt and
findings only to:

`ephemeral/reviews/20260711-p1-ref-events-claude-round3.md`

Use the canonical critical, bug, design, and nit labels. Finish with a binary
consensus verdict scoped strictly to phase 1.
