package netmgr

import (
	"context"
	"errors"
	"strings"
	"testing"
)

var errRun = errors.New("nmcli failed")

// fakeRunner records the commands it is asked to run and returns canned output.
type fakeRunner struct {
	output string
	err    error
	calls  [][]string
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.output, f.err
}

func (f *fakeRunner) lastCall() []string {
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakeRunner) calledWith(substr string) bool {
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), substr) {
			return true
		}
	}
	return false
}

func testNM(r runner) *NM {
	return &NM{
		run: r,
		ap: APConfig{
			Interface:   "wlan0",
			HotspotName: "hub-os-config-ap",
			SSID:        "albyhub-setup",
			Channel:     6,
			GatewayIP:   "192.168.4.1",
		},
	}
}

// --- pure parsing ---

func TestParseScanList(t *testing.T) {
	out := strings.Join([]string{
		"HomeWiFi:90:WPA2",
		"HomeWiFi:60:WPA2",          // duplicate, lower signal
		`Cafe\:Guest:75:`,           // open network, escaped colon in SSID
		"SecureCorp:80:WPA2 802.1X", // enterprise -> filtered
		":50:WPA2",                  // hidden (empty SSID) -> skipped
		"OpenSpot:40:",
	}, "\n")

	got := parseScanList(out)

	if len(got) != 3 {
		t.Fatalf("expected 3 networks, got %d: %+v", len(got), got)
	}
	// Sorted by signal descending.
	if got[0].SSID != "HomeWiFi" || got[0].Signal != 90 || !got[0].Secured {
		t.Errorf("got[0] = %+v; want HomeWiFi/90/secured", got[0])
	}
	if got[1].SSID != "Cafe:Guest" || got[1].Signal != 75 || got[1].Secured {
		t.Errorf("got[1] = %+v; want Cafe:Guest/75/open", got[1])
	}
	if got[2].SSID != "OpenSpot" || got[2].Secured {
		t.Errorf("got[2] = %+v; want OpenSpot/open", got[2])
	}
}

func TestHasConfiguredWiFi(t *testing.T) {
	out := strings.Join([]string{
		"HomeWiFi:802-11-wireless",
		"Wired connection 1:802-3-ethernet",
		"hub-os-config-ap:802-11-wireless",
	}, "\n")
	if !hasConfiguredWiFi(out, "hub-os-config-ap") {
		t.Error("expected configured WiFi (HomeWiFi) to be detected")
	}

	onlyOurs := strings.Join([]string{
		"Wired connection 1:802-3-ethernet",
		"hub-os-config-ap:802-11-wireless",
	}, "\n")
	if hasConfiguredWiFi(onlyOurs, "hub-os-config-ap") {
		t.Error("hotspot + ethernet only should not count as configured WiFi")
	}
}

func TestParseActiveSSID(t *testing.T) {
	out := "no:OtherNet\nyes:HomeWiFi\nno:Cafe"
	if got := parseActiveSSID(out); got != "HomeWiFi" {
		t.Errorf("parseActiveSSID = %q; want HomeWiFi", got)
	}
	if got := parseActiveSSID("no:OtherNet\nno:Cafe"); got != "" {
		t.Errorf("parseActiveSSID with none active = %q; want empty", got)
	}
}

// --- methods drive nmcli via the runner ---

func TestScanNetworksInvokesNmcliAndParses(t *testing.T) {
	r := &fakeRunner{output: "HomeWiFi:88:WPA2\nOpenSpot:30:"}
	nm := testNM(r)

	nets, err := nm.ScanNetworks(context.Background())
	if err != nil {
		t.Fatalf("ScanNetworks: %v", err)
	}
	if len(nets) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(nets))
	}
	cmd := strings.Join(r.lastCall(), " ")
	if !strings.Contains(cmd, "-t") || !strings.Contains(cmd, "SSID,SIGNAL,SECURITY") ||
		!strings.Contains(cmd, "dev wifi list") {
		t.Errorf("unexpected scan command: %q", cmd)
	}
}

func TestSaveAndConnectSecuredCreatesProfileWithPSK(t *testing.T) {
	r := &fakeRunner{}
	nm := testNM(r)

	if err := nm.SaveAndConnect(context.Background(), "HomeWiFi", "supersecret"); err != nil {
		t.Fatalf("SaveAndConnect: %v", err)
	}
	if !r.calledWith("connection add") {
		t.Error("expected a 'connection add'")
	}
	if !r.calledWith("HomeWiFi") {
		t.Error("expected the SSID in the command")
	}
	if !r.calledWith("supersecret") {
		t.Error("expected the PSK in the command")
	}
	if !r.calledWith("wpa-psk") {
		t.Error("expected wpa-psk key management for a secured network")
	}
}

func TestSaveAndConnectOpenNetworkOmitsSecurity(t *testing.T) {
	r := &fakeRunner{}
	nm := testNM(r)

	if err := nm.SaveAndConnect(context.Background(), "OpenSpot", ""); err != nil {
		t.Fatalf("SaveAndConnect: %v", err)
	}
	if r.calledWith("wpa-psk") || r.calledWith("wifi-sec") {
		t.Error("open network must not set any wifi security")
	}
}

func TestStartHotspotConfiguresOpenApSharedMode(t *testing.T) {
	r := &fakeRunner{}
	nm := testNM(r)

	if err := nm.StartHotspot(context.Background()); err != nil {
		t.Fatalf("StartHotspot: %v", err)
	}
	if !r.calledWith("albyhub-setup") {
		t.Error("expected AP SSID in commands")
	}
	if !r.calledWith("ipv4.method shared") {
		t.Error("expected shared ipv4 method (NM runs DHCP/DNS)")
	}
	if !r.calledWith("192.168.4.1") {
		t.Error("expected gateway IP")
	}
	if !r.calledWith("mode ap") {
		t.Error("expected AP mode")
	}
	// An open AP must not carry a PSK.
	if r.calledWith("wpa-psk") {
		t.Error("setup AP must be open (no wpa-psk)")
	}
}

func TestStopHotspotBringsConnectionDown(t *testing.T) {
	r := &fakeRunner{}
	nm := testNM(r)

	if err := nm.StopHotspot(context.Background()); err != nil {
		t.Fatalf("StopHotspot: %v", err)
	}
	if !r.calledWith("hub-os-config-ap") {
		t.Error("expected the hotspot connection name in stop commands")
	}
}

func TestConnectActivatesProfile(t *testing.T) {
	r := &fakeRunner{}
	nm := testNM(r)

	if err := nm.Connect(context.Background(), "HomeWiFi"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !r.calledWith("connection up") || !r.calledWith("HomeWiFi") {
		t.Errorf("unexpected connect command: %v", r.calls)
	}
}

func TestConnectReturnsErrorOnFailure(t *testing.T) {
	r := &fakeRunner{err: errRun}
	nm := testNM(r)

	if err := nm.Connect(context.Background(), "HomeWiFi"); err == nil {
		t.Fatal("expected an error when activation fails (e.g. wrong password)")
	}
}

func TestDeleteConnectionRemovesProfile(t *testing.T) {
	r := &fakeRunner{}
	nm := testNM(r)

	if err := nm.DeleteConnection(context.Background(), "HomeWiFi"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}
	if !r.calledWith("connection delete") || !r.calledWith("HomeWiFi") {
		t.Errorf("unexpected delete command: %v", r.calls)
	}
}
