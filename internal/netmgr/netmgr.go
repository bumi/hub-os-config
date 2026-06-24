// Package netmgr controls WiFi and the setup access point through
// NetworkManager's nmcli CLI. NetworkManager owns the AP lifecycle, DHCP, and
// DNS (in "shared" mode), so this package only issues nmcli commands and parses
// their terse output.
package netmgr

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Network is a WiFi network discovered by a scan.
type Network struct {
	SSID     string `json:"ssid"`
	Signal   int    `json:"signal"`   // 0-100
	Security string `json:"security"` // e.g. "WPA2"; empty for open
	Secured  bool   `json:"secured"`
}

// Manager controls WiFi and the setup hotspot. It is an interface so callers
// can be tested without NetworkManager present.
type Manager interface {
	ScanNetworks(ctx context.Context) ([]Network, error)
	IsWiFiConfigured(ctx context.Context) (bool, error)
	ConnectWiFi(ctx context.Context, ssid, psk string, hidden bool) error
	DeleteConnection(ctx context.Context, ssid string) error
	StartHotspot(ctx context.Context) error
	StopHotspot(ctx context.Context) error
	CurrentSSID(ctx context.Context) (string, error)
}

// APConfig describes the setup access point.
type APConfig struct {
	Interface   string
	HotspotName string // NetworkManager connection name for our AP
	SSID        string
	Channel     int
	GatewayIP   string
}

// runner executes an external command and returns its combined output.
type runner interface {
	run(ctx context.Context, name string, args ...string) (string, error)
}

// NM is the NetworkManager-backed Manager.
type NM struct {
	run runner
	ap  APConfig
}

// New returns an NM that shells out to the real nmcli binary.
func New(ap APConfig) *NM {
	return &NM{run: execRunner{}, ap: ap}
}

func (n *NM) nmcli(ctx context.Context, args ...string) (string, error) {
	return n.run.run(ctx, "nmcli", args...)
}

// ScanNetworks returns visible WiFi networks, deduped by SSID (highest signal),
// open and personal-WPA only, sorted by signal descending.
func (n *NM) ScanNetworks(ctx context.Context) ([]Network, error) {
	out, err := n.nmcli(ctx, "-t", "-f", "SSID,SIGNAL,SECURITY", "dev", "wifi", "list")
	if err != nil {
		return nil, err
	}
	return parseScanList(out), nil
}

// IsWiFiConfigured reports whether a saved WiFi connection exists other than our
// own hotspot profile.
func (n *NM) IsWiFiConfigured(ctx context.Context) (bool, error) {
	out, err := n.nmcli(ctx, "-t", "-f", "NAME,TYPE", "connection", "show")
	if err != nil {
		return false, err
	}
	return hasConfiguredWiFi(out, n.ap.HotspotName), nil
}

// ConnectWiFi creates an autoconnecting profile for the network and activates
// it, blocking until it is connected or the context deadline elapses. It uses
// `nmcli device wifi connect`, which auto-negotiates security (open / WPA2 /
// WPA3) and enables autoconnect, so NetworkManager reconnects on the next boot.
// Set hidden for a manually-entered SSID so NM actively probes for it (needed
// for hidden networks; harmless for visible ones). An error means the
// credentials are wrong or the network is unreachable, which the caller
// surfaces before committing. The radio must be free (AP stopped).
func (n *NM) ConnectWiFi(ctx context.Context, ssid, psk string, hidden bool) error {
	args := []string{"device", "wifi", "connect", ssid}
	if psk != "" {
		args = append(args, "password", psk)
	}
	args = append(args, "ifname", n.ap.Interface)
	if hidden {
		args = append(args, "hidden", "yes")
	}
	if _, err := n.nmcli(ctx, args...); err != nil {
		return fmt.Errorf("connecting to %q: %w", ssid, err)
	}
	return nil
}

