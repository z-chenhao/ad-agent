# Development readiness and acceptance record

Updated 2026-09-04. No architecture confirmation, Pi login, or verification code is
currently required from the user. The TikTok developer app is pending approval.

## Fixed decisions

- Single-user local application with a Go host and React UI.
- Independent AdBackend and runtime replacement axes.
- Pi 0.84.4 full-agent sidecar, J-agent at commit `6ddcaee` with a model-only bridge,
  and Claude Agent SDK 0.3.260 as a peer runtime.
- ChatGPT OAuth or explicit direct HTTP model configuration, with Luna through OAuth as
  the default and provider/model/protocol/auth pinned to each session.
- Read analysis, model-authored drafts, and operator approval are separate.
- Local Sandbox AdBackend data is explicitly fictional and never used as a TikTok fallback.
- Active versus staged workflow skills are generated from one validated manifest.
- One Ad Agent supports `single_advertiser` and `portfolio` host scopes; this is not a
  separate AgencyAgent product or runtime.

## Current evidence

| Area            | Result                                              | What it proves                                                            |
| --------------- | --------------------------------------------------- | ------------------------------------------------------------------------- |
| Build           | `make test` passed at v0.12                         | Go, Pi/J/Claude bridges, React bundle, and skill validation               |
| Go tests        | `go test -race ./...` passed at v0.12               | Domain, harness, lifecycle sandbox, analysis, memory, HTTP, OAuth, adapters |
| Runtime tests   | Pi, J, and Claude bridge protocol tests passed      | Framing, selection, checkpoint, continuation, and tool isolation invariants |
| Pi CLI          | live Luna read, analysis, and draft passed          | Pi -> Luna -> Go tool continuation on local sandbox                       |
| J CLI           | live Luna read, restore, analysis, and draft passed | J owns the loop; bridge only supplies the model                           |
| Pi Web          | six Playwright tests passed, including live Luna    | SSE, draft, approval, read-back, reload, desktop, and mobile              |
| J Web           | the same six Playwright tests passed with live Luna | The complete Web workflow is runtime-neutral rather than Pi-only          |
| Claude runtime  | local bridge and fake-process tests passed          | SDK composition and Go protocol; no live Anthropic request was made       |
| Direct providers | validation and bridge tests passed                 | explicit provider registration and secret boundary; no live vendor request |
| TikTok adapter  | read/write HTTP-fake tests passed                   | reads plus budget/status wire shapes and write outcome classification     |
| OAuth callback  | local security and smoke tests passed               | state, replay defense, exchange, local credential file                    |
| TikTok platform | not run                                             | app approval, scopes, advertiser binding, and real data remain unverified |

The latest successful probes do not guarantee provider availability. A prior transport
failure terminated without a tool call or draft and later passed unchanged. External
provider failure remains an explicit state.

## v0.8 harness and workspace alignment

Implemented in source on top of the v0.7 workflow expansion:

- forced intent-specific grounding before the model loop;
- same-turn staging follow-through for actionable operator requests;
- isolated read-only analysis with server-issued datasets and calculations;
- concurrent independent reads with host-serialized mutations;
- pending presentation records, server enrichment, and terminal suggestions;
- isolated automatic extraction of filtered durable operator facts;
- a focused shadcn/ui and Tailwind workspace with overview, hierarchy, change review,
  assistant, activity, and memory surfaces;
- an explicit Commercial Agents alignment matrix and editable Excalidraw architecture.

Local v0.8 acceptance evidence:

- all 18 skill packages passed the skill-creator structural validator;
- skill registry tests confirm nine active entries, staged exclusion, generated enum,
  and required-tool availability;
- the repository English check reports no Chinese text in authored files;
- Go race tests, Node tests, and the React production build passed;
- Pi and J local-sandbox Web suites each passed five ordinary tests, including partial-card
  replacement, authentication and CSRF, hierarchy consistency, activity and memory
  inspection, and mobile containment;
- Pi and J each passed the opt-in real Luna browser test: one exact local-sandbox budget draft,
  no pre-approval mutation, authenticated confirmation, read-back verification, and
  post-reload persistence;
- forced grounding, presentation deduplication, staging follow-through, digest
  enrichment, terminal close, isolated memory extraction, and J concurrent-call tests
  passed under the race detector;
- active skills were reviewed against installed tool names.
- a real Pi/Luna local-sandbox turn completed in 41.2 seconds, loaded
  `daily-account-briefing`, executed account, campaign, report, and pending-change reads,
  presented trusted metrics and suggestions, and created no draft.

## v0.9 backend and model selection evidence

- AdBackend now composes Reader and Writer; the host receives Reader and the independent
  approval service receives Writer.
