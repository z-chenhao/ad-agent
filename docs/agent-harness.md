# Agent harness

This record evaluates Ad Agent against its own runtime-neutral operating contract. It
uses observable behavior and downstream contracts rather than SDK names or file layout.
It describes application behavior and authority independently of implementation language.

## Capability matrix

| Capability                           | Current implementation                                                                                                                                   | Status                                                        |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| One contract across runtimes         | The compiled kernel, capability-filtered skill index and typed tools are supplied to Built-in Runtime, Pi, Codex and Claude                              | Implemented                                                   |
| Stable and dynamic prompt separation | A private compiler builds a byte-stable kernel, scope policy, tool-derived capabilities, and skill index; bounded per-turn JSON stays in the user prompt | Implemented; provider cache directives remain adapter-private |
| Capability-gated skills              | One validated manifest generates runtime discovery; unsupported workflows remain staged and cannot be loaded                                             | Implemented                                                   |
| Ground before analysis               | The Agent Application performs intent-specific report or pending-change grounding before the model loop                                                  | Implemented                                                   |
| Requested-change follow-through      | An optional second pass for a leading change imperative with no stage attempt; diagnostic/advisory wording alone does not trigger it                     | Implemented                                                   |
| Separate analysis agent              | Advertiser and Manager scopes start isolated read-only delegates over server-issued reports with no mutation tools                                       | Implemented                                                   |
| Server-trusted presentation          | Presentation records are validated and enriched from host records; stage success automatically emits the exact change preview                            | Implemented                                                   |
| Progressive UI feedback              | Tool-start deltas and pending presentation records stream before trusted replacement records                                                             | Implemented                                                   |
| Terminal suggestions                 | `present_suggestions` is issued with the final presentation batch and closes the tool surface after success                                              | Implemented in advertiser scope                               |
| Parallel independent work            | Native engines may schedule independent calls; the application validates each callback and Change Service serializes mutations                           | Adapter-dependent                                             |
| Human approval and read-back         | Draft, authenticated host approval, revalidation, one write attempt, and reconciliation are distinct states                                              | Implemented                                                   |
| Durable business memory              | Explicit tools plus an isolated post-turn extractor support account-scoped recall, upsert, and deletion                                                  | Implemented                                                   |
| Replaceable runtime                  | A private `Runtime` seam supports Built-in Runtime, Pi, Codex and Claude without moving business authority                                               | Implemented                                                   |
| Replaceable backend                  | `AdBackend` has persistent Sandbox and TikTok MAPI implementations behind typed Reader, Writer, and Creator capabilities                                 | Implemented locally; live platform acceptance remains pending |
| Host-bound manager operation         | One Agent supports Advertiser and Manager scopes; advertiser IDs route only among host-authorized backends                                               | Implemented for Sandbox                                       |

## Loop and safety policy

The Advertiser and Manager main loops have no fixed model-turn ceiling. The selected
runtime continues until the model ends the turn, a successful terminal presentation
closes its tool surface, the caller cancels, the host deadline expires, or a safety
limit fails. The host still enforces at most 64 business-tool calls, bounded arguments,
bounded tool results, report and presentation limits, credential isolation, and
Change Service-controlled approval.

Analysis delegates and post-turn memory extraction remain independently bounded. They
are narrow internal jobs with one typed result contract, not continuations of the main
operator loop.

Codex does not expose a portable model-round counter. For bounded internal passes its
adapter conservatively counts business-tool dispatches against the allowance; zero still
means an unbounded main loop. Closing tools denies subsequent business callbacks while
allowing a final answer. Native helper tools do not gain backend or change authority.

The follow-through reminder is private, best-effort policy, not a language parser or
authorization signal. It recognizes only narrow leading English imperatives (optionally
prefixed by "please"). Other wording and languages still reach the same model and staging
tools, without a forced reminder. This favors a missed reminder over manufacturing a
mutation request from a question or diagnostic recommendation. No additional classifier,
workflow engine, or model call is introduced to determine intent.

Delegate failures distinguish deadline, cancellation, runtime failure, execution allowance,
interruption, and missing validated submission. A parent reply may complete while a delegate
fails; its persisted tool activity must retain the unsuccessful outcome. Provider error
messages and private reasoning are not diagnostic metadata.

## Deliberate boundaries

- Model transport is separate from runtime. Built-in Runtime, Pi and Codex support ChatGPT OAuth and direct
  HTTP configuration (Codex: Responses only); Claude uses explicit Anthropic API
  credentials. Luna through ChatGPT OAuth is the default, not a public product contract.
- Agent Application owns account authority, evidence and events. Change Service owns
  approval, execution and audit. Web Workspace never owns a write decision.
- Analysis uses typed datasets and server calculations rather than arbitrary code
  execution. Current tasks do not justify a general code-execution sandbox.
- TikTok workflows become runtime-loadable only when typed tools, backend behavior, and
  wire tests exist. Documentation alone is not a runnable capability.
- Manager mode is a scope of the same host and runtime. Cross-account intent becomes
  independent account-bound drafts and approvals.
- Prompt caching is not generalized into the Go runtime contract. Cache controls remain
  private to each provider adapter until a portable semantic contract is demonstrated.

## Remaining validation work

1. Add golden conversations as each staged workflow becomes executable.
2. Verify TikTok authorization, field semantics, attribution, and reconciliation after
   developer-app approval.
3. Validate the TikTok Writer against one controlled object before enabling it outside
   HTTP-fake tests.
