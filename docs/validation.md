# Validation and release gates

Implementation tests, browser acceptance and provider/platform acceptance are separate.
Passing CI does not establish advertising effectiveness or live integration.

## Reproducible local gate

Requires the Go and Node versions documented in the README. From a clean source checkout:

```sh
npm ci --ignore-scripts
make smoke
make test
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
npm audit --audit-level=high
npm run check
npm exec prettier -- --check . --ignore-path .gitignore
make test-web
make test-web-manager
./bin/ad-agent verify
```

CI runs race-enabled Go tests, builds the bridge JavaScript before its protocol tests,
checks types, English and formatting, and runs Advertiser/Manager browser suites.
Explicit test entrypoints fail on missing builds instead of accepting empty globs.
Browser failure uploads contain test screenshots, never operator state or credentials.
Ordinary tests use isolated local Sandbox state and HTTP fakes, not model credentials.
Do not run concurrent Playwright invocations into the same output directory.

`make smoke` also runs in the browser CI job after the source build. It uses a new
temporary Sandbox and a credential-free child environment to check campaign/report
consistency, report provenance, restart persistence, an empty change ledger and the
deterministic harness. It removes only its own temporary state. This is source-install
acceptance, not browser or live-model acceptance. See [maintenance](maintaining.md) for
review and evidence handling.

### Alpha release review — 2026-09-05

- A clean staged-source checkout passed dependency installation, race-enabled Go tests,
  `go vet`, TypeScript checks, bridge builds and the CLI verification command.
- The current Advertiser browser suite passed 32 non-model checks; Manager passed its
  browser check. Two opt-in live-model cases were deliberately skipped.
- Pi, Built-in, Codex and Claude bridge protocol tests passed. These include controlled
  transport tests, not four live-provider acceptances.
- With Go 1.26.8, `govulncheck` found no vulnerabilities in called or imported packages.
  One advisory remains in a required module whose affected package is not imported.
  `npm audit` reported no known dependency vulnerabilities at review time.
- The operator trace audit covered 22 turns: no missing or duplicate terminal events
  and no unmatched tool starts/results. Five failed turns and one cancellation remain
  historical failures, not successful acceptance. The concurrency/report-capacity issue
  was corrected; intermittent provider availability is not guaranteed by local tests.

An initial product-tour attempt was cancelled by a recording-client selector timeout.
A separately authorized attempt in a fresh Sandbox then completed: 13 successful tool
calls, two final cards, matching saved context, no browser errors and no changes.
Its result is replayed in the [product tour](product-tour.md), without another model call.
The model overstated profitability from ROAS; a focused performance-insights skill
correction passed structural/application checks but has not had another live-model probe.

## Acceptance boundaries

| Gate                 | Required evidence                                                                      | Not established                         |
| -------------------- | -------------------------------------------------------------------------------------- | --------------------------------------- |
| Domain/application   | Stage/apply policy, exact review, read-back, reconciliation, persistence, cancellation | Live delivery                           |
| Sandbox              | Seeded replay, causal/budget/metric invariants, restart, isolation, time advancement   | TikTok calibration or prediction        |
| TikTok HTTP fake     | Request mappings, partial/error/missing-field handling and source isolation            | Permissions or actual platform defaults |
| Runtime bridges      | Tool authority, transcript pairing, public text, isolation, cancellation, checkpoints  | Current provider availability           |
| Browser              | Navigation, settings, exact approvals, charts, streaming and replay                    | Live-model reasoning quality            |
| Optional model probe | One runtime/model path at a point in time                                              | Other runtimes or platform access       |
| Controlled TikTok    | Actual authorized reads/writes and verified outcomes                                   | Still pending developer approval        |

## Regression inventory

- Budget, creation and rule operations share stage/apply policy. Exact delivery,
  creative and targeting fields appear in review. Read-back rejects missing fields,
  wrong definitions/status and foreign accounts. Partial/unknown writes are not retried.
- Saved audiences gate cohorts; unsupported definitions fail closed. Approved rules
  execute on virtual time. Lifetime spend survives days; reach is query-window scoped
  and non-additive. Clock side effects are atomic and share the account writer lease.
- Advancing time refreshes all report pages. Partial-day ROAS is reported value divided
  by spend, with coverage limitations retained. Missing comparison data is not healthy
  performance or zero.
