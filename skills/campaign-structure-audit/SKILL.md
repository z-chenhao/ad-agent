---
name: campaign-structure-audit
description: Auditing campaign, ad group, and ad hierarchy for disabled parents, orphaned delivery, budget placement, and scope mismatches.
---

# Campaign structure audit

Audit the three-level TikTok hierarchy as a graph, not as three unrelated lists. TikTok
campaigns own the objective; ad groups contain ads with shared targeting, budget,
schedule, placement, bidding, and optimization settings; ads own the delivered creative
configuration.

## Traversal

1. Call `list_campaigns`.
2. For each campaign in scope, call `list_ad_groups` with its host-returned ID.
3. For each ad group in scope, call `list_ads` with its host-returned ID.
4. Use `get_entity` before describing or drafting a mutable field.

Keep IDs opaque and preserve parent links. Absence from a bounded or failed list is not
proof that an object does not exist.

## Checks supported now

- enabled child beneath a disabled parent;
- disabled object with spend inside the inspected window;
- campaign or ad group with no visible children, labeled with list completeness;
- budget set at a different level than the operator assumes;
- duplicated or ambiguous names that make operational selection unsafe;
- objective inconsistency only when the objective is actually returned.

## Checks that remain unknown

Current entity reads do not expose schedule, review/policy state, bid strategy,
placement, audience, optimization event, identity, creative asset, or learning phase.
Do not diagnose these from operation status. TikTok's official ad-group flow confirms
these are distinct setup dimensions:
https://ads.tiktok.com/help/article/create-ad-group?lang=en

## Output contract

Produce a hierarchy summary and findings ordered by operational risk. For each finding,
state the exact IDs, confirmed condition, missing evidence, and reversible next step.
Present affected objects with `present_entities`. Any status proposal is a separate
single-object draft; never cascade parent and children implicitly.
