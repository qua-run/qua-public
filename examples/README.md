# Examples

Every example here runs as-is — no unpublished SDKs, no placeholders. They
implement the wire contract in [`docs/x402.md`](../docs/x402.md) directly, so
they double as reference implementations for your own integration.

| Example | What it shows |
| --- | --- |
| [`go/`](go/) | Server middleware in ~200 lines of Go stdlib: 402 challenge → verify → handler → settle → receipt. `go run .` |
| [`python/`](python/) | The same middleware for FastAPI/Starlette, stdlib HTTP only. `python main.py` |
| [`curl/`](curl/) | A full payment cycle by hand: curl the 402, sign EIP-3009 legs with `sign.py`, replay with `X-PAYMENT`. |

All examples default to the **staging facilitator + Base Sepolia testnet**, so
you can try the full flow with a few cents of testnet USDC. Point
`QUA_FACILITATOR` / `QUA_NETWORK` / `QUA_ASSET` at production when you're
ready.

CI builds the Go example and import-checks the Python ones on every push, so
they can't silently rot.
