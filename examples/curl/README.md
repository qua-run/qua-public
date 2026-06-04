# curl example — pay an x402 endpoint by hand

This walks through paying a protected endpoint with `curl` + a small Python
script for the EIP-3009 signing. No SDK, no magic, just bytes.

## Prereqs

- A funded Base Sepolia (testnet) payer wallet (a few cents of USDC).
- A protected endpoint URL.
- `python3 -m pip install eth-account eth-utils`.

## Step 1 — Get the 402 challenge

```bash
curl -i https://example.com/protected
# HTTP/1.1 402 Payment Required
# Content-Type: application/json
#
# {"x402Version":1,"accepts":[{"scheme":"exact","network":"base-sepolia",...}]}
```

Save the `accepts[0]` block as `req.json`.

## Step 2 — Sign the EIP-3009 leg

```python
# sign.py
import json, time, secrets, base64
from eth_account import Account
from eth_account.messages import encode_typed_data

PAYER_KEY = "0x..."           # your testnet payer key
req = json.load(open("req.json"))

# Domain for USDC on Base Sepolia. Read these from the chain or hardcode.
domain = {
    "name":              req["extra"]["name"],       # "USD Coin"
    "version":           req["extra"]["version"],    # "2"
    "chainId":           84532,                       # base-sepolia
    "verifyingContract": req["asset"],
}

# Net leg = gross - fee.amount. Use whole gross if no fee advertised.
gross = int(req["maxAmountRequired"])
fee   = int(req.get("fee", {}).get("amount", "0"))
net   = gross - fee

now    = int(time.time())
nonce  = "0x" + secrets.token_hex(32)
validBefore = now + req.get("maxTimeoutSeconds", 60)

msg = {
    "types": {
        "EIP712Domain": [
            {"name":"name","type":"string"},
            {"name":"version","type":"string"},
            {"name":"chainId","type":"uint256"},
            {"name":"verifyingContract","type":"address"},
        ],
        "TransferWithAuthorization": [
            {"name":"from","type":"address"},
            {"name":"to","type":"address"},
            {"name":"value","type":"uint256"},
            {"name":"validAfter","type":"uint256"},
            {"name":"validBefore","type":"uint256"},
            {"name":"nonce","type":"bytes32"},
        ],
    },
    "primaryType": "TransferWithAuthorization",
    "domain": domain,
    "message": {
        "from":        Account.from_key(PAYER_KEY).address,
        "to":          req["payTo"],
        "value":       net,
        "validAfter":  0,
        "validBefore": validBefore,
        "nonce":       nonce,
    },
}
signed = Account.sign_message(encode_typed_data(full_message=msg), PAYER_KEY)

payload = {
    "x402Version": 1,
    "scheme":  req["scheme"],
    "network": req["network"],
    "payload": {
        "from":        msg["message"]["from"],
        "to":          msg["message"]["to"],
        "value":       str(net),
        "validAfter":  "0",
        "validBefore": str(validBefore),
        "nonce":       nonce,
        "v": hex(signed.v),
        "r": hex(signed.r),
        "s": hex(signed.s),
    },
}
print(base64.b64encode(json.dumps(payload).encode()).decode())
```

```bash
python3 sign.py > payment.b64
```

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

Add a second signed leg (`feeAuthorization`) in the `payload`:

```python
fee_msg = { ...same shape..., "to": req["fee"]["recipient"], "value": fee }
fee_signed = Account.sign_message(encode_typed_data(full_message=fee_msg), PAYER_KEY)
payload["feeAuthorization"] = { ... }
```

See `docs/x402.md` §2 for the full envelope shape.
