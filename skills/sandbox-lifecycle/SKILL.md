---
name: sandbox-lifecycle
description: Creating and verifying fictional campaign, ad-group, and ad objects inside one isolated Sandbox AdBackend environment through approval-gated drafts.
---

# Sandbox lifecycle

Use this workflow only when `get_advertiser_context` identifies the backend as
`sandbox`. It validates the product lifecycle and approval harness; it is not evidence
that TikTok accepted the same fields or that an object can deliver.

## Establish the environment

Read advertiser context and state the environment ID. An environment is a persistent,
isolated namespace: objects created in another environment or TikTok account are not
available here. List the relevant parent level before choosing a name or parent. Never
reuse an ID supplied only in chat.

## Build one level at a time

- Campaign: require a name and operation status. Accept an objective and a budget only
  as fictional planning fields. Supply `budget` and `budget_mode` together or neither.
- Ad group: read its campaign first. Require `parent_id`, name, and status. A sandbox ad
  group may carry a budget pair, but it does not model placement, targeting, schedule,
  optimization event, or bidding.
- Ad: read its ad group first. Require `parent_id`, name, and status. Do not attach
  budget, objective, identity, copy, destination, pixel, or creative fields because the
  current sandbox contract does not model them.

Default to `DISABLE` unless the operator explicitly requests an enabled fictional
object. An enabled draft is spend-increasing risk even though the sandbox cannot spend.
Use `stage_entity_create` exactly once for the requested level and then
`present_change_preview`. The tool creates only a draft; chat never approves it.

## Verify after host approval

After the operator uses the separate approval control, query the created level under
its exact parent. Confirm the returned ID, parent, name, status, and optional budget.
If it is absent, report the change state and do not create a replacement. If approval
has not occurred, describe the object as staged, never created.

## Output contract

Return: environment; requested level and parent; fields represented; fields deliberately
not simulated; change ID and state; and the exact read-back still required. Never call
this a platform sandbox, live campaign, publish, delivery test, or MAPI validation.
