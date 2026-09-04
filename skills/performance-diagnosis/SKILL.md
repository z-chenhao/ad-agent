---
name: performance-diagnosis
description: Explaining a performance movement through bounded period comparisons, rankings, contribution analysis, counter-evidence, and data limitations.
---

# Performance diagnosis

Diagnose a specified movement by progressively localizing it. Never begin with a
favorite explanation such as creative fatigue, targeting, or budget.

## Establish comparable evidence

Read advertiser context and anchor local-sandbox dates to its historical
`latest_date`. Fetch equal recent and previous windows with the same source, level,
scope, currency, timezone, metric definitions, and attribution basis. Reject complete
ranking claims when either window is partial. Do not compare a full week with a partial
day or mix attribution windows.

## Diagnostic tree

1. **Magnitude:** quantify absolute and relative change in spend, impressions, clicks,
   conversions, and value where present.
2. **Funnel localization:** determine whether movement first appears in delivery,
   traffic efficiency, conversion efficiency, or value per conversion. Preserve the
   correct denominator for every ratio.
3. **Contribution:** use `run_analysis` with current-turn dataset handles to identify
   which same-level objects mathematically contributed to the account movement.
4. **Counter-evidence:** look for important objects that moved in the opposite direction
   or for volume too low to support a stable ratio.
5. **Operational context:** check known status and pending changes. Mark schedule,
   learning phase, review, bid, targeting, creative content, and event health as unknown
   unless a typed tool returned them.

TikTok describes learning as a volatile calibration period and warns that pauses and
material edits can affect it, but the current tools cannot observe learning status:
https://ads.tiktok.com/help/article/learning-phase?lang=en

## Evidence labels

- **Finding:** directly computed or returned by a complete compatible dataset.
- **Lead:** timing and metric path are consistent, but a required causal variable is
  unavailable.
- **Not supported:** available evidence contradicts the hypothesis.
- **Not visible:** the required field or source is absent.

Require the analysis delegate to use server calculations and `submit_analysis`.
Present the trusted evidence record first, followed by the decision-relevant finding,
counter-evidence, limitation, and next discriminating read or test. Contribution is not
causation and platform attribution is not incrementality.
