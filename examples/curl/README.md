# curl example — pay an x402 endpoint by hand

This walks through paying a protected endpoint with `curl` + a small Python
script for the EIP-3009 signing. No SDK, no magic, just bytes.

## Prereqs

- A funded Base Sepolia (testnet) payer wallet (a few cents of USDC).
- A protected endpoint URL (run [`../go/`](../go/) or [`../python/`](../python/)
  locally if you don't have one).
- `python3 -m pip install eth-account`.

## Step 1 — Get the 402 challenge

```bash
curl -i https://example.com/protected
# HTTP/1.1 402 Payment Required
# Content-Type: application/json
#
# {"x402Version":1,"accepts":[{"scheme":"exact","network":"base-sepolia",...}]}
```

Save the `accepts[0]` block as `req.json`.

## Step 2 — Sign the EIP-3009 leg(s)

[`sign.py`](sign.py) (in this directory) reads `req.json`, signs the
`TransferWithAuthorization` leg — and the second platform-fee leg
automatically, when the challenge advertises a `fee` — and prints the
base64 `X-PAYMENT` header value:

```bash
export PAYER_KEY=0x...        # your testnet payer key
python3 sign.py > payment.b64
```

What it does, in short: build the EIP-712 domain from the challenge
(`extra.name` / `extra.version` / chain ID / `asset`), compute the net leg as
`gross - fee.amount`, sign `TransferWithAuthorization(from, to, value,
validAfter, validBefore, nonce)` with a fresh 32-byte nonce per leg, and
base64 the envelope from `docs/x402.md` §2. Read the script — it's ~100 lines
and every field maps 1:1 to the spec.

## Step 3 — Replay with the X-PAYMENT header

```bash
curl -i https://example.com/protected \
  -H "X-PAYMENT: $(cat payment.b64)"
# HTTP/1.1 200 OK
# X-PAYMENT-RESPONSE: <base64 receipt>
# Content-Type: application/json
#
# {"results":["…"]}
```

Decode the receipt:

```bash
echo "$X_PAYMENT_RESPONSE" | base64 -d | jq .
# { "success": true, "transaction": "0x…", "amount": "990", "fee": "10", … }
```

That's a full payment cycle in 3 steps. The SDK does the same thing in
~one line — but doing it by hand once is the best way to understand what
the wire looks like.

## If a platform fee is advertised

Nothing extra to do — `sign.py` detects the `fee` block in the challenge and
adds the second signed leg (`feeAuthorization`, paying `fee.amount` to
`fee.recipient`) so the two legs sum to the gross price. See `docs/x402.md`
§2 for the envelope shape.
