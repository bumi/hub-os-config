package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State is a written record of the last mode and WiFi attempt. It is persisted
// for observability/debugging on the device; the app does not read it back (the
// live status comes from in-memory state).
type State struct {
	LastMode    string   `json:"last_mode,omitempty"`
	LastAttempt *Attempt `json:"last_attempt,omitempty"`
}

// Attempt records the outcome of the most recent WiFi configuration attempt.
type Attempt struct {
	SSID   string `json:"ssid"`
	Result string `json:"result"` // "success" | "failed"
	Reason string `json:"reason,omitempty"`
}

// SaveState writes state to path atomically (temp file + rename).
func SaveState(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
