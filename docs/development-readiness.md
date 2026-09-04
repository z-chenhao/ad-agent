# Development readiness and acceptance record

Updated 2026-09-04. No architecture confirmation, Pi login, or verification code is
currently required from the user. The TikTok developer app is pending approval.

## Fixed decisions

- Single-user local application with a Go host and React UI.
- Independent AdBackend and runtime replacement axes.
- Pi 0.84.4 full-agent sidecar and J-agent at commit `6ddcaee` with a model-only bridge.
- Existing ChatGPT OAuth, Luna default, and explicit session-pinned selection across the
  seven Codex models enumerated by the installed Pi SDK.
- Read analysis, model-authored drafts, and operator approval are separate.
- Fixture data is explicitly fictional and never used as a live fallback.
- Active versus staged workflow skills are generated from one validated manifest.

## Current evidence

| Area            | Result                                              | What it proves                                                            |
| --------------- | --------------------------------------------------- | ------------------------------------------------------------------------- |
| Build           | `make test` passed at v0.8                          | Go, Pi/J bridges, and React production bundle build                       |
| Go tests        | `go test -race ./...` passed at v0.8                | Domain, harness, analysis, memory, HTTP, OAuth, and adapters              |
| Runtime tests   | Pi close/partial and J replay/concurrency passed    | Bridge, continuation, terminal, and independent-read invariants           |
| Pi CLI          | live Luna read, analysis, and draft passed          | Pi -> Luna -> Go tool continuation on fixture                             |
| J CLI           | live Luna read, restore, analysis, and draft passed | J owns the loop; bridge only supplies the model                           |
| Pi Web          | six Playwright tests passed, including live Luna    | SSE, draft, approval, read-back, reload, desktop, and mobile              |
| J Web           | the same six Playwright tests passed with live Luna | The complete Web workflow is runtime-neutral rather than Pi-only          |
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
- Pi and J fixture Web suites each passed five ordinary tests, including partial-card
  replacement, authentication and CSRF, hierarchy consistency, activity and memory
  inspection, and mobile containment;
- Pi and J each passed the opt-in real Luna browser test: one exact fixture budget draft,
  no pre-approval mutation, authenticated confirmation, read-back verification, and
  post-reload persistence;
- forced grounding, presentation deduplication, staging follow-through, digest
  enrichment, terminal close, isolated memory extraction, and J concurrent-call tests
  passed under the race detector;
- active skills were reviewed against installed tool names.
- a real Pi/Luna fixture turn completed in 41.2 seconds, loaded
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
- Pi and J each completed a real `gpt-5.4-mini` fixture read through ChatGPT OAuth. The
  first J attempt ended as `provider_failed` before any tool call; an unchanged retry
  completed. This is evidence of fail-closed handling and eventual availability, not a
  guarantee of provider uptime.

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
2. Activate staged skills incrementally only after their typed backend, tools, fixture,
   wire tests, and platform evidence exist.
3. Add golden conversations when each staged workflow becomes executable; staged
   documentation is not a runnable capability.
4. Run the implemented TikTok Writer against one controlled object with explicit caps,
   per-change approval, read-back, and unknown-outcome reconciliation before treating
   it as live-platform validated.

## Reproduction

```sh
make cli
make test
./bin/ad-agent chat --runtime j --session diagnosis-j --json --message 'Compare the latest seven days with the prior seven days at campaign level. Use the analysis delegate, cite computed evidence, and do not create a draft.'
```

The optional historical Pi probe consumes ChatGPT quota:

```sh
node scripts/check-pi-readiness.mjs "$PWD/node_modules/@earendil-works/pi-coding-agent/dist/index.js"
```

It diagnoses the pinned SDK transport only; it does not prove the product stack.
