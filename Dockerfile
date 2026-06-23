# Local run/test image for hub-os-config (Mac/amd64/arm64).
#
# The real WiFi/AP logic can't run in a container, so a mock nmcli stands in and
# the app is configured to bind Setup Mode to all interfaces. This lets you boot
# the app and exercise the real web UI and JSON API locally.

# --- build ---
FROM golang:1.21-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=docker-dev" \
    -o /out/hub-os-config ./cmd/hub-os-config

# --- runtime (mirrors the real Debian / Raspberry Pi OS target) ---
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Mock nmcli so the app runs without a real WiFi radio.
COPY docker/mock-nmcli.sh /usr/local/bin/nmcli
RUN chmod +x /usr/local/bin/nmcli

# Test config + seeded Alby Hub .env + runtime dirs.
COPY docker/config.toml /etc/hub-os-config/config.toml
COPY docker/albyhub.env /opt/albyhub/.env
RUN mkdir -p /var/lib/hub-os-config

COPY --from=build /out/hub-os-config /usr/local/bin/hub-os-config

EXPOSE 80 8090

# On boot, run the app — the same as the systemd unit's ExecStart on the device.
CMD ["/usr/local/bin/hub-os-config", "run"]
