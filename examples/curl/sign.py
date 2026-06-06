#!/usr/bin/env python3
"""Sign an x402 payment by hand: req.json in, X-PAYMENT header out.

Reads the `accepts[0]` block of a 402 challenge from req.json, signs the
EIP-3009 TransferWithAuthorization leg(s) — two legs when a platform fee is
advertised — and prints the base64 X-PAYMENT header value to stdout.

    pip install eth-account
    export PAYER_KEY=0x...     # testnet payer key, never a mainnet key you care about
    python sign.py > payment.b64

See README.md in this directory for the full walkthrough.
"""
import base64
import json
import os
import secrets
import sys
import time

from eth_account import Account
from eth_account.messages import encode_typed_data

CHAIN_IDS = {"base": 8453, "base-sepolia": 84532}

EIP712_TYPES = {
    "EIP712Domain": [
        {"name": "name", "type": "string"},
        {"name": "version", "type": "string"},
        {"name": "chainId", "type": "uint256"},
        {"name": "verifyingContract", "type": "address"},
    ],
    "TransferWithAuthorization": [
        {"name": "from", "type": "address"},
        {"name": "to", "type": "address"},
        {"name": "value", "type": "uint256"},
        {"name": "validAfter", "type": "uint256"},
        {"name": "validBefore", "type": "uint256"},
        {"name": "nonce", "type": "bytes32"},
    ],
}


def sign_leg(key, domain, payer, to, value, valid_before):
    """Sign one EIP-3009 TransferWithAuthorization and return its wire dict."""
    nonce = "0x" + secrets.token_hex(32)
    message = {
        "from": payer,
        "to": to,
        "value": value,
        "validAfter": 0,
        "validBefore": valid_before,
        "nonce": nonce,
    }
    signed = Account.sign_message(
        encode_typed_data(full_message={
            "types": EIP712_TYPES,
            "primaryType": "TransferWithAuthorization",
            "domain": domain,
            "message": message,
        }),
        key,
    )
    return {
        "from": payer,
        "to": to,
        "value": str(value),
        "validAfter": "0",
        "validBefore": str(valid_before),
        "nonce": nonce,
        "v": hex(signed.v),
        "r": hex(signed.r),
        "s": hex(signed.s),
    }


def main():
    key = os.environ.get("PAYER_KEY")
    if not key:
        sys.exit("set PAYER_KEY to your (testnet!) payer private key")
    req = json.load(open("req.json"))

    domain = {
        "name": req["extra"]["name"],        # "USDC"
        "version": req["extra"]["version"],  # "2"
        "chainId": CHAIN_IDS[req["network"]],
        "verifyingContract": req["asset"],
    }
    payer = Account.from_key(key).address
    valid_before = int(time.time()) + req.get("maxTimeoutSeconds", 60)

    # Net leg = gross - fee.amount. Whole gross if no fee is advertised.
    gross = int(req["maxAmountRequired"])
    fee = int((req.get("fee") or {}).get("amount", "0"))
    net = gross - fee

    envelope = {
        "x402Version": 1,
        "scheme": req["scheme"],
        "network": req["network"],
        "payload": sign_leg(key, domain, payer, req["payTo"], net, valid_before),
    }
    if fee > 0:
        # Second authorization: the platform fee leg. The two legs sum to gross.
        envelope["feeAuthorization"] = sign_leg(
            key, domain, payer, req["fee"]["recipient"], fee, valid_before,
        )

    print(base64.b64encode(json.dumps(envelope).encode()).decode())


if __name__ == "__main__":
    main()
