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
}

type Web struct {
	NormalPort int `toml:"normal_port"`
}

type Connectivity struct {
	PrimaryURL          string `toml:"primary_url"`
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
		},
		Web: Web{
			NormalPort: 8090,
		},
		Connectivity: Connectivity{
			PrimaryURL:          "https://getalby.com/api/internal/info",
			FallbackURL:         "https://www.google.com/generate_204",
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
