// Package web serves the configuration UI and JSON API. The same handlers run
// in both Setup Mode (bound to the AP gateway, with captive-portal redirects)
// and Normal Mode (bound to the LAN). All external collaborators are injected
// as interfaces so the server is testable without hardware.
package web

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/getAlby/hub-os-config/internal/config"
	"github.com/getAlby/hub-os-config/internal/netmgr"
)

//go:embed static
var staticFS embed.FS

// ManagedEnvKeys are the Alby Hub .env keys the Advanced section may change.
var ManagedEnvKeys = []string{"RELAY", "LDK_ESPLORA_SERVER", "LN_BACKEND_TYPE"}

// EnvStore reads and updates the managed keys of the Alby Hub .env file.
type EnvStore interface {
	Get() (map[string]string, error)
	Apply(updates map[string]string) error
}

// Rebooter schedules a system reboot. Implementations return promptly; the
// actual reboot may be delayed so the HTTP response can be delivered first.
type Rebooter interface {
	Reboot() error
}

// AppStatus is the operational status the state machine exposes to the UI.
type AppStatus struct {
	Mode        string          `json:"mode"`
	Online      bool            `json:"online"`
	LastAttempt *config.Attempt `json:"last_attempt,omitempty"`
}

// WiFiCreds is a chosen WiFi network and password.
type WiFiCreds struct {
	SSID     string
	Password string
}

// SaveRequest is a validated configuration change handed to the Save callback.
// WiFi is nil when only advanced settings change.
type SaveRequest struct {
	WiFi     *WiFiCreds
	Advanced map[string]string
}

// Deps are the Server's injected collaborators.
type Deps struct {
	NM     netmgr.Manager
	Env    EnvStore
	Status func() AppStatus
	// Save applies a validated change. It owns testing WiFi before committing
	// and rebooting, so it may run asynchronously; the outcome is observed via
	// Status.
	Save      func(SaveRequest)
	PortalURL string // e.g. http://192.168.4.1/ (Setup Mode captive target)
	Captive   bool   // when true, redirect unknown paths to PortalURL
}

// Server serves the config UI and API.
type Server struct {
	deps Deps
	mux  *http.ServeMux
}

// NewServer builds a Server with its routes registered.
func NewServer(d Deps) *Server {
	s := &Server{deps: d, mux: http.NewServeMux()}
	s.mux.HandleFunc("/api/networks", s.handleNetworks)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/save", s.handleSave)
	s.mux.HandleFunc("/", s.handleRoot)
	return s
}

// Handler returns the HTTP handler for binding to a listener.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		// Unknown path: in Setup Mode this is an OS captive-portal probe (DNS
		// is hijacked to us) — redirect so the device opens the portal.
		if s.deps.Captive {
			http.Redirect(w, r, s.deps.PortalURL, http.StatusFound)
			return
		}
		http.NotFound(w, r)
		return
	}
	page, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "config page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(page)
}

func (s *Server) handleNetworks(w http.ResponseWriter, r *http.Request) {
	nets, err := s.deps.NM.ScanNetworks(r.Context())
	if err != nil {
		http.Error(w, "scan failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, nets)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ssid, _ := s.deps.NM.CurrentSSID(r.Context())
	writeJSON(w, http.StatusOK, struct {
		AppStatus
		CurrentSSID string `json:"current_ssid"`
	}{
		AppStatus:   s.deps.Status(),
		CurrentSSID: ssid,
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	cur, err := s.deps.Env.Get()
	if err != nil {
		http.Error(w, "reading config failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, cur)
}

type saveRequest struct {
	WiFi *struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
	} `json:"wifi"`
	Advanced map[string]string `json:"advanced"`
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req saveRequest
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // cap the request body at 64 KiB
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	advanced := filterManaged(req.Advanced)
	if req.WiFi == nil && len(advanced) == 0 {
		http.Error(w, "nothing to save", http.StatusBadRequest)
		return
	}

	if req.WiFi != nil {
		if err := validateWiFi(req.WiFi.SSID, req.WiFi.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err := validateAdvanced(advanced); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validation passed; hand off. The Save callback tests WiFi (if any) before
	// committing and rebooting, so it returns before the work completes — the UI
	// observes the result via /api/status.
	out := SaveRequest{Advanced: advanced}
	if req.WiFi != nil {
		out.WiFi = &WiFiCreds{SSID: req.WiFi.SSID, Password: req.WiFi.Password}
	}
	s.deps.Save(out)

	writeJSON(w, http.StatusOK, map[string]any{"status": "applying"})
}

func filterManaged(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := map[string]string{}
	for _, k := range ManagedEnvKeys {
		if v, ok := in[k]; ok && v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
