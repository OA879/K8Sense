package licence

// Bundled Ed25519 public keys (base64 std encoding). These are VERIFY-ONLY —
// they cannot sign, so shipping them in the binary is safe. The matching
// private keys never leave K8sense's signing infrastructure (see
// scripts/sign-licence). Regenerate BOTH pairs before any real release; the
// values here are development keys.
const (
	// licencePublicKeyB64 verifies customer licence files.
	licencePublicKeyB64 = "wegQO5vjabuL4fbsCcVTE1DtKTrXyYOW++QcOLe8SSU="

	// trialPublicKeyB64 verifies locally-generated 14-day trial licences,
	// signed with a separate key so a leaked trial key can't mint paid licences.
	trialPublicKeyB64 = "8mFkdzynpUbIX4UgIAlY4xnPZF+wVvm8VGbL25SbZk4="

	// trialPrivateKeyB64 is embedded because trials are generated and signed
	// on the user's own machine (they are machine-fingerprint-bound and
	// non-transferable, so a self-signed trial has no resale value). It is a
	// SEPARATE key from the customer-licence key, which never ships — a leaked
	// trial key can therefore only mint machine-bound trials, never paid tiers.
	trialPrivateKeyB64 = "SZbI9wwuMBk66KTsG/G0wlwLzPruXBBIFLchJIiOrrHyYWR3PKelRshfhSAgCVjjGc9kX7BW+bxUZsvblJtmTg=="
)
