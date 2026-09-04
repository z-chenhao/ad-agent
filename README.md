# Ad Agent

Ad Agent is a single-user, local-first TikTok advertising operations assistant. The
same product runs in `single_advertiser` scope for a direct advertiser or `portfolio`
scope for an operator managing several authorized advertiser accounts. Portfolio is a
scope, not a separate "AgencyAgent" persona. Go owns
advertising data, deterministic calculations, tools, approvals, and audit state. Pi,
J-agent, or Claude Agent SDK can own the model loop behind the same runtime seam. Model
connection is a separate choice: Pi and J support ChatGPT OAuth or an explicit direct
HTTP provider; Claude uses an Anthropic API key. The default remains
`openai-codex/gpt-5.6-luna` through ChatGPT OAuth. React uses shadcn/ui-compatible local
components and Tailwind CSS against the same Go host.

The repository currently provides a complete local `Sandbox AdBackend` CLI and Web loop
plus a TikTok MAPI AdBackend verified against an HTTP fake for reads and writes. Local
sandbox data is fictional. Without local TikTok authorization, the app sends no TikTok
request. TikTok writes remain disabled by default and have not passed the controlled-live
gate.

## Run

Requirements: Go 1.26+ and Node 24.14+. The default path also requires ChatGPT OAuth
already configured through Pi. Commands do not change the global Pi model and do not
start login automatically. A live-model command consumes provider quota.

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

### Direct advertiser and portfolio scope

The default `--scope single_advertiser` binds one advertiser to the host. The portfolio
sandbox binds three isolated fictional advertisers behind one authorized scope while
reusing the same runtime, model connection, AdBackend contract, change ledger, and
approval service:

```sh
./bin/ad-agent inspect --scope portfolio --sandbox-environment portfolio-lab
./bin/ad-agent report --scope portfolio --sandbox-environment portfolio-lab
./bin/ad-agent chat --scope portfolio --sandbox-environment portfolio-lab \
  --message 'Triage the portfolio, drill into the account needing attention, and stage no change without account-level evidence.'
./bin/ad-agent serve --scope portfolio --sandbox-environment portfolio-lab
```

Portfolio reporting preserves each account's currency, timezone, attribution,
freshness, and limitations; it deliberately produces no cross-currency total. A batch
request becomes independent advertiser-scoped drafts, each requiring separate approval.
Live portfolio composition is not enabled yet because it requires explicit host-bound
TikTok authorization for every advertiser.

### Runtime and model connections

Runtime implementation and provider authentication are independent axes:

| Runtime | ChatGPT OAuth | Direct HTTP API key | Notes |
| --- | --- | --- | --- |
| Pi SDK | Yes | Anthropic Messages, OpenAI Responses, or OpenAI-compatible Chat Completions | Pi owns the full agent session |
| J-agent SDK | Yes | The same three protocols | J owns the Go model/tool loop; its TypeScript bridge is transport only |
| Claude Agent SDK | No | Anthropic Messages | Official SDK subprocess with only host advertising tools |

Direct connections are explicit; the host never guesses a wire protocol from a URL.
Only the environment-variable **name** is stored in session configuration. The key value
stays in process memory and never enters the Web UI, model context, SQLite, logs, or Git.

Example OpenAI-compatible provider through Pi or J:

```sh
export DEEPSEEK_API_KEY='set locally; never commit'
./bin/ad-agent chat --runtime pi --session deepseek-lab \
  --model-auth api_key --provider deepseek --model deepseek-chat \
  --model-api openai-completions --model-base-url https://api.deepseek.com \
  --model-api-key-env DEEPSEEK_API_KEY --model-context-window 128000 \
  --model-max-output-tokens 8192 --message 'Read the account and list its campaigns.'
```

Example Claude Agent SDK runtime:

```sh
export ANTHROPIC_API_KEY='set locally; never commit'
./bin/ad-agent chat --runtime claude --session claude-lab \
  --model-auth api_key --provider anthropic --model YOUR_CLAUDE_MODEL_ID \
  --model-api anthropic-messages --model-base-url https://api.anthropic.com \
  --model-api-key-env ANTHROPIC_API_KEY --model-context-window 200000 \
  --model-max-output-tokens 8192 --message 'Read the account and list its campaigns.'
```

The Web capability panel can choose among models advertised by the running host. Start
the server with the direct flags above to advertise that configured model; API keys are
never entered in the browser.

## Sandbox AdBackend

The Sandbox AdBackend is a persistent fictional advertising environment, not a catalog
of scripted cases. Each `--sandbox-environment` value is an isolated storage namespace.
It starts from the same documented seed, then retains created campaigns, ad groups, ads,
budget/status writes, sessions, approvals, and audit history across process restarts.

```sh
./bin/ad-agent inspect --backend sandbox --sandbox-environment planning --level campaign
./bin/ad-agent chat --backend sandbox --sandbox-environment planning \
  --session planning --message 'Draft a disabled traffic campaign named Autumn launch.'
./bin/ad-agent changes --backend sandbox --sandbox-environment planning --session planning
./bin/ad-agent approve --backend sandbox --sandbox-environment planning \
  --session planning --id CHANGE_ID
```

Creation follows the real hierarchy: create a campaign, read it, stage an ad group under
it, approve, then do the same for an ad. New objects are not visible in another
environment. The current sandbox intentionally does not simulate targeting, scheduling,
bidding, identity, creative assets, review, or delivery for new objects. Failure
injection belongs to tests, not selectable product environments.

## Architecture

![Ad Agent system architecture](docs/architecture.png)

The Go host is the invariant safety and domain boundary. Pi, J-agent, and Claude Agent
SDK are peer, replaceable runtime adapters; model transport varies inside that boundary.
The local sandbox and TikTok MAPI are independent `AdBackend` implementations. The
editable source is available in
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

The direct-advertiser workspace follows the operating flow instead of placing every control on one page:
Home shows the briefing and decision queue, Campaigns drills through one hierarchy level
at a time, Changes owns approval and reconciliation, and the assistant remains available
as a side rail. Its inspector shows public activity and saved business memory, not model
reasoning.
Portfolio mode replaces Home and Campaigns with an advertiser-level triage table and
account drill-down prompts while retaining the same assistant and Changes approval
surface.

The model can create budget/status drafts and, in the sandbox only, object-creation
drafts; it cannot approve or apply them. The Sandbox AdBackend supports approval end to
end. TikTok implements the Reader/Writer AdBackend,
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

The direct scope installs nine backend-neutral operating skills plus a creator-gated
sandbox lifecycle skill. Portfolio scope installs a dedicated `portfolio-operations`
workflow over its account-routing tools. They include evidence requirements, decision trees, metric semantics,
output contracts, and failure boundaries. Nine additional TikTok workflows are documented under
`skills/_staged` but deliberately not exposed until their typed tools exist. See
[`docs/tiktok-workflow-coverage.md`](docs/tiktok-workflow-coverage.md) for the evidence,
API areas, implementation gates, and source review. External-skill research and the
adoption snapshot are recorded in [`docs/skill-research.md`](docs/skill-research.md).

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
make test-sandbox
```

Ordinary tests need neither model credentials nor TikTok credentials. Live-model,
local-sandbox, HTTP-fake, TikTok-platform-sandbox, and controlled-live evidence are
recorded separately in
[`docs/development-readiness.md`](docs/development-readiness.md).
