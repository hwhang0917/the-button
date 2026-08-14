# Security Policy

## Supported Versions

Only the latest release is supported. The production instance at
https://button.runfridge.dev always runs the latest tag, and there are no
backports — if you find an issue in an older version, please verify it
against the latest release first.

## Reporting a Vulnerability

- **Email:** hwhang0917@gmail.com (also published at
  [`/.well-known/security.txt`](https://button.runfridge.dev/.well-known/security.txt))
- Or open a private report via GitHub:
  [Security → Report a vulnerability](https://github.com/hwhang0917/the-button/security/advisories/new)

Please include steps to reproduce and, if relevant, the affected endpoint or
component. This is a solo hobby project — expect an initial response within a
week, not an SLA. Please don't open public issues for security problems until
a fix has shipped.

## Scope

In scope:

- The Go server (`internal/`, `main.go`): auth/session handling, the link-code
  device transfer flow, quota enforcement, anything letting a player mint
  stars/coins/cards outside the game rules.
- The web client (`web/`): XSS, injection via nicknames or other user input.
- The release pipeline (`.github/workflows/`): supply-chain issues in the
  build or deploy path.

Out of scope:

- Denial of service or load testing against button.runfridge.dev — it runs on
  a Raspberry Pi; you will win, and it proves nothing.
- Vulnerabilities requiring a compromised device or browser.
- Reports from automated scanners with no demonstrated impact.
