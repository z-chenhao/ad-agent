# Security

Ad Agent is a single-operator, loopback application. Do not expose its application
listener, state directory or operator key publicly. The OAuth callback is a separate
limited listener, not a reverse proxy to the application.

Keep `.data*` private (0700), files containing keys private (0600), and credentials out
of issues, screenshots, logs and Git. Model providers receive the advertising context
required for their requested tasks; use only authorized accounts and data.

Report vulnerabilities privately through the repository's GitHub private vulnerability
reporting feature when available. If unavailable, request a private maintainer channel
without posting exploit details or secrets publicly. Do not attach private state or
runtime transcripts. No public endpoint or live ad write is authorized by a bug report.

Alpha releases have no production support or security SLA. Unknown mutation outcomes
must be reconciled rather than retried. Provider credentials and privileges should be
revoked through their official controls if compromised.
