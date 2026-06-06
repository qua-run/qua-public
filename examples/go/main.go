// Example: protect an HTTP endpoint with x402 per-call payments.
//
// Self-contained — Go stdlib only, no SDK import. These ~200 lines are the
// entire integration: issue the 402 challenge, relay the signed payment to a
// Qua-compatible facilitator for verification, run your handler, settle
// on-chain, and hand the receipt back to the caller. Use it as a reference
// for your own integration, or as a starting point until the standalone Go
// SDK ships as a separate module.
//
// Run it:
//
//	export QUA_API_KEY=ap_test_...    # https://qua.run/dashboard
//	export QUA_PAYOUT_ADDRESS=0x...   # your pinned payout wallet
//	go run .
//
// Poke it:
//
//	curl -i localhost:8080/search                           # 402 + requirements
//	curl -i localhost:8080/search -H "X-PAYMENT: <signed>"  # 200 + receipt
//
// See ../curl/ for producing the signed header by hand, and ../../docs/x402.md
// for the wire contract this file implements.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Canonical USDC deployments (docs/x402.md §7).
const (
	usdcBase        = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913" // base, chain 8453
	usdcBaseSepolia = "0x036CbD53842c5426634e7929541eC2318f3dCF7e" // base-sepolia, chain 84532
)

// ---- Wire types (docs/x402.md) ----

// Fee advertises the platform take-rate carved out of the gross price.
type Fee struct {
	Recipient string `json:"recipient"`
	Bps       int    `json:"bps"`
	Amount    string `json:"amount"`
}

// PaymentRequirement is one entry in the 402 challenge's `accepts` array.
type PaymentRequirement struct {
	Scheme            string         `json:"scheme"`
	Network           string         `json:"network"`
	MaxAmountRequired string         `json:"maxAmountRequired"`
	Resource          string         `json:"resource"`
	Description       string         `json:"description"`
	MimeType          string         `json:"mimeType"`
	PayTo             string         `json:"payTo"`
	Asset             string         `json:"asset"`
	MaxTimeoutSeconds int            `json:"maxTimeoutSeconds"`
	Extra             map[string]any `json:"extra,omitempty"`
	Fee               *Fee           `json:"fee,omitempty"`
}

// ---- Middleware ----

// Config carries everything an integration needs.
type Config struct {
	APIKey      string // Bearer token for the facilitator (ap_live_… / ap_test_…)
	PayTo       string // wallet that receives settled USDC; must match your pinned payout address
	Facilitator string // e.g. https://facilitator.qua.run
	Network     string // "base" or "base-sepolia"
	Asset       string // USDC contract address for Network
}

// Middleware holds immutable config and is safe for concurrent use.
type Middleware struct {
	cfg  Config
	http *http.Client
}

// New builds a Middleware. Settlement waits for on-chain confirmation, so the
// client timeout is generous.
func New(cfg Config) *Middleware {
	return &Middleware{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}}
}

