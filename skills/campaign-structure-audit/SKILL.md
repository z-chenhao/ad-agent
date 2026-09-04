---
name: campaign-structure-audit
description: Auditing campaign, ad group, and ad hierarchy for disabled parents, orphaned delivery, budget placement, and scope mismatches.
---

# Campaign structure audit

Traverse campaign to ad group to ad, respecting parent provenance. Inspect exact
objects before describing mutable fields. Look for enabled children under disabled
parents, disabled objects with recent spend, campaigns with no visible children, and
budgets configured at a different level than the operator assumes.

Operation status is not delivery status. Current tools do not expose every schedule,
review, bid, placement, targeting, or optimization field, so an audit must separate
confirmed structure findings from unavailable checks. Present the affected objects by
server-issued IDs and offer status changes only as separate drafts.
