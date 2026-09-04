---
name: campaign-building
description: Planning and staging objective-specific campaigns, ad groups, ads, schedules, placements, bids, and validation prerequisites.
---

# Campaign building

This workflow remains staged for TikTok MAPI. The local sandbox can validate a small
three-level lifecycle, but it deliberately omits the dependencies needed to publish a
real ad.

## Required planning brief

Collect business outcome, destination (website/app/shop/lead), geography, schedule,
total and daily budget, target CPA/ROAS when applicable, attribution basis, audience
constraints, offer, landing page, identity, creative assets, and compliance constraints.
Do not choose an objective from a vague request such as “get more sales.”

## Dependency graph

- Campaign: objective, campaign type, special industry, campaign budget mode/value,
  and objective-specific app/catalog/shop fields.
- Ad group: parent campaign, promotion type, placements, location/language/audience,
  schedule/timezone, budget, billing event, optimization event, bid strategy/value,
  pixel/app/shop identity, and attribution settings.
- Ad: parent ad group, identity, creative/media, copy, CTA, destination/deep link,
  tracking parameters, and review-sensitive declarations.

TikTok's official flow confirms that ad-group creation combines placement, targeting,
budget/schedule, bidding, and optimization, while the ad form depends on campaign and
ad-group selections:
https://ads.tiktok.com/help/article/create-ad-group?lang=en
https://ads.tiktok.com/help/article/ad-set-up?lang=en-GB

## Future execution contract

`get_campaign_prerequisites` must return allowed enums and dependent fields for the
bound account. `stage_campaign_bundle` must produce a reviewable DAG with one idempotency
key per planned create, disabled initial status, explicit ordering, compensation notes,
and no hidden uploads. Validation failure at one level must not leave an enabled partial
campaign. The model may draft; only host approval may execute.

Do not activate this skill until sandbox lifecycle tests, MAPI HTTP wire tests,
platform-sandbox validation, UI review of every field, read-back, and duplicate-create
reconciliation all pass.
