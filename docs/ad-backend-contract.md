# AdBackend contract v0

Status: experimental implementation baseline v0.12. Local sandbox and TikTok HTTP
adapters implement reads and host-only writes. The sandbox additionally implements a
creator lifecycle. Live advertiser semantics have not yet been validated.

## Decision

TikTok MAPI is an external protocol. `tiktokmapi.Backend` and `sandbox.Backend`
implement the same complete AdBackend. AgentRuntime and AdBackend vary independently:
changing Pi, J-agent, or a future runtime does not rewrite advertising integration;
changing the data source does not rewrite conversation, analysis, or approval behavior.

`Portfolio` is a host-side scope/router over a bounded set of complete account bindings;
it is not another AdBackend and does not weaken account identity. Each binding carries
its Reader, optional Writer/Creator, and Policy. The router validates advertiser scope
before every operation and routes approval by the source stored on the change.

```go
type Reader interface {
    Account(context.Context) (Account, error)
    List(context.Context, EntityQuery) ([]Entity, error)
    Get(context.Context, Level, string) (Entity, error)
    Report(context.Context, ReportQuery) (Report, error)
}

type Writer interface {
    Write(context.Context, WriteRequest) WriteOutcome
}

type Backend interface {
    Reader
    Writer
}

type Creator interface {
    Create(context.Context, CreateRequest) (Entity, error)
}
```

The unified Backend is split by capability at composition time: the agent host receives
Reader and the change service receives Writer. A lifecycle-capable environment may also
give the change service the experimental Creator slice. It must not be stretched into
a generic `Execute(name, map)` to cover documented future APIs. Each staged workflow
earns a typed capability interface from actual consumer fields and platform evidence.

## Invariants

- The host binds connection, backend, environment, and advertiser.
- Portfolio membership is also host-bound. An advertiser ID in a tool call selects only
  among pre-authorized bindings and never acts as a credential.
- Reads never alter remote state.
- Local-sandbox, HTTP-fake, TikTok-platform-sandbox, and controlled-live evidence never
  mix or silently fall back.
- Reports preserve metric semantics, attribution, freshness, and completeness.
- Analysis cannot authorize a write.
- Only host approval reaches a host-only writer.
- Creation is a staged change; child creation requires a current read of its parent and
  post-approval read-back. A lost create outcome is never blindly retried.
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

Portfolio reports are collections of these account reports. They retain account-level
currency, timezone, attribution, completeness, and limitations and do not expose a
cross-currency total. Partial failure is represented on the affected account rather than
hiding all available accounts or silently substituting sandbox data.

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
2. implement local sandbox and TikTok adapters with the same semantic contract;
3. add HTTP wire tests for query encoding, pagination, business errors, rate limits,
   cancellation, missing fields, and cross-account rejection;
4. add agent and UI tests for grounded success, unavailable data, and refusal;
5. validate permitted fields and semantics in TikTok's platform sandbox or a controlled
   account;
6. only then expose the tools and activate the manifest entry.

## Writes

The change ledger is not part of Backend. A host-only writer is injected only into the
change service. The model never obtains it.

The TikTok writer currently supports single-object campaign/ad-group budget updates and
campaign/ad-group/ad operation-status updates. It is disabled by default. The writer
reports not-sent, rejected, acknowledged, or unknown. The service owns
`staged -> applying -> applied|failed|expired|indeterminate`, atomic claims, approval
records, revalidation, guardrails, execution attempts, read-back, and reconciliation.
Network calls never hold the database transaction lock. A crash after possible send is
unknown, not safe to replay.

## Sandbox lifecycle

The local sandbox implements `Creator` for campaign, ad-group, and ad shells. Parent
relationships are enforced, identifiers are generated by the environment, and each
environment persists under a separate SQLite namespace. Campaigns may carry objective
and a budget pair, ad groups may carry a budget pair, and ads currently contain only
name/status/parent. This narrow vocabulary validates hierarchy, staging, approval,
persistence, and query behavior; it does not pretend to model TikTok targeting,
scheduling, bidding, identity, creative, review, or publishing semantics.

TikTok creation stays outside Backend until an objective-specific consumer and real
platform evidence justify a typed creator contract. Sandbox capability is therefore not
silently exposed in TikTok composition.

## Testing

Local Sandbox AdBackend contract tests prove domain behavior. An independent HTTP fake
proves TikTok wire behavior. TikTok's platform sandbox proves scope and endpoint
behavior. A controlled test
advertiser proves live report reconciliation and, only with explicit authorization,
write outcomes. None substitutes for another.