- The TikTok MAPI Writer has HTTP-fake coverage for campaign/ad-group budget updates and
  campaign/ad-group/ad status updates, including acknowledged, rejected, unknown, and
  not-sent outcomes. No live TikTok write was attempted.
- Runtime model selection is validated against seven installed `openai-codex` models,
  stored on the public session, propagated to main, analysis, and memory calls, and
  rejected if a later turn tries to mix a different model checkpoint into that session.
- Pi and J each completed a real `gpt-5.4-mini` local-sandbox read through ChatGPT OAuth. The
  first J attempt ended as `provider_failed` before any tool call; an unchanged retry
  completed. This is evidence of fail-closed handling and eventual availability, not a
  guarantee of provider uptime.

## v0.11 runtime, sandbox, and skill acceptance

The Sandbox AdBackend is now a persistent advertising environment rather than a menu of
scripted cases. Each environment starts from the canonical fictional seed and stores its
own created/updated entities under an environment key. Permission, timeout, partial,
rejected, and unknown behavior remains covered by test fakes instead of changing normal
product identity.

Deterministic acceptance tests verify:

- campaign -> ad-group -> ad creation, hierarchy validation, list/get, persistence
  across composition, and isolation between environment IDs;
- creation is staged by the agent and materializes only after separate host approval
  and matching read-back;
- ordinary budget/status writes retain approval and read-back semantics;
- creator-only tools and `sandbox-lifecycle` are absent from non-creator backends;
- Pi/J direct model configuration validates provider, model, protocol, HTTPS endpoint,
  environment-variable name, and token limits without persisting the key value;
- Claude Agent SDK is composed at the same Runtime layer with built-in coding tools,
  settings, plugins, and automatic skills disabled.

The nine backend-neutral active skills and creator-gated sandbox lifecycle skill now
contain business evidence prerequisites, diagnostic trees, metric definitions, output
contracts, and explicit missing-data behavior. All nine staged TikTok workflows contain
realistic dependency and activation gates. External repositories informed structure;
TikTok platform claims remain grounded in official sources.

These tests do not validate a direct vendor credential, a live Claude turn, TikTok
scopes, payload acceptance, platform timing, or Ads Manager reconciliation. No live
model or TikTok call was made during the v0.11 validation run.

## v0.12 portfolio acceptance

Portfolio scope is implemented for the local sandbox. It composes three fictional,
independently persisted advertiser environments behind a host-authorized account router.
Go, HTTP, and React expose account listing, account-level performance, drill-down tools,
independent account-scoped budget/status drafts, and the shared approval surface.

Deterministic tests verify that out-of-scope advertiser IDs are rejected, the model has
no apply tool, two-account batch intent creates separate unapplied changes, approval
routes each change to its original account backend, and a write persists only in the
selected advertiser namespace across restart. Portfolio totals deliberately do not
combine currencies. Live TikTok portfolio bindings remain unimplemented and unclaimed.

## Callback and tunnel

The registered HTTPS ngrok root callback remains a valid fallback. Port 3000 runs only
the callback handler and never the management app or a file server. Localhost is the
preferred single-user route where the TikTok portal accepts it. Do not resubmit an app
solely to switch redirect URL while approval is pending.

The authorization URL must be copied from TikTok My Apps. `oauth-start` preserves its
portal parameters and replaces only the one-time state. The callback stores no raw state
and never logs or displays the authorization code or token. Any email verification code
is entered only on TikTok's page.

## Remaining acceptance gates

1. After app approval, validate the generated advertiser authorization URL, exact scope,
   advertiser binding, hierarchy reads, report fields, timezone, attribution, and Ads
   Manager reconciliation.
2. Activate staged skills incrementally only after their typed backend, tools, local
   sandbox, wire tests, and platform evidence exist.
3. Add golden conversations when each staged workflow becomes executable; staged
   documentation is not a runnable capability.
4. Run the implemented TikTok Writer against one controlled object with explicit caps,
   per-change approval, read-back, and unknown-outcome reconciliation before treating
   it as live-platform validated.
5. Define and verify explicit authorization/configuration for every account before
   enabling live `portfolio` scope; never accept an advertiser ID as proof of access.

## Reproduction

```sh
make cli
make test
make test-sandbox
./bin/ad-agent chat --runtime j --session diagnosis-j --json --message 'Compare the latest seven days with the prior seven days at campaign level. Use the analysis delegate, cite computed evidence, and do not create a draft.'
```

The optional historical Pi probe consumes ChatGPT quota:

```sh
node scripts/check-pi-readiness.mjs "$PWD/node_modules/@earendil-works/pi-coding-agent/dist/index.js"
```

It diagnoses the pinned SDK transport only; it does not prove the product stack.
