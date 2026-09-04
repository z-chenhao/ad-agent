# Commercial Agents alignment

Reference: Anthropic `commerce-agents` at commit `fd4d592` (2026-08-31). This comparison
uses behavior and contracts, not matching file names or SDK choices. Merchant-only
catalog, cart, customer, and order semantics are outside the advertising domain.

## Alignment matrix

| Commercial Agents capability                   | Ad Agent implementation                                                                                                | Status                                                     |
| ---------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| One operating contract across runtime variants | `AGENT.md`, the generated active-skill index, and the same typed Go tools are supplied to Pi and J-agent               | Aligned                                                    |
| Capability-gated skills                        | One validated manifest generates runtime discovery; staged TikTok workflows remain unavailable until their tools exist | Aligned                                                    |
| Ground before analysis                         | The Go harness performs intent-specific report or pending-change grounding before the model loop                       | Aligned                                                    |
| Follow through on requested changes            | A second bounded pass runs if an actionable request ends without a stage attempt                                       | Aligned                                                    |
| Separate analysis agent                        | `run_analysis` starts an isolated read-only delegate over server-issued snapshots and deterministic calculations       | Aligned                                                    |
| Server-trusted presentation                    | Presentation records are validated and enriched from host records; model text cannot invent entity state               | Aligned                                                    |
| Progressive UI feedback                        | Tool-start deltas and pending presentation records stream before trusted replacement records                           | Aligned; event granularity is host-specific                |
| Close after suggestions                        | `present_suggestions` is terminal and both runtime adapters remove the remaining tool surface                          | Aligned                                                    |
| Parallel independent work                      | Pi uses its agent engine; J prefetches independent calls concurrently; the Go host serializes mutations                | Aligned                                                    |
| Human approval and read-back                   | Draft, authenticated host approval, revalidation, one write attempt, and reconciliation are separate states            | Aligned                                                    |
| Durable business memory                        | Explicit tools plus an isolated post-turn extractor support account-scoped recall, upsert, and deletion                | Aligned                                                    |
| Provider-context efficiency                    | Pi compaction and provider-native cached-input accounting are retained; J checkpoints its native model history         | Equivalent intent, provider-specific mechanism             |
| Replaceable runtime                            | A private Go `Runtime` seam supports Pi and a true J-owned model/tool loop without moving business authority           | Aligned and intentionally runtime-neutral                  |
| Multiple deployment modes                      | Local CLI and Web share one host; no managed-cloud or MCP deployment surface                                           | Intentionally excluded for the current single-user product |
| Merchant backend                               | `AdBackend` provides fixture and TikTok MAPI implementations                                                           | Domain-equivalent seam                                     |
| Merchant portal journey                        | Overview, hierarchy, change review, persistent assistant, approvals, activity, and memory inspector                    | Product-flow aligned, advertising-specific                 |

## What is deliberately different

- ChatGPT OAuth and `openai-codex/gpt-5.6-luna` replace Anthropic credentials and Claude
  runtimes. Model transport is a private adapter choice, not an advertising contract.
- Go owns advertising authority, provenance, approval, audit, and SSE. React never owns
  a write decision. This preserves runtime replacement with Pi, J-agent, Claude SDK, or a
  future adapter at a settled turn boundary.
- Analysis uses bounded typed datasets and server calculations instead of arbitrary code
  execution. The current tasks do not justify a general sandbox.
- TikTok workflows are activated only when typed tools, fixture evidence, wire tests, and
  platform evidence exist. Documentation alone never becomes a claimed capability.
- Managed Agents and MCP deployment are not current-user requirements. Adding them now
  would widen authentication, tenancy, and public-contract scope without evidence.

## Remaining non-parity work

Core harness behavior is aligned. Remaining work is advertising-domain breadth and live
platform acceptance:

1. implement and activate staged workflows one vertical slice at a time;
2. add golden conversations as each staged workflow becomes executable;
3. verify TikTok authorization, field semantics, attribution, and reconciliation after
   developer-app approval;
4. keep live writes disabled until a separate controlled-write acceptance gate passes.
