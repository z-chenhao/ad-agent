# Advertising capabilities

Status: source-derived capability map, 2026-09-05. Primary protocol evidence is TikTok's
official Business API SDK at commit
[`f809c39`](https://github.com/tiktok/tiktok-business-api-sdk/tree/f809c396520df2d7b201a9ccc5378d822b728ed3).
SDK presence proves neither app scope nor live advertiser support. All MAPI mutation
claims below are HTTP-fake evidence until developer-app approval permits platform tests.

## Operating-domain map

The runtime exposes five advertiser skills, not one skill per endpoint or screen. A skill
provides judgment and evidence rules; typed tools provide capability; only the operator
approval service provides write authority.

| Operating domain           | Daily advertiser jobs                                                                  | Reads                                                                                                       | Approval-gated drafts                                                             |
| -------------------------- | -------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| Account operations         | Readiness, daily triage, hierarchy, pacing, delivery, finance, change review           | Context, hierarchy, reports, pending changes, balance, transactions                                         | Budget and status                                                                 |
| Performance insights       | Comparable-window monitoring, funnel diagnosis, contributor and creative investigation | Reports, isolated analysis, assets, review state                                                            | None                                                                              |
| Campaign operations        | Build and maintain conventional traffic or web-conversion delivery                     | Identity, asset, audience, targeting, measurement, exact objects                                            | Disabled campaign bundle; ad-group delivery/targeting; ad creative; budget/status |
| Audience and assets        | Reusable targeting, identity, creative, catalog, lead-form, and comment work           | Audience/overlap, targeting options, identity, asset/review, catalog health, form schema, exact-ad comments | Saved/lookalike audience; reply/hide/unhide/delete comment                        |
| Measurement and automation | Source readiness, attribution limits, event activity, rule audit                       | Sources, event stats, attribution, rule history                                                             | Pixel/offline source; automated rule                                              |

Manager scope has one additional guide for account triage and advertiser-bound reads or
changes. It preserves each advertiser's identity, currency, timezone, attribution, and
approval boundary; it is not a separate agent persona.

## Typed MAPI coverage

| Capability                                                                | Official v1.3 family                        | Sandbox                    | TikTok adapter          | Evidence                                |
| ------------------------------------------------------------------------- | ------------------------------------------- | -------------------------- | ----------------------- | --------------------------------------- |
| Campaign/ad-group/ad reads and reporting                                  | Campaign, Adgroup, Ad, Integrated Reporting | Yes                        | Yes                     | Contract + HTTP fake                    |
| Budget and delivery status                                                | Campaign/Adgroup update, status update      | Yes                        | Yes                     | Approval/read-back + HTTP fake          |
| Conventional campaign creation                                            | Campaign create, Adgroup create, Ad create  | Persistent disabled bundle | Ordered disabled bundle | HTTP fake; live pending                 |
| Ad-group bid, schedule start/end, placement, audience, location, language | Adgroup update                              | Persistent                 | Yes                     | HTTP fake; live pending                 |
| Ad creative binding, copy, CTA, destination                               | Ad update                                   | Persistent                 | Patch update            | HTTP fake; live pending                 |
| Saved and lookalike audiences                                             | DMP saved/custom audience                   | Persistent                 | Yes                     | HTTP fake; live pending                 |
| Identity, creative asset, review readiness                                | Identity, Creative Management, File         | Yes                        | Yes                     | HTTP-fake reads                         |
| Targeting discovery                                                       | Tool APIs                                   | Yes                        | Yes                     | HTTP-fake reads                         |
| Pixel and offline source creation                                         | Pixel, Offline Events                       | Persistent                 | Yes                     | HTTP fake; live pending                 |
| Event and attribution diagnostics                                         | Measurement, App, Reporting                 | Yes                        | Yes                     | HTTP-fake reads                         |
| Automated rule audit and creation                                         | Optimizer Rule                              | Persistent                 | Yes                     | HTTP fake; activation semantics pending |
| Exact-ad comment read and action                                          | Comments                                    | Persistent                 | Yes                     | HTTP fake; live pending                 |
| Catalog/product-set health                                                | Catalog                                     | Yes                        | Read only               | HTTP-fake reads                         |
| Lead-form inventory/schema                                                | Lead Generation                             | Yes                        | Read only               | HTTP-fake reads; no lead values         |
| Balance and transaction monitoring                                        | Business Center payment                     | Persistent snapshots       | BC-bound reads          | HTTP fake; live pending                 |

Campaign creation is deliberately a typed dependency graph, not arbitrary JSON. Traffic
and web-conversion are the only initial objectives. Every new campaign, ad group, and ad
starts disabled. Preparation verifies current identity, approved asset, audience,
targeting, and measurement dependencies. Apply records request/resource IDs at each
step; partial and unknown outcomes are never blindly retried.

The official automated-rule create model has no status member. The adapter therefore
does not send an invented status field. Rule activation/default behavior is explicitly
unverified and must be tested on the platform before controlled live use.

## Staged platform-native domain

Smart Plus and GMV Max remain one staged skill. Their store, material, identity,
authorization, session, review, and reporting semantics are not safely equivalent to a
conventional campaign bundle. They need native types, policy, and platform evidence
before activation.

## Deliberate exclusions

- raw MAPI paths or arbitrary JSON tools;
- customer-list rows, lead values, event payloads, or personal identifiers in model context;
- asset binary upload, catalog destructive mutation, appeal, sharing, blocked-word lists,
  payment transfer, member/permission changes, or app authorization changes;
- model approval, autonomous publishing, scheduled optimization, or automatic fallback
  from TikTok to Sandbox;
- claiming Smart Plus, GMV Max, automated-rule activation, or live mutation semantics
  from SDK availability alone.

## Activation gate

After app approval, validate exact scopes and controlled advertiser readbacks in this
order: account/report, resource dependencies, one disabled campaign bundle, one
single-object update, audience/source creation, comments, billing, and automated rules.
Record platform-sandbox and controlled-live evidence separately from local Sandbox and
HTTP-fake tests.