// Require wraps next so every request must carry a valid, settled payment of
// price (integer USDC base units: "1000" = 0.001 USDC).
func (m *Middleware) Require(price string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := PaymentRequirement{
			Scheme:            "exact",
			Network:           m.cfg.Network,
			MaxAmountRequired: price,
			Resource:          "http://" + r.Host + r.URL.Path,
			Description:       "example protected endpoint",
			MimeType:          "application/json",
			PayTo:             m.cfg.PayTo,
			Asset:             m.cfg.Asset,
			MaxTimeoutSeconds: 60,
			Extra:             map[string]any{"name": "USDC", "version": "2"},
		}

		header := r.Header.Get("X-PAYMENT")
		if header == "" {
			challenge(w, req, "")
			return
		}

		// The decoded header is relayed to the facilitator as-is; the
		// middleware never needs to introspect the signed authorization.
		raw, err := base64.StdEncoding.DecodeString(header)
		if err != nil || !json.Valid(raw) {
			challenge(w, req, "malformed X-PAYMENT header")
			return
		}
		payment := json.RawMessage(raw)

		// 1. Verify — signature, amount, expiry, payer. The facilitator's call.
		var verdict struct {
			IsValid       bool   `json:"isValid"`
			InvalidReason string `json:"invalidReason"`
			Payer         string `json:"payer"`
		}
		if err := m.post("/verify", payment, req, &verdict); err != nil {
			http.Error(w, "payment verification failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if !verdict.IsValid {
			challenge(w, req, "payment verification rejected: "+verdict.InvalidReason)
			return
		}

		// 2. Do the work the caller paid for. The response is buffered so the
		// settlement receipt can still be attached as a header afterwards
		// (headers must precede the body on the wire).
		buf := &bufferedResponse{header: make(http.Header), status: http.StatusOK}
		next.ServeHTTP(buf, r)
		if buf.status >= 500 {
			buf.flush(w) // upstream failed — replay the failure, charge nothing
			return
		}

		// 3. Settle on-chain. Idempotent by (network, payer, nonce): a retry
		// with the same signed authorization returns the original receipt.
		var receipt struct {
			Success     bool   `json:"success"`
			Network     string `json:"network"`
			Transaction string `json:"transaction"`
			Payer       string `json:"payer,omitempty"`
			Asset       string `json:"asset,omitempty"`
			Amount      string `json:"amount,omitempty"` // NET to the developer
			Fee         string `json:"fee,omitempty"`
			FeeTx       string `json:"feeTransaction,omitempty"`
			ErrorReason string `json:"errorReason,omitempty"`
		}
		if err := m.post("/settle", payment, req, &receipt); err != nil {
			http.Error(w, "settlement failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if !receipt.Success {
			http.Error(w, "settlement failed: "+receipt.ErrorReason, http.StatusBadGateway)
			return
		}
		if receipt.Asset == "" {
			receipt.Asset = req.Asset
		}
		if receipt.Amount == "" {
			// Older facilitators don't echo the net amount; fall back to gross.
			receipt.Amount = req.MaxAmountRequired
		}
		out, _ := json.Marshal(receipt)
		w.Header().Set("X-PAYMENT-RESPONSE", base64.StdEncoding.EncodeToString(out))
		buf.flush(w)
	})
}

// post sends the x402 facilitator envelope to {Facilitator}{path}.
func (m *Middleware) post(path string, payment json.RawMessage, req PaymentRequirement, out any) error {
	body, err := json.Marshal(map[string]any{
		"x402Version":         1,
		"paymentPayload":      payment,
		"paymentRequirements": req,
	})
	if err != nil {
		return err
	}
	hreq, err := http.NewRequest(http.MethodPost, m.cfg.Facilitator+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)

	resp, err := m.http.Do(hreq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("facilitator %s returned %d: %s", path, resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, out)
}

// challenge writes the 402 Payment Required response with requirements.
func challenge(w http.ResponseWriter, req PaymentRequirement, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"x402Version": 1,
		"accepts":     []PaymentRequirement{req},
		"error":       errMsg,
	})
}

// bufferedResponse holds the handler's response until settlement completes.
type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header         { return b.header }
func (b *bufferedResponse) WriteHeader(code int)        { b.status = code }
func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }

func (b *bufferedResponse) flush(w http.ResponseWriter) {
	for k, vv := range b.header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(b.status)
	_, _ = b.body.WriteTo(w)
}

// ---- The actual app ----

func main() {
	cfg := Config{
		APIKey:      os.Getenv("QUA_API_KEY"),
		PayTo:       os.Getenv("QUA_PAYOUT_ADDRESS"),
		Facilitator: envOr("QUA_FACILITATOR", "https://staging.facilitator.qua.run"),
		Network:     envOr("QUA_NETWORK", "base-sepolia"),
		Asset:       envOr("QUA_ASSET", usdcBaseSepolia),
	}
	if cfg.APIKey == "" || cfg.PayTo == "" {
		log.Fatal("set QUA_API_KEY and QUA_PAYOUT_ADDRESS (see README)")
	}
	if cfg.Network == "base" && cfg.Asset == usdcBaseSepolia {
		cfg.Asset = usdcBase
	}

	pay := New(cfg)
	// 1000 base units = 0.001 USDC per call.
	http.Handle("/search", pay.Require("1000", http.HandlerFunc(searchHandler)))

	log.Println("listening on :8080 — try: curl -i localhost:8080/search")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"results": ["the answer is 42"]}`))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
