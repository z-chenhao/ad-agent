# Commercial Agents alignment

Reference: Anthropic `commerce-agents` at commit `fd4d592` (2026-08-31). This comparison
uses behavior and contracts, not matching file names or SDK choices. Merchant-only
catalog, cart, customer, and order semantics are outside the advertising domain.

## Alignment matrix

| Commercial Agents capability                   | Ad Agent implementation                                                                                                    | Status                                                     |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| One operating contract across runtime variants | `prompts/ad-agent-system.md`, the capability-filtered skill index, and typed Go tools are supplied to Pi, J-agent, and Claude Agent SDK | Aligned                                                    |
| Capability-gated skills                        | One validated manifest generates runtime discovery; staged TikTok workflows remain unavailable until their tools exist     | Aligned                                                    |
| Ground before analysis                         | The Go harness performs intent-specific report or pending-change grounding before the model loop                           | Aligned                                                    |
| Follow through on requested changes            | A second bounded pass runs if an actionable request ends without a stage attempt                                           | Aligned                                                    |
| Separate analysis agent                        | Direct and portfolio scopes start isolated read-only delegates over server-issued reports with no mutation tools            | Aligned                                                    |
| Server-trusted presentation                    | Presentation records are validated and enriched from host records; model text cannot invent entity state                   | Aligned                                                    |
| Progressive UI feedback                        | Tool-start deltas and pending presentation records stream before trusted replacement records                               | Aligned; event granularity is host-specific                |
| Close after suggestions                        | `present_suggestions` is terminal and both runtime adapters remove the remaining tool surface                              | Aligned                                                    |
| Parallel independent work                      | Pi uses its agent engine; J prefetches independent calls concurrently; the Go host serializes mutations                    | Aligned                                                    |
| Human approval and read-back                   | Draft, authenticated host approval, revalidation, one write attempt, and reconciliation are separate states                | Aligned                                                    |
| Durable business memory                        | Explicit tools plus an isolated post-turn extractor support account-scoped recall, upsert, and deletion                    | Aligned                                                    |
| Provider-context efficiency                    | Pi compaction and provider-native cached-input accounting are retained; J checkpoints its native model history             | Equivalent intent, provider-specific mechanism             |
| Replaceable runtime                            | A private Go `Runtime` seam supports Pi, a true J-owned loop, and Claude Agent SDK without moving business authority       | Aligned and intentionally runtime-neutral                  |
| Multiple deployment modes                      | Local CLI and Web share one host; no managed-cloud or MCP deployment surface                                               | Intentionally excluded for the current single-user product |
| Merchant backend                               | `AdBackend` provides local Sandbox and TikTok MAPI implementations; Sandbox adds an experimental Creator slice             | Domain-equivalent seam                                     |
| Merchant portal journey                        | Overview, hierarchy, change review, persistent assistant, approvals, activity, and memory inspector                        | Product-flow aligned, advertising-specific                 |
| Host-bound multi-resource operation            | One Ad Agent supports direct and portfolio scopes; portfolio advertiser IDs route only among host-authorized AdBackends    | Mechanism aligned; advertising-specific scope              |

## What is deliberately different

- Commercial Agents defines `ShoppingAgent` and `MerchantAgent`; it does not define an
  `AgencyAgent`. Its reusable pattern is host-bound merchant/operator/session context and
  backend-mediated scope. Ad Agent applies that pattern to advertiser portfolios without
  copying merchant terminology or creating a second agent persona.

- Model connection is explicit and separate from runtime. Pi and J support ChatGPT OAuth
  plus direct Anthropic/OpenAI-compatible protocols; Claude Agent SDK is a peer runtime
  using an Anthropic API key. Luna through ChatGPT OAuth is the default, not a hard-coded
  product contract.
- Go owns advertising authority, provenance, approval, audit, and SSE. React never owns
  a write decision. This preserves runtime replacement with Pi, J-agent, Claude SDK, or a
  future adapter at a settled turn boundary.
- Analysis uses bounded typed datasets and server calculations instead of arbitrary code
  execution. The current tasks do not justify a general code-execution sandbox.
- TikTok workflows are activated only when typed tools, persistent local-sandbox evidence, wire
  tests, and platform evidence exist. Documentation alone never becomes a claimed capability.
- Managed Agents and MCP deployment are not current-user requirements. Adding them now
  would widen authentication, tenancy, and public-contract scope without evidence.
- Portfolio mode is a scope of the same host and runtime. Account IDs are authorized
  resources inside the portfolio, not authentication principals. Cross-account batch
  intent decomposes into independent drafts and approvals.

## Remaining non-parity work

Core harness behavior is aligned. Remaining work is advertising-domain breadth and live
platform acceptance:

1. implement and activate staged workflows one vertical slice at a time;
2. add golden conversations as each staged workflow becomes executable;
3. verify TikTok authorization, field semantics, attribution, and reconciliation after
   developer-app approval;
4. validate the implemented TikTok Writer with a controlled object before enabling it
   outside HTTP-fake tests.
