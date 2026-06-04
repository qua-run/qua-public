# Qua — per-call payments for agents

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
- [`examples/`](examples/) — minimal client snippets in Go, Python, and curl.
  Copy-pastable; they hit the public staging facilitator.

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
3. Wrap an HTTP handler with the SDK (Go example below) and serve it.

```go
import "github.com/qua-run/qua-go"   // <-- once published

pay := qua.New(qua.Config{
    APIKey:      os.Getenv("QUA_API_KEY"),       // ap_live_…
    PayTo:       os.Getenv("QUA_PAYOUT_ADDRESS"), // 0x…
    Facilitator: "https://facilitator.qua.run",
})
http.Handle("/search", pay.Require("1000", searchHandler)) // 1000 base units = 0.001 USDC
http.ListenAndServe(":8080", nil)
```

The full SDK source ships in a separate Go module once that decision is
made. Until then, the protocol doc above + the curl example in
[`examples/curl/`](examples/curl/) is enough to integrate by hand.

## Security

This repo intentionally exposes nothing that could be used to attack the
hosted facilitator. If you find an issue (in the protocol, in an example, or
in a published SDK), report it privately to security@qua.run. Please don't
file public issues for security findings.

## License

Apache 2.0. See [LICENSE](LICENSE).
