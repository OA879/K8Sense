// Package licence implements K8sense's fully-offline licence enforcement:
// an Ed25519-signed licence file is verified against a bundled public key at
// startup, unlocking Pro/Enterprise features. No network call is ever made —
// it works air-gapped, which is a hard requirement for the target
// (bank / regulated) customers.
package licence

import (
	"encoding/json"
	"time"
)

// Tier is the licence tier. Free is the default when no valid licence is
// present.
type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// Payload is the signed portion of a licence file — every field except the
// signature. It is marshaled with encoding/json (deterministic field order)
// to produce the exact bytes that are signed and verified, so the signer
// (scripts/sign-licence) and this verifier must serialize it identically.
type Payload struct {
	CustomerID   string `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	Tier         Tier   `json:"tier"`
	MaxClusters  int    `json:"max_clusters"`
	SeatCount    int    `json:"seat_count"`
	IssuedAt     string `json:"issued_at"`  // YYYY-MM-DD
	ExpiresAt    string `json:"expires_at"` // YYYY-MM-DD
	IsTrial      bool   `json:"is_trial,omitempty"`
}

// File is the on-disk licence: the signed Payload plus a base64 signature.
type File struct {
	Payload
	Signature string `json:"signature"`
}

// Info is the resolved, runtime view of the current licence, returned by
// Validate and exposed to the frontend. Valid indicates the signature and
// expiry both checked out; an invalid/missing licence still returns Info with
// Tier=Free so callers can treat it uniformly.
type Info struct {
	Tier         Tier   `json:"tier"`
	CustomerName string `json:"customerName,omitempty"`
	MaxClusters  int    `json:"maxClusters"`
	SeatCount    int    `json:"seatCount"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
	IsTrial      bool   `json:"isTrial"`
	Valid        bool   `json:"valid"`
	InGrace      bool   `json:"inGrace"`
	Message      string `json:"message,omitempty"`
}

// FreeInfo is the default licence state: Free tier, one cluster, not valid
// (nothing to validate).
func FreeInfo(message string) Info {
	return Info{Tier: TierFree, MaxClusters: 1, SeatCount: 1, Message: message}
}

// canonicalBytes returns the exact bytes that are signed/verified for p.
func (p Payload) canonicalBytes() ([]byte, error) {
	return json.Marshal(p)
}

// parseFile unmarshals a licence file's JSON.
func parseFile(data []byte) (File, error) {
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, errValidation("licence file is not valid JSON")
	}

	if file.Signature == "" {
		return File{}, errValidation("licence file has no signature")
	}

	return file, nil
}

type validationError struct{ msg string }

func (e validationError) Error() string { return e.msg }

func errValidation(msg string) error { return validationError{msg: msg} }

// parseDate parses a YYYY-MM-DD licence date.
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
