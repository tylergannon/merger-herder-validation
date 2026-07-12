# P1 Webhook Lifecycle Session

started: 2026-07-11 21:58 CST
worktree: `/Users/tyler/src/merger-herder-validation/.worktrees/p1-webhook-lifecycle`
branch: `codex/p1-webhook-lifecycle`
base: `147fe25625b850557fd313258891e1a55d3205e8`

## Goal

Implement roadmap phase 2: signed immutable webhook delivery, attempts and
redelivery, semantic duplication, caller-controlled order, and exact
workflow-run lifecycle events through completion.

## Decisions

- decision: Event bytes remain immutable. Delivery metadata and attempts are
  separate records.
- decision: The control caller selects a GUID for each delivery, which supplies
  withholding and arbitrary order without a background dispatcher.
- decision: Redelivery reuses GUID and raw bytes; semantic duplication copies
  raw bytes under a new GUID.
- decision: Workflow transitions mutate the authoritative run first and then
  append one immutable event for the resulting state.
- decision: HMAC-SHA256 covers the exact stored bytes and is emitted in
  `X-Hub-Signature-256` with GitHub event and delivery headers.

## Activity

- merge: PR #2 merged as `147fe25 Derive P1 events from real Git refs (#2)`.
- command: created this phase worktree directly from verified `origin/main`.
- skill_use: `session-worklog` source=pagerguild/core-tools -> record phase
  design, proof, review, and publication state.
- implementation: App fixtures may configure a webhook destination and secret;
  control can deliver by GUID, redeliver, duplicate, and transition runs.
- implementation: delivery performs network I/O outside the world lock and
  records each attempt separately from immutable event bytes.
- proof: real receiver independently checks exact-body HMAC, headers, order,
  invalid signature, redelivery identity, semantic duplicate identity, and run
  lifecycle correlation.
- review: Claude round 1 found the full phase proof insufficient because push
  delivery/`HOOK-06` was checked off but only workflow events crossed HTTP.
- review_resolution: the receiver now verifies a real push delivery; requested
  queued identity is reasserted; rejected/failed receiver attempts remain
  recorded but return a control-plane error instead of `200`.
- review: Claude rounds 2 and 3 verified all prior findings resolved. Final
  verdict: phase-2 proof sufficient with no critical, bug, or design finding.

## Proof And Closeout

- consensus: phase 2 sufficient after three Claude Opus rounds.
- remaining: phase 3 cancellation races and real workflow execution.
- branch_pr: committed as `e1b51ab`, pushed on
  `codex/p1-webhook-lifecycle`, and opened as
  `https://github.com/tylergannon/merger-herder-validation/pull/3`.
