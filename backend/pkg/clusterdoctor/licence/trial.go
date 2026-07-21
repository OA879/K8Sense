package licence

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

// trialDays is the trial length. Trials are Pro-tier.
const trialDays = 14

// MachineFingerprint returns a stable, non-reversible identifier for this
// machine (SHA-256 over hostname + the first non-loopback MAC address), used
// to bind a trial licence so it can't be copied to another machine.
func MachineFingerprint() string {
	h := sha256.New()

	if host, err := os.Hostname(); err == nil {
		h.Write([]byte(host))
	}

	if mac := firstMAC(); mac != "" {
		h.Write([]byte(mac))
	}

	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func firstMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
			continue
		}

		return iface.HardwareAddr.String()
	}

	return ""
}

// GenerateTrial creates a locally-signed 14-day Pro trial licence file
// (machine-fingerprint-bound via CustomerID) and writes it to path. Returns
// the resolved Info. Non-renewable by design: callers should refuse if a
// trial for this machine already expired (enforced at the handler layer).
func GenerateTrial(path string) (Info, error) {
	priv, err := base64.StdEncoding.DecodeString(trialPrivateKeyB64)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		return Info{}, fmt.Errorf("bundled trial key is invalid")
	}

	now := time.Now()

	payload := Payload{
		CustomerID:   "trial-" + MachineFingerprint(),
		CustomerName: "Trial",
		Tier:         TierPro,
		MaxClusters:  20, //nolint:mnd // trial mirrors Pro cluster limit
		SeatCount:    1,
		IssuedAt:     now.Format("2006-01-02"),
		ExpiresAt:    now.AddDate(0, 0, trialDays).Format("2006-01-02"),
		IsTrial:      true,
	}

	msg, err := payload.canonicalBytes()
	if err != nil {
		return Info{}, fmt.Errorf("canonicalizing trial: %w", err)
	}

	sig := ed25519.Sign(ed25519.PrivateKey(priv), msg)

	file := File{Payload: payload, Signature: base64.StdEncoding.EncodeToString(sig)}

	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return Info{}, fmt.Errorf("marshaling trial: %w", err)
	}

	if err := os.WriteFile(path, out, 0o600); err != nil { //nolint:mnd
		return Info{}, fmt.Errorf("writing trial licence: %w", err)
	}

	return verify(out)
}
