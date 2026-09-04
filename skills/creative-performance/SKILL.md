---
name: creative-performance
description: Finding weak or promising ads from ad-level performance and separating a measured creative lead from an unsupported creative cause.
---

# Creative performance

Use ad-level outcomes to identify what deserves creative inspection. Do not treat an ad
ID as if its video, copy, identity, or landing page had been observed.

## Cohort and evidence

Read the parent ad group, list its ads, and request equal ad-level windows under the same
source, attribution, optimization context, and dates. Use `run_analysis` for rankings,
shares, trends, and contribution across several ads. Separate ads with meaningful
delivery from ads with insufficient exposure; do not invent a universal spend or
impression cutoff.

Trace the observed funnel: impressions -> clicks -> conversions -> value. A declining
CTR may be consistent with a hook or audience issue; a stable CTR with declining CVR may
point downstream; neither proves creative causality. Frequency, reach, first-frame hold,
video-view milestones, destination-click metrics, asset age, and audience overlap are
not currently available.

TikTok's official performance-creative guidance recommends TikTok-first vertical,
sound-on, safe-zone-aware work and refreshing existing ad groups when performance shows
a consistently declining trend:
https://ads.tiktok.com/help/article/creative-best-practices
Treat this as a hypothesis framework, not a diagnosis from aggregate metrics alone.

## Output contract

For each lead, provide exact ad and parent IDs, window, delivery volume, measured funnel
movement, confidence, and the next asset-level evidence required. Label it `promising`,
`weak lead`, `declining trend`, or `insufficient delivery`; reserve `creative fatigue`
for cases where trend plus frequency/asset evidence is actually available.

Use `present_entities` only with host-returned objects. Do not invent hooks, audience
personas, review state, replacement claims, or creative assets. Any disable proposal is
a separate delivery-operation draft.