// DeleteConnection removes a saved connection profile (used to discard a
// profile whose password failed verification).
func (n *NM) DeleteConnection(ctx context.Context, ssid string) error {
	if _, err := n.nmcli(ctx, "connection", "delete", ssid); err != nil {
		return fmt.Errorf("deleting %q: %w", ssid, err)
	}
	return nil
}

// StartHotspot brings up the open setup access point in NM "shared" mode.
func (n *NM) StartHotspot(ctx context.Context) error {
	_, _ = n.nmcli(ctx, "connection", "delete", n.ap.HotspotName)

	addArgs := []string{
		"connection", "add",
		"type", "wifi",
		"ifname", n.ap.Interface,
		"con-name", n.ap.HotspotName,
		"autoconnect", "no",
		"ssid", n.ap.SSID,
		"802-11-wireless.mode", "ap",
		"802-11-wireless.band", "bg",
		"802-11-wireless.channel", strconv.Itoa(n.ap.Channel),
		"ipv4.method", "shared",
		"ipv4.addresses", n.ap.GatewayIP + "/24",
	}
	if _, err := n.nmcli(ctx, addArgs...); err != nil {
		return fmt.Errorf("creating hotspot profile: %w", err)
	}
	if _, err := n.nmcli(ctx, "connection", "up", n.ap.HotspotName); err != nil {
		return fmt.Errorf("starting hotspot: %w", err)
	}
	return nil
}

// StopHotspot tears the setup access point down. Best-effort: an absent profile
// is not an error (it may already be gone).
func (n *NM) StopHotspot(ctx context.Context) error {
	_, _ = n.nmcli(ctx, "connection", "down", n.ap.HotspotName)
	_, _ = n.nmcli(ctx, "connection", "delete", n.ap.HotspotName)
	return nil
}

// CurrentSSID returns the SSID the device is currently associated with, or "".
func (n *NM) CurrentSSID(ctx context.Context) (string, error) {
	out, err := n.nmcli(ctx, "-t", "-f", "ACTIVE,SSID", "dev", "wifi", "list")
	if err != nil {
		return "", err
	}
	return parseActiveSSID(out), nil
}

// --- pure parsing helpers ---

// splitTerse splits an nmcli `-t` line on unescaped colons, unescaping `\:`
// and `\\`.
func splitTerse(line string) []string {
	var fields []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line):
			cur.WriteByte(line[i+1])
			i++
		case c == ':':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	fields = append(fields, cur.String())
	return fields
}

func parseScanList(out string) []Network {
	bySSID := map[string]Network{}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := splitTerse(line)
		if len(f) < 3 {
			continue
		}
		ssid := f[0]
		if ssid == "" {
			continue // hidden network
		}
		security := strings.TrimSpace(f[2])
		if strings.Contains(security, "802.1X") {
			continue // enterprise networks not supported
		}
		signal, _ := strconv.Atoi(strings.TrimSpace(f[1]))
		net := Network{
			SSID:     ssid,
			Signal:   signal,
			Security: security,
			Secured:  security != "",
		}
		if existing, ok := bySSID[ssid]; !ok || signal > existing.Signal {
			bySSID[ssid] = net
		}
	}

	nets := make([]Network, 0, len(bySSID))
	for _, n := range bySSID {
		nets = append(nets, n)
	}
	sort.Slice(nets, func(i, j int) bool {
		if nets[i].Signal != nets[j].Signal {
			return nets[i].Signal > nets[j].Signal
		}
		return nets[i].SSID < nets[j].SSID
	})
	return nets
}

func hasConfiguredWiFi(out, hotspotName string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := splitTerse(line)
		if len(f) < 2 {
			continue
		}
		name, typ := f[0], f[1]
		if typ == "802-11-wireless" && name != hotspotName {
			return true
		}
	}
	return false
}

func parseActiveSSID(out string) string {
	for _, line := range strings.Split(out, "\n") {
		f := splitTerse(line)
		if len(f) >= 2 && f[0] == "yes" {
			return f[1]
		}
	}
	return ""
}
