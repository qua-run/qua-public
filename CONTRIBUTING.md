# Contributing

Thanks for looking. This repo is the **public protocol surface** of Qua: the
wire spec ([`docs/x402.md`](docs/x402.md)) and runnable client/server
examples. The hosted facilitator's source is intentionally not here — see
"What's NOT here" in the [README](README.md).

## What contributions land here

- **Spec clarifications** — anything in `docs/x402.md` that's ambiguous,
  under-specified, or doesn't match observed facilitator behavior. These are
  the most valuable issues you can file.
- **Examples** — fixes to the existing Go / Python / curl examples, or new
  ones in other languages (see the open issues for wanted ones). Examples
  must be self-contained and run against the staging facilitator with no
  unpublished dependencies.
- **Docs** — corrections, clarity, broken links.

## What doesn't

- Security findings. **Never** open a public issue for those — see
  [SECURITY.md](SECURITY.md) (`security@qua.run`).
- Feature requests for the hosted service itself (dashboard, pricing,
  networks). Reach out via the dashboard instead; this repo tracks the
  protocol, not the product.

## Ground rules for examples

1. Self-contained: stdlib first, minimal well-known deps where unavoidable
   (e.g. `eth-account` for EIP-712 signing).
2. They must implement `docs/x402.md` exactly — the spec is the source of
   truth. If the spec and an example disagree, that's a bug in one of them;
   file it.
3. CI must pass: the Go example builds with `go vet` clean and `gofmt`
   formatting; Python examples must import cleanly.

## Dev loop

```bash
# Go example
cd examples/go && gofmt -l . && go vet ./... && go build ./...

# Python examples
pip install fastapi eth-account
cd examples/python && python -c "import main"
cd examples/curl   && python -c "import sign"
```

## License

Apache-2.0. By contributing you agree your contributions are licensed under
the same terms.
