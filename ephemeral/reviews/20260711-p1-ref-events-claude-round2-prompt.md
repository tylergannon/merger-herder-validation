# P1 Ref Events Review - Claude Round 2

## Goal

Re-review roadmap phase 1 after the round-1 findings. Determine whether the
current implementation and real-Git tests are sufficient proof of fetch and ref
transition contracts plus exact immutable pending event and queued workflow
derivation.

## Round-1 Revisions

Read the round-1 artifact and inspect the current diff.

- Receive-pack CGI output is now buffered. A post-receive ref-snapshot failure
  records an inspectable observation error and returns a server failure instead
  of silently returning success without an event.
- The proof now runs two concurrent pushes to distinct refs in the same
  repository and asserts two correct zero-to-SHA events, no workflow runs, no
  observation errors, and race-detector cleanliness.
- Immutable push bodies now include `forced` and an App sender. The proof
  distinguishes release creation, fast-forward, exact-lease non-fast-forward
  rebuild, default landing, and deletion flags.

## Review Instructions

Adjudicate every prior critical, bug, and design finding and check the revisions
for new correctness, protocol, buffering, concurrency, or proof problems. Run
the proof. Do not edit implementation, tests, contracts, roadmap, or docs. Write
the exact prompt and findings only to:

`ephemeral/reviews/20260711-p1-ref-events-claude-round2.md`

Use these labels:

- **critical:** must fix before proceeding.
- **bug:** demonstrable incorrect behavior, broken contract, race, or
  regression.
- **design:** architecture, boundary, scope, maintainability, or proof issue
  materially likely to cause problems.
- **nit:** small cleanup that should not block progress.

Finish with a consensus verdict scoped strictly to phase 1.
