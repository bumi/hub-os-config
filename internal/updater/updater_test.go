package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateReplacesExecutable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("NEW-BINARY-CONTENT"))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "hub-os-config")
	writeFile(t, target, "OLD-BINARY")

	if err := Update(context.Background(), http.DefaultClient, srv.URL, target); err != nil {
		t.Fatalf("Update: %v", err)
	}

	data, _ := os.ReadFile(target)
	if string(data) != "NEW-BINARY-CONTENT" {
		t.Fatalf("target not replaced: %q", data)
	}
	fi, _ := os.Stat(target)
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("target mode = %v; want 0755", fi.Mode().Perm())
	}
}

func TestUpdateCreatesTargetWhenAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("BINARY"))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "hub-os-config")
	if err := Update(context.Background(), http.DefaultClient, srv.URL, target); err != nil {
		t.Fatalf("Update: %v", err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "BINARY" {
		t.Fatalf("target content = %q", data)
	}
}

func TestUpdateFailsOnNon2xxAndLeavesOriginal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "hub-os-config")
	writeFile(t, target, "OLD-BINARY")

	if err := Update(context.Background(), http.DefaultClient, srv.URL, target); err == nil {
		t.Fatal("expected error on 404")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "OLD-BINARY" {
		t.Errorf("original must be untouched on failure, got %q", data)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected no temp leftover, found %d entries", len(entries))
	}
}

func TestUpdateFailsOnUnreachableServer(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	target := filepath.Join(t.TempDir(), "hub-os-config")
	writeFile(t, target, "OLD-BINARY")

	if err := Update(context.Background(), http.DefaultClient, deadURL, target); err == nil {
		t.Fatal("expected error for unreachable server")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "OLD-BINARY" {
		t.Errorf("original must be untouched, got %q", data)
	}
}

func TestUpdateLeavesNoTempLeftoverOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("BINARY"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "hub-os-config")
	if err := Update(context.Background(), http.DefaultClient, srv.URL, target); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected only the target file, found %d entries", len(entries))
	}
}
