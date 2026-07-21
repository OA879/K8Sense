package clusterdoctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxLogoBytes caps an uploaded white-label logo. Logos are stored as data
// URIs and shipped to every page load, so a large one would bloat the app.
const maxLogoBytes = 512 * 1024

// Branding is an enterprise customer's white-label configuration. Zero values
// mean "use K8sense defaults", so an absent config file is a valid state.
type Branding struct {
	ProductName  string `json:"productName,omitempty"`
	PrimaryColor string `json:"primaryColor,omitempty"`
	// LogoDataURI is a full `data:image/...;base64,...` string so the logo
	// needs no separate asset request and works air-gapped.
	LogoDataURI  string `json:"logoDataUri,omitempty"`
	HidePoweredBy bool  `json:"hidePoweredBy,omitempty"`
}

// DefaultProductName is used whenever branding doesn't override it.
const DefaultProductName = "K8sense"

// Name returns the product name to display, falling back to the default.
func (b Branding) Name() string {
	if strings.TrimSpace(b.ProductName) == "" {
		return DefaultProductName
	}

	return b.ProductName
}

// LoadBranding reads the branding config from path. A missing file is not an
// error — it yields zero-value Branding (i.e. stock K8sense).
func LoadBranding(path string) (Branding, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Branding{}, nil
	}

	if err != nil {
		return Branding{}, fmt.Errorf("reading branding config: %w", err)
	}

	var b Branding
	if err := json.Unmarshal(data, &b); err != nil {
		return Branding{}, fmt.Errorf("parsing branding config: %w", err)
	}

	return b, nil
}

// SaveBranding validates and writes the branding config to path.
func SaveBranding(path string, b Branding) error {
	if err := b.Validate(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating branding directory: %w", err)
	}

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding branding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing branding config: %w", err)
	}

	return nil
}

// Validate rejects configs that would break rendering: an oversized logo, a
// non-data-URI logo (which would trigger an external request and break
// air-gapped installs), or a malformed colour.
func (b Branding) Validate() error {
	if b.LogoDataURI != "" {
		if !strings.HasPrefix(b.LogoDataURI, "data:image/") {
			return fmt.Errorf("logo must be a data:image/... URI so it works offline")
		}

		if len(b.LogoDataURI) > maxLogoBytes {
			return fmt.Errorf("logo exceeds %d KB limit", maxLogoBytes/1024)
		}
	}

	if b.PrimaryColor != "" && !isHexColor(b.PrimaryColor) {
		return fmt.Errorf("primaryColor must be a hex colour like #3B82F6")
	}

	return nil
}

func isHexColor(s string) bool {
	if len(s) != 4 && len(s) != 7 {
		return false
	}

	if s[0] != '#' {
		return false
	}

	for _, c := range s[1:] {
		isDigit := c >= '0' && c <= '9'
		isLower := c >= 'a' && c <= 'f'
		isUpper := c >= 'A' && c <= 'F'

		if !isDigit && !isLower && !isUpper {
			return false
		}
	}

	return true
}
