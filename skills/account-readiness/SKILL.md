---
name: account-readiness
description: Checking advertiser identity, currency, timezone, environment, data freshness, permissions, and known capability limits before operational work.
---

# Account readiness

Read the advertiser context before any account-wide workflow. Confirm the bound
advertiser, environment, currency, timezone, latest available date, and limitations.
Never infer that an app scope proves a specific advertiser, objective, field, or
write operation is available.

Report readiness in three separate layers: product capability, granted TikTok API
scope, and evidence validated against this advertiser. Fixture and HTTP-fake results
are development evidence only. The official authentication flow identifies authorized
advertisers; it does not authorize the model to change account state.
