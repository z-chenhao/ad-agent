# Local development and diagnosis

## Start

```sh
npm ci --ignore-scripts
make build
./bin/ad-agent serve --addr 127.0.0.1:8090 --data-dir .data/local-alpha
```

Open the printed local URL. Sign in using the private `operator-key` file in the selected
state directory. Do not put the key in a URL or share it. Browsing Sandbox data and using
operator controls require no model login. Chat requires the chosen provider connection.
Choose a free loopback port; never stop an unrelated service to free the default port.

For Manager, use `--scope manager --sandbox-environment manager-lab` and a separate state
directory/listener. Built-in/Pi/Codex OAuth resolves the operator's existing Pi login and refreshes
it when needed; it does not change global defaults. Model connections can use an explicitly named environment
variable supplied when starting the process, or an operator-entered key kept only in server
memory. Restarting the server clears entered keys; it does not erase saved model settings.

## Workspace settings

Ad Desk keeps navigation labels separate from a single content-page heading. The top
toolbar owns account identity (or the Manager account selector), currency, and timezone;
the report bar owns dates and period controls. Content headers do not repeat account
identity. The assistant retains its own explicit context for the next message.

Settings separates five concerns:

- **Model**: existing Pi ChatGPT OAuth; OpenRouter PKCE authorization; or an explicit
  HTTP provider, protocol, base URL, model ID, and API key. HTTP presets do not discover
  models or infer a protocol from a URL. Token limits are operator declarations.
- **Runtime**: Built-in Runtime, Pi SDK, Codex App Server, or Claude Agent SDK when the bridge is built. Codex
  requires ChatGPT OAuth or Responses HTTP. Claude
  requires Anthropic Messages/API-key transport. Saving retains the conversation when
  the ad source is unchanged. The next turn rebuilds saved public context if execution
  changed; private checkpoints never cross adapters/model connections. History, cards,
  drafts and the unsent composer text remain. Changing the ad source starts a new session.
- **Skills**: upload a Markdown SKILL.md with `name` and `description` frontmatter,
  declare required installed tools, and enable/disable operator guidance. Maximum 32 KB
  per document and 24 custom skills. Uploads cannot replace built-ins, install code,
  create tools, or grant write authority. Never include credentials in skill text.
  Adding a skill prepares a draft; **Save settings** installs it together with other
  edits, so uploading does not discard unsaved model or guardrail configuration.
- **Ad connection**: switch between isolated local Sandbox environments and an already
  authorized TikTok advertiser. New TikTok connections are read-only; developer approval
  and advertiser authorization remain prerequisites. Meta is unavailable, not a fallback.
- **Guardrails**: account-currency minimum/maximum budget and maximum percentage change.
  These apply to new drafts, already staged drafts at execution, and scheduled Sandbox
  rules. Approval, account binding, reconciliation, and read-back cannot be disabled.

Settings updates wait briefly for ordinary reads, then reject if the workspace is still
busy. Credentials are not stored in SQLite, localStorage, sessionStorage, or diagnostic
logs. OpenRouter returns to `/model-auth/callback` on the authenticated local workspace;
the single-use code is exchanged server-side using S256 PKCE. This is independent of the
isolated TikTok advertiser callback. Reauthorize OpenRouter after a server restart.

Manager supports the same model/runtime selection and manager-scoped skill imports.
Its account bindings and per-advertiser guardrails stay account-scoped rather than becoming
one global setting. In v1 alpha, workspace configuration belongs to the Web server;
CLI invocations still select their runtime/model through explicit flags. Persisted budget
limits are re-read by the Change Service and Sandbox rules across both entrypoints.

## Logs and trace correlation

