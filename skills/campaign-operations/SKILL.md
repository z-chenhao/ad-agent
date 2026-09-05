---
name: campaign-operations
description: "Building disabled campaign bundles and preparing reviewed campaign, ad-group, targeting, bid, schedule, creative, budget, and delivery changes."
---

# Campaign operations

Treat campaign construction as a dependency graph, not three independent create calls.

## Preflight and build

For a new bundle, ground account currency/timezone and inventory. Establish objective, destination,
optimization event, billing event, budget mode/value, bid strategy/value, pacing,
schedule, placements, geography, audience inclusions/exclusions, identity, approved
asset, copy, CTA, and HTTPS destination. For `WEB_CONVERSIONS`, require an active pixel
and selected optimization event. Use only IDs returned by typed tools. Separate required
missing business inputs from optional schema defaults; do not interrogate the operator
about every field when a supported default is appropriate.

`stage_campaign_bundle` prepares one campaign, one ad group, and 1-20 ads. Every new
object starts `DISABLE`; activation is a separate decision after review. Check included
and excluded audiences for conflict. Asset readiness is not a promise that the new ad
will pass review.

The approved apply is sequential because TikTok needs the campaign ID before the ad
group and the ad-group ID before ads. If a later step fails, the change becomes partial/
indeterminate with created resource IDs. Never retry the whole bundle automatically;
inspect and reconcile first.

## Existing delivery changes

Use `stage_ad_group_update` for bid, schedule start/end, targeting, or a combined reviewed
ad-group update. Omitted fields remain unchanged. A targeting replacement can reset the
served population and disturb learning; describe exposure and uncertainty. Use the
dedicated budget tool for a simple one-field edit.
Inspect dependencies relevant to the requested fields, not the whole creation checklist.
Distinguish daily from lifetime budget and a bid cap from a target efficiency metric.
Evaluate pacing against elapsed account-local time; unspent budget alone does not prove
that increasing budget will buy more delivery. Bid, audience, inventory, and review can
remain the binding constraint.

Use `stage_ad_creative_update` for copy, CTA, destination, identity, or approved asset.
Check destination consistency, identity authorization, asset kind, and review state.
Material creative edits can trigger platform review. Use `stage_status_change` for
explicit enable/disable only after checking parent state.

Every stage call creates an exact host preview and never applies. Return the dependency
check, resulting draft, review implications, and the explicit approval or reconciliation
gate.
