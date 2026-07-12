# P1 Webhook Lifecycle Review - Claude Round 3

## Exact Prompt

> Make the final phase-2 consensus determination. The remaining round-2 design
> item is addressed: this slice now parses the real-push event body and asserts
> `ref == refs/heads/R` and `after` equals the feature SHA supplied to the actual
> smart-HTTP push, before delivering those exact bytes and verifying HMAC.
>
> Inspect the revision, run the focused proof, and adjudicate whether any
> critical, bug, or design finding remains. Do not edit implementation or docs.
> Write the exact prompt and findings only to
> `ephemeral/reviews/20260711-p1-webhook-lifecycle-claude-round3.md`, then give a
> binary phase-2 proof-sufficiency verdict.

## Proof Commands Run

- `go build ./...` — clean.
- `go vet ./dtu/` — clean.
- `gofmt -l dtu/delivery.go dtu/delivery_test.go` — clean (no output).
- `go test ./dtu/ -run TestWebhookDeliveryAndWorkflowLifecycle -v -count=1` — PASS (0.27s).
- `go test ./dtu/ -run TestWebhookDeliveryAndWorkflowLifecycle -race -count=1` — PASS.
- `go test ./... -count=1` — PASS.

The round-3 revision touches only `dtu/delivery_test.go` (mtime 22:10);
`dtu/delivery.go` is unchanged from round 2 (mtime 22:06). This is a
test-only strengthening — no implementation or contract behavior moved.

## Adjudication Of The Round-2 Residual DESIGN — **RESOLVED**

Round 2 left one non-blocking DESIGN item: the delivered `push` body's
`ref`/`after`-SHA identity was proven only as _opaque bytes_ (byte-equality against
the stored event) plus a chain back through the prior ref-events slice, rather than
being re-parsed in this slice the way `assertWorkflowBody` re-parses the
`workflow_run` actions.

That gap is now closed in-slice:

- `dtu/delivery_test.go:62` captures the real feature SHA from the fixture
  (`git rev-parse feature`), and `:64` performs the actual smart-HTTP push
  (`git push … main feature feature:refs/heads/R`), so `R`'s objectname is the
  feature commit's SHA. The server derives the push event from the real
  post-receive ref delta (`dtu/git.go:96-102,144-173`), setting
  `Ref: "refs/heads/R"` and `After: <feature SHA>` in `appendPushEvent`
  (`dtu/git.go:184-212`).
- New helper `assertDeliveredPushIdentity` (`dtu/delivery_test.go:128-140`)
  unmarshals the stored `push` body and asserts `ref == "refs/heads/R"` and
  `after == featureSHA`, invoked at `:68` **before** any delivery.
- The exact same object is then delivered over real HTTP (`:82`) and the wire
  capture is checked for byte-equality against `push.Body` plus a receiver-verified
  HMAC (`assertDelivery`, `:83` → `:160-165`, `validSignature` `:167-172`).

Because `assertDeliveredPushIdentity` pins the semantic identity of `push.Body`
and `assertDelivery` proves the wire bytes are byte-identical to `push.Body`, the
`ref`/`after` identity now holds directly against the bytes that cross the wire —
this is logically equivalent to parsing the captured wire body, since the two byte
strings are asserted equal. The checked roadmap box "Prove … `HOOK-06` across real
HTTP delivery" (`ephemeral/P1_ENVIRONMENT_ROADMAP.md:81`) is now fully
self-contained within this slice for the push identity, not merely chained through
the prior ref-events proof. `HOOK-06`'s `ref`/`after` contract is satisfied.

## Adjudication Of All Prior Findings

- **CRITICAL (round 1)** — `push` webhook never crossed the wire — **RESOLVED**
  (round 2), still holds. The push GUID is delivered over real HTTP with
  `X-GitHub-Event: push`, exact stored bytes, and a receiver-verified HMAC
  (`:82-83`, `final.DeliveryAttempts[0]` pinned `push`/`202` at `:123`).
- **DESIGN (round 1)** — `DeliverEvent` returned `200` on failure — **RESOLVED**
  (round 2), still holds. Transport failure → `502` "webhook delivery failed"
  (`dtu/delivery.go:66-68`); non-2xx receiver → `502` "webhook receiver rejected
  delivery" (`:70-72`); attempt appended under lock _before_ the `502` branch so
  failures remain observable (`:62-65`). Exercised at `:89-90`.
- **DESIGN (round 1)** — `requested`/`queued` identity implicit — **RESOLVED**
  (round 2), still holds (`assertWorkflowBody(t, requested, runID, "queued", nil)`,
  `:74`).
- **DESIGN (round 2)** — delivered push identity proven as opaque bytes only —
  **RESOLVED** (this round; see above).
- **NIT (round 1)** — invalid signature simulated by mutating the secret —
  **CARRIED, adequate**. `dtu/delivery.go:35-37` still appends `-invalid`;
  malformed/absent-header rejection remains out of scope. No action.
- **NIT (round 1)** — `AttemptedAt` stamped pre-round-trip; outbound inherits
  inbound context — **CARRIED, immaterial** under the virtual clock
  (`dtu/delivery.go:23,39,54`). No action.
- **NIT (round 2)** — a deliberately-rejected delivery and a genuine transport
  failure both surface as `502`, distinguished only by message — **CARRIED,
  acceptable**. The attempt is still individually inspectable via `/state`
  (`final.DeliveryAttempts[2].StatusCode == 401`, `:123`).

## Regression Check

No regressions. The round-3 change is additive: one new assertion helper and its
single call site (`:68`, `:128-140`) that only _reads_ the push event body. The
delivery-attempt sequence, counts, and status codes are unchanged
(`push(202) → completed(202) → inProgress invalid(401) → completed redelivery(202)
→ duplicate(202)`, asserted `:120-124`), transition guards are intact
(`dtu/delivery.go:119-127`, asserted `:116-117`), bodies remain immutable and the
lock-free `event.Body` read stays race-clean (`-race` PASS), and no non-test file
changed. Build, vet, gofmt, the targeted test, `-race`, and the full suite all
pass.

## Remaining Findings

None at CRITICAL, BUG, or DESIGN. Two round-1 NITs and one round-2 NIT are carried
as documented, non-blocking scope notes; they do not gate phase 2. Phase 3
cancellation and `act`/Docker workflow execution remain correctly out of scope.

## Verdict

**SUFFICIENT.** Proof is sufficient for roadmap phase 2.

Every prior CRITICAL and DESIGN finding is resolved, including the last round-2
residual: the immutable `push` webhook's `ref == refs/heads/R` / `after == <real
feature SHA>` identity is now asserted in-slice against the exact bytes that cross
real HTTP with a receiver-verified HMAC-SHA256, alongside the fully proven
`workflow_run` `requested`/`in_progress`/`completed` lifecycle, caller-controlled
ordering, invalid-signature `502` rejection with retained attempts, same-GUID/body
redelivery, new-GUID/same-body semantic duplication, and the
`queued → in_progress → completed` transition guard. Only non-blocking NITs remain.
Phase-2 consensus: proof-sufficient.
