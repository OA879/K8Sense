// Command sign-licence produces an Ed25519-signed K8sense customer licence
// file. It runs on K8sense's SIGNING infrastructure only — the private key it
// reads must never ship with the app. Typical use (after a Stripe payment):
//
//	K8SENSE_LICENCE_PRIVATE_KEY=<base64> go run ./scripts/sign-licence \
//	  -customer-id cust_abc -customer-name "Abbey Mortgage Bank Plc" \
//	  -tier pro -max-clusters 20 -seats 5 -expires 2027-07-20 \
//	  -out abbey.k8sense-licence
//
// The signed bytes must be byte-identical to what the app verifies, so this
// reuses encoding/json field-order serialization exactly as the validator does.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type payload struct {
	CustomerID   string `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	Tier         string `json:"tier"`
	MaxClusters  int    `json:"max_clusters"`
	SeatCount    int    `json:"seat_count"`
	IssuedAt     string `json:"issued_at"`
	ExpiresAt    string `json:"expires_at"`
	IsTrial      bool   `json:"is_trial,omitempty"`
}

type licenceFile struct {
	payload
	Signature string `json:"signature"`
}

func main() {
	customerID := flag.String("customer-id", "", "customer id")
	customerName := flag.String("customer-name", "", "customer name")
	tier := flag.String("tier", "pro", "tier: pro | enterprise")
	maxClusters := flag.Int("max-clusters", 20, "max clusters")
	seats := flag.Int("seats", 1, "seat count")
	expires := flag.String("expires", "", "expiry date YYYY-MM-DD")
	out := flag.String("out", "licence.k8sense-licence", "output path")
	flag.Parse()

	privB64 := os.Getenv("K8SENSE_LICENCE_PRIVATE_KEY")
	if privB64 == "" {
		fatal("K8SENSE_LICENCE_PRIVATE_KEY env var is required (base64 Ed25519 private key)")
	}

	priv, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		fatal("private key is not a valid base64 Ed25519 key")
	}

	if *customerID == "" || *expires == "" {
		fatal("-customer-id and -expires are required")
	}

	if _, err := time.Parse("2006-01-02", *expires); err != nil {
		fatal("-expires must be YYYY-MM-DD")
	}

	p := payload{
		CustomerID:   *customerID,
		CustomerName: *customerName,
		Tier:         *tier,
		MaxClusters:  *maxClusters,
		SeatCount:    *seats,
		IssuedAt:     time.Now().Format("2006-01-02"),
		ExpiresAt:    *expires,
	}

	msg, err := json.Marshal(p)
	if err != nil {
		fatal("marshaling payload: " + err.Error())
	}

	sig := ed25519.Sign(ed25519.PrivateKey(priv), msg)

	file := licenceFile{payload: p, Signature: base64.StdEncoding.EncodeToString(sig)}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		fatal("marshaling licence: " + err.Error())
	}

	if err := os.WriteFile(*out, data, 0o600); err != nil { //nolint:mnd
		fatal("writing licence: " + err.Error())
	}

	fmt.Printf("wrote %s (tier=%s, expires=%s)\n", *out, *tier, *expires)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "sign-licence: "+msg)
	os.Exit(1)
}
