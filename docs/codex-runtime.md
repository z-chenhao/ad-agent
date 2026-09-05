# Codex Runtime

## Decision

Codex App Server is a peer Runtime implementation, not a replacement for the API Server,
Agent Application or Change Service. Current consumers are Advertiser and Manager turns
and their bounded analysis/memory jobs. The downstream contract remains
`Runtime.Run(ctx, Request, Hooks)`: compiled instructions and account-bound tools enter;
public assistant text, correlated tool calls and a settled checkpoint leave.

The adapter pins `@openai/codex` to `0.144.5`. Its TypeScript bridge launches the packaged
native binary over private JSON-RPC stdio. There is no new listener, browser RPC proxy,
plugin platform or public native-session API. The protocol is experimental and private.

## Ownership

| Mechanism                                                         | Owner                                         |
| ----------------------------------------------------------------- | --------------------------------------------- |
| Native model loop, compaction and thread history                  | Codex App Server                              |
| Connection choice, account binding, prompt assembly, tool schemas | Agent Application                             |
| Advertising reads and staging                                     | Application callbacks through `Hooks.Execute` |
| Approval, execution and verified read-back                        | Change Service only                           |
| Ordered public activity and final answer                          | Application events; normalized native text    |
| Private native transcripts and checkpoint references              | Runtime-owned files under the state directory |

The native sequence is `initialize` → `thread/start` or `thread/fork` → `turn/start`.
Experimental `item/tool/call` requests return through application hooks. Native approvals
are rejected: they are neither operator approval nor an advertising write grant.
No native event bypasses application validation or becomes a public SSE payload verbatim.

## Isolation and lifecycle

- Native home and working directory are private, separate from global Codex settings.
- Threads and turns have no workspace environments. Shell, file/patch, browser, web
  search, MCP, apps, plugins, hooks, memories and native multi-agent capabilities are off.
- Automatic repository instructions and bundled/global skill contents are disabled.
  Advertising skills remain available only through application-owned `load_skill`.
- The pinned Luna catalog selects restricted Code Mode (`exec`/`wait`) despite the
  disabled feature default. Its V8 wrapper has no filesystem, network or Node APIs.
  The tested nested catalogue contains supplied advertising tools plus native
  `update_plan` and empty `skills.list/read` utilities. This is **not** a claim that native
  tools vanish or that Pi and Codex execute identical loops.
- Business dispatch is serialized, including parallel native requests. Successful
  terminal presentation closes queued and subsequent business callbacks. The native
  engine may then finish its answer. The main loop has no fixed round ceiling; the
  application's deadline, cancellation, output-size and 64-business-call limits remain.
- Codex exposes no portable round counter. Bounded internal jobs conservatively consume
  their allowance per business-tool dispatch. This deliberate adapter policy can end an
  internal job sooner than Pi; failures remain visible, never fabricated as success.
- Cancellation terminates the bridge and native child process tree. Interrupted or failed
  turns do not produce an authoritative checkpoint and are not retried automatically.
- A checkpoint binds the compiled system, tools and model. The next settled turn forks
  that native thread into a new private reference, preserving the previous checkpoint.
  Checkpoints cannot cross Runtime identities. User histories are not rewritten.
- Public commentary/final text is streamed; private reasoning, auth events, provider
  transcripts and raw native errors are never forwarded to product logs or events.

## Connections and limitations

ChatGPT OAuth reuses Pi's credential resolver/refresh only. Codex owns the model loop.
The resolver initializes its credential snapshot before `isUsingOAuth` is checked;
disabling that initialization falsely rejects a valid saved login.
The existing account is pinned during token refresh, and native auth persistence is
ephemeral. No new login is silently started and no global model defaults are changed.

Direct HTTP requires explicitly selected OpenAI Responses, URL, model and credential
reference. No endpoint guessing or protocol fallback is performed. Pi remains the route
for OpenAI-compatible Chat Completions, including the configured DeepSeek preset.
Claude Agent SDK remains an Anthropic-only peer. Model availability depends on the account
and endpoint, not the presence of an adapter.

The native engine owns output-token policy. The shared max-output declaration is not a
request-token cap for Codex; the UI states this limitation. Native token usage is translated
into per-turn usage, separating cached input. Native plans remain private rather than
becoming another product workflow or approval mechanism.

## Validation and rejected alternatives

Ordinary tests launch the actual pinned native binary against a loopback HTTP fake. They
inspect the outgoing tool catalogue/instructions, run the restricted Code Mode advertising
callback, pair results and fork checkpoints. This is native-runtime/HTTP-fake evidence,
not a live OAuth or model-quality claim. Go protocol tests cover malformed/duplicate frames,
rejected calls, cancellation and the unlimited main-loop setting. See [validation](validation.md).

Replacing the product server with App Server was rejected: it would duplicate or displace
account binding, evidence provenance and approvals. Using a thin SDK wrapper instead of
App Server would not provide the same dynamic-tool and bidirectional event boundary.
Automatically exposing native tools or advertising the private protocol as stable was
also rejected. Replacing the native loop with Pi would defeat the independent Runtime axis.

DeepSeek Harness is not another dependency in this alpha. Its plugin kernel, sessions,
storage and scheduling overlap application-owned mechanisms. Its session-log and context
provenance design is worth studying separately; connecting a DeepSeek model through Pi's
HTTP provider is independent and does not require adopting that harness. A future adapter
needs a concrete consumer and measured benefit, not just another framework option.

Protocol references: [OpenAI App Server](https://learn.chatgpt.com/docs/app-server),
[DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness).
