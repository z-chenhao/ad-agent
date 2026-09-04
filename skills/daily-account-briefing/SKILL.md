---
name: daily-account-briefing
description: Producing a prioritized start-of-day briefing from current delivery, performance, account limitations, and pending changes.
---

# Daily account briefing

Build an operator queue, not a generic KPI recap.

## Evidence pull

In one independent read round, fetch advertiser context, campaigns, the newest usable
campaign-level window, and pending changes. Anchor dates to `latest_date`; never treat a
saved memory or yesterday's briefing as current evidence. Use a comparison window only
when it is equal in length, non-overlapping, and compatible in level, currency,
timezone, and attribution.

For every metric, retain scope and denominator. Compute ratios from summed inputs:
CTR = clicks / impressions, CPC = spend / clicks, CPA = spend / conversions, and ROAS =
revenue / spend. A missing numerator is not zero and a zero denominator makes the ratio
unavailable.

## Prioritization

Rank only evidence-backed items using three independent judgments:

- **Impact:** observed spend/value exposure or breadth of affected objects.
- **Urgency:** failed/indeterminate change, stopped delivery, or a time-sensitive trend.
- **Confidence:** complete comparable data outranks a partial window or unsupported
  causal lead.

Put unresolved applying or indeterminate writes first because a duplicate action can be
riskier than a performance movement. Then surface material delivery changes, efficiency
changes, concentration, and data-quality blockers. Do not invent universal percentage
alerts; compare with the operator's explicit goals or describe the observed movement.

## Brief format

Return three to six decision items. Each item must contain: observed fact; exact object
scope and inclusive dates; material metric and denominator; confidence/limitation; the
decision required; and one smallest next action. Separate `act now`, `investigate`, and
`watch`. Use `present_digest` only after server records support every item.

Do not manufacture alerts from unavailable balance, policy review, learning phase,
schedule, bid, pixel, audience, or rule data.
