---
name: portfolio-operations
description: Triaging several authorized advertiser accounts, drilling into one account, and preparing independent account-scoped drafts for a batch operation without unsafe cross-currency aggregation.
---

# Portfolio operations

Use this workflow only when the host is in portfolio scope. Portfolio scope means
the authenticated operator can act on a bounded advertiser set; it does not make
advertiser IDs credentials and it does not create a new agent runtime.

## Cross-account triage

1. Call `list_advertisers` to establish the authorized set and account metadata.
2. Call `get_portfolio_performance` for one explicit, comparable date window.
3. For non-trivial ranking or multi-account synthesis, call
   `run_portfolio_analysis` with that report ID. The isolated delegate receives
   only the bounded report and a submission tool; it has no staging authority.
4. Partition interpretation by currency, timezone, attribution basis, and data
   completeness. Never add monetary values across currencies or call missing rows
   zero.
5. Rank only on a named decision rule. A useful default is to flag incomplete
   measurement first, then material spend with weak ROAS, then delivery loss; it
   is not a causal diagnosis.
6. Drill into a flagged account with `list_account_entities` and
   `get_account_entity` before recommending an object change.

## Batch management

Treat a batch as a collection of independent changes:

- Verify every advertiser and exact object in this session.
- State the common intent and the per-account reason.
- Stage at most 20 items in one turn with `stage_account_budget_change` or
  `stage_account_status_change`. In a lifecycle-capable sandbox,
  `stage_account_entity_create` may prepare campaign, ad-group, or ad shells;
  child creation requires a current parent read.
- Preserve the advertiser's own currency on every budget draft.
- Report partial staging item by item. Do not roll a successful local draft back
  because another item was rejected.
- Explain that approval and execution are separate per advertiser; no model text
  is approval and there is no cross-account atomic commit.

## Decision checks

Before changing delivery, examine parent status, learning state, schedule,
budget, bid strategy, audience size, creative review, event readiness, and data
lag when those fields are available. When they are not available through the
installed tools, name the missing evidence instead of guessing.

For budget changes, compare equal windows, distinguish budget constraint from
weak demand or auction competitiveness, and avoid scaling an account whose
measurement is incomplete. For pauses, identify whether the target is a parent
that would stop several children.

## Output contract

Return:

- portfolio and reporting window;
- separate account findings with currency and completeness;
- the named ranking or triage rule;
- exact account and object IDs only from tools;
- one preview per staged change;
- rejected or unverified items;
- the next smallest drill-down or approval step.

Do not claim cross-account causality, forecast guaranteed lift, or summarize a
heterogeneous portfolio as one ROAS.
