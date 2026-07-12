# P1 Webhook Lifecycle Review - Claude Round 1

## Exact Prompt

> # P1 Webhook Lifecycle Review - Claude Round 1
>
> ## Goal
>
> Review roadmap phase 2 as proof, not merely code. Immutable pending push and
> workflow bodies must be deliverable over real HTTP with GitHub delivery/event
> headers and HMAC-SHA256 over exact bytes. Control must support withholding,
> arbitrary order, invalid signatures, attempts, same-GUID/body redelivery, and
> new-GUID/same-body semantic duplication. Authoritative runs must move through
> queued, in-progress, and completed while emitting exactly correlated immutable
> events. Delivery and transition failures must remain observable.
>
> Phase 3 cancellation and workflow execution, and later MergeHerder/live proof,
> are explicitly outside this slice.
>
> ## Instructions
>
> Review the complete diff, roadmap, proof doc, implementation, and real HTTP
> test. Run proof commands. Find protocol errors, races, mutable-body mistakes,
> incorrect signature or identity behavior, invalid state transitions, swallowed
> delivery failures, and proof overstatement. Do not edit implementation or docs.
> Write the exact prompt and findings only to:
>
> `ephemeral/reviews/20260711-p1-webhook-lifecycle-claude-round1.md`
>
> Label findings critical, bug, design, or nit using the established consensus
> definitions. Finish with an explicit verdict on proof sufficiency for phase 2.

## Proof Commands Run

- `go build ./...` — clean.
- `go vet ./dtu/` — clean.
- `go test ./dtu/ -run TestWebhookDeliveryAndWorkflowLifecycle -v` — PASS (0.23s).
- `go test ./...` — PASS.
- `go test ./dtu/ -run TestWebhookDeliveryAndWorkflowLifecycle -race` — PASS.
- `gofmt -l dtu/delivery.go dtu/delivery_test.go` — clean.

The implementation is correct, race-clean, and the HTTP mechanics (exact bytes,
headers, HMAC-SHA256, redelivery, semantic duplication, transition guards) are
genuinely exercised over a real `httptest` receiver that independently verifies
the signature. The findings below concern proof scope, not code defects.

## Findings

### CRITICAL — `push` webhook delivery over real HTTP is claimed but never exercised; HOOK-06 "across real HTTP delivery" is overstated

The roadmap now marks these two items `[x]`:

- `ephemeral/P1_ENVIRONMENT_ROADMAP.md:71` — "Deliver signed immutable `push`
  and `workflow_run` payloads to a configured receiver."
- `ephemeral/P1_ENVIRONMENT_ROADMAP.md:81` — "Prove `CI-01`, `CI-02`, and
  `HOOK-06` across real HTTP delivery."

`HOOK-06` (`GITHUB_SERVICE_CONTRACT.md:191`) is specifically the **`push`
webhook for `R` and the default branch**. But `TestWebhookDeliveryAndWorkflowLifecycle`
never delivers a `push` event over HTTP. The four `control.DeliverEvent` calls
(`dtu/delivery_test.go:80,83,88,101`) target only `workflow_run` events
(`completed`, `inProgress`, redelivered `completed`, and the `completed`
duplicate). The `push` event produced by `git push ... feature:refs/heads/R`
(`dtu/delivery_test.go:63`) is asserted to exist only implicitly, and its
signed body, `X-GitHub-Delivery`/`X-GitHub-Event` headers, and HMAC-SHA256 over
its exact bytes are never presented to the receiver.

Consequences:

- The goal's requirement that "immutable pending **push** and workflow bodies
  must be deliverable over real HTTP ... HMAC-SHA256 over exact bytes" is proven
  only for `workflow_run`, not `push`.
- "Prove ... `HOOK-06` across real HTTP delivery" is an overstatement: no push
  webhook crosses the wire in this slice. HOOK-06's ref/`after`-SHA body was
  proven at the _state_ layer in the prior ref-events slice, but not "across
  real HTTP delivery" as the checked box asserts.

The delivery code path is event-type agnostic (it reads `event.Event` and
`event.Body` generically in `dtu/delivery.go:38-47`), so a push almost certainly
_would_ deliver correctly — but for a "proof, not merely code" review that is
exactly the gap: the claim is asserted, not demonstrated. Add one
`DeliverEvent` of the `push` GUID and assert the receiver observes
`X-GitHub-Event: push`, the exact stored bytes, and a valid signature.

