"""Example: protect a FastAPI endpoint with Qua.

This is the Python SDK equivalent of examples/go/main.go.

    pip install qua-py  # hypothetical published path; adjust on publish
    pip install fastapi uvicorn
    python main.py
"""
import os

from fastapi import FastAPI
from qua import QuaMiddleware  # hypothetical


app = FastAPI()
app.add_middleware(
    QuaMiddleware,
    api_key=os.environ["QUA_API_KEY"],            # ap_live_…
    pay_to=os.environ["QUA_PAYOUT_ADDRESS"],      # 0x…
    facilitator="https://facilitator.qua.run",
    # Per-route prices in integer base units of USDC (1000 = 0.001 USDC).
    prices={"/search": "1000"},
)


@app.get("/search")
def search():
    return {"results": ["the answer is 42"]}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8080)
