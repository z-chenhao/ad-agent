---
name: automated-rules
description: Auditing automated rules and execution history, then staging bounded rule or binding changes with conflict checks.
---

# Automated rules

This workflow remains staged until rule definitions, bindings, schedules, and execution
history can be read and simulated.

For each rule, resolve bound objects, enabled state, timezone, evaluation schedule,
lookback window, metric and attribution definition, condition operator/threshold,
action, cooldown, notification, creation source, and recent executions. Distinguish a
rule that evaluated false from one that failed, lacked data, or did not run.

Audit conflicts before advice: two rules can alternately raise/lower budget, pause and
enable the same object, or compound spend across overlapping parents. Estimate the
maximum single action and worst-case cumulative exposure within the schedule. Unknown
metric completeness or attribution must block simulation, not be treated as zero.

A future draft includes a human-readable predicate, exact bound IDs, example true/false
evaluation, historical simulation window, affected-object count, action cap, cooldown,
conflicts, and rollback/disable plan. Create, edit, bind, unbind, enable, disable, and
delete remain separate approvals. Syntactic validation never justifies activation.
