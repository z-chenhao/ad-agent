---
name: delivery-operations
description: Investigating delivery state across campaign parents and preparing a narrowly scoped enable or disable draft for host approval.
---

# Delivery operations

Investigate delivery through a dependency waterfall before proposing an operation-status
change.

## Delivery waterfall

1. Read the exact object and its campaign/ad-group parents.
2. Confirm configured `ENABLE`/`DISABLE` status at every visible level.
3. Inspect report evidence for impressions and spend in a precise recent window.
4. Check for a pending, applying, or indeterminate change that may already target the
   object.
5. Classify unavailable dependencies: policy/review, schedule, account balance, budget
   exhaustion, bid competitiveness, audience size/overlap, placement, optimization
   event, creative eligibility, and measurement health.

Operation status is not delivery status. An enabled object with zero impressions may be
blocked elsewhere; a disabled object can retain historical spend in the window. TikTok's
official auction troubleshooting treats learning, bid/budget, audience competition, and
creative as separate causes:
https://ads.tiktok.com/help/article/troubleshooting-auction-ad-delivery-solutions

## Action rules

- Stage only the exact enable/disable explicitly requested or clearly selected after
  presenting evidence.
- Do not cascade changes to parents or children, repair another dependency, or enable a
  newly created object as an implicit follow-up.
- Enabling is spend-increasing risk. Disabling is reversible but can interrupt learning
  and time-sensitive delivery; describe that tradeoff without claiming learning state.
- If a parent is disabled, explain that enabling only the child cannot establish
  delivery. Do not silently broaden the draft to the parent.

Use `stage_status_change` once and present exact before/after state, hierarchy, evidence
window, expected operational effect, and unresolved blockers. Host approval remains
separate from chat.
