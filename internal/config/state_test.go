package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveStateWritesJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	in := State{
		LastMode: "normal",
		LastAttempt: &Attempt{
			SSID:   "HomeWiFi",
			Result: "failed",
			Reason: "authentication failure",
		},
	}
	if err := SaveState(path, in); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	var out State
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("state file is not valid JSON: %v", err)
	}
	if out.LastMode != "normal" {
		t.Errorf("LastMode = %q; want normal", out.LastMode)
	}
	if out.LastAttempt == nil || out.LastAttempt.SSID != "HomeWiFi" ||
		out.LastAttempt.Result != "failed" || out.LastAttempt.Reason != "authentication failure" {
		t.Errorf("LastAttempt wrong: %+v", out.LastAttempt)
	}
}

func TestSaveStateLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := SaveState(path, State{LastMode: "setup"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only the state file, found %d entries", len(entries))
	}
}
