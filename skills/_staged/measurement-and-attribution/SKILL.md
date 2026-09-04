---
name: measurement-and-attribution
description: Auditing pixels, app and offline event sources, event activity, attribution assumptions, and optimization-event readiness.
---

# Measurement and attribution

This workflow remains staged until pixel, app, offline-event, and event-stat reads are
typed. Audit source ownership, active events, last activity, match and volume signals,
optimization eligibility, attribution setting, and reporting lag. Do not transmit test
events, create sources, or alter event definitions during a diagnostic. A missing event
is not zero conversions unless the platform explicitly reports zero under a complete
window.
