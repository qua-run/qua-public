"""Example: protect a FastAPI endpoint with x402 per-call payments.

Self-contained — no SDK import. The middleware below is the whole
integration: issue the 402 challenge, relay the signed payment to a
Qua-compatible facilitator for verification, run the route, settle
on-chain, and attach the receipt header. The Python SDK equivalent of
examples/go/main.go.

    pip install fastapi uvicorn
    export QUA_API_KEY=ap_test_...    # https://qua.run/dashboard
    export QUA_PAYOUT_ADDRESS=0x...   # your pinned payout wallet
    python main.py

Then:

    curl -i localhost:8080/search                           # 402 + requirements
    curl -i localhost:8080/search -H "X-PAYMENT: <signed>"  # 200 + receipt

See ../curl/ for producing the signed header by hand, and ../../docs/x402.md
for the wire contract this file implements.
"""
import base64
import json
import os
import urllib.error
import urllib.request

from fastapi import FastAPI
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse

# Canonical USDC deployments (docs/x402.md §7).
USDC_BASE = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"          # base, chain 8453
USDC_BASE_SEPOLIA = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"  # base-sepolia, chain 84532


class FacilitatorError(Exception):
    """Transport/HTTP failure talking to the facilitator (-> 502, retryable)."""


class QuaMiddleware(BaseHTTPMiddleware):
    """Charge a per-call USDC price on selected routes via the x402 protocol.

    prices maps a path to its price in integer USDC base units
    ("1000" = 0.001 USDC). Paths not in the map pass through unpaid.
    """

    def __init__(self, app, *, api_key, pay_to, facilitator, prices,
                 network="base-sepolia", asset=USDC_BASE_SEPOLIA):
        super().__init__(app)
        self.api_key = api_key
        self.pay_to = pay_to
        self.facilitator = facilitator.rstrip("/")
        self.prices = prices
        self.network = network
        self.asset = asset

    async def dispatch(self, request, call_next):
        price = self.prices.get(request.url.path)
        if price is None:
            return await call_next(request)

        requirement = self._requirement(request, price)

        header = request.headers.get("X-PAYMENT")
        if not header:
            return self._challenge(requirement)
        try:
            payment = json.loads(base64.b64decode(header, validate=True))
        except (ValueError, json.JSONDecodeError):
            return self._challenge(requirement, error="malformed X-PAYMENT header")

        # 1. Verify — signature, amount, expiry, payer. The facilitator's call.
        try:
            verdict = self._post("/verify", payment, requirement)
        except FacilitatorError as exc:
            return JSONResponse({"error": f"payment verification failed: {exc}"}, status_code=502)
        if not verdict.get("isValid"):
            reason = verdict.get("invalidReason", "")
            return self._challenge(requirement, error=f"payment verification rejected: {reason}")

        # 2. Do the work the caller paid for.
        response = await call_next(request)
        if response.status_code >= 500:
            return response  # upstream failed — charge nothing

        # 3. Settle on-chain. Idempotent by (network, payer, nonce): a retry
        # with the same signed authorization returns the original receipt.
        try:
            receipt = self._post("/settle", payment, requirement)
        except FacilitatorError as exc:
            return JSONResponse({"error": f"settlement failed: {exc}"}, status_code=502)
        if not receipt.get("success"):
            reason = receipt.get("errorReason", "")
            return JSONResponse({"error": f"settlement failed: {reason}"}, status_code=502)

        receipt.setdefault("asset", requirement["asset"])
        receipt.setdefault("amount", requirement["maxAmountRequired"])
        encoded = base64.b64encode(json.dumps(receipt).encode()).decode()
        response.headers["X-PAYMENT-RESPONSE"] = encoded
        return response

    def _requirement(self, request, price):
        return {
            "scheme": "exact",
            "network": self.network,
            "maxAmountRequired": price,
            "resource": str(request.url.replace(query="")),
            "description": "example protected endpoint",
            "mimeType": "application/json",
            "payTo": self.pay_to,
            "asset": self.asset,
            "maxTimeoutSeconds": 60,
            "extra": {"name": "USDC", "version": "2"},
        }

    def _challenge(self, requirement, error=""):
        return JSONResponse(
            {"x402Version": 1, "accepts": [requirement], "error": error},
            status_code=402,
        )

    def _post(self, path, payment, requirement):
        """Send the x402 facilitator envelope to {facilitator}{path}.

        Synchronous stdlib HTTP keeps the example dependency-free; swap in an
        async client (httpx) for production loads.
        """
        body = json.dumps({
            "x402Version": 1,
            "paymentPayload": payment,
            "paymentRequirements": requirement,
        }).encode()
        req = urllib.request.Request(
            self.facilitator + path,
            data=body,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.api_key}",
            },
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                return json.load(resp)
        except urllib.error.HTTPError as exc:
            detail = exc.read(1 << 20).decode(errors="replace")
            raise FacilitatorError(f"facilitator {path} returned {exc.code}: {detail}") from exc
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
            raise FacilitatorError(str(exc)) from exc


app = FastAPI()
app.add_middleware(
    QuaMiddleware,
    api_key=os.environ.get("QUA_API_KEY", "ap_test_replace-me"),
    pay_to=os.environ.get("QUA_PAYOUT_ADDRESS", "0x0000000000000000000000000000000000000000"),
    facilitator=os.environ.get("QUA_FACILITATOR", "https://staging.facilitator.qua.run"),
    network=os.environ.get("QUA_NETWORK", "base-sepolia"),
    asset=os.environ.get("QUA_ASSET", USDC_BASE_SEPOLIA),
    # Per-route prices in integer USDC base units (1000 = 0.001 USDC).
    prices={"/search": "1000"},
)


@app.get("/search")
def search():
    return {"results": ["the answer is 42"]}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8080)
