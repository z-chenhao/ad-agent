---
name: measurement-and-attribution
description: Auditing pixels, app and offline event sources, event activity, attribution assumptions, and optimization-event readiness.
---

# Measurement and attribution

This workflow remains staged until pixel, app, Events API, offline-set, event-stat,
diagnostic, optimization-event, and attribution-setting reads are typed.

## Source inventory

For every destination, identify the bound pixel/app/offline set, advertiser ownership,
connection method, event names, last-seen time, recent volume, deduplication key support,
match-quality signals, diagnostics severity, optimization eligibility, and affected ads.
Never place event payloads, customer identifiers, cookies, IP addresses, or device IDs
in model context.

## Attribution audit

Record click-through, view-through, and engaged-view windows separately; distinguish
platform conversions, real-time conversions, MMP/analytics reports, and modeled or
assisted metrics. Never sum conversions across windows or systems. A discrepancy is not
automatically tracking loss: timezone, attribution window, identity, reporting lag,
deduplication, and source-of-truth semantics can all differ. TikTok's official overview
defines CTA, VTA, EVTA, and attribution windows as separate concepts:
https://ads.tiktok.com/help/article/attribution-overview?lang=en

## Diagnostic output

Return confirmed source mapping, event freshness/volume, active diagnostics, campaigns
depending on each event, attribution configuration, discrepancies, and the smallest
verification step. A missing event is not zero conversions unless a complete platform
response explicitly reports zero.

Test-event transmission, source creation, event-rule edits, first-party-cookie changes,
and optimization-event changes are separate mutations. The initial implementation must
remain read-only and use TikTok's diagnostics severity/affected-object data rather than
inventing fixes:
https://ads.tiktok.com/help/article/web-diagnostics
