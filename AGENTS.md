# Ad Agent engineering instructions

This file is for coding agents working in this repository.
`prompts/ad-agent-system.md` is the runtime kernel, not an engineering instruction file.
The Agent Application compiles it with one scope policy, tool-derived capabilities, and
a scope-filtered skill index. Communicate with
the repository owner in their chosen language, but keep all repository artifacts and
product UI in English.

The Web product is **Ad Desk**; its embedded assistant is **Ad Agent**. Keep the
repository, CLI, package names, and persistence namespaces as `ad-agent`. This is a
UI branding distinction, not a new project, runtime, or version.

Read these documents before making changes:

- `docs/architecture.md`
- `docs/ad-backend-contract.md`
- `docs/validation.md`
- `docs/capabilities.md` for workflow or TikTok capability changes

## Orientation and ownership

Start with `git status --short --branch` and inspect the relevant callers and tests.
Existing changes and local state belong to the operator. Never reset them to make a test pass.

| Area                                | Source of truth                                        |
| ----------------------------------- | ------------------------------------------------------ |
| Startup and composition             | `cmd/ad-agent`, `internal/app`                         |
| Business contracts                  | `internal/ads`, `docs/ad-backend-contract.md`          |
| Tools and change authority          | `internal/agenthost`, `internal/manager`               |
| Runtime protocol/adapters           | `internal/runtime`, `runtime/`                         |
| Prompt composition                  | `internal/prompting`, `prompts/`                       |
| Skill discovery and requirements    | `skills/manifest.json`, `internal/agenthost/skills.go` |
| Public HTTP/events                  | `internal/httpapi`, `web/src/types.ts`                 |
| Persistence and private diagnostics | `internal/store`                                       |
| Provider and simulation semantics   | `internal/tiktokmapi`, `internal/sandbox`              |

Put a rule in its owning layer: executable safety in the Change Service; argument rules
in tool schemas/descriptions; cross-task judgment in the kernel; domain decisions in skills;
volatile account evidence in per-turn context. Do not duplicate a rule across all four.

## Boundaries

- Agent Application owns account binding, business tools, sessions, snapshots and events.
  Change Service owns policy, approval, write execution and read-back. These services and
  the API Server are implemented in Go; use component names, not languages, for architecture roles.
- React/TypeScript owns the UI. Built-in Runtime, Pi SDK, Codex App Server and Claude Agent SDK are peer
  runtime adapters. Each owns its native model/tool loop; application services own
  advertising tools, account authority and public events.
- Built-in Runtime is the project's lightweight execution loop, with a model-only
  TypeScript transport. Use `builtin` for file/package/config names and `Builtin` for
  the Go adapter. Preserve the actual upstream loop dependency and license notice;
  do not expose its project name as the product Runtime name.
- TikTok MAPI and the local sandbox separately implement AdBackend. Runtime and backend
  remain independent replacement axes.
- `advertiser` and `manager` are application-bound scopes of the same Ad Agent, not
  separate personas or runtimes. Manager routes to independent account-bound
  AdBackends and never turns an advertiser ID into authentication authority.
- The Change Service is the only writer. The model has no apply tool; chat is not
  approval.
- The Agent Application binds identity and environment. Sandbox never masks a live error or claims
  to be live data.
- Runtime and model connection are independent axes. Built-in Runtime and Pi accept ChatGPT OAuth or an
  explicit HTTP provider; Codex accepts ChatGPT OAuth or OpenAI Responses. Claude accepts
  Anthropic API-key transport only. The default is `openai-codex/gpt-5.6-luna` through
  ChatGPT OAuth. Never infer a protocol from a URL, persist a key value, silently change
  a session model, or alter the user's global defaults.
- Explicitly initialize Pi networking. Disable default coding tools and automatic Pi
  contexts, extensions, skills, and prompts. Claude exposes only application tools.
  Codex uses a version-pinned private stdio process, private native state, no workspace
  environments, no native shell/files/network tools or MCP, and disabled ambient skills.
  Its native plan/empty-skill utilities and model-selected restricted Code Mode are not
  advertising authority. See `docs/codex-runtime.md` for the tested boundary.
- Credentials never enter model business context, SSE, logs, or Git. Provider
  transcripts remain private.
- Conversation identity belongs to the application, not a Runtime. Explicit runtime/model
  changes retain source-bound history and drafts, but never reuse an incompatible or
  stale native checkpoint. Rebuild from bounded public records; old tool outcomes and
  dataset handles grant no current evidence or approval. A new ad source requires a
  separate conversation. Preserve failed/cancelled outcomes during context restoration.
