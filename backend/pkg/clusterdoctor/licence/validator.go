package licence

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"
)

// graceDays is how long an expired licence keeps Pro features before locking,
// per K8SENSE_CONTEXT.md ("14-day grace period before features lock").
const graceDays = 14

// Validate reads and verifies the licence file at path. A missing file is not
// an error — it returns Free tier silently (so users who haven't activated
// aren't nagged). A present-but-invalid file returns Free with a message.
func Validate(path string) Info {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return FreeInfo("")
	}

	if err != nil {
		return FreeInfo("Could not read licence file")
	}

	info, err := verify(data)
	if err != nil {
		return FreeInfo(err.Error())
	}

	return info
}

// verify parses licence JSON, checks the signature against the appropriate
// bundled public key, and evaluates expiry + grace period.
func verify(data []byte) (Info, error) {
	file, err := parseFile(data)
	if err != nil {
		return Info{}, err
	}

	pubB64 := licencePublicKeyB64
	if file.IsTrial {
		pubB64 = trialPublicKeyB64
	}

	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return Info{}, errors.New("bundled public key is invalid")
	}

	sig, err := base64.StdEncoding.DecodeString(file.Signature)
	if err != nil {
		return Info{}, errors.New("licence signature is malformed")
	}

	msg, err := file.Payload.canonicalBytes()
	if err != nil {
		return Info{}, errors.New("could not canonicalize licence")
	}

	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		return Info{}, errors.New("licence signature does not verify")
	}

	return evaluateExpiry(file)
}

// evaluateExpiry turns a signature-valid licence into runtime Info, applying
// the grace window. An expired-but-in-grace licence stays on its paid tier
// with InGrace=true; past grace it downgrades to Free.
func evaluateExpiry(file File) (Info, error) {
	expires, err := parseDate(file.ExpiresAt)
	if err != nil {
		return Info{}, errors.New("licence expiry date is invalid")
	}

	info := Info{
		Tier:         file.Tier,
		CustomerName: file.CustomerName,
		MaxClusters:  file.MaxClusters,
		SeatCount:    file.SeatCount,
		ExpiresAt:    file.ExpiresAt,
		IsTrial:      file.IsTrial,
		Valid:        true,
	}

	now := time.Now()
	graceEnd := expires.AddDate(0, 0, graceDays)

	switch {
	case now.Before(expires.AddDate(0, 0, 1)): // valid through the expiry day
		return info, nil
	case now.Before(graceEnd):
		info.InGrace = true
		info.Message = fmt.Sprintf("Licence expired on %s — %d-day grace period active", file.ExpiresAt, graceDays)

		return info, nil
	default:
		downgraded := FreeInfo(fmt.Sprintf("Licence expired on %s; grace period ended", file.ExpiresAt))
		downgraded.CustomerName = file.CustomerName

		return downgraded, nil
	}
}
