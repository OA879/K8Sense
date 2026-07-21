package clusterdoctor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

func TestRoleAtLeast(t *testing.T) {
	t.Parallel()

	cases := []struct {
		have     clusterdoctor.Role
		required clusterdoctor.Role
		want     bool
	}{
		{clusterdoctor.RoleViewer, clusterdoctor.RoleViewer, true},
		{clusterdoctor.RoleViewer, clusterdoctor.RoleOperator, false},
		{clusterdoctor.RoleViewer, clusterdoctor.RoleAdmin, false},
		{clusterdoctor.RoleOperator, clusterdoctor.RoleViewer, true},
		{clusterdoctor.RoleOperator, clusterdoctor.RoleOperator, true},
		{clusterdoctor.RoleOperator, clusterdoctor.RoleAdmin, false},
		{clusterdoctor.RoleAdmin, clusterdoctor.RoleAdmin, true},
		{clusterdoctor.RoleAdmin, clusterdoctor.RoleViewer, true},
		// An unknown role must never satisfy a requirement.
		{clusterdoctor.Role("root"), clusterdoctor.RoleViewer, false},
	}

	for _, tc := range cases {
		if got := tc.have.AtLeast(tc.required); got != tc.want {
			t.Errorf("Role(%q).AtLeast(%q) = %v, want %v", tc.have, tc.required, got, tc.want)
		}
	}
}

func TestLoadRoleDefaultsWhenAbsent(t *testing.T) {
	t.Parallel()

	role, err := clusterdoctor.LoadRole(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("LoadRole on missing file: %v", err)
	}

	if role != clusterdoctor.DefaultRole {
		t.Errorf("got %q, want default %q", role, clusterdoctor.DefaultRole)
	}
}

func TestLoadRoleRejectsUnknownRole(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "role.json")
	if err := os.WriteFile(path, []byte(`{"role":"superuser"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	role, err := clusterdoctor.LoadRole(path)
	if err != nil {
		t.Fatalf("LoadRole: %v", err)
	}

	// An unrecognised role must fall back to the default, never be honoured.
	if role != clusterdoctor.DefaultRole {
		t.Errorf("got %q, want default %q", role, clusterdoctor.DefaultRole)
	}
}

func TestSaveAndLoadRoleRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "role.json")
	if err := clusterdoctor.SaveRole(path, clusterdoctor.RoleViewer); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}

	role, err := clusterdoctor.LoadRole(path)
	if err != nil {
		t.Fatalf("LoadRole: %v", err)
	}

	if role != clusterdoctor.RoleViewer {
		t.Errorf("got %q, want viewer", role)
	}
}

func TestSaveRoleRejectsInvalid(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "role.json")
	if err := clusterdoctor.SaveRole(path, clusterdoctor.Role("wheel")); err == nil {
		t.Error("expected an error saving an unknown role")
	}
}

func TestBrandingValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		b       clusterdoctor.Branding
		wantErr bool
	}{
		{"empty is valid", clusterdoctor.Branding{}, false},
		{"good hex", clusterdoctor.Branding{PrimaryColor: "#3B82F6"}, false},
		{"short hex", clusterdoctor.Branding{PrimaryColor: "#abc"}, false},
		{"bad hex", clusterdoctor.Branding{PrimaryColor: "blue"}, true},
		{"hex without hash", clusterdoctor.Branding{PrimaryColor: "3B82F6"}, true},
		{"data uri logo", clusterdoctor.Branding{LogoDataURI: "data:image/png;base64,AAAA"}, false},
		// A remote logo URL would break air-gapped installs.
		{"remote logo rejected", clusterdoctor.Branding{LogoDataURI: "https://cdn.example.com/l.png"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.b.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestBrandingOversizedLogoRejected(t *testing.T) {
	t.Parallel()

	big := "data:image/png;base64," + string(make([]byte, 600*1024))

	if err := (clusterdoctor.Branding{LogoDataURI: big}).Validate(); err == nil {
		t.Error("expected an error for an oversized logo")
	}
}

func TestBrandingNameFallsBackToDefault(t *testing.T) {
	t.Parallel()

	if got := (clusterdoctor.Branding{}).Name(); got != clusterdoctor.DefaultProductName {
		t.Errorf("got %q, want %q", got, clusterdoctor.DefaultProductName)
	}

	if got := (clusterdoctor.Branding{ProductName: "  "}).Name(); got != clusterdoctor.DefaultProductName {
		t.Errorf("whitespace name should fall back, got %q", got)
	}

	if got := (clusterdoctor.Branding{ProductName: "AcmeOps"}).Name(); got != "AcmeOps" {
		t.Errorf("got %q, want AcmeOps", got)
	}
}

func TestLoadBrandingMissingFileIsNotAnError(t *testing.T) {
	t.Parallel()

	b, err := clusterdoctor.LoadBranding(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadBranding on missing file: %v", err)
	}

	if b.Name() != clusterdoctor.DefaultProductName {
		t.Errorf("expected stock branding, got %q", b.Name())
	}
}
