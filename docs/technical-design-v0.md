# Ad Agent technical design v0

Status: implementation baseline v0.12, 2026-09-04. Pi, J-agent, and Claude Agent SDK
share one Go runtime seam. The React local sandbox loop is persistent and lifecycle
capable; the TikTok HTTP adapter has read/write HTTP-fake evidence only. The developer
app is still pending platform approval.

TikTok contract baseline: official Business API SDK commit
[`f809c39`](https://github.com/tiktok/tiktok-business-api-sdk/tree/f809c396520df2d7b201a9ccc5378d822b728ed3).

## Decision summary

Build one local advertising operations agent for one authenticated operator. It can use
a host-bound `single_advertiser` scope or a host-authorized `portfolio` of independently
bound advertisers. This is one agent contract, not a separate agency persona. Go owns domain truth, tools, safety gates, change state,
audit, HTTP, and SSE. React owns presentation. Pi, J-agent, and Claude Agent SDK are
replaceable private runtime adapters. Authentication/transport is orthogonal: Pi and J
support ChatGPT OAuth or an explicit direct provider; Claude Agent SDK supports direct
Anthropic API-key transport. `openai-codex/gpt-5.6-luna` through ChatGPT OAuth remains
the default.

The product adopts a capability-gated harness:

1. A static operating contract plus a generated index of active workflow skills.
2. Stable JSON-schema tools routed through one Go executor.
3. Host-bound account identity and per-session read provenance.
4. An isolated, read-only analysis delegate for multi-slice computation.
5. Staged changes with host-only approval, revalidation, apply, and reconciliation.
6. Server-enriched presentation records and a typed lifecycle event stream.
7. A manifest that separates working skills from official TikTok workflows whose
   typed backend and tools have not yet been implemented.

The smallest honest design is not a generic ad-platform framework. The current stable
seams are the host safety boundary and typed workflow contracts. Platform wire models,
runtime checkpoints, skill recipes, and tool names remain repository-private and
experimental.

## Requirements, users, and evidence

Current users: a direct advertiser operator managing one authorized advertiser, or a
media buyer triaging several advertiser accounts already authorized in a portfolio.

Current working jobs:

- validate account context and data limitations;
- produce a daily briefing;
- inspect campaign, ad group, and ad hierarchy;
- monitor and diagnose performance with deterministic evidence;
- analyze ad-level performance without claiming unsupported creative causes;
- prepare one budget or operation-status draft;
- review, discard, and reconcile staged changes.
- rank portfolio accounts without collapsing currency or attribution semantics, drill
  into one account, and prepare independent account-scoped drafts.

Documented but not yet installed jobs include campaign building, audiences, identity and
creative assets, measurement, automated rules, comments, catalog commerce, billing, and
Smart Plus/GMV Max. The official SDK proves that API families exist, not that this app's
scope, advertiser, region, or account type may use them.

Downstream contracts:

- TikTok Business API v1.3 and the app's granted scopes;
- Pi's `openai-codex-responses` transport, the current Codex model allowlist, and
  ChatGPT OAuth;
- direct Anthropic Messages, OpenAI Responses, or OpenAI-compatible Chat Completions
  selected with explicit HTTPS base URL, provider/model, token limits, and key-variable
  name;
- Claude Agent SDK's subprocess/session contract and Anthropic API-key policy;
- the Go domain, store, executor, and HTTP/SSE protocol;
- React's discriminated presentation records;
- SQLite session, provenance, change, approval, attempt, memory, and audit records.

## Mechanism versus policy

### Invariant mechanism

- Validate every tool call against its complete JSON schema before dispatch.
- Bind advertiser, backend, environment, and operator in the host, never model input.
- Treat external strings as untrusted data with no authority.
- Preserve report scope, timezone, dates, currency, attribution, freshness, and
  completeness.
- Establish mutation provenance only through parent-session entity reads.
- Route writes through `draft -> approve -> revalidate -> apply -> reconcile`.
- Never retry a write whose send outcome is unknown.
- Commit runtime checkpoints only after a settled, paired turn.
- Expose public lifecycle facts, never private reasoning, provider state, or credentials.

### Current policy

Single user, local deployment, TikTok first, direct-advertiser scope by default,
portfolio scope initially available against the local sandbox only, Luna/ChatGPT OAuth as the default, medium
reasoning, six parent tool rounds, two analysis delegates, eight child rounds, one object
per change, and TikTok writes disabled by default are current policies. They are not
public runtime semantics.

## Harness alignment assessment

The first implementation already had fencing, schemas, grounding, an analysis child,
staging, approval separation, trusted UI records, memory, events, and replaceable runtimes. Its
main shortcoming was product composition: only three hard-coded skills were visible and
the tool schema duplicated their names.

v0.8 closes the runtime-neutral harness and operator-loop gaps:

- every skill has `name` and `description` frontmatter;
- `skills/manifest.json` owns active versus staged status and required tools;
- the host generates the skill index and `load_skill` enum from active entries;
- staged skills remain inspectable source artifacts but cannot be loaded by the model;
- the contract requires a same-turn staging attempt when an authorized request is
  actionable, while keeping approval outside chat.
- forced host grounding precedes model work for performance and change-review intents;
- both runtimes expose tool-start deltas, support concurrent independent reads, and
  stop after the terminal suggestion presentation;
- `present_digest` produces a server-enriched decision queue and presentation calls
  emit an immediate pending UI record before their trusted replacement;
- a post-turn isolated pass on the session model extracts only durable operator preferences,
  constraints, and goals into account-scoped memory;
- the React portal follows the operator journey through overview, hierarchy, change
  review, assistant, activity, and memory rather than exposing one dashboard page.

v0.11 adds peer Claude runtime composition, explicit model authentication/protocol
configuration, a persistent lifecycle sandbox, and professionally expanded skills with
evidence and output contracts. Remaining work is domain capability expansion, not
missing core harness architecture:
the staged TikTok workflows require typed tools and backend support, and real MAPI
acceptance remains blocked on developer-app approval. A generic code-execution sandbox
is deliberately excluded because deterministic report slices and calculations cover
the current advertising analysis contract with a smaller attack surface. See
`commercial-agents-alignment.md` for the evidence-by-capability comparison.

v0.12 adds portfolio scope without forking the runtime or domain model. A portfolio is
an experimental host-side routing boundary containing account-bound Reader/Writer/
Creator/Policy bindings. It adds account-list, portfolio-report, account-object, and
account-scoped staging tools. It does not add apply authority to the model. Each sandbox
advertiser persists in a separate namespace, and the Web/CLI reuse the same approval
ledger and model configuration.

## Architecture

```text
Authenticated operator -> React -> Go HTTP/SSE host -> SQLite
                                   |
                                   +-> scope
                                   |     +-> single advertiser -> one AdBackend
                                   |     +-> portfolio -> account router -> N AdBackends
                                   |
                                   +-> session-pinned provider/model
                                   |     +-> Pi full-agent sidecar
                                   |     +-> J-agent loop -> model-only bridge
                                   |     +-> Claude Agent SDK sidecar
                                   |     +-> ChatGPT OAuth (Pi/J)
                                   |     +-> direct HTTP + env API key
                                   |
                                   +-> one tool executor -> provenance and gates
                                                         -> AdBackend (Reader + Writer)
                                                              +-> persistent local sandbox
                                                                  +-> Creator
                                                                  +-> environment storage
                                                              +-> TikTok MAPI

Host approval route -> change service -> host-only Writer or Creator slice
Analysis delegate   -> bounded snapshots and deterministic calculations only
```

Go owns all business authority. Runtime adapters own model/provider lifecycle only.
React never calls TikTok directly and never decides whether a change is allowed.

The local Sandbox AdBackend is a deterministic, stateful advertising environment. Each
environment begins from a canonical fictional seed and persists its own campaigns, ad
groups, ads, updates, sessions, approvals, and audit records in an environment-keyed
SQLite namespace. It implements Reader, Writer, and an experimental Creator slice.
Product failure behavior is exercised through test fakes rather than named scenarios.
This is product acceptance evidence, not TikTok protocol or platform evidence.

Portfolio scope does not merge account data into a new universal backend. It retains a
bounded map from authorized advertiser ID to a complete AdBackend binding. Reads and
writes route only after scope validation; change records retain the underlying account
source so approval and reconciliation return to the same binding. The portfolio report
is a list of account reports, never a synthetic financial total.

## Runtime seam

```go
type Runtime interface {
    Run(context.Context, Request, Hooks) (Result, error)
}

type Hooks struct {
    Execute    func(context.Context, Call) ToolResult
    Emit       func(Event) // public tool/presentation deltas only
    CloseAfter func(Call, ToolResult) bool
}
```

The seam is repository-private and experimental. It contains the static contract,
fenced context, ordered tools, budget, and an opaque checkpoint reference. It does not
contain a TikTok credential or an approval mark. Switching runtime is allowed only at a
settled turn boundary and starts a new runtime session from portable user-visible facts.
Provider-native reasoning and checkpoint state never migrate between adapters or model
selections.

`ModelSelection` declares provider, model, reasoning, authentication mode, and, for
direct models, protocol, HTTPS base URL, key environment-variable name, context window,
and output limit. Pi and J register that provider with Pi's in-memory `ModelRuntime` and
never persist the key. Claude Agent SDK is a peer runtime, not a Pi wrapper; it receives
only an Anthropic Messages API-key selection and in-process SDK MCP tools that call back
to Go. Personal Claude login is deliberately unsupported for third-party embedding.

## Analysis delegate

`run_analysis(question, dataset_refs)` starts a fixed read-only child using the selected
runtime. It receives server-issued report handles rather than the parent conversation.
Its only tools read delegated snapshots, slice typed dimensions, perform deterministic
calculations, report progress, and submit a strict result.

Portfolio scope uses the same isolation rule through `run_portfolio_analysis`: the
delegate receives one server-issued portfolio report and only a strict submission tool.
It cannot read a different advertiser, stage a change, or apply anything. Account-level
currency, timezone, completeness, and limitations remain in the submitted result.

The child cannot stage, discard, approve, apply, access arbitrary URLs or files, or
delegate again. Numeric findings must reference server-computed evidence. Contribution
and correlation are not causality. Child entity IDs do not grant parent mutation
provenance.

## Skill and capability model

The manifest is the repository-private source of truth:

- `active`: every required tool exists and the runtime may load the skill;
- `staged`: the official workflow is designed, but at least one typed backend/tool or
  validation gate is missing; the runtime cannot load it.

Skills are workflow policy, not authority. Tool schemas and executor gates remain
authoritative. A skill never calls raw MAPI paths, reads tokens, or turns a platform API
family into permission. See `tiktok-workflow-coverage.md` for the complete matrix.

## Active tool surface

Reads: advertiser context, campaign hierarchy, exact entity, performance report,
pending changes, and account-scoped memory.

Analysis: bounded delegate plus typed dataset, slice, calculation, progress, and submit
tools.

Drafts: campaign/ad-group budget, campaign/ad-group/ad operation status, and, only when
the backend exposes Creator, sandbox campaign/ad-group/ad creation. The model has no
apply tool.

Presentation: trusted metrics, entities, change preview, and suggestion chips.

Workflow: generated `load_skill` over active manifest entries only.

## Report semantics

Every report preserves source, environment, advertiser, request ID, inclusive dates,
timezone, fetched time, data-as-of when known, level, dimensions, filters, currency,
metric definitions, attribution basis, completeness, truncation, and limitations.

Money uses decimal values. Missing is distinct from zero. Ratios use summed numerator
and denominator. Different hierarchy levels are never summed. A capped preview cannot
prove an account-wide ranking; full bounded snapshots remain available to deterministic
analysis.

## Change lifecycle

The model may stage exactly one field family on one grounded object or one creator
request in a lifecycle-capable environment. The server records before/after or create
payload, grounded parent, source, reason, expiry, and risk. The approval route is an
authenticated host action with CSRF protection and an atomic state claim.

Application re-reads the object, checks source drift, reruns guardrails, sends at most
one write attempt, and reads back. Outcomes distinguish not-sent, rejected,
acknowledged, and unknown. Unknown becomes indeterminate and cannot be blindly retried.

## Web contract

The Go server exposes account or portfolio scope, hierarchy, turn, change, memory,
configuration, and health
endpoints on loopback. POST turns stream typed SSE events. React handles `(turnId, seq)`
idempotently and only renders known discriminated card types. Unknown cards show a safe
fallback. Approval is never optimistic and waits for server read-back.

The React workspace uses shadcn/ui-compatible local components and Tailwind CSS. Its
desktop shell has focused Home, Campaigns, and Changes routes plus a persistent assistant
rail; mobile moves navigation and assistant into explicit sheets. Home prioritizes an
account briefing and decision queue, Campaigns drills one hierarchy level at a time, and
Changes is the only approval surface. The activity and memory inspector exposes public
tool lifecycle facts and account-scoped business memory, never provider reasoning.
In portfolio mode, an advertiser triage page replaces the direct account overview and
hierarchy routes. Deep diagnosis proceeds by explicit advertiser drill-down; each draft
shows and preserves its advertiser source.

## Memory extraction

After a successful main turn, the host may run a separate session on the selected model
with exactly one
private `record_memory_fact` tool. It receives only user and assistant text, never tool
payloads, account objects, or the main provider checkpoint. The host accepts at most
three durable operator-stated preferences, constraints, or goals and rejects credentials,
identifiers, metrics, transient status, budgets, personal data, and audience data. Facts
upsert by a normalized account-scoped key. Extraction is best-effort and cannot fail the
main turn; local operators can disable it with `--auto-memory=false`.

## Deliberately not generalized

- no universal Meta/Google/TikTok domain model;
- no arbitrary JSON MAPI executor or plugin marketplace;
- no universal planner/researcher/synthesizer role system;
- no provider transcript in a public event or database contract;
- no autonomous optimization or chat-based approval;
- no separate AgencyAgent persona, shared credential bucket, cross-currency portfolio
  total, or live portfolio binding until each TikTok advertiser authorization is proven;
- no live implementation for a staged skill until typed tools and real platform evidence
  exist.

## Validation

Required local gates:

```sh
make cli
make test
./bin/ad-agent inspect
```

The skill manifest must parse, metadata must match each `SKILL.md`, active entries must
name installed tools, staged entries must not enter `load_skill`, and tracked authored
files must contain no Chinese UI or documentation text. Live-model and live-MAPI checks
remain opt-in and are recorded separately.
