# Security Policy

## Supported Versions

We currently support the latest minor version on the `main` branch.
Older tagged versions are best-effort.

## Reporting a Vulnerability

Please do **not** open a public GitHub issue for security vulnerabilities.

Email **security@mirador.org** with:

- A description of the issue
- Steps to reproduce
- The version (commit SHA or tag) you observed it on
- Any proof-of-concept or exploit details

We aim to acknowledge new reports within two business days and will
coordinate a fix and disclosure timeline with you.

## Scope

In scope:

- The detector and extraction logic in this module.
- Embedded bridge configurations that produce incorrect or unsafe
  correlation IDs.

Out of scope:

- Vulnerabilities in upstream `go-ethereum`, `zap`, or other
  dependencies (please report those upstream).
- The on-chain bridge protocols themselves.
