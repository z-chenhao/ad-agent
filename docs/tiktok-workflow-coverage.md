# TikTok workflow coverage

Status: source-derived capability map, 2026-09-04. Primary evidence is TikTok's
official Business API SDK at commit
[`f809c39`](https://github.com/tiktok/tiktok-business-api-sdk/tree/f809c396520df2d7b201a9ccc5378d822b728ed3).
Portal documentation linked from the SDK remains authoritative for account-specific
availability. SDK presence does not prove app scope, advertiser eligibility, regional
availability, or product support.

## How to read this document

`Active` means every tool named by the skill exists in the Go host today. `Staged`
means the official TikTok workflow has been extracted into a skill, but the runtime
cannot load it because at least one typed read, draft, guardrail, or platform validation
is missing.

This distinction prevents two failures: a narrow agent that knows only reporting, and a
wide agent that hallucinates capabilities from API documentation.

## Daily advertiser job map

| Workflow                         | Status | Daily operator job                                                           | Official API evidence                             | Current implementation                                 |
| -------------------------------- | ------ | ---------------------------------------------------------------------------- | ------------------------------------------------- | ------------------------------------------------------ |
| Account readiness                | Active | Confirm advertiser, environment, currency, timezone, freshness, and limits   | Authentication; Account Management                | Bound account context                                  |
| Daily account briefing           | Active | Decide what needs attention first                                            | Reporting; Campaign; Adgroup; Ad                  | Reports, hierarchy, pending changes                    |
| Campaign structure audit         | Active | Find parent/child status and budget-scope problems                           | Campaign; Adgroup; Ad get endpoints               | Three-level hierarchy and exact reads                  |
| Performance monitoring           | Active | Track spend, value, delivery, and goal pace                                  | Integrated Reporting                              | Comparable report windows                              |
| Performance diagnosis            | Active | Explain movement and contributors                                            | Integrated Reporting                              | Isolated analyst and deterministic evidence            |
| Budget operations                | Active | Review pacing and draft a bounded budget change                              | Campaign and Adgroup update; Reporting            | Draft plus approval-gated Backend write                |
| Delivery operations              | Active | Investigate and draft enable/disable                                         | Campaign, Adgroup, and Ad status update           | Draft plus approval-gated Backend write                |
| Creative performance             | Active | Rank ad-level performance and identify investigation leads                   | Ad get; Integrated Reporting; Creative Management | Ad-level data, no asset-detail claims                  |
| Change governance                | Active | Review, discard, apply, or reconcile drafts                                  | Campaign, Adgroup, and Ad update families         | Host-owned ledger, approval, read-back, reconciliation |
| Campaign building                | Staged | Create objective-specific campaign/ad-group/ad drafts                        | Campaign Creation; Adgroup; Ad; Tool              | Requires typed prerequisites and bundle draft          |
| Targeting and audiences          | Staged | Review targeting, overlap, custom/saved audiences, and lookalikes            | Audience; Recommend Tool; Tool                    | Requires audience and targeting readers                |
| Creative and identity operations | Staged | Reuse assets, inspect identities/review, and prepare creative drafts         | Creative Management; File; Identity; Ad ACO       | Requires asset/review readers and drafts               |
| Measurement and attribution      | Staged | Audit pixels, app/offline events, event activity, and optimization readiness | Measurement; App Management; Reporting            | Requires event-source and stats readers                |
| Automated rules                  | Staged | Audit rules, bindings, conflicts, and execution history                      | Automated Rules                                   | Requires rule and result readers plus staged mutations |
| Comment moderation               | Staged | Triage comments, draft replies, and prepare moderation actions               | Comments                                          | Requires comment/thread reads and staged actions       |
| Catalog commerce                 | Staged | Monitor feed, product, set, and catalog-video health                         | Catalog                                           | Requires typed catalog health contracts                |
| Billing and account finance      | Staged | Monitor balance, transactions, budget, and runway                            | Business Center and payment APIs                  | Requires finance-safe read contracts                   |
| Smart Plus and GMV Max           | Staged | Review native campaigns, materials, stores, products, and reports            | Smart Plus and GMV Max endpoint families          | Requires native types, not legacy coercion             |

## Official endpoint families behind the staged work

### Campaign creation

The official Campaign, Adgroup, and Ad APIs expose get, create, update, and status-update
operations. The SDK also exposes dynamic ad-group quota and validation tools. Activation
requires an objective-specific typed draft; one generic JSON payload would hide required
dependencies among destination, placement, schedule, optimization, billing, bid,
audience, identity, and creative.

### Targeting and audiences

The Audience API includes custom-audience list/get/create/update/delete, rule-based
audiences, lookalikes, sharing, saved audiences, and audience overlap. Tool APIs expose
locations, interests, behaviors, devices, languages, hashtags, targeting search, and
targeting details. These flows require privacy controls that keep source audience rows
outside model context.

### Creative and identity

Official APIs expose asset library video/image reads and uploads, identities and owned
post information, portfolios, ACO creative material, creative asset sharing/deletion,
and review/appeal state for newer campaign types. Reading, drafting, uploading, deleting,
sharing, appealing, and publishing are distinct authorities and must not be combined.

### Measurement

Measurement APIs expose pixel list/create/update, pixel events and event statistics, and
offline event sets. App Management exposes app metadata and optimization events.
Diagnostics should be read-only first: source ownership, last activity, event volume,
match signals, optimization eligibility, attribution basis, and lag.

### Automated rules

The official family exposes rule list/get/create/update, object binding, and execution
result list/get. A rule draft needs a simulation window, affected objects, conflict
detection, schedule, action bounds, and worst-case spend impact before activation.

### Comments

The Comments API exposes comment list, related comments, replies, hide/unhide, delete,
export tasks, and blocked-word operations. Comment text is untrusted. Posting and
destructive moderation require exact-item previews and host approval.

### Catalog commerce

Catalog endpoints cover catalogs, feeds and logs, products and logs, product sets,
catalog videos, and related commerce assets. Health is not equivalent to delivery: a
successful feed or product operation does not prove an ad is serving.

### Billing

Business Center APIs expose advertiser balance and transaction records alongside wider
asset, member, billing-group, and transfer operations. The initial workflow is read-only
and minimizes sensitive finance data. Transfers and authority changes stay out of scope.

### Smart Plus and GMV Max

The current official SDK includes native Smart Plus campaigns, ad groups, ads, review
information, material reports, plus GMV Max campaigns, sessions, reports, stores, videos,
identities, and exclusive authorization. These are independent domain variants and must
not be disguised as conventional auction objects.

## Harness behavior required for the skill suite

- The static prompt contains only the active skill index.
- `load_skill` accepts only manifest entries marked active.
- A staged skill remains source-visible to engineers but invisible to the model.
- Every active skill must reference only installed tool names.
- A change request that is executable attempts staging in the same turn; staging never
  implies approval.
- Unsupported work returns the missing typed capability, not a retry suggestion and not
  a fabricated result.
- Skills contain workflow judgment; schemas contain arguments; executor gates contain
  authority and safety.

## Activation sequence

The next product slice should be measurement health because it improves the reliability
of every performance and optimization conclusion without creating spend. Implement in
this order:

1. `list_event_sources` and `get_event_stats` domain contracts;
2. deterministic fixture cases for healthy, silent, delayed, partial, and unauthorized
   sources;
3. TikTok pixel/app/offline adapters and HTTP wire tests;
4. agent and React evidence cards;
5. sandbox or controlled-account reconciliation;
6. activate `measurement-and-attribution` in the manifest.

After measurement, implement creative/identity reads, then audience/targeting reads,
then automated-rule reads. Campaign creation should wait until all of its prerequisite
read contracts exist. Comments, catalog commerce, billing, and Smart Plus/GMV Max can be
scheduled according to the actual advertiser's business model.

## Deliberately excluded for now

- raw MAPI path or arbitrary JSON tools;
- customer-list rows or personal identifiers in model context;
- model-authorized publishing, deletion, payment, transfer, or permission changes;
- automatic campaign optimization or scheduled autonomous writes;
- claiming a staged skill is available merely because TikTok documents its endpoint.
