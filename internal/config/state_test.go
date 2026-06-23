package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStateMissingFileReturnsZeroValue(t *testing.T) {
	s, err := LoadState(filepath.Join(t.TempDir(), "nostate.json"))
	if err != nil {
		t.Fatalf("LoadState missing file errored: %v", err)
	}
	if s.LastMode != "" || s.LastAttempt != nil {
		t.Fatalf("expected zero State, got %+v", s)
	}
}

func TestSaveStateThenLoadRoundTrips(t *testing.T) {
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

	out, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if out.LastMode != "normal" {
		t.Errorf("LastMode = %q; want normal", out.LastMode)
	}
	if out.LastAttempt == nil || out.LastAttempt.SSID != "HomeWiFi" ||
		out.LastAttempt.Result != "failed" || out.LastAttempt.Reason != "authentication failure" {
		t.Errorf("LastAttempt round-trip wrong: %+v", out.LastAttempt)
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
