---
name: performance-insights
description: "Monitoring and diagnosing delivery, funnel, value, and creative performance with comparable windows, contribution analysis, counter-evidence, and explicit uncertainty."
---

# Performance insights

Start from the decision and metric movement, not from a favorite explanation.

## Comparable evidence

For period-over-period comparisons, prefer equal-length, non-overlapping windows with the same source, level, entity scope,
currency, timezone, attribution basis, and completeness. Anchor Sandbox dates to
`latest_date`. Compute ratios from summed inputs: CTR = clicks / impressions, CPC =
spend / clicks, CPA = spend / conversions, CVR = conversions / clicks, CPM = spend /
impressions * 1000, and ROAS = revenue / spend. Missing input is not zero; a zero
denominator makes the ratio unavailable.
Honor a requested seasonal or pacing comparison, but disclose unequal coverage and use
the appropriate elapsed-time baseline. Reported conversions may include view-through
attribution: reported CVR is not necessarily the probability of purchase after a click.

ROAS measures attributed value per unit of ad spend, not profit. Even a ROAS above one
does not establish profitability: product costs, discounts, refunds, fulfillment and fees
may be absent. When asked about profit or a break-even target, establish the revenue basis
and verified contribution margin/costs first. Without them, report advertising efficiency
and leave profitability unresolved; do not invent a universal profitable-ROAS threshold.

## Diagnostic decisions

1. Quantify absolute and relative movement in spend, impressions, clicks, conversions,
   and value.
2. Localize the first material break to delivery, traffic efficiency, conversion
   efficiency, or value per conversion.
3. Use `run_analysis` on current-turn report handles when contribution, ranking, or
   multi-row calculation is needed. A returned headline metric needs no delegate.
   Contribution is arithmetic, not causation.
4. Find counter-evidence: meaningful objects moving the other way, unstable low-volume
   ratios, incomplete periods, or attribution lag.
5. Read the smallest lower level that can discriminate among hypotheses.

These are decision checks, not mandatory stages. Stop when the available evidence
answers the question or when another read cannot distinguish the competing explanations.

Label conclusions as `finding` (directly returned/computed), `lead` (consistent but
missing a causal variable), `not supported`, or `not visible`. Do not turn timing into
causality or platform attribution into incrementality.

## Creative diagnosis

Join ad-level reports to ads, asset metadata, identity, and review state. Compare like
with like where possible: placement, audience, optimization goal, spend maturity, and
time in market. A high-spend winner can dominate account movement without proving the
creative concept caused it. Creative fatigue requires exposure/frequency or repeated
cohort evidence; declining CTR alone is only a lead. Separate hook/attention signals,
click intent, post-click conversion, and value. When video milestone, reach, frequency,
or landing-page-view metrics are unavailable, name the missing discriminator.

Present trusted metrics first. Then state the movement, top contributors, counter-
evidence, limitations, and one next read or controlled test. Never invent universal
thresholds or guaranteed lift.
