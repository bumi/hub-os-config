// Package captive manages the NetworkManager dnsmasq drop-in that resolves
// every DNS query to the setup gateway while the access point is up. Combined
// with the web server redirecting unknown paths to the portal, this makes
// phones and laptops auto-open the "sign in to network" page on connect.
package captive

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDropInPath is where NetworkManager reads extra dnsmasq config for
// "shared" (hotspot) connections.
const DefaultDropInPath = "/etc/NetworkManager/dnsmasq-shared.d/captive.conf"

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
