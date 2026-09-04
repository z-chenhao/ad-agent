# Security

Ad Agent is an experimental, single-user, local-first application, not a multi-tenant
advertising service. Live TikTok writes are disabled by default. Local-sandbox,
HTTP-fake, TikTok-platform-sandbox, and controlled-live results must remain visibly
distinct.

## Reporting a vulnerability

Use the repository's private GitHub Security Advisory. Do not place an App Secret,
access token, OAuth code, operator key, advertiser data, or runtime-directory content
in a public issue. Include the affected version, reproduction conditions, expected
impact, and a credential-free minimal reproduction.

## Credential boundary

- ChatGPT OAuth remains in the local Pi user directory and never enters this repository.
- Direct model API keys are read from an explicitly named process environment variable.
  Only the variable name and non-secret endpoint/model metadata may be stored. Claude
  Agent SDK receives `ANTHROPIC_API_KEY` only in its isolated child environment.
- TikTok App Secret is read only from the local process environment. Access tokens are
  stored in a local credential file with mode `0600`.
- `.data*/`, `.env*`, databases, logs, and runtime checkpoints must not be committed.
- Pi, J, and Claude model subprocesses do not inherit `AD_AGENT_TIKTOK_*` or `TIKTOK_*`
  variables.
- OAuth callback responses and logs never echo an authorization code, token, or full
  query string.

If a credential enters Git history, revoke or rotate it first, then clean the history.
Deleting only the current file is not remediation.
