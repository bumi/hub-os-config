// Package config loads the tool's own settings from /etc/hub-os-config/config.toml
// (with sensible built-in defaults) and persists runtime state to
// /var/lib/hub-os-config/state.json.
package config

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds the tool's runtime settings. TOML field values overlay the
// defaults; any field absent from the file keeps its default.
type Config struct {
	AccessPoint  AccessPoint  `toml:"access_point"`
	Web          Web          `toml:"web"`
	Connectivity Connectivity `toml:"connectivity"`
	Paths        Paths        `toml:"paths"`
}

type AccessPoint struct {
	SSID      string `toml:"ssid"`
	Channel   int    `toml:"channel"`
	GatewayIP string `toml:"gateway_ip"`
	Interface string `toml:"interface"`
}

type Web struct {
	NormalPort int `toml:"normal_port"`
}

type Connectivity struct {
	// PrimaryURL is fetched first; any 2xx means online.
	PrimaryURL string `toml:"primary_url"`
	// FallbackURL must be a generate_204-style endpoint over plain HTTP; only an
	// exact 204 counts as online. Plain HTTP keeps it working before NTP has set
	// the clock (a Pi has no RTC, so HTTPS/TLS can fail right after boot), and the
	// exact-204 check rejects captive-portal interception (which returns 200).
	FallbackURL         string `toml:"fallback_url"`
	ProbeTimeoutSeconds int    `toml:"probe_timeout_seconds"`
	RetryWindowSeconds  int    `toml:"retry_window_seconds"`
}

type Paths struct {
	AlbyHubEnv string `toml:"albyhub_env"`
	StateFile  string `toml:"state_file"`
}

// Default returns the configuration used when no config file is present, and
// the base onto which a config file's values are overlaid.
func Default() Config {
	return Config{
		AccessPoint: AccessPoint{
			SSID:      "albyhub-setup",
			Channel:   6,
			GatewayIP: "192.168.4.1",
			Interface: "wlan0",
		},
		Web: Web{
			NormalPort: 8090,
		},
		Connectivity: Connectivity{
			PrimaryURL:          "https://getalby.com/api/internal/info",
			FallbackURL:         "http://connectivitycheck.gstatic.com/generate_204",
			ProbeTimeoutSeconds: 5,
			RetryWindowSeconds:  1800, // 30 minutes
		},
		Paths: Paths{
			AlbyHubEnv: "/opt/albyhub/.env",
			StateFile:  "/var/lib/hub-os-config/state.json",
		},
	}
}

// Load returns the configuration from path, overlaid on Default. A missing
// file yields Default with no error; invalid TOML returns an error.
func Load(path string) (Config, error) {
	c := Default()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Default(), err
	}
	return c, nil
}

// ProbeTimeout is the per-probe HTTP timeout.
func (c Config) ProbeTimeout() time.Duration {
	return time.Duration(c.Connectivity.ProbeTimeoutSeconds) * time.Second
}

// RetryWindow is how long to keep retrying connectivity, once previously
// online, before declaring the device offline and reverting to Setup Mode.
func (c Config) RetryWindow() time.Duration {
	return time.Duration(c.Connectivity.RetryWindowSeconds) * time.Second
}