- Account/object/date context is captured per user turn, outside execution activity.
  Public commentary and tools retain event order and readable subagent labels. Tool
  timers measure execution in milliseconds; private reasoning is never displayed.
- Result cards precede growing activity and the final interpretation. In-place updates
  preserve card order; replay preserves evidence, exact approval controls and failure
  status. Dedicated suggestions edit the composer but never execute or approve.
- Briefings require one to three distinct evidence-backed findings and concrete next
  steps. Blank, repeated and ungrounded items fail atomically. This is structural
  validation, not automated proof of semantic quality.
- Runtime/model settings retain source-bound history, cards and unsent text. Changed
  execution bindings or unsettled turns clear incompatible native checkpoints. Changes
  to the system/tool contract also invalidate native state before execution. Public
  history is bounded and pageable; it never revives old dataset or approval authority.
- Source changes create separate conversations. Settings reject active leases. Custom
  skills remain scope/capability-filtered and cannot weaken mandatory safety.
- Report capacity is reserved before concurrent upstream reads. Identical completed
  queries reuse their turn snapshot. The eight-snapshot resource bound is documented
  in the tool, not confused with model quota or advertising budget.
- Authentication expiry returns to login without retrying writes. Late responses cannot
  invalidate a new login. Rate-limit feedback does not retain the entered operator key.
- Metadata logs correlate HTTP requests and turns without bodies, credentials, arbitrary
  error text or native transcripts. Known failure categories remain diagnosable.

See [Web design](web-design.md), [runtime boundaries](codex-runtime.md), and
[local logs](local-development.md) for their owning contracts.

## Bounded model evidence

The following authorized checks were recorded on 2026-09-05 against isolated local
Sandbox environments. They are historical acceptance evidence, not current availability
guarantees or a broad evaluation.

| Connection                      | Recorded result                                                                      | Boundary                                                                         |
| ------------------------------- | ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------- |
| Pi + ChatGPT OAuth + Luna       | Web budget and creative-copy flows passed                                            | Paired events, unique preview, no write before approval, exact read-back, reload |
| Codex + ChatGPT OAuth + Luna    | Read-only CLI probe passed after OAuth initialization fix                            | One native runtime path, not full Web acceptance                                 |
| Built-in + ChatGPT OAuth + Luna | Initial probe failed after tools; an authorized follow-up passed with six tool calls | No four-tool cap; intermittent provider failure not proven permanently resolved  |
| Claude / other HTTP providers   | Local bridge and configuration tests                                                 | Live credentials/provider acceptance not completed                               |

A later operator trace contains a classified provider transport failure and a cancelled
turn with report-capacity failures. Do not retroactively invent causes for older generic
failures. Source-level and controlled-stream regressions do not prove live-model prose
quality or latency.

## Optional provider probes

Obtain explicit authorization: these consume provider quota. Use a new private data
directory and do not use the operator's working Sandbox for acceptance writes.

```sh
AD_AGENT_LIVE_E2E=1 AD_AGENT_E2E_DATA_DIR=acceptance-luna \
  AD_AGENT_E2E_PORT=18483 npm run test:e2e --workspace=@ad-agent/web -- --grep 'real Luna'
```

After `make build`, a bounded read-only CLI probe is:

```sh
./bin/ad-agent chat --runtime pi --data-dir .data/probe-pi --session acceptance-pi --events \
  --message 'Read the account and list its campaigns. Do not stage changes.'
```

Use `--runtime codex` or `--runtime builtin` for those adapters; Claude requires an
explicit Anthropic API-key connection. Runtime/model continuity checks in ordinary CI
use instrumented runtimes and controlled streams, not live-model handoff evaluation.

## Platform and publication gate

Developer approval, scopes, advertiser authorization, controlled reads and bounded
write/read-back acceptance remain required. Keep TikTok writes off until these pass.
Local-Sandbox and HTTP-fake results must never be labeled platform-Sandbox or live evidence.

The initial source version is `0.1.0-alpha.1`; schemas may change during alpha. Preserve
private state backups. [CHANGELOG](../CHANGELOG.md) records user-facing releases, not
development-turn history. Review tracked sources and secret exclusion before committing;
repository visibility and default-branch changes require their own authorization.
