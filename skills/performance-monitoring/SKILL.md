---
name: performance-monitoring
description: Summarizing KPI health, pacing, trends, and goal progress from comparable TikTok reporting windows without causal diagnosis.
---

# Performance monitoring

Monitoring answers “what changed and where,” not “why.”

## Define the observation

Read advertiser context, then request one or two equal, non-overlapping windows at one
hierarchy level. State inclusive dates, account timezone, currency, attribution basis,
freshness, completeness, and requested scope. For an in-progress or partial period,
compare the same elapsed portion or label it partial.

## Metric chain

Follow the funnel without mixing denominators:

- delivery: spend and impressions;
- traffic: clicks, CTR = clicks/impressions, CPC = spend/clicks;
- conversion: conversions, conversion rate using the explicitly defined denominator,
  CPA = spend/conversions;
- value: revenue and ROAS = revenue/spend only when the revenue mapping is validated.

Aggregate first, then divide. Never average row-level CTR, CPA, or ROAS. Never sum
campaign and ad-group rows. TikTok distinguishes all clicks from destination clicks and
defines multiple conversion-rate denominators, so use only the exact metric returned:
https://ads.tiktok.com/help/article/basic-data?lang=en

## Decision rules

- Compare against a target only if the operator stated it or it exists as an explicit
  saved goal under this same account.
- Describe concentration when a small number of objects account for a large observed
  share, but do not call it dependency risk without business context.
- Mark low-volume ratios as unstable rather than inventing a minimum sample threshold.
- A platform-attributed conversion is not incrementality. Keep click/view attribution
  windows and third-party reporting separate.

Use `present_metrics` for the supported record. Escalate to `performance-diagnosis`
only when the request asks for drivers, contributors, or counter-evidence.
