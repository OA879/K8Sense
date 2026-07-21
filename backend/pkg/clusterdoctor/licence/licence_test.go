package licence

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// licenceTestPrivateKey is the dev signing key matching licencePublicKeyB64 in
// public_key.go. Present only in this test to exercise the real verify path;
// the app binary never contains it.
const licenceTestPrivateKey = "VvUwfYsdlw6GDKI2LZk17/q1iZ0uMi7Aw8i79xrfFzfB6BA7m+Npu4vh9uwJxVMTUO0pOtfJg5b75Bw4t7xJJQ=="

func signPro(t *testing.T, expires string) []byte {
	t.Helper()

	priv, err := base64.StdEncoding.DecodeString(licenceTestPrivateKey)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}

	p := Payload{
		CustomerID: "cust_x", CustomerName: "X Bank", Tier: TierPro,
		MaxClusters: 20, SeatCount: 5,
		IssuedAt: "2026-01-01", ExpiresAt: expires,
	}
	msg, _ := p.canonicalBytes()
	sig := ed25519.Sign(ed25519.PrivateKey(priv), msg)
	data, _ := json.Marshal(File{Payload: p, Signature: base64.StdEncoding.EncodeToString(sig)})

	return data
}

func TestValidateMissingFileIsFree(t *testing.T) {
	info := Validate(filepath.Join(t.TempDir(), "nope.licence"))
	if info.Tier != TierFree || info.Valid {
		t.Fatalf("expected Free/invalid, got %+v", info)
	}
}

func TestValidateSignedProLicence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.licence")
	if err := os.WriteFile(path, signPro(t, "2035-01-01"), 0o600); err != nil {
		t.Fatal(err)
	}

	info := Validate(path)
	if !info.Valid || info.Tier != TierPro || info.CustomerName != "X Bank" {
		t.Fatalf("expected valid Pro, got %+v", info)
	}
}

func TestTamperedLicenceRejected(t *testing.T) {
	data := signPro(t, "2035-01-01")

	var f File
	_ = json.Unmarshal(data, &f)
	f.CustomerName = "Tampered Bank" // change a signed field, keep old signature
	tampered, _ := json.Marshal(f)

	path := filepath.Join(t.TempDir(), "t.licence")
	_ = os.WriteFile(path, tampered, 0o600)

	if info := Validate(path); info.Valid {
		t.Fatalf("tampered licence must not validate, got %+v", info)
	}
}

func TestExpiredBeyondGraceDowngrades(t *testing.T) {
	past := time.Now().AddDate(0, 0, -graceDays-2).Format("2006-01-02")
	path := filepath.Join(t.TempDir(), "e.licence")
	_ = os.WriteFile(path, signPro(t, past), 0o600)

	info := Validate(path)
	if info.Tier != TierFree || info.Valid {
		t.Fatalf("expired-beyond-grace should downgrade to Free, got %+v", info)
	}
}

func TestGenerateTrialIsValidPro(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trial.licence")

	info, err := GenerateTrial(path)
	if err != nil {
		t.Fatalf("GenerateTrial: %v", err)
	}

	if !info.Valid || info.Tier != TierPro || !info.IsTrial {
		t.Fatalf("expected valid Pro trial, got %+v", info)
	}

	// And it must validate when read back from disk.
	if reloaded := Validate(path); !reloaded.Valid || !reloaded.IsTrial {
		t.Fatalf("reloaded trial invalid: %+v", reloaded)
	}
}
