# P1 Webhook Lifecycle Review - Claude Round 2

## Exact Prompt

> Re-review phase 2 after the round-1 critical and design findings. The real HTTP
> receiver now receives and independently verifies an immutable `push` event in
> addition to workflow events. The test also reasserts requested/queued run
> identity. Delivery attempts are always retained, but transport failures and
> non-2xx receiver responses now return a control-plane `502` rather than `200`.
>
> Adjudicate every prior critical, bug, and design finding, run the proof, and
> check for regressions. Do not edit implementation or docs. Write the exact
> prompt and findings only to
> `ephemeral/reviews/20260711-p1-webhook-lifecycle-claude-round2.md`. Use canonical
> finding labels and finish with a binary phase-2 proof-sufficiency verdict.

## Proof Commands Run

- `go build ./...` — clean.
- `go vet ./dtu/` — clean.
- `gofmt -l dtu/delivery.go dtu/delivery_test.go` — clean.
- `go test ./dtu/ -run TestWebhookDeliveryAndWorkflowLifecycle -v -count=1` — PASS (0.26s).
- `go test ./dtu/ -run TestWebhookDeliveryAndWorkflowLifecycle -race -count=1` — PASS.
- `go test ./... -count=1` — PASS.

## Adjudication Of Prior Findings

### CRITICAL (round 1) — `push` webhook never crossed the wire — **RESOLVED**

The push event is now delivered over real HTTP and independently verified:

- `dtu/delivery_test.go:66` binds `push` from `created.PendingEvents` (the body
  produced by `git push … feature:refs/heads/R` at `:63`).
- `dtu/delivery_test.go:80` issues `control.DeliverEvent{GUID: push.GUID}`, so the
  event-type-agnostic path in `dtu/delivery.go:38-49` signs `event.Body` and sends
  it with `X-GitHub-Delivery`, `X-GitHub-Event: push`, and the HMAC header.
- `dtu/delivery_test.go:81` asserts the receiver's capture via `assertDelivery(…,
push, secret, true)`, which checks GUID, `Event == "push"`, exact byte equality
  against the stored body, and a **valid** independently-recomputed HMAC
  (`assertDelivery`/`validSignature`, `:146,:151-156`). The receiver itself also
  verifies the signature and answers `202` (`:32-36`).
- `dtu/delivery_test.go:121` pins `DeliveryAttempts[0].GUID == push.GUID` with
  `StatusCode == 202`.

The checked roadmap boxes ("Deliver signed immutable `push` … payloads",
`ephemeral/P1_ENVIRONMENT_ROADMAP.md:72`; "Prove … `HOOK-06` across real HTTP
delivery", `:81`) are now truthful: a push webhook crosses the wire with correct
headers, exact bytes, and a receiver-verified signature. See residual DESIGN
below for the one remaining nuance (opaque-byte identity).

### DESIGN (round 1) — control `DeliverEvent` returned `200` on failure — **RESOLVED**

`dtu/delivery.go:66-73` now returns `http.StatusBadGateway` (`502`) with a
distinguishing message on both failure modes — `"webhook delivery failed"` for a
transport error and `"webhook receiver rejected delivery"` for a non-2xx receiver
status — while the successful path still returns `200` (`:74`). `ControlClient.post`
surfaces any non-2xx as an error (`dtu/client.go:98-104`), so a caller now learns
of rejection without a `/state` round-trip. The test exercises this directly: the
deliberate invalid-signature delivery is asserted to return a non-nil error
(`dtu/delivery_test.go:87-89`).

Retention is preserved: the attempt is appended under lock (`dtu/delivery.go:62-65`)
**before** the `502` branch, so failed attempts remain observable via `/state`.
`dtu/delivery_test.go:118-121` confirms all 5 attempts (including the `401` at
index 2) are retained despite the caller having received a `502`.

### DESIGN (round 1) — `requested`/`queued` identity was implicit — **RESOLVED**

`dtu/delivery_test.go:72` now re-asserts the requested action in-slice via
`assertWorkflowBody(t, requested, runID, "queued", nil)`, validating action, run
ID, attempt, and status. Together with the existing `in_progress` and `completed`
assertions (`:78-79`), the three-action `workflow_run` identity claim is now
self-contained.

### NIT (round 1) — invalid signature simulated by mutating the secret — **CARRIED, adequate**

`dtu/delivery.go:35-37` still appends `-invalid` to the secret, producing a
well-formed-but-wrong `sha256=<hex>` that drives the receiver's `401`. As noted in
round 1, this does not exercise malformed/absent-header rejection; that is out of
scope here. No action required.

### NIT (round 1) — `AttemptedAt` stamped before round-trip; outbound inherits inbound context — **CARRIED, immaterial**

`dtu/delivery.go:23,54` still reads `now` before `client.Do`, and `:39` still uses
`request.Context()` for the outbound call. Immaterial under the virtual clock and
in-test lifetimes. No action required.

## Regression Check

No regressions introduced by the round-2 changes:

- Attempt retention is unchanged and independent of the response status — the
  append precedes every early return (`dtu/delivery.go:62-73`); the `502` path does
  not skip recording.
- Delivery-attempt ordering/count assertion still holds exactly:
  `push(202) → completed(202) → inProgress invalid(401) → completed redelivery(202)
