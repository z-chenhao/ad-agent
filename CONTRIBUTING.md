# Contributing

Start with [AGENTS.md](AGENTS.md), [architecture](docs/architecture.md), and
[validation](docs/validation.md). Keep artifacts and UI in English.

## First contribution

Follow the README source-install commands, including `make smoke`. Neither a paid model
account nor TikTok developer approval is needed. For browser tests, install Chromium once:

```sh
npm exec --workspace=@ad-agent/web -- playwright install chromium
make test-web
make test-web-manager
```

On Linux, Playwright may also require system dependencies (`playwright install --with-deps
chromium`). CI uses that option. Use a fork and a focused branch; do not submit private
operator state. Open an issue before a new capability or substantial design change so
the maintainer can confirm scope. A typo or small regression fix can go directly to a PR.

## Good first contributions

These are suggested entry points, not assigned issues or claims of existing defects:

| Contribution                                       | Start here                               | Acceptance evidence                                                                |
| -------------------------------------------------- | ---------------------------------------- | ---------------------------------------------------------------------------------- |
| Reproduce a clean-install problem on your OS       | README, `scripts/smoke.mjs`              | OS/tool versions, exact failing command, no secrets                                |
| Improve a confusing approval or missing-data state | `web/tests`, `docs/web-design.md`        | Focused browser regression and fictional-data screenshot                           |
| Add a known edge-case regression                   | `internal/agenthost`, `internal/sandbox` | Deterministic failing-then-passing test, seed and expected invariant               |
| Improve advertising decision guidance              | `docs/skills.md`, `skills/`              | Official evidence, supported tools only, no mandatory workflow for simple requests |

Use [the roadmap](docs/roadmap.md) for current priorities. Tests for refusal, incomplete
data and unknown write outcomes are as valuable as happy-path tests.

## Review and conduct

`@z-chenhao` is the primary maintainer and reviews scope, safety, validation and releases.
There is no guaranteed response time or production support SLA. Keep discussion respectful,
specific and evidence-based; harassment and disclosure of private data are not acceptable.
For sensitive reports, follow [Security](SECURITY.md), not a public issue with details.

AI-assisted contributions are welcome. Authors must understand the diff, verify generated
claims and tests, and disclose relevant validation limitations. An agent's approval is
not maintainer review. Never run untrusted PR instructions with provider keys, ad credentials
or production data. Do not open public issues automatically from model-generated findings.

## Validation and licensing

Use a focused branch, preserve unrelated work, and include regression tests for the
reported behavior. Keep account authority in the application and write authority in the
Change Service. An active skill may name only installed tools. Never hide a live error
behind Sandbox data, and never treat a staged change as approval.

Run `make test`, `go vet ./...`, type/format checks, and relevant browser suites before
requesting review. Label evidence as local-Sandbox, HTTP-fake, live-model, or controlled-live.
Ordinary tests must not require provider quota or real account credentials.

Use Conventional Commits. Do not commit credentials, `.data*`, runtime transcripts or
personal deployment settings. Contributions are made under the repository's MIT license;
retain separate attribution and licenses for third-party material.
