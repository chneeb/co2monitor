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
