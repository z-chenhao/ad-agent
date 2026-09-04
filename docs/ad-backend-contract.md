# AdBackend contract v0

Status: experimental implementation baseline v0.7. Fixture and read-only TikTok HTTP
adapters exist. Live advertiser semantics have not yet been validated.

## Decision

TikTok MAPI is an external protocol. `tiktokmapi.Backend` and `fixture.Backend`
implement the same narrow read contract. AgentRuntime and AdBackend vary independently:
changing Pi, J-agent, or a future runtime does not rewrite advertising integration;
changing the data source does not rewrite conversation, analysis, or approval behavior.

```go
type Backend interface {
    Account(context.Context) (Account, error)
    List(context.Context, EntityQuery) ([]Entity, error)
    Get(context.Context, Level, string) (Entity, error)
    Report(context.Context, ReportQuery) (Report, error)
}
```

This interface supports the current active workflow set. It must not be stretched into
a generic `Execute(name, map)` to cover documented future APIs. Each staged workflow
earns a typed capability interface from actual consumer fields and platform evidence.

## Invariants

- The host binds connection, backend, environment, and advertiser.
- Reads never alter remote state.
- Fixture, sandbox, and live data never mix or silently fall back.
- Reports preserve metric semantics, attribution, freshness, and completeness.
- Analysis cannot authorize a write.
- Only host approval reaches a host-only writer.
- Tokens, secrets, raw headers, and unrestricted TikTok payloads remain inside auth and
  adapter boundaries.

## Data contracts

IDs are opaque strings. Money uses decimal values. Domain enums are explicit. TikTok
wire structs remain private to `tiktokmapi` and are converted at the adapter boundary.
Unknown platform fields are not carried through `map[string]any` merely for convenience.

Account context includes currency, timezone, status, latest data date, source, verified
capabilities, and limitations. An app scope does not prove that every objective, region,
account, object, or field supports the same operation.

A report includes:

- source, environment, connection, advertiser, and upstream request IDs;
- requested and covered dates, inclusive-date rule, timezone, fetched time, and data-as-of;
- level, dimensions, filters, currency, metric definitions, and attribution basis;
- complete or partial state, rows/pages obtained, known total, truncation, and gaps;
- decimal money, integer counts, nullable values, and missing reasons.

An empty set, permission denial, failed second page, and unsupported metric are distinct.
Multiple pages are not automatically a transactional snapshot.

## Calculation rules

The host stores immutable report snapshots and issues handles. The model sees a bounded
preview; deterministic calculations use the complete bounded snapshot.

- ROAS is summed value divided by summed spend, never an average of row ROAS.
- CPA, CTR, CVR, CPM, and CPC follow the same summed-input rule.
- Zero denominator or missing required input makes a ratio unavailable.
- Incompatible level, filters, attribution, currency, timezone, or period blocks direct
  comparison.
- Contribution is not causality.

The live revenue metric remains unconfigured by default. App, website, and TikTok Shop
value semantics must be selected and validated for the advertiser before ROAS is shown.

## Capability evolution

New TikTok domains use narrow typed interfaces owned by their real workflows, for
example `AudienceReader`, `MeasurementReader`, or `RuleReader`. They are composed beside
Backend rather than added as optional untyped payloads. A second real consumer may later
justify a shared interface; until then these contracts stay private and experimental.

Before a staged skill becomes active:

1. define the exact read and draft fields the skill needs;
2. implement fixture and TikTok adapters with the same semantic contract;
3. add HTTP wire tests for query encoding, pagination, business errors, rate limits,
   cancellation, missing fields, and cross-account rejection;
4. add agent and UI tests for grounded success, unavailable data, and refusal;
5. validate permitted fields and semantics in official sandbox or a controlled account;
6. only then expose the tools and activate the manifest entry.

## Writes

The change ledger is not part of Backend. A host-only writer is injected only into the
change service. The model never obtains it.

The writer reports not-sent, rejected, acknowledged, or unknown. The service owns
`staged -> applying -> applied|failed|expired|indeterminate`, atomic claims, approval
records, revalidation, guardrails, execution attempts, read-back, and reconciliation.
Network calls never hold the database transaction lock. A crash after possible send is
unknown, not safe to replay.

## Testing

Fixture contract tests prove domain behavior. An independent HTTP fake proves TikTok
wire behavior. Official sandbox proves scope and endpoint behavior. A controlled test
advertiser proves live report reconciliation and, only with explicit authorization,
write outcomes. None substitutes for another.
