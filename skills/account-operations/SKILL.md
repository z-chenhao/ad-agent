---
name: account-operations
description: "Running the advertiser operating cadence: readiness, daily triage, hierarchy and delivery checks, finance monitoring, and governed budget or status changes."
---

# Account operations

Use this skill for the daily operating loop, not as a generic dashboard narration.

## Establish the operating frame

Use current `get_advertiser_context` evidence. Bind every conclusion to the returned advertiser,
backend, environment, currency, timezone, latest usable date, and limitations. A tool
being installed proves product capability, not TikTok permission or account eligibility.
Sandbox, HTTP-fake, platform-sandbox, and controlled-live evidence are different grades.

For a daily brief, read campaigns, the newest complete campaign-level period, and
pending changes concurrently. Unresolved applying or indeterminate changes rank first:
duplicating an uncertain write is more dangerous than waiting on a performance lead.
Then prioritize stopped delivery, material spend/value movement, measurement blockers,
and time-sensitive opportunities according to the operator's goal. Keep the relevant
scope, dates, denominator, and uncertainty next to each finding. Use
`present_digest` only when its claims resolve to current server records.

## Delivery and hierarchy

Walk campaign -> ad group -> ad only as far as the decision requires. Parent status can
stop all children; an enabled child under a disabled parent is not delivering. Distinguish
configured budget from spend, budget scope from child allocation, and lack of delivery
from weak efficiency. Do not infer review, schedule, bid competitiveness, learning phase,
or audience saturation unless a typed read returned it.

## Finance and runway

Use `get_billing_balance` and `list_billing_transactions` for cash readiness and ledger
movement. Keep currency and freshness on every value. Estimate runway only from a named
spend window and label it as `available balance / observed daily spend`, not a billing
guarantee. Never initiate transfers, change payment authority, or expose invoice/customer
details. A Business Center requirement is a capability limitation, not zero balance.

## Governed changes

Before budget or status work, call `get_entity` on the exact object. For budgets, confirm
mode, calculate absolute and percentage delta, and separate scaling from reallocation.
Do not split a proposal to evade host limits. For pausing, state downstream reach. For
enabling, verify parents and identify missing measurement/review evidence.

`stage_budget_change` and `stage_status_change` create drafts only. The host renders the
exact preview; chat is never approval. If an outcome is unknown or indeterminate, stop
and reconcile instead of drafting a duplicate. Use `discard_change` only on explicit
operator request.

For a daily brief, an `act`, `investigate`, and `watch` queue can help prioritize.
For a direct edit, show its draft and remaining approval gate without a full account audit.
