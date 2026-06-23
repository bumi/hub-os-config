package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getAlby/hub-os-config/internal/netmgr"
)

// --- fakes ---

type fakeNM struct {
	nets []netmgr.Network
	ssid string
}

func (f *fakeNM) ScanNetworks(context.Context) ([]netmgr.Network, error) { return f.nets, nil }
func (f *fakeNM) IsWiFiConfigured(context.Context) (bool, error)         { return false, nil }
func (f *fakeNM) SaveAndConnect(context.Context, string, string) error   { return nil }
func (f *fakeNM) Connect(context.Context, string) error                  { return nil }
func (f *fakeNM) DeleteConnection(context.Context, string) error         { return nil }
func (f *fakeNM) StartHotspot(context.Context) error                     { return nil }
func (f *fakeNM) StopHotspot(context.Context) error                      { return nil }
func (f *fakeNM) CurrentSSID(context.Context) (string, error)            { return f.ssid, nil }

type fakeEnv struct {
	current map[string]string
}

func (f *fakeEnv) Get() (map[string]string, error) { return f.current, nil }
func (f *fakeEnv) Apply(map[string]string) error   { return nil }

type saveCapture struct {
	called bool
	req    SaveRequest
}

func newTestServer(captive bool) (*Server, *saveCapture) {
	nm := &fakeNM{
		nets: []netmgr.Network{{SSID: "HomeWiFi", Signal: 88, Security: "WPA2", Secured: true}},
		ssid: "HomeWiFi",
	}
	env := &fakeEnv{current: map[string]string{"RELAY": "wss://current", "LN_BACKEND_TYPE": "LDK"}}
	cap := &saveCapture{}
	s := NewServer(Deps{
		NM:        nm,
		Env:       env,
		Status:    func() AppStatus { return AppStatus{Mode: "setup", Online: false} },
		Save:      func(req SaveRequest) { cap.called = true; cap.req = req },
		PortalURL: "http://192.168.4.1/",
		Captive:   captive,
	})
	return s, cap
}

func do(s *Server, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// --- tests ---

func TestRootServesConfigPage(t *testing.T) {
	s, _ := newTestServer(false)
	rec := do(s, http.MethodGet, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d; want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q; want text/html", ct)
	}
}

func TestNetworksEndpointReturnsScanResults(t *testing.T) {
	s, _ := newTestServer(false)
	rec := do(s, http.MethodGet, "/api/networks", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	var nets []netmgr.Network
	if err := json.Unmarshal(rec.Body.Bytes(), &nets); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(nets) != 1 || nets[0].SSID != "HomeWiFi" {
		t.Fatalf("unexpected networks: %+v", nets)
	}
}

func TestStatusEndpoint(t *testing.T) {
	s, _ := newTestServer(false)
	rec := do(s, http.MethodGet, "/api/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	var st struct {
		Mode        string `json:"mode"`
		Online      bool   `json:"online"`
		CurrentSSID string `json:"current_ssid"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if st.Mode != "setup" || st.Online {
		t.Errorf("unexpected status: %+v", st)
	}
	if st.CurrentSSID != "HomeWiFi" {
		t.Errorf("current_ssid = %q; want HomeWiFi", st.CurrentSSID)
	}
}

func TestConfigEndpointReturnsCurrentAdvanced(t *testing.T) {
	s, _ := newTestServer(false)
	rec := do(s, http.MethodGet, "/api/config", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	var adv map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &adv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if adv["RELAY"] != "wss://current" || adv["LN_BACKEND_TYPE"] != "LDK" {
		t.Errorf("unexpected advanced config: %+v", adv)
	}
}

func TestSaveWiFiHandsOffToSaver(t *testing.T) {
	s, cap := newTestServer(true)
	rec := do(s, http.MethodPost, "/api/save", `{"wifi":{"ssid":"HomeWiFi","password":"mypassword"}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !cap.called || cap.req.WiFi == nil {
		t.Fatalf("Save not called with WiFi: %+v", cap)
	}
	if cap.req.WiFi.SSID != "HomeWiFi" || cap.req.WiFi.Password != "mypassword" {
		t.Errorf("Save got %+v", cap.req.WiFi)
	}
}

func TestSaveOpenNetworkAllowsEmptyPassword(t *testing.T) {
	s, cap := newTestServer(true)
	rec := do(s, http.MethodPost, "/api/save", `{"wifi":{"ssid":"OpenSpot","password":""}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if cap.req.WiFi == nil || cap.req.WiFi.SSID != "OpenSpot" || cap.req.WiFi.Password != "" {
		t.Errorf("open save got %+v", cap.req.WiFi)
	}
}

func TestSaveAdvancedHandsOffToSaver(t *testing.T) {
	s, cap := newTestServer(true)
	body := `{"advanced":{"RELAY":"wss://relay.example.com","LN_BACKEND_TYPE":"BARK"}}`
	rec := do(s, http.MethodPost, "/api/save", body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if cap.req.Advanced["LN_BACKEND_TYPE"] != "BARK" || cap.req.Advanced["RELAY"] != "wss://relay.example.com" {
		t.Errorf("advanced not passed through: %+v", cap.req.Advanced)
	}
	if cap.req.WiFi != nil {
		t.Errorf("expected no WiFi in advanced-only save, got %+v", cap.req.WiFi)
	}
}

func TestSaveRejectsLongSSID(t *testing.T) {
	s, cap := newTestServer(true)
	long := strings.Repeat("x", 33)
	rec := do(s, http.MethodPost, "/api/save", `{"wifi":{"ssid":"`+long+`","password":"mypassword"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
	if cap.called {
		t.Error("must not hand off on validation error")
	}
}

func TestSaveRejectsShortPassword(t *testing.T) {
	s, cap := newTestServer(true)
	rec := do(s, http.MethodPost, "/api/save", `{"wifi":{"ssid":"HomeWiFi","password":"short"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
	if cap.called {
		t.Error("must not hand off on validation error")
	}
}

func TestSaveRejectsInvalidBackendType(t *testing.T) {
	s, cap := newTestServer(true)
	rec := do(s, http.MethodPost, "/api/save", `{"advanced":{"LN_BACKEND_TYPE":"FOO"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
	if cap.called {
		t.Error("must not hand off on validation error")
	}
}

func TestSaveRejectsInvalidURL(t *testing.T) {
	s, cap := newTestServer(true)
	rec := do(s, http.MethodPost, "/api/save", `{"advanced":{"RELAY":"not a url"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
	if cap.called {
		t.Error("must not hand off on validation error")
	}
}

func TestSaveRejectsEmptyRequest(t *testing.T) {
	s, cap := newTestServer(true)
	rec := do(s, http.MethodPost, "/api/save", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
	if cap.called {
		t.Error("must not hand off when nothing to save")
	}
}

func TestCaptiveModeRedirectsUnknownPaths(t *testing.T) {
	s, _ := newTestServer(true)
	rec := do(s, http.MethodGet, "/generate_204", "")
	if rec.Code != http.StatusFound {
		t.Fatalf("captive: status = %d; want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "http://192.168.4.1/" {
		t.Errorf("captive redirect Location = %q", loc)
	}
}

func TestNormalModeDoesNotRedirectUnknownPaths(t *testing.T) {
	s, _ := newTestServer(false)
	rec := do(s, http.MethodGet, "/generate_204", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("normal: status = %d; want 404", rec.Code)
	}
}
