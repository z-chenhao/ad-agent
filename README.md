# Ad Agent

Ad Agent is a single-user, local-first TikTok advertising operations assistant. Go owns
advertising data, deterministic calculations, tools, approvals, and audit state. Pi or
J-agent owns the model loop through ChatGPT OAuth. The default is
`openai-codex/gpt-5.6-luna`; the CLI and Web UI can select another validated Codex model
for a new session. React uses shadcn/ui-compatible local components and
Tailwind CSS against the same Go host.

The repository currently provides a complete fixture-based CLI and Web loop plus a
TikTok MAPI AdBackend verified against an HTTP fake for reads and writes. Fixture data is
fictional. Without local TikTok authorization, the app sends no TikTok request. TikTok
writes remain disabled by default and have not passed the live-platform gate.

## Run

Requirements: Go 1.26+, Node 24.14+, and ChatGPT OAuth already configured through Pi.
Commands do not change the global Pi model and do not start login automatically. A
live-model command consumes ChatGPT quota.

```sh
npm ci --ignore-scripts
make cli
./bin/ad-agent inspect
./bin/ad-agent report --level campaign --start 2022-07-11 --end 2022-07-17
./bin/ad-agent chat --message 'Compare the latest seven days with the prior seven days and identify which campaign contributed most to the ROAS change.'
./bin/ad-agent chat --runtime j --session j-lab --message 'Read the account and list its campaigns.'
./bin/ad-agent chat --model gpt-5.4-mini --session mini-lab --message 'Give me a concise account briefing.'
```

Use `--json` for a final structured result, `--events` for public lifecycle NDJSON, and
`--session` to select a session. State is kept under `.data/`, which must remain private
with mode `0700`. Runtime and model are pinned to a session; create a new session to
change either one.
Successful turns use a separate best-effort pass to retain durable business preferences;
use `--auto-memory=false` to disable that local feature.

## Architecture

![Ad Agent system architecture](docs/architecture.png)

The Go host is the invariant safety and domain boundary. Pi and J-agent are private,
replaceable runtime adapters; fixture and TikTok MAPI are independent `AdBackend`
implementations. The editable source is available in
[`docs/architecture.excalidraw`](docs/architecture.excalidraw).

The capability-by-capability comparison with Anthropic Commercial Agents is documented
in [`docs/commercial-agents-alignment.md`](docs/commercial-agents-alignment.md).
Repository automation follows [`AGENTS.md`](AGENTS.md); the product runtime receives the
separate [`prompts/ad-agent-system.md`](prompts/ad-agent-system.md) system contract.

## Web and approvals

```sh
make build
./bin/ad-agent serve --addr 127.0.0.1:8080
```

Open the printed local URL and sign in with the key from the printed `operator-key`
path. Enter that key only in the local app. The main app refuses port 3000 because that
port is reserved for the isolated OAuth callback.

The workspace follows the operating flow instead of placing every control on one page:
Home shows the briefing and decision queue, Campaigns drills through one hierarchy level
at a time, Changes owns approval and reconciliation, and the assistant remains available
as a side rail. Its inspector shows public activity and saved business memory, not model
reasoning.

The model can create budget and status drafts, but it cannot approve or apply them. The
fixture AdBackend supports approval end to end. TikTok implements the same AdBackend,
but its Writer is injected only when the operator starts the host with explicit write
enablement and all three budget guardrails:

```sh
./bin/ad-agent serve --backend tiktok --tiktok-advertiser ADVERTISER_ID \
  --enable-tiktok-writes --tiktok-min-budget 20 --tiktok-max-budget 500 \
  --tiktok-max-budget-delta-percent 10
```

Do not enable that mode until app approval and a controlled write acceptance test. It
supports one approved campaign/ad-group budget update or campaign/ad-group/ad status
update per change; it does not create ads.

```sh
./bin/ad-agent chat --message 'Read campaign_example_1 and draft a change from its current total budget to 55 USD. Do not apply it.'
./bin/ad-agent changes --session local
./bin/ad-agent approve --session local --id CHANGE_ID
```

Approval applies exactly one staged change. A result is shown as applied only after
read-back confirmation. Unknown results are reconciled through reads and are not
blindly retried.

## Workflow coverage

The runtime installs nine working skills for account readiness, daily briefing,
hierarchy audit, monitoring, diagnosis, budgets, delivery, creative performance, and
change governance. Nine additional TikTok workflows are documented under
`skills/_staged` but deliberately not exposed until their typed tools exist. See
[`docs/tiktok-workflow-coverage.md`](docs/tiktok-workflow-coverage.md) for the evidence,
API areas, and implementation gates.

## TikTok authorization after app approval

TikTok's advertiser authorization URL must come from My Apps. Do not construct it.
Start the callback-only listener with the exact registered redirect URL, then prepare
the authorization URL in another terminal:

```sh
export AD_AGENT_TIKTOK_APP_ID='stored only in the local environment'
export AD_AGENT_TIKTOK_APP_SECRET='stored only in the local environment'
./bin/ad-agent oauth-callback --addr 127.0.0.1:3000 --redirect-url 'http://localhost:3000/'
./bin/ad-agent oauth-start --redirect-url 'http://localhost:3000/' --authorization-url 'URL_COPIED_FROM_TIKTOK_MY_APPS'
```

Enter any email verification code only on TikTok's page. Never send a code, App Secret,
authorization code, or token to chat or Git.

## Verify

```sh
make test
```

Ordinary tests need neither model credentials nor TikTok credentials. Live-model,
fixture, HTTP-fake, sandbox, and live-platform evidence are recorded separately in
[`docs/development-readiness.md`](docs/development-readiness.md).
