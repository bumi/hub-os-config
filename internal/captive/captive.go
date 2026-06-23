// Package captive implements captive-portal behavior for Setup Mode:
//
//   - an HTTP handler that redirects OS connectivity-probe requests (and any
//     other hijacked-DNS request) to the config portal, so phones and laptops
//     auto-open the "sign in to network" page on connect; and
//   - management of the NetworkManager dnsmasq drop-in that resolves every DNS
//     query to the setup gateway while the access point is up.
package captive

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// DefaultDropInPath is where NetworkManager reads extra dnsmasq config for
// "shared" (hotspot) connections.
const DefaultDropInPath = "/etc/NetworkManager/dnsmasq-shared.d/captive.conf"

// ProbePaths are the URL paths major operating systems request to detect a
// captive portal.
var ProbePaths = []string{
	"/generate_204",              // Android
	"/gen_204",                   // Android
	"/hotspot-detect.html",       // Apple iOS/macOS
	"/library/test/success.html", // Apple
	"/ncsi.txt",                  // Windows NCSI
	"/connecttest.txt",           // Windows
	"/canonical.html",            // NetworkManager / GNOME / Firefox
	"/success.txt",               // Firefox
}

// RedirectHandler returns a handler that responds with a 302 redirect to
// portalURL for any request. Because DNS is hijacked to the gateway while the
// AP is up, every connectivity probe lands here and the 302 makes the client's
// captive-portal assistant open portalURL.
func RedirectHandler(portalURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, portalURL, http.StatusFound)
	}
}

const dropInTemplate = `# Installed by hub-os-config: captive-portal DNS hijack.
# Resolves all DNS queries to the setup gateway. NetworkManager applies this
# only while a "shared" (hotspot) connection is active, so it has no effect in
# Normal Mode.
address=/#/%s
`

// WriteDNSRedirect writes the dnsmasq drop-in that resolves all DNS queries to
// gatewayIP, creating the parent directory if needed. It is idempotent.
func WriteDNSRedirect(path, gatewayIP string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(dropInTemplate, gatewayIP)
	return os.WriteFile(path, []byte(content), 0o644)
}
