# AdBackend contract

Status: alpha. Local Sandbox and TikTok HTTP
adapters implement reads, Change Service-controlled writes, and typed daily operations. Sandbox
also implements private lifecycle and hourly-simulation controls. TikTok wire shapes have
HTTP-fake evidence; live advertiser semantics remain unvalidated while app approval is
pending.

## Decision

TikTok MAPI is an external protocol. `tiktokmapi.Backend` and `sandbox.Backend`
implement the same complete AdBackend. AgentRuntime and AdBackend vary independently:
changing Pi, Codex, or a future runtime does not rewrite advertising integration;
changing the data source does not rewrite conversation, analysis, or approval behavior.

Manager scope is a host-side router over a bounded set of complete account bindings;
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

type CommonAdsReader interface {
    // typed identities, assets, audiences, targeting, measurement,
    // lead-form metadata, catalogs/product sets, and automated rules
}

type OperationPlanner interface {
    PrepareOperation(context.Context, OperationRequest) (OperationPlan, error)
}

type Operations interface {
    OperationPlanner
    ApplyOperation(context.Context, OperationPlan) OperationOutcome
    ReconcileOperation(context.Context, OperationPlan, OperationOutcome) (bool, error)
}

type OperationsReader interface {
    ListComments(context.Context, string, int) ([]Comment, error)
    GetBillingBalance(context.Context) (BillingBalance, error)
    ListBillingTransactions(context.Context, string, string) ([]BillingTransaction, error)
}
```

The unified Backend is split by authority at composition time: the agent host receives
Reader, CommonAdsReader, OperationsReader, and OperationPlanner;
the change service alone receives Writer, Creator, and operation execution. It must not be stretched into
a generic `Execute(name, map)` to cover documented future APIs. Each staged workflow
earns a typed capability interface from actual consumer fields and platform evidence.

`CommonAdsReader` is an experimental typed extension implemented by both current
backends. The portable concepts are intentionally limited to advertising-system concepts
that also exist outside TikTok: identities, creative assets, audiences, targeting,
measurement sources, lead-form schema, catalogs/product sets, and automated rules. TikTok
wire payloads remain private. Comments and billing remain explicit typed slices because
they have different privacy and authority semantics. Smart Plus and GMV Max are not
coerced into the conventional campaign contract because they are TikTok-native domains.

## Invariants

- The host binds connection, backend, environment, and advertiser.
- Manager membership is also application-bound. An advertiser ID in a tool call selects only
  among pre-authorized bindings and never acts as a credential.
- Reads never alter remote state.
- Local-sandbox, HTTP-fake, TikTok-platform-sandbox, and controlled-live evidence never
  mix or silently fall back.
- Reports preserve metric semantics, attribution, freshness, and completeness.
- Analysis cannot authorize a write.
- Only host approval reaches a Change Service-controlled writer.
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

`limitations` contains decision-relevant evidence gaps and qualifications, not product
documentation. Sandbox identity is carried by `Source` and the workspace source indicator;
simulation assumptions and validation guidance live in the simulator documentation.
Do not remove missing-data, backfill, attribution or unverified-review caveats to simplify UI.

Manager reports are collections of these account reports. They retain account-level
currency, timezone, attribution, completeness, and limitations and do not expose a
cross-currency total. Partial failure is represented on the affected account rather than
hiding all available accounts or silently substituting sandbox data.

## Calculation rules

The application retains report snapshots in current-turn memory and issues handles. The
model sees a bounded preview; deterministic calculations use the complete bounded snapshot.
There is no independent persisted dataset table: presented evidence can survive in saved
events, but full turn-local report maps are not reconstructed from SQLite on restart.

- ROAS is summed value divided by summed spend, never an average of row ROAS.
- CPA, CTR, CVR, CPM, and CPC follow the same summed-input rule.
- Zero denominator or missing required input makes a ratio unavailable.
- Incompatible level, filters, attribution, currency, timezone, or period blocks direct
  comparison.
- Contribution is not causality.

The live revenue metric remains unconfigured by default. App, website, and TikTok Shop
value semantics must be selected and validated for the advertiser before ROAS is shown.

## Capability evolution

New TikTok domains use typed interfaces owned by their real workflows. Cross-platform
advertising reads currently compose under experimental `CommonAdsReader`; mutations earn
separate request types and approval policy rather than weakening `WriteRequest` or using
optional untyped payloads. A second live provider must validate the semantic mapping before
this extension becomes stable.

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

The change ledger is not part of Backend. A Change Service-controlled writer is injected only into the
change service. The model never obtains it.

The TikTok writer supports single-object campaign/ad-group budget updates and
campaign/ad-group/ad operation-status updates. The typed Operations slice additionally
supports disabled conventional campaign bundles, ad-group bid, budget, schedule start/end,
placement and targeting fields,
ad creative fields, saved/lookalike audiences, automated rules, comment moderation/reply,
and pixel/offline source creation. TikTok mutation is disabled by default. The writer
reports not-sent, rejected, acknowledged, or unknown. The service owns
`staged -> applying -> applied|failed|expired|indeterminate`, atomic claims, approval
records, revalidation, guardrails, execution attempts, read-back, and reconciliation.
Network calls never hold the database transaction lock. A crash after possible send is
unknown, not safe to replay.

Campaign-bundle creation is an ordered three-call operation. Every acknowledged request
and returned resource ID is retained. Failure after a campaign or ad group is created is
`partial`, requires operator reconciliation, and is never replayed as a whole. The
official automated-rule create model does not expose a status field; the adapter does not
invent one, and activation behavior remains a platform-validation gate.

## Sandbox lifecycle

The local Sandbox implements persistent conventional campaign operations, not named test
scenarios. Parent relationships are enforced and environment-generated identifiers,
targeting, schedules, bids, identities, creative bindings, audiences, rules, event
sources, comments, balance snapshots, and transactions survive restart in one isolated
SQLite namespace. Created campaign bundles always begin disabled. Existing immutable
hourly facts do not change after an operation; future simulated delivery observes current
hierarchy, schedule, targeting, bid, creative, and budget state.

The Sandbox-private causal simulator generates cohort opportunities, eligibility, paced
participation, sampled competitor populations, ranked auction outcomes, clearing cost,
impressions, clicks, conversions, order value, attribution, and reports in that order.
Frequency, saturation, creative fatigue, learning state, seasonality, tracking loss,
reporting delay, and backfill persist with the environment. High-level efficiency metrics
are derived from events and spend. Model calibration, private true outcomes, competitor
state, and debug traces are not AdBackend fields and cannot enter Agent context. This is a
configurable behavioral abstraction, not a TikTok auction twin or a prediction of platform
lift. The complete boundary and assumptions are documented in `sandbox-simulator.md`.

## Testing

Local Sandbox AdBackend contract tests prove domain behavior. An independent HTTP fake
proves TikTok wire behavior. TikTok's platform sandbox proves scope and endpoint
behavior. A controlled test
advertiser proves live report reconciliation and, only with explicit authorization,
write outcomes. None substitutes for another.
