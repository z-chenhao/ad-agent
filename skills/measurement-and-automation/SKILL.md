---
name: measurement-and-automation
description: "Operating measurement readiness and bounded automation through event-source checks, attribution evidence, source creation, rule audits, and approval-gated rule drafts."
---

# Measurement and automation

Measurement determines whether optimization and diagnosis are trustworthy; automation
amplifies both good and bad assumptions.

## Measurement readiness

Inventory supported pixels, apps, and offline sources with ownership, status, event
names, last activity, and limitations. Read aggregate event counts over an explicit
window. Confirm that an optimization event exists and has recent volume before using it.
Missing rows are not zero unless coverage is complete.

Preserve timezone, click/view attribution windows, event definition, lag, and source-of-
truth semantics. Analytics or MMP discrepancies can arise from windows, identity,
deduplication, consent, timezone, and revision lag. Never put event payloads, cookies,
IPs, customer identifiers, or uploaded offline rows into model context.

`stage_event_source_create` creates only pixel or offline-source metadata. It does not
install website code, configure Events API, upload events, or prove receipt. After
approval, verify source ID, status, and event configuration separately.
Do not submit event types when the backend supports metadata creation only. Source
existence is not integration health. Partial event coverage cannot establish that a
missing event never happened; tracking problems affect reported outcomes, not necessarily
actual purchases.

## Automated rules

Read existing rules and execution history first. Detect overlapping targets, conditions,
schedules, and opposing actions. Translate intent into metric, operator, value, lookback,
targets, schedule, and action. Prefer `NOTIFY` when evidence is weak. `PAUSE` needs a
recovery path. `CHANGE_BUDGET` needs an explicit bounded value, budget mode, and direction;
an increase can authorize future additional spend.

Before `stage_automated_rule_create`, evaluate the condition against available recent
complete reports when supported. This is a preflight estimate, not a historical rule
execution or guaranteed backtest. State target scope, largest plausible budget exposure,
and missing evidence. Low-volume CPA and delayed conversions can cause premature pauses;
require an operator-appropriate spend or signal threshold, not an invented universal one.
TikTok's v1.3 create
model does not expose initial status, so creation may activate according to provider
defaults; prefer `NOTIFY` until controlled platform validation. The initial portable
schedule is every 30 minutes because that is the only unambiguous recurring SDK enum in
this slice. A staged or acknowledged rule does not prove a future execution will occur.

Return source readiness, attribution limits, rule conflicts, simulation assumptions,
and the exact approval/reconciliation gate.
