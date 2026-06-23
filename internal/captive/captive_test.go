package captive

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedirectHandlerRedirectsProbePathsToPortal(t *testing.T) {
	h := RedirectHandler("http://192.168.4.1/")

	for _, path := range ProbePaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("%s: status = %d; want 302", path, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "http://192.168.4.1/" {
			t.Errorf("%s: Location = %q; want portal URL", path, loc)
		}
	}
}

func TestRedirectHandlerRedirectsArbitraryPath(t *testing.T) {
	h := RedirectHandler("http://192.168.4.1/")
	req := httptest.NewRequest(http.MethodGet, "/anything/at/all", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d; want 302", rec.Code)
	}
	if rec.Header().Get("Location") != "http://192.168.4.1/" {
		t.Fatalf("unexpected Location: %q", rec.Header().Get("Location"))
	}
}

func TestIncludesKnownProbePaths(t *testing.T) {
	want := []string{"/generate_204", "/hotspot-detect.html", "/ncsi.txt", "/connecttest.txt"}
	for _, w := range want {
		found := false
		for _, p := range ProbePaths {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("ProbePaths missing %q", w)
		}
	}
}

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
