package captive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDNSRedirectCreatesDropIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "captive.conf")
	if err := WriteDNSRedirect(path, "192.168.4.1"); err != nil {
		t.Fatalf("WriteDNSRedirect: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading drop-in: %v", err)
	}
	if !strings.Contains(string(data), "address=/#/192.168.4.1") {
		t.Errorf("drop-in missing DNS hijack directive:\n%s", data)
	}
}

func TestWriteDNSRedirectCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dnsmasq-shared.d", "captive.conf")
	if err := WriteDNSRedirect(path, "10.0.0.1"); err != nil {
		t.Fatalf("WriteDNSRedirect with missing parent dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("drop-in not created: %v", err)
	}
}

func TestWriteDNSRedirectIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "captive.conf")
	if err := WriteDNSRedirect(path, "192.168.4.1"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if err := WriteDNSRedirect(path, "192.168.4.1"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Errorf("re-writing changed content:\n%s\nvs\n%s", first, second)
	}
}