→ duplicate(202)` (`dtu/delivery_test.go:80,84,87,94,107` vs `:121`).
- Redelivery still preserves GUID and exact bytes (`:94-98`); the semantic
  duplicate still deep-copies the body under a new GUID
  (`dtu/delivery.go:90-94`, asserted `:104`).
- Transition guards intact: `queued→in_progress`, `queued|in_progress→completed`
  with nonempty conclusion, and rejection of a second terminal transition
  (`dtu/delivery.go:119-127`, asserted `:114-116`).
- No other call site depends on `DeliverEvent` returning `200`; only
  `client.go:49` references it, and the test is the sole caller.
- Bodies remain immutable; the lock-free `event.Body` read after `RUnlock`
  (`dtu/delivery.go:24,38-39`) is safe because bodies are never mutated post-create.
  `-race` is clean.

## New / Residual Findings

### DESIGN — delivered `push` body identity (`ref`/`after` SHA) is proven as opaque bytes, not re-parsed in-slice

`assertDelivery` (`dtu/delivery_test.go:144-149`) proves the receiver got bytes
**byte-identical to the stored `push` event** and a valid signature, but — unlike
the symmetric `assertWorkflowBody` treatment given to `requested`/`in_progress`/
`completed` — it does not parse the push payload to re-confirm `ref ==
refs/heads/R` and `after == <head SHA>`. HOOK-06's ref/after-SHA identity therefore
still chains through the prior ref-events slice (which proved the stored push body
at the state layer) plus this slice's exact-bytes equality, rather than being
re-asserted directly against the wire bytes here. The chain is sound and the
verdict does not depend on closing it; a one-shot parse of the delivered push body
(mirroring `assertWorkflowBody`) would make the "`HOOK-06` across real HTTP
delivery" box fully self-contained. Non-blocking.

### NIT — a deliberately-rejected delivery is now surfaced to the caller as a `502` error

Because rejection is now a control-plane `502` (the round-1 DESIGN ask), the
_intended_ proof scenario "deliberate invalid signature is rejected" is expressed
as an expected error at the call site (`dtu/delivery_test.go:87-89`, `err == nil`
is the failure). This is correct and the attempt remains inspectable via `/state`;
noting only that "delivery attempted and correctly rejected" and "delivery failed"
share the `502` channel, distinguished solely by the response message. Acceptable
for this proof.

## Verdict

**SUFFICIENT.** Proof is sufficient for roadmap phase 2.

The round-1 CRITICAL is resolved — an immutable `push` webhook now crosses real
HTTP with correct GitHub headers, exact stored bytes, and a receiver-verified
HMAC-SHA256 — and both round-1 DESIGN findings are resolved (control-plane `502`
on transport failure / receiver rejection with attempts still retained, and
in-slice re-assertion of `requested`/`queued` identity). Build, vet, gofmt, the
targeted test, `-race`, and the full suite all pass. The remaining DESIGN/NIT
items are optional hardening and do not gate phase 2; phase 3 cancellation and
`act`/Docker workflow execution remain correctly out of scope.
