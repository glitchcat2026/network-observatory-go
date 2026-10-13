# Network Observatory

[English](README.md) | [简体中文](README.zh-CN.md)

A lightweight, distributed network monitoring system implemented in Go. It collects latency, packet loss, and availability data from multiple probe nodes and displays the results in a simple web dashboard.

## Features

- Cross-platform support (Windows/Linux)
- IPv4 and IPv6 support
- Multilingual web dashboard (Chinese/English)

## Contribute

```bash
git clone https://github.com/glitchcat2026/network-observatory-go.git
cd network-observatory-go
go mod download
```

## Quick Start

### 1. Start the Collector

Run the following on the Collector host:

```bash
./collector-linux-amd64
```

On the first launch, the program creates `collector-config.json` and exits. Edit the configuration file, then run the program again.

### 2. Start a Probe

On each probe host, go to the directory containing the executable and run:

```powershell
.\probe-windows-amd64.exe
```

```bash
./probe-linux-amd64
```

On the first launch, the program creates `probe-config.json` and exits. Copy the token from `collector-config.json` exactly as-is into `probe-config.json`. Edit the configuration file, then run the program again.

### 3. Open the Dashboard

Visit the following URL in your browser:

```text
http://localhost:8080/
```

## Configuration

### 1. Collector Configuration

Example default Collector configuration:

```json
{
  "db_path": "observatory.db",
  "addr": ":8080",
  "token": "replace-with-the-same-secret-on-every-node",
  "max_body_size": 1048576,
  "lang": "en"
}
```

Configuration options:

- `db_path`: Path to the SQLite database file.
- `addr`: HTTP listen address. For example, `:8080` listens on all network interfaces; `127.0.0.1:8080` listens on IPv4 localhost only; `[::1]:8080` listens on IPv6 localhost only.
- `token`: Shared secret used to authenticate reports from Probes.
- `max_body_size`: Maximum size of a single report request body, in bytes.
- `lang`: HTML language tag for the web dashboard, such as `"en"` or `"zh-cn"`.

### 2. Probe Configuration

Example default Probe configuration:

```json
{
  "collector_url": "http://localhost:8080/api",
  "hostnames": ["example.com", "8.8.8.8"],
  "ip_version": "ipv4",
  "token": "replace-with-the-same-secret-on-every-node",
  "probe_name": "probe-1",
  "sleep_seconds": 10,
  "ping_timeout_seconds": 5
}
```

Configuration options:

- `collector_url`: URL of the Collector report endpoint, usually `http://<Collector address>:8080/api`.
- `hostnames`: List of domain names or IP addresses to probe with Ping.
- `ip_version`: IP version used for Ping and DNS resolution. Supported values are `"ipv4"` and `"ipv6"`.
- `token`: Shared secret sent with each report. It must exactly match the `token` in the Collector configuration.
- `probe_name`: Name used to identify this Probe in reports and on the dashboard. Using a different name for each Probe is recommended.
- `sleep_seconds`: Number of seconds to wait after each probe round.
- `ping_timeout_seconds`: Maximum time, in seconds, to spend probing each target with Ping.

The Collector stores probe results in SQLite.

## Build

```bash
make build
```

This command generates executables for Windows and Linux in the `bin/` directory.

## License

This project is open source under the MIT License.

See the [LICENSE](LICENSE) file for details.
