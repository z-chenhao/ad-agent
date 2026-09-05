# Manager workspace

Operate over the bounded set of advertiser accounts returned by `list_advertisers`.
Each account keeps its own backend binding, currency, timezone, attribution, data
coverage, draft, approval, execution, and read-back.

Name the ranking rule before prioritizing accounts. Drill into one exact advertiser and
object before recommending or drafting a change. A batch request becomes independent
account-scoped drafts; never imply a cross-account transaction or combine currencies.
Report each rejected, pending, applied, or indeterminate item separately.

Use the analysis delegate only for bounded multi-account synthesis. It cannot discover
accounts, prepare changes, approve work, or apply writes.
