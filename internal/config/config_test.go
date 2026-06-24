package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultValues(t *testing.T) {
	c := Default()

	if c.AccessPoint.SSID != "albyhub-setup" {
		t.Errorf("default SSID = %q; want albyhub-setup", c.AccessPoint.SSID)
	}
	if c.AccessPoint.Channel != 6 {
		t.Errorf("default channel = %d; want 6", c.AccessPoint.Channel)
	}
	if c.AccessPoint.GatewayIP != "192.168.4.1" {
		t.Errorf("default gateway = %q; want 192.168.4.1", c.AccessPoint.GatewayIP)
	}
	if c.AccessPoint.Interface != "wlan0" {
		t.Errorf("default interface = %q; want wlan0", c.AccessPoint.Interface)
	}
	if c.Web.NormalPort != 8090 {
		t.Errorf("default normal port = %d; want 8090", c.Web.NormalPort)
	}
	if c.Connectivity.PrimaryURL != "https://getalby.com/api/internal/info" {
		t.Errorf("default primary URL = %q", c.Connectivity.PrimaryURL)
	}
	if c.Connectivity.FallbackURL != "http://connectivitycheck.gstatic.com/generate_204" {
		t.Errorf("default fallback URL = %q", c.Connectivity.FallbackURL)
	}
	if c.ProbeTimeout() != 5*time.Second {
		t.Errorf("default probe timeout = %v; want 5s", c.ProbeTimeout())
	}
	if c.RetryWindow() != 30*time.Minute {
		t.Errorf("default retry window = %v; want 30m", c.RetryWindow())
	}
	if c.Paths.AlbyHubEnv != "/opt/albyhub/.env" {
		t.Errorf("default albyhub env path = %q", c.Paths.AlbyHubEnv)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("Load missing file errored: %v", err)
	}
	if c.AccessPoint.SSID != "albyhub-setup" || c.Web.NormalPort != 8090 {
		t.Fatalf("missing-file load did not return defaults: %+v", c)
	}
}

func TestLoadOverridesOnlySpecifiedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `
[access_point]
ssid = "my-device-setup"
channel = 11

[web]
normal_port = 9000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.AccessPoint.SSID != "my-device-setup" {
		t.Errorf("SSID not overridden: %q", c.AccessPoint.SSID)
	}
	if c.AccessPoint.Channel != 11 {
		t.Errorf("channel not overridden: %d", c.AccessPoint.Channel)
	}
	if c.Web.NormalPort != 9000 {
		t.Errorf("port not overridden: %d", c.Web.NormalPort)
	}
	// Unspecified fields keep their defaults.
	if c.AccessPoint.GatewayIP != "192.168.4.1" {
		t.Errorf("gateway should keep default, got %q", c.AccessPoint.GatewayIP)
	}
	if c.Connectivity.PrimaryURL != "https://getalby.com/api/internal/info" {
		t.Errorf("primary URL should keep default, got %q", c.Connectivity.PrimaryURL)
	}
}

func TestLoadInvalidTomlErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("this is = = not valid toml ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}
