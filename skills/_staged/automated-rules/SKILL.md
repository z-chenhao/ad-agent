---
name: automated-rules
description: Auditing automated rules and execution history, then staging bounded rule or binding changes with conflict checks.
---

# Automated rules

This workflow remains staged until rule definitions and execution results can be read.
Audit enabled state, object bindings, conditions, action, schedule, recent outcomes,
conflicts, and maximum spend impact. A future rule change is a staged draft with an
explicit simulation window and affected-object list. The agent must not activate a
rule merely because the rule validates syntactically.
