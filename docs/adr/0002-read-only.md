# 0002. Read-only, no account

Status: accepted, 2026-11-06.

## Decision

The CLI reads public data. It has no cart, checkout, order or account command, and it stores no credentials, cookies or session. That keeps the tool safe to hand to an agent.

Request pace stays low: 500 ms between requests, at most one request at a time, bounded retries. No bulk or mirror feature.

## Consequences

- No `auth` command. Exit code 3 is unused and kept so the exit codes stay stable across the author's shop CLIs.
