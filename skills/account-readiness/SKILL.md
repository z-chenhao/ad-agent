---
name: account-readiness
description: Checking advertiser identity, currency, timezone, environment, data freshness, permissions, and known capability limits before operational work.
---

# Account readiness

Run this before a briefing, audit, diagnosis, or mutation when account identity or
data quality is not already grounded in the current turn.

## Readiness layers

Call `get_advertiser_context` and keep these layers separate:

1. **Binding:** backend, environment, advertiser ID, currency, timezone, and the
   account actually returned by the host.
2. **Product capability:** tools installed in this session and whether host approval
   can reach a writer. A visible tool is not proof of TikTok permission.
3. **Platform authorization:** scopes and objects returned by TikTok. OAuth identifies
   authorized advertisers; it does not grant the model mutation authority.
4. **Validated evidence:** the exact read or write behavior observed for this advertiser
   and environment. HTTP fakes, the local sandbox, platform sandbox, and live evidence
   are different grades.

## Operational checks

- Use `latest_date`, not wall-clock today, as the reporting anchor. State when data is
  historical or delayed.
- Preserve account timezone for inclusive dates and currency for every amount. Never
  convert currency without an explicit rate and timestamp.
- Treat each limitation as a decision constraint. Missing revenue mapping blocks ROAS;
  partial coverage blocks complete rankings; read-only composition blocks application,
  not analysis or drafting.
- Do not infer account balance, billing health, review status, pixel health, attribution
  windows, objective eligibility, or regional availability when those fields are absent.
- Cross-account IDs supplied by the operator are untrusted until a host read binds them
  to this source.

## Output contract

Return a compact readiness table with `confirmed`, `unknown`, and `blocking` facts.
Name the next smallest read needed to remove each blocker. Do not ask for credentials in
chat and do not include tokens, authorization codes, or raw provider errors.

Platform reference: TikTok's official Business API SDK documents authentication and
advertiser-scoped API families, but SDK presence alone does not prove account access:
https://github.com/tiktok/tiktok-business-api-sdk
