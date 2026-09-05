# Contributing

Start with [AGENTS.md](AGENTS.md), [architecture](docs/architecture.md), and
[validation](docs/validation.md). Keep artifacts and UI in English.

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
