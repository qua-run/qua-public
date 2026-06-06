# Qua — per-call payments for agents

[![ci](https://github.com/qua-run/qua-public/actions/workflows/ci.yml/badge.svg)](https://github.com/qua-run/qua-public/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![x402](https://img.shields.io/badge/protocol-x402-8A2BE2)](docs/x402.md)

Qua lets you charge a small USDC fee per HTTP request, settled on Base. It's
useful when you sell access to an API by the call (LLM proxies, search APIs,
scrapers, MCP tools) and don't want to mess with Stripe, accounts, or
monthly bills.

```
client                          your server                       Qua
  │     GET /search             │                                  │
  │ ──────────────────────────► │                                  │
  │ ◄─────────────────────────  │ 402 Payment Required             │
  │     (Payment requirements)  │  + PaymentRequirements JSON      │
  │                             │                                  │
  │     GET /search             │                                  │
  │     X-PAYMENT: <signed>     │                                  │
  │ ──────────────────────────► │                                  │
  │                             │  POST /verify  ────────────────► │
  │                             │  ◄──────────────────────  200 OK │
  │                             │  POST /settle  ────────────────► │
  │                             │  ◄──────────────────────  receipt│
  │ ◄─────────────────────────  │ 200 + X-PAYMENT-RESPONSE         │
```

This repo holds the *public* artefacts: SDK types, protocol reference,
client examples. The facilitator itself is a hosted service at
`https://facilitator.qua.run` — you don't run it.

## What's here

- [`docs/x402.md`](docs/x402.md) — the wire format, plain English. Read this
  if you want to know what an `X-PAYMENT` header actually contains.
- [`examples/`](examples/) — **runnable, self-contained** integrations in Go,
  Python, and curl. No unpublished dependencies: each one implements the wire
  contract directly, so they double as reference implementations. CI builds
  them on every push.

## What's NOT here

- The facilitator source, the settlement verifier, the relayer, the
  reconciler, the dashboard, the database migrations. Those are operational
  details of the hosted service; publishing them would only help an attacker
  reason about the production system. The protocol (above) is the
  integration contract, not the implementation.

## Getting started

1. Get an API key at <https://qua.run/dashboard> (or via the Control API if
   you have programmatic access).
2. Pin a payout address. This is the wallet that receives settled USDC; the
   facilitator will refuse to settle to any other address. Set it via
   `POST /v1/routes`, see `docs/x402.md` §"Routes".
3. Wrap an HTTP handler and serve it. The integration shape:

```go
pay := New(Config{
    APIKey:      os.Getenv("QUA_API_KEY"),        // ap_live_…
    PayTo:       os.Getenv("QUA_PAYOUT_ADDRESS"), // 0x…
    Facilitator: "https://facilitator.qua.run",
})
http.Handle("/search", pay.Require("1000", searchHandler)) // 1000 base units = 0.001 USDC
http.ListenAndServe(":8080", nil)
```

That `New` / `Require` middleware is ~200 lines of stdlib Go — the complete,
runnable version lives in [`examples/go/main.go`](examples/go/main.go)
(`go run .` and you're serving paid endpoints). Python equivalent in
[`examples/python/`](examples/python/), or do the whole dance by hand with
[`examples/curl/`](examples/curl/).

Standalone SDK packages (Go module / PyPI / npm) are on the
[roadmap](https://github.com/qua-run/qua-public/issues) — until they ship,
vendoring the example middleware is the supported path, and the protocol doc
is the contract it implements.

## Security

This repo intentionally exposes nothing that could be used to attack the
hosted facilitator. If you find an issue (in the protocol, in an example, or
in a published SDK), report it privately to security@qua.run. Please don't
file public issues for security findings.

## License

Apache 2.0. See [LICENSE](LICENSE).
