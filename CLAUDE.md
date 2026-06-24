# co2monitor — project notes

## Project structure

- `main.go` — Prometheus exporter; exposes `/metrics` on `:8080`
- `meter/meter.go` — HID device access; encryption/decryption logic
- `meter/meter_test.go` — integration tests (require physical device at `/dev/hidraw8`)
- `go.mod` / `go.sum` — Go modules (migrated from `dep`; `Gopkg.toml`/`Gopkg.lock` are obsolete)

## Build

```
go build ./...
GOOS=freebsd GOARCH=amd64 go build   # cross-compile for FreeBSD
GOOS=linux GOARCH=arm GOARM=6 go build  # Raspberry Pi
```

## Device protocol

Two protocol variants exist; the code auto-detects on first read via checksum:

- **Encrypted** (older devices, e.g. AirCO2NTROL Mini `04d9:a052`): 8-byte XOR-obfuscated
  HID reports. A random key is sent to the device via `HIDIOCSFEATURE(9)` ioctl
  (`0xc0094806`) at open time. The `decrypt()` function reverses the obfuscation.
- **Plaintext** (newer TFA Dostmann devices, e.g. AIRCO2NTROL Coach): raw HID bytes are
  readable directly; checksum at `byte[3] == (byte[0]+byte[1]+byte[2]) & 0xff`.

Override auto-detection with `--encrypted` or `--plaintext` flags.

Operation codes: `0x42` = temperature (°K/16 − 273.15), `0x50` = CO₂ (ppm).

## FreeBSD compatibility

The ioctl constant `0xc0094806` encodes identically on Linux and FreeBSD (both direction
bits set → `0xC0000000`; same size/type/number fields). FreeBSD 14+ uses the same
`/dev/hidrawN` naming. The code compiles and runs on FreeBSD without changes.

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/pkg/errors` | Wrapped errors |
| `github.com/prometheus/client_golang` | Prometheus metrics |
| `gopkg.in/alecthomas/kingpin.v2` | CLI flags |
| `github.com/stretchr/testify` | Test assertions |
| `github.com/eclipse/paho.mqtt.golang` | MQTT publishing |

## MQTT

Optional MQTT publishing via `--mqtt-host <host:port>` and `--mqtt-topic <topic>`. Both flags must be provided together; omitting both keeps Prometheus-only behavior.

Payload published on each measurement:
```json
{"temperature": 21.5, "co2": 843, "co2_detected": false, "linkquality": 255, "timestamp": 1719187200}
```

- `co2_detected` is true when CO₂ ≥ `--mqtt-co2-threshold` (default: 1800 ppm)
- `timestamp` is a Unix timestamp (seconds) of when the measurement was published
- `--mqtt-interval` controls the minimum time between publishes (default: `1m`); first measurement always publishes immediately
- Connection failure at startup is fatal; mid-run disconnections auto-reconnect via paho
- Each instance uses a unique client ID derived from hostname and device name (e.g. `co2monitor-raspberrypi-hidraw2`) to avoid broker conflicts when running multiple instances on the same or different hosts

## systemd (Debian)

A `co2monitor.service` unit file is included. Install and enable:

```sh
go build -o co2monitor . && sudo cp co2monitor /usr/local/bin/
sudo useradd --system --no-create-home --shell /usr/sbin/nologin co2monitor
sudo cp co2monitor.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now co2monitor
```

Two configuration options are documented in the unit file:

**Option A — environment file** (default): copy `co2monitor.env` to `/etc/co2monitor/config` and edit it there. Keeps the unit file untouched.

```sh
sudo mkdir /etc/co2monitor
sudo cp co2monitor.env /etc/co2monitor/config
```

**Option B — inline**: comment out the `EnvironmentFile` and `$VAR`-based `ExecStart`, uncomment the direct line:

```ini
ExecStart=/usr/local/bin/co2monitor /dev/hidraw2 :8080 --mqtt-host raspberrypi:1883 --mqtt-topic sensors/co2
```

After any change to the unit file: `sudo systemctl daemon-reload && sudo systemctl restart co2monitor`.

HID device access requires the `co2monitor` user to be in the `input` group (`SupplementaryGroups=input` in the unit file) and a udev rule:

```
# /etc/udev/rules.d/99-co2monitor.rules
SUBSYSTEM=="hidraw", ATTRS{idVendor}=="04d9", MODE="0660", GROUP="input"
```

Reload udev with `sudo udevadm control --reload-rules && sudo udevadm trigger`.
