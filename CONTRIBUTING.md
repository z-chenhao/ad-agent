# Contributing

Repository interfaces are private and experimental unless a document says otherwise.
Read `AGENTS.md`, `AGENT.md`, and the design documents before changing runtime,
AdBackend, tool, skill, or approval behavior. Do not generalize an interface without a
real consumer and evidence.

Before handing off a change, run:

```sh
make cli
make test
```

Ordinary tests must not require ChatGPT OAuth, TikTok credentials, or the public
internet. Live-model, live-MAPI, and live-write checks are explicit opt-in tests and
must report their environment and authorization scope. Never commit credentials,
advertiser data, runtime checkpoints, build output, or `.data/`.

Use Conventional Commits when a commit is explicitly requested.
