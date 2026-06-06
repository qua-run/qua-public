# Changelog

## v0.2.0 — 2026-06-06

- **Examples are now fully self-contained and runnable.** The Go and Python
  server examples no longer import unpublished SDK packages — each implements
  the complete 402 → verify → settle → receipt flow directly against the wire
  contract in `docs/x402.md` (Go: stdlib only; Python: FastAPI + stdlib HTTP).
- `examples/curl/sign.py` extracted into a real script, with the platform-fee
  second leg implemented (the README previously left it as an exercise).
- CI: every push builds the Go example (`gofmt` + `go vet` + `go build`) and
  import-checks the Python examples.
- `examples/README.md` index added.
- LICENSE replaced with the full canonical Apache-2.0 text so GitHub detects
  the license correctly (was showing "Other").
- Spec fix in `docs/x402.md`: the `X-PAYMENT` envelope is encoded with
  **standard** base64 (with padding), not URL-safe base64 as previously
  stated. Standard base64 is what the facilitator and SDKs implement.
- CONTRIBUTING.md added.

## v0.1.0 — 2026-06-04

- Initial public artefacts: protocol reference (`docs/x402.md`), Go / Python /
  curl client examples, security policy.
