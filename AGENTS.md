# Ad Agent engineering instructions

This file is for coding agents working in this repository. `AGENT.md` is the static
operating contract for the advertising agent. Communicate with the repository owner
in their chosen language, but keep all repository artifacts and product UI in English.

Read these documents before making changes:

- `docs/technical-design-v0.md`
- `docs/ad-backend-contract.md`
- `docs/development-readiness.md`
- `docs/tiktok-workflow-coverage.md` for workflow or TikTok capability changes

## Boundaries

- Go owns the domain, AdBackend, executor, snapshots, approvals, audit, HTTP, and SSE.
- React/TypeScript owns the UI. The Pi sidecar owns Pi sessions; J-agent owns a second
  model/tool loop in Go; J's TypeScript bridge only provides provider/OAuth transport.
- TikTok MAPI and fixture separately implement AdBackend. Runtime and backend remain
  independent replacement axes.
- The host change service is the only writer. The model has no apply tool; chat is not
  approval.
- The host binds identity and environment. Fixture never masks a live error or claims
  to be live data.
- Both runtimes explicitly select `openai-codex/gpt-5.6-luna`. Never silently change
  the model or the user's global defaults.
- Explicitly initialize Pi networking. Disable default coding tools and automatic Pi
  contexts, extensions, skills, and prompts. J exposes only advertising tools supplied
  by the Go host.
- Credentials never enter model business context, SSE, logs, or Git. Provider
  transcripts remain private.
- Port 3000 serves only the isolated OAuth callback. Localhost is preferred; ngrok is
  a fallback. The actual redirect URL comes from local configuration.
- Always distinguish fixture, HTTP fake, sandbox, and live evidence, and distinguish
  design, implementation, test, and platform validation.

## Change and validation rules

- Inspect files and Git state first. Preserve existing work. Do not commit, push, or
  publish without an explicit request.
- Change only the current scope. Do not prebuild a universal advertising platform or
  plugin framework.
- Active skills must name only installed tools. Put official but unsupported workflows
  under `skills/_staged`; do not expose them through `load_skill`.
- J-agent must own a real model/tool loop; a wrapper around the full Pi loop is not a J
  integration.
- Do not create or enable live ads, change live budgets, or alter permissions without
  explicit scope and product approval.
- Cover success, rejection, missing or partial data, cancellation, and timeout. Never
  blindly retry an unknown write outcome.
- Local Pi/J CLI and Web loops plus MAPI wire tests are implemented. Consult the
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
