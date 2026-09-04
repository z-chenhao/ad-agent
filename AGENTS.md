# Ad Agent engineering instructions

This file is for coding agents working in this repository.
`prompts/ad-agent-system.md` is the product runtime system contract. Communicate with
the repository owner in their chosen language, but keep all repository artifacts and
product UI in English.

Read these documents before making changes:

- `docs/technical-design-v0.md`
- `docs/ad-backend-contract.md`
- `docs/development-readiness.md`
- `docs/tiktok-workflow-coverage.md` for workflow or TikTok capability changes

## Boundaries

- Go owns the domain, AdBackend, executor, snapshots, approvals, audit, HTTP, and SSE.
- React/TypeScript owns the UI. Pi owns Pi sessions; J-agent owns a second model/tool
  loop in Go and uses TypeScript only for provider transport; Claude Agent SDK is a peer
  runtime adapter with only host-supplied advertising tools.
- TikTok MAPI and the local sandbox separately implement AdBackend. Runtime and backend
  remain independent replacement axes.
- `single_advertiser` and `portfolio` are host-bound scopes of the same Ad Agent, not
  separate personas or runtimes. Portfolio routes to independent account-bound
  AdBackends and never turns an advertiser ID into authentication authority.
- The host change service is the only writer. The model has no apply tool; chat is not
  approval.
- The host binds identity and environment. Sandbox never masks a live error or claims
  to be live data.
- Runtime and model connection are independent axes. Pi and J accept ChatGPT OAuth or an
  explicit direct HTTP provider using one declared protocol; Claude Agent SDK accepts
  Anthropic API-key transport only. The default is `openai-codex/gpt-5.6-luna` through
  ChatGPT OAuth. Never infer a protocol from a URL, persist a key value, silently change
  a session model, or alter the user's global defaults.
- Explicitly initialize Pi networking. Disable default coding tools and automatic Pi
  contexts, extensions, skills, and prompts. J and Claude expose only advertising tools
  supplied by the Go host.
- Credentials never enter model business context, SSE, logs, or Git. Provider
  transcripts remain private.
- Port 3000 serves only the isolated OAuth callback. Localhost is preferred; ngrok is
  a fallback. The actual redirect URL comes from local configuration.
- Always distinguish local-sandbox, HTTP-fake, TikTok-platform-sandbox, and controlled-live
  evidence, and distinguish design, implementation, test, and platform validation.

## Change and validation rules

- Inspect files and Git state first. Preserve existing work. Do not commit, push, or
  publish without an explicit request.
- Change only the current scope. Do not prebuild a universal advertising platform or
  plugin framework.
- Active skills must name only installed tools. Put official but unsupported workflows
  under `skills/_staged`; do not expose them through `load_skill`.
- J-agent must own a real model/tool loop; a wrapper around the full Pi loop is not a J
  integration.
- TikTok MAPI and the local sandbox implement the Reader/Writer AdBackend. The local
  sandbox additionally implements the experimental Creator lifecycle slice. The agent
  host receives reads and staging tools; only the approval service receives Writer or
  Creator execution authority.
- Sandbox environments are persistent, isolated advertising state, not named test
  scenarios. Put permission, timeout, partial, rejected, and unknown outcomes in tests or
  explicit fault injectors, never in ordinary environment identity.
- Portfolio reports preserve account currency, timezone, attribution, freshness, and
  limitations. Never synthesize a cross-currency total. Batch requests become
  independent advertiser-scoped drafts with independent approval and reconciliation.
- Do not create or enable live ads, change live budgets, or alter permissions without
  explicit write enablement, bounded policy, one-change product approval, and read-back.
- Cover success, rejection, missing or partial data, cancellation, and timeout. Never
  blindly retry an unknown write outcome.
- Local Pi/J CLI and Web loops, local Claude bridge tests, and MAPI wire tests are
  implemented. Consult the
  readiness record before making any live-data claim.

Current baseline:

```sh
make cli
make test
./bin/ad-agent inspect
```

Live-model probes in `docs/development-readiness.md` consume the user's ChatGPT quota.
A probe does not prove that the Go bridge, analysis delegate, React UI, or MAPI passed
its own acceptance gate.
