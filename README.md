# hub-os-config

System configuration service for an Alby Hub Raspberry Pi appliance.

On boot it checks whether the device has working internet. If not, it brings up
an **open WiFi access point** hosting a **captive-portal** configuration website
where the user picks a WiFi network and sets a few Alby Hub options. After the
user confirms, the device reboots onto the new network. Once online, the same
config UI stays reachable on the LAN on a dedicated port.

It is a single static Go binary, built for ARM (Raspberry Pi OS / Debian with
**NetworkManager**).

## How it works

One long-lived systemd service runs a two-mode state machine:

- **Setup Mode** — no WiFi configured, or no internet. Brings up a NetworkManager
  hotspot (open AP `albyhub-setup`, gateway `192.168.4.1`), hijacks DNS, and
  serves the config UI as a captive portal on `192.168.4.1:80`.
- **Normal Mode** — WiFi connected and online. No AP. Serves the same UI on the
  LAN at `:8090` (`http://<device-ip>:8090`, or `http://albyhub.local:8090`).

A supervisor monitors internet reachability (probing `getalby.com` with a Google
fallback). If a previously-online device loses connectivity for longer than the
retry window (default 30 min), it reverts to Setup Mode; a bad WiFi change
self-heals the same way on the next boot.

WiFi credentials are stored by NetworkManager. The Advanced section edits Alby
Hub's `/opt/albyhub/.env` (`RELAY`, `LDK_ESPLORA_SERVER`, `LN_BACKEND_TYPE`),
preserving all other keys. Every save triggers a reboot to apply changes.

## Build

```bash
build/build.sh 0.1.0      # -> dist/hub-os-config-{armv6,armv7,arm64}
```

| Target | Pi models |
|--------|-----------|
| armv6  | Zero / Zero W / 1 |
| armv7  | 2 / 3 (32-bit) |
| arm64  | 3 / 4 / 5 (64-bit) |

## Run locally (Docker)

The WiFi/AP logic can't run off-device, but you can still boot the app and
exercise the real web UI and JSON API on your machine. A mock `nmcli` stands in
for NetworkManager and Setup Mode binds to all interfaces.

```bash
docker compose up --build
```

- **Setup Mode UI:** http://localhost:8080 — the first-run captive-portal flow.
- **Normal Mode UI:** http://localhost:8090 — shown after a successful connect.

The container starts in Setup Mode with five mock networks. Selecting **FailNet**
demonstrates the wrong-password path (the test fails, the portal returns with an
error); any other network "connects" and the app switches to Normal Mode. The
on-device reboot is a no-op locally. (If port 8080 is taken — e.g. by a local
Alby Hub — edit the mapping in `docker-compose.yml`.)

Test assets live in `docker/` (`mock-nmcli.sh`, `config.toml`, `albyhub.env`).

## Deployment

The OS image ships this binary and its systemd unit — there is no install step.
Reference files for image builders live in `packaging/`:

- `packaging/hub-os-config.service` → `/etc/systemd/system/` (runs `hub-os-config run`)
- `packaging/config.toml.example` → optional `/etc/hub-os-config/config.toml`

The service just runs `hub-os-config run`. It writes the captive-portal dnsmasq
drop-in itself when it enters Setup Mode, so nothing else is required.

## Updating

```bash
hub-os-config update      # downloads the replacement binary and swaps it in
systemctl restart hub-os-config   # (or reboot) to run the new version
```

The download URL is hard-coded in `cmd/hub-os-config/update.go` (`updateURL`).
The swap is atomic; the running process keeps the old binary until restart.

## Configuration

Optional `/etc/hub-os-config/config.toml` overrides defaults; see
`config.toml.example`. Runtime requirements on the device: NetworkManager (with
its bundled dnsmasq for shared-connection DHCP/DNS).

## Development

```bash
go test ./...
```

`netmgr` (nmcli), `system` (reboot/.env ownership), and the app `Controller`
(real listeners/hotspot) are host-touching glue validated on hardware; the rest
is unit-tested with fakes.

## Layout

```
cmd/hub-os-config/   entry point + subcommands + packaging assets
internal/
  app/           state machine, supervisor loop, real controller
  connectivity/  internet probe + hysteresis monitor
  netmgr/        nmcli wrappers + terse-output parsing
  captive/       portal probe redirects + dnsmasq drop-in
  web/           HTTP server + JSON API (+ embedded UI)
  envfile/       /opt/albyhub/.env read/modify/write (atomic)
  config/        config.toml + state.json
  system/        reboot + env-file store
  updater/       download + atomic self-replace
build/build.sh   cross-compile script
packaging/       systemd unit + config example (for the OS image)
```