| File under the selected state directory | Contents                                                                                             |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `logs/server.jsonl`                     | Request ID, HTTP method/route template, status, duration, byte count                                 |
| `logs/agent-trace.jsonl`                | Request/turn IDs, event sequence, tool duration, allowlisted failure code, terminal outcome/duration |
| `state.db`                              | Authenticated application events, sessions, drafts, evidence and audit                               |
| `runtime/`                              | Private SDK checkpoints/transcripts; not ordinary diagnostic attachments                             |

Each JSONL stream rotates at 10 MiB and keeps one previous file (`.1`). Directories/files
remain 0700/0600. Logs omit request bodies, query values, credentials, business payloads
and private provider reasoning. The response header `X-Request-ID` joins HTTP to Agent
events. CLI events have a turn ID without an HTTP request ID.

```sh
tail -f .data/local-alpha/logs/server.jsonl .data/local-alpha/logs/agent-trace.jsonl
rg 'request_ID_OR_turn_ID' .data/local-alpha/logs/*.jsonl
./bin/ad-agent chat --data-dir .data/cli-lab --session diagnosis --events \
  --message 'Read the account and list its campaigns. Do not stage changes.'
```

For a UI problem, record the local time, page, action, request ID and turn ID. Check the
request status/duration, then tool start/finish pairs and the terminal event. Inspect
authenticated stored events for detailed public errors; do not copy raw SDK transcripts
into public issues. Model unavailability, application errors and MAPI failures are separate.

HTTP 200 on an Agent event stream only means the stream was accepted. Inspect the terminal
outcome and individual `tool.finished` events; a completed reply can contain an unsuccessful
analysis delegate or an expected policy refusal. Tool `duration_ms` measures execution time,
including delegated model work, not the parent's preceding model generation. Known failures
have an allowlisted `error_code`; other errors become `tool_failed` in the metadata log.
Historical records cannot acquire missing diagnostic detail retroactively.

New `turn.started` traces identify the selected runtime. Failed `turn.completed` events
and metadata traces include a fixed, allowlisted `error_code`: authentication, provider
rate/transport/history rejection, timeout, cancellation, or a generic runtime failure.
Raw provider error messages remain excluded. Successful tool calls do not imply the
following model request succeeded. A failed model exchange does not trigger a replay of
the entire turn or its business tools. Older generic failures cannot be assigned a more
precise cause retroactively.

Public assistant text retains its message identity through Runtime, events and Web replay.
Completion-only text is emitted before the associated tools, with already-streamed text
deduplicated. Private reasoning is not public commentary; a tool-selecting response with
only reasoning and tool calls has no public speech to render.

The Web workspace returns to login when an authenticated request receives 401 and stops
issuing protected follow-up requests until sign-in. It never automatically retries writes
or Agent turns. Saved conversations and drafts remain in the state directory; review them
after reconnecting. Initial `/auth` 401 before login is expected, not a service failure.

## Sandbox controls

Time advancement is an authenticated operator action, not an agent skill/tool. Rules use
the virtual clock. The header's Sandbox clock is shared by Today, Campaigns, and Creatives;
advancing within the same date refreshes reports without resetting the selected object.
Dates and clock displays use the advertiser timezone. A partial current day can still show
reported-to-date ROAS when spend and purchase value are present. Missing value or zero spend
remains unavailable; a partial report does not establish a complete diagnostic comparison.
New sources without linked delivery have no telemetry; creating a
resource is not evidence of measurement traffic. Developer-only hidden causal traces:

```sh
./bin/ad-agent simulation-trace --data-dir .data/simulation-lab \
  --sandbox-environment simulation-lab --simulation-hours 24
```

This command advances its isolated environment. Do not use it against a user's testing
environment when only reading status is intended. See [simulator](sandbox-simulator.md).

## TikTok callback

App approval and advertiser authorization remain external gates. Use the exact callback
URL registered in the TikTok app. Localhost is preferred only if accepted for that app;
a tunnel can target the isolated callback listener. It must not expose this application's
operator API or state. Do not change a working registered callback merely to match an example.
Keep live writes disabled until controlled read and bounded write/read-back acceptance.
