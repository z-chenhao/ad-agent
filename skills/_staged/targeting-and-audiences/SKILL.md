---
name: targeting-and-audiences
description: Inspecting targeting, overlap, saved and custom audiences, lookalikes, exclusions, and privacy-safe audience readiness.
---

# Targeting and audiences

This workflow remains staged until typed audience, targeting-option, estimated-size,
and overlap reads exist.

## Audit model

Read each ad group's inclusions and exclusions across geography, age, language, device,
interest/behavior, saved audience, custom audience, lookalike, engagement audience, and
automatic/expanded targeting. Resolve IDs through the official Tool and Audience APIs;
never infer definition, size, recency, source, or eligibility from a name.

Evaluate four separate risks:

- eligibility: audience is ready, shared to the advertiser, and allowed for objective;
- privacy: source data and sensitive attributes remain outside model context;
- reach: estimated size is adequate for the goal without inventing a universal cutoff;
- interference: overlap, exclusions, and duplicate ad groups may fragment delivery.

Custom audience uploads can take time to become available and availability has a
minimum matched-size requirement; these are platform states to read, not assumptions:
https://ads.tiktok.com/help/article/manage-custom-audience?lang=en

## Future output and mutations

Return targeting clauses exactly as configured, estimated reach with timestamp, overlap
matrix, unknowns, and a testable recommendation. Audience creation, upload, sharing,
replacement, deletion, and ad-group targeting changes are distinct drafts. Never expose
customer rows, hashes, emails, phone numbers, device IDs, or upload files to the model,
logs, memory, or cards.