### DESIGN — the control `DeliverEvent` endpoint returns `200 OK` even when delivery transport-fails or the receiver rejects it

`dtu/delivery.go:56-66` records the outcome (transport `Error`, or `StatusCode`
`401`/`5xx`) into the `DeliveryAttempt` and then unconditionally
`writeJSON(response, http.StatusOK, attempt)`. Failures are therefore observable
via `/state` (satisfying "delivery failures must remain observable"), and the
test relies on this by inspecting `final.DeliveryAttempts[…]`. But the control
_caller_ receives an identical `200` whether the delivery reached a `202`
receiver, was rejected `401`, or never connected at all. A caller that only
checks the control HTTP status (as `ControlClient.post` does — it treats any
2xx as success, `dtu/client.go`) cannot distinguish "delivered and accepted"
from "delivered and rejected" from "transport failed" without a second `/state`
round-trip. This is acceptable for the current proof (the test reads state) but
is a thin contract: consider returning a non-2xx, or a discriminated field, when
`deliveryErr != nil` so a caller can react without polling state. Not blocking.

### DESIGN — the `requested` action body is not re-asserted in this slice's test, leaving CI-02 identity for `requested` implicit

`assertWorkflowBody` (`dtu/delivery_test.go:76-77`) validates run ID / attempt /
status / conclusion only for `in_progress` and `completed`. For `requested` the
test does `requested := findEvent(...)` and then only checks `requested.GUID != ""`
(`dtu/delivery_test.go:66,105-107`). CI-02 (`GITHUB_SERVICE_CONTRACT.md:189`)
requires the `requested`/`queued` action to identify the same run ID, attempt,
head branch, and SHA. Within this slice that correlation is unverified; it rests
on the prior ref-events proof. Given phase 2 explicitly re-lists "Emit
`workflow_run` actions `requested`, `in_progress`, and `completed` with exact ...
identity," a one-line `assertWorkflowBody(t, requested, runID, "queued", nil)`
would close the loop and make the three-action identity claim self-contained.

### NIT — invalid-signature simulation mutates the secret rather than the digest

`dtu/delivery.go:35-38` simulates a bad signature by appending `-invalid` to the
secret before computing HMAC. This produces a well-formed `sha256=<hex>` header
that fails verification, which correctly drives the receiver's `401` path and is
adequate. It does not, however, exercise malformed/absent-header rejection (a
different failure mode). Fine for the stated scope; noting only so a later slice
that claims broader signature-rejection coverage does not over-read this one.

### NIT — `AttemptedAt` is stamped from `w.now` captured before the round-trip, and the outbound request inherits the inbound control context

`now` is read under `RLock` at `dtu/delivery.go:23` and used for
`AttemptedAt` at `:54`, i.e. before `client.Do`. Because the clock is virtual
and only advances via `/clock/advance`, this is immaterial, but the timestamp
names attempt-_start_, not completion. Separately, the outbound delivery uses
`request.Context()` (`:39`), so a caller disconnect cancels the delivery — a
reasonable choice, worth a comment. Neither affects the proof.

## Verdict

**Proof is sufficient for the `workflow_run` half of phase 2; NOT yet
sufficient for the full checked scope because of the CRITICAL finding.**

What is convincingly proven over real HTTP: withholding (no auto-dispatch),
caller-controlled arbitrary ordering (`completed` before `in_progress`),
deliberate invalid signatures recorded with the receiver's `401`, valid
deliveries recorded `202`, same-GUID/same-body redelivery, new-GUID/same-body
semantic duplication, the `queued -> in_progress -> completed` transition guard
(including rejection of a second terminal transition), and exact
`X-GitHub-Delivery`/`X-GitHub-Event`/`X-Hub-Signature-256`-HMAC-SHA256 over the
stored bytes independently verified by the receiver. The implementation is
race-clean and bodies are immutable (delivery reads `event.Body` lock-free,
which is safe precisely because bodies are never mutated after creation, and the
semantic duplicate deep-copies via `append([]byte(nil), source.Body...)`).

The gap: the roadmap marks "Deliver signed immutable **push** ... payloads" and
"Prove **HOOK-06** across real HTTP delivery" as done, but no `push` webhook is
delivered over HTTP anywhere in the suite. Deliver the push GUID and assert the
receiver's headers/bytes/signature to make the checked boxes truthful; the
DESIGN/NIT items are optional hardening.
