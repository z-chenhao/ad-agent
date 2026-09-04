---
name: catalog-commerce
description: Monitoring catalog, feed, product, product-set, and catalog-video health for commerce and dynamic product ads.
---

# Catalog commerce

This workflow remains staged until typed catalog, feed, upload log, product, product-set,
catalog-video, store-binding, and ad-usage reads exist.

Build a health chain: catalog ownership -> feed schedule/fetch -> parse/import result ->
product approval/availability -> identifier and variant consistency -> price/currency/
inventory -> image/video readiness -> product-set membership -> campaign usage. Preserve
counts at each stage so the agent can localize loss rather than report one health score.

Prioritize rejected or stale products by affected active ads, recent spend, inventory,
and business importance. A successful feed request does not prove products imported; an
approved product does not prove it is in a used set; a healthy catalog does not prove an
ad delivers. Keep TikTok Shop, offsite catalogs, Video Shopping Ads, and GMV Max native
semantics separate.

Return affected catalog/feed/set/product IDs, latest successful and failed runs, error
categories, coverage ratios, downstream campaign exposure, and a reversible remediation
plan. Feed changes, product edits, set membership, deletions, store bindings, and video
generation are distinct drafts with separate validation.
