# Security policy

Found a security issue? Please report it privately. **Do not** open a public
issue or PR.

- Email: **security@qua.run**
- PGP key: <https://qua.run/.well-known/pgp-key.txt>
- Response SLA: ack within 24 hours; remediation timeline communicated within
  72 hours of ack.

## Scope

In scope:
- The hosted facilitator at `https://facilitator.qua.run` (and the
  `staging.facilitator.qua.run` staging deployment).
- The dashboard at `https://qua.run` and `staging.dashboard.qua.run`.
- The public protocol documented in `docs/x402.md`.
- The published SDKs (Go / Python / Node) once they're available as
  separate repos.

Out of scope (please don't report these, they're known characteristics):
- Rate-limit responses (429) under load. Working as intended.
- The fact that `/metrics` is open unless `METRICS_BEARER` is set. This is
  the documented default for back-compat with internal scrapers.
- DDoS at the network layer. Use a real DDoS service for that — it's not
  something we can fix in application code.

## Acknowledgments

Researchers who report valid issues (with reproducible PoCs) will be credited
in the relevant CHANGELOG entry and on `https://qua.run/security`, unless
they prefer to stay anonymous.
