// Example: protect an HTTP endpoint with Qua.
//
// This is what an integrator's main.go looks like. The SDK does the 402
// dance + EIP-3009 signing + facilitator round-trips for you.
//
//   go run .
//
// Then:
//   curl localhost:8080/search           # → 402 Payment Required
//   curl localhost:8080/search -H "X-PAYMENT: <signed>"   # → 200
//
// (See ../curl/ for how to produce that signed header by hand.)
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/qua-run/qua-go" // hypothetical published path; adjust on publish
)

func main() {
	pay := qua.New(qua.Config{
		// Bearer-authenticate to the facilitator with your developer key.
		APIKey: os.Getenv("QUA_API_KEY"), // ap_live_…
		// The wallet that receives settled USDC. Must match the
		// `allowed_payout_address` you pinned on this developer.
		PayTo: os.Getenv("QUA_PAYOUT_ADDRESS"), // 0x…
		// The public Qua facilitator. Use staging.facilitator.qua.run for testnet.
		Facilitator: "https://facilitator.qua.run",
	})

	// Wrap any http.Handler. The price is in integer base units of USDC:
	// 1000 = 0.001 USDC. The 402 challenge will advertise this as
	// MaxAmountRequired; the client signs it; we verify + settle.
	http.Handle("/search", pay.Require("1000", http.HandlerFunc(searchHandler)))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"results": ["the answer is 42"]}`))
}
