package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State is persisted runtime state, surviving reboots so the captive portal
// can report what happened on the last WiFi attempt.
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

// LoadState reads state from path. A missing file yields a zero State with no
// error.
func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	return s, nil
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
