---
name: budget-operations
description: Reviewing budget pacing and preparing one bounded campaign or ad group budget draft for host approval.
---

# Budget operations

Budget work changes exposure and can disturb delivery; separate analysis, proposal,
draft, and approval.

## Preflight

Read the exact campaign or ad group, advertiser currency, budget amount, and budget
mode. Never reinterpret daily as lifetime or switch modes through a value-only tool.
Read comparable performance when the request is about pacing, allocation, or scaling.
Record whether learning phase, schedule, bid strategy, target CPA/ROAS, campaign budget
optimization, and child budgets are visible; they usually are not in the current tool
surface.

TikTok's current guidance distinguishes daily and lifetime budgets, states minimums by
level, and recommends bounded, infrequent increases that depend on learning state:
https://ads.tiktok.com/resources/help/article/budget?lang=en
Those are platform guidance, not permission to override the host policy or proof that
one account/object accepts a value.

## Analyze the proposal

- Calculate absolute and percentage delta from the grounded current value.
- For pacing, state observed spend divided by elapsed days and distinguish a projection
  from a committed budget. Do not extrapolate a partial or delayed window silently.
- For reallocation, treat the total as fixed unless the operator explicitly increases
  it; name the source object and destination object separately.
- Do not recommend scale from ROAS alone when conversion volume, attribution, marginal
  performance, learning state, or business capacity is unknown.
- Never split a blocked change into smaller drafts to evade `MaxDeltaPercent` or other
  host guardrails.

## Execute one draft

Use `stage_budget_change` for one grounded campaign or ad group and
`present_change_preview`. Include current value/mode, proposed value, currency,
absolute/percentage delta, supporting window, expected effect as a hypothesis, known
risks, and missing evidence. A staged record does not change the backend.

Only the authenticated host approval control can apply it. If application is unknown or
indeterminate, stop and reconcile; do not create a replacement draft.
