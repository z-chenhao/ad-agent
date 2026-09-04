---
name: change-governance
description: Reviewing, explaining, reconciling, or discarding staged changes while preserving independent host approval and audit semantics.
---

# Change governance

Treat the host ledger as authoritative. Read it before reviewing, discarding, or
discussing a retry.

## State semantics

- `staged`: proposal only; no backend request has been sent.
- `applying`: one approval attempt owns the change; do not interfere.
- `applied`: read-back matched the requested state or created object.
- `failed`: known rejection or known pre-send failure; inspect the note before redraft.
- `expired`: provenance or target changed, or the approval window elapsed.
- `discarded`: proposal intentionally closed without application.
- `indeterminate`: a request may have been sent but the effect is not confirmed.

Never translate “yes,” “go ahead,” or any chat content into host approval. Approval is
an authenticated control outside the model tool surface and applies one ledger record.

## Review

Use `get_pending_changes`, then `present_change_preview` for the exact server-authored
record. Verify source/environment, target or create parent, before/after or create
fields, currency, spend-increasing flag, reason, expiry, and current state. Group records
for explanation only; never merge their authority or imply atomic batch behavior.

## Failure handling

Discard only the exact `staged` change named by the operator. For `indeterminate`, use
the host's read-only reconciliation control and do not stage or apply a replacement
until resolved. Do not blindly retry a create because it can duplicate an object. A
failed draft may be recreated only after explaining whether the previous attempt was
known not to have taken effect.

Return state, evidence supporting it, safe next action, and whether backend state is
confirmed. Never claim that staging changed TikTok or that a sandbox approval proves
live-platform behavior.
