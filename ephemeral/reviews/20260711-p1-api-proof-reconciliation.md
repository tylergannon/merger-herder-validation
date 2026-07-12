# P1 API Proof Consensus Decision

Decision: The implementation and tests constitute a valid, workable real
network/process/Git proof for the declared P1 API boundary. They do not
discharge live-GitHub conformance or the complete MergeHerder P1 scenario.

Evidence: `dtu`, `cmd/dtu-github`, `docs/p1-api-proof.md`, the agreed endpoint
claims, real `go-github` and raw-HTTP assertions, real Git smart HTTP, virtual
expiry, executable startup, race-enabled tests, and two Claude review rounds.

Accepted findings: exact-path routing, control path traversal, isolated
permission-alternative tests, OS-independent Git casing, raw token-success
assertions, Git ref resolution outside the world lock, subprocess public-listener
coverage, rejection mutation checks, insufficient Git write-permission proof,
and unambiguous repository-name selection within an installation.

Rejected findings: Round 1 claimed `contents:read` could not authorize targeted
PR reads. Current GitHub primary documentation explicitly lists at least one of
Pull Requests read or Contents read for this endpoint; Claude independently
confirmed that evidence in round 2.

Deferred findings: The subprocess test proves lifecycle and both listeners but
does not duplicate the complete in-process golden journey. Unknown token-body
fields and the provisional inactive/validation statuses remain inputs to the
named live-conformance layer. These do not weaken the declared local proof.

Review artifacts:

- `ephemeral/reviews/20260711-p1-api-proof-codex.md`
- `ephemeral/reviews/20260711-p1-api-proof-claude-round1.md`
- `ephemeral/reviews/20260711-p1-api-proof-claude-round2.md`

Proof still required: final formatting, vet, race-enabled test suite, diff
checks, and PR CI. Outside this PR, live-GitHub status/error conformance and the
real MergeHerder/Octokit P1 system scenario remain required.
