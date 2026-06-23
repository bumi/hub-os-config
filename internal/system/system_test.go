package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvStoreApplyUpdatesManagedKeysPreservingOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "albyhub.env")
	original := "# Alby Hub\nUNRELATED=keepme\nRELAY=old\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewEnvStore(path, []string{"RELAY", "LDK_ESPLORA_SERVER", "LN_BACKEND_TYPE"})
	err := store.Apply(map[string]string{
		"RELAY":           "wss://new",
		"LN_BACKEND_TYPE": "BARK",
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	want := "# Alby Hub\nUNRELATED=keepme\nRELAY=wss://new\nLN_BACKEND_TYPE=BARK\n"
	if got != want {
		t.Fatalf("env file wrong after Apply.\n got: %q\nwant: %q", got, want)
	}
}

func TestEnvStoreGetReturnsPresentManagedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "albyhub.env")
	content := "RELAY=wss://r\nUNRELATED=x\nLN_BACKEND_TYPE=LDK\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewEnvStore(path, []string{"RELAY", "LDK_ESPLORA_SERVER", "LN_BACKEND_TYPE"})
	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got["RELAY"] != "wss://r" || got["LN_BACKEND_TYPE"] != "LDK" {
		t.Errorf("unexpected managed values: %+v", got)
	}
	if _, ok := got["UNRELATED"]; ok {
		t.Errorf("Get leaked an unmanaged key: %+v", got)
	}
	if _, ok := got["LDK_ESPLORA_SERVER"]; ok {
		t.Errorf("Get returned an absent key: %+v", got)
	}
}

func TestEnvStoreGetOnMissingFileReturnsEmpty(t *testing.T) {
	store := NewEnvStore(filepath.Join(t.TempDir(), "nope.env"), []string{"RELAY"})
	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get missing file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %+v", got)
	}
}

func TestRebooterInvokesSystemctlReboot(t *testing.T) {
	done := make(chan []string, 1)
	r := &Rebooter{
		delay: 0,
		run: func(name string, args ...string) error {
			done <- append([]string{name}, args...)
			return nil
		},
	}

	if err := r.Reboot(); err != nil {
		t.Fatalf("Reboot: %v", err)
	}

	select {
	case cmd := <-done:
		if cmd[0] != "systemctl" || cmd[1] != "reboot" {
			t.Errorf("reboot ran %v; want systemctl reboot", cmd)
		}
	case <-time.After(time.Second):
		t.Fatal("reboot command was never invoked")
	}
}