- Port 3000 serves only the isolated OAuth callback. Localhost is preferred; ngrok is
  a fallback. The actual redirect URL comes from local configuration.
- Always distinguish local-sandbox, HTTP-fake, TikTok-platform-sandbox, and controlled-live
  evidence, and distinguish design, implementation, test, and platform validation.

## Change and validation rules

- Inspect files and Git state first. Preserve existing work. Do not commit, push, or
  publish without an explicit request.
- Change only the current scope. Do not prebuild a universal advertising platform or
  plugin framework.
- Active skills must name only installed tools. Keep manifest and frontmatter metadata
  synchronized and gate discovery by scope/capabilities. Put unsupported workflows under
  `skills/_staged`; do not expose them through `load_skill`. Favor decision guidance over
  mandatory step sequences; a simple request need not load a skill or analysis delegate.
- Keep native Runtime protocols private. Do not forward App Server methods to the Web
  client or equate native SDK approval with business approval. New native versions need
  tool-catalog, context isolation, cancellation and checkpoint regression tests.
- TikTok MAPI and the local sandbox implement the Reader/Writer AdBackend. The local
  sandbox additionally implements the experimental Creator lifecycle slice. The Agent Application
  receives reads and staging tools; only the approval service receives Writer or
  Creator execution authority.
- Sandbox environments are persistent, isolated advertising state, not named test
  scenarios. Put permission, timeout, partial, rejected, and unknown outcomes in tests or
  explicit fault injectors, never in ordinary environment identity.
- Manager reports preserve account currency, timezone, attribution, freshness, and
  limitations. Never synthesize a cross-currency total. Batch requests become
  independent advertiser-scoped drafts with independent approval and reconciliation.
- Do not create or enable live ads, change live budgets, or alter permissions without
  explicit write enablement, bounded policy, one-change product approval, and read-back.
- Cover success, rejection, missing or partial data, cancellation, and timeout. Never
  blindly retry an unknown write outcome.
- Every new mutation needs policy at stage and apply, exact approval fields, account-bound
  read-back of submitted values, partial/unknown handling, and regression tests. An ID or
  successful HTTP status is not verified completion.
- New Sandbox resources need explicit modeled behavior or an honest partial/unsupported
  result. Derive metrics from causal events, preserve budget limits and query-window reach,
  keep seeded replay deterministic, and keep hidden simulator truth out of business tools.
- Use component names in architecture diagrams. Keep editable Excalidraw and rendered PNG
  synchronized; inspect the rendering for overlap and incorrect authority arrows.
- Refresh or remake the promotional video only for a major product release, not for
  routine feature, patch, UI or documentation releases. Keep dated footage and its
  recording boundary documented instead of continuously re-recording. If a safety,
  privacy, licensing or materially misleading claim requires correction, flag it to
  the owner and prefer a narrow correction or withdrawal over a full remake.

## Verification and handoff

Choose checks by the changed boundary, then run the release gate before a release commit:

| Change              | Required focused evidence                                                           |
| ------------------- | ----------------------------------------------------------------------------------- |
| Policy or operation | Stage/apply rejection, exact preview, read-back mismatch, unknown outcome           |
| Sandbox             | Causal invariants, restart, isolation, time advancement and concurrent writer tests |
| MAPI                | HTTP-fake request/response, account isolation and partial/error cases               |
| Runtime or prompts  | Bridge contracts, compiled capabilities, tool/result pairing and scope tests        |
| Web                 | Type check/build plus affected Advertiser/Manager browser flows                     |
| Logging             | Request-to-turn correlation, streaming behavior and secret/body exclusion           |

For Web changes, follow `docs/web-design.md`: shared typography roles, one owner for
each action, concise context, and no nested context scroller. Preserve visible approval
and failure information when reducing text; do not hide required review fields for density.

See `docs/validation.md` for the complete release gate and `docs/local-development.md`
for private server and Agent trace logs. Do not attach raw runtime transcripts to issues.
Real-model tests are opt-in, consume quota, and must use isolated local Sandbox state;
obtain authorization before adding costly probes. Do not silently replace the selected model.

Current baseline:

```sh
make cli
make test
./bin/ad-agent inspect
```

Live-model probes in `docs/validation.md` consume the user's ChatGPT quota.
A probe does not prove that the Go bridge, analysis delegate, React UI, or MAPI passed
its own acceptance gate.

Keep repository-owned release/schema identifiers at the current initial alpha baseline;
vendor protocols, dependency versions, and optimistic resource revisions are not release
numbers. Do not rewrite user databases merely to normalize a source version. End with
changed behavior, checks actually run, known limitations, and the local test URL if started.
