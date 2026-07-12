# P1 Webhook Lifecycle Proof

## Boundary

Roadmap phase 2 delivers the immutable event bodies created by Git ref and
workflow transitions. The private control caller chooses which GUID to deliver
and when; no background dispatcher can collapse withholding, order, duplicate,
or race scenarios.

Each delivery sends the exact stored bytes with:

- `X-GitHub-Delivery`;
- `X-GitHub-Event`;
- `X-Hub-Signature-256` containing HMAC-SHA256 over those bytes; and
- `Content-Type: application/json`.

Attempts retain destination, HTTP status or transport error, signature mode,
GUID, and virtual attempt time. Redelivery reuses GUID and body. A semantic
duplicate copies the body under a new GUID.

## Workflow Lifecycle

The control plane permits only these authoritative transitions:

- `queued` to `in_progress`, producing action `in_progress`; and
- `queued` or `in_progress` to `completed` with a nonempty conclusion,
  producing action `completed`.

Every body retains run ID, attempt, workflow identity, trigger event, head
branch, head SHA, resulting status, and conclusion. A completed run rejects
further transitions.

## Real HTTP Proof

`TestWebhookDeliveryAndWorkflowLifecycle` starts a real receiver which
independently verifies HMAC over the received bytes. It proves:

1. created events remain withheld until selected;
2. completed delivery can precede in-progress delivery;
3. deliberate invalid signatures reach the receiver and record its `401`;
4. valid deliveries record `202`;
5. redelivery preserves exact GUID and bytes;
6. a semantic duplicate preserves bytes under a new GUID;
7. workflow actions and conclusions correlate with the same run ID/attempt; and
8. a completed run cannot transition again.

Phase 3 still owns Actions cancellation races and real workflow execution
through pinned `act` and Docker.
