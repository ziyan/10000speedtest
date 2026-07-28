# 10000speedtest

A command-line download/upload speed test for the China Telecom Guangdong
network. It reproduces, on the terminal, the test that the web page at
[`https://10000.gd.cn/#/speed`](https://10000.gd.cn/#/speed) runs in the browser.

广东电信网络的命令行上传/下载测速工具。它在终端中复现了网页
[`https://10000.gd.cn/#/speed`](https://10000.gd.cn/#/speed) 在浏览器里运行的测速。

> The test servers (`gz.10000gd.tech`) are only reachable from within the China
> Telecom Guangdong network, so the tool must be run from a host on that network.
>
> 测速服务器（`gz.10000gd.tech`）仅可从广东电信网络内部访问，因此本工具必须在该网络内的主机上运行。

![Live download and upload bar charts in the terminal](docs/chart.png)

## How it works

The web client drives its test with many parallel HTTP connections against a
regional server (for example `gz.10000gd.tech:12348`):

| Stage    | Request                                  | Behaviour                                   |
| -------- | ---------------------------------------- | ------------------------------------------- |
| Download | `GET /shmfile/<sizeMiB>?tag=<random>`    | server streams `sizeMiB` MiB of random data |
| Upload   | `POST /upload?tag=<random>`              | client streams a body, server sinks it      |

`tag` is a random cache-buster, and each request opens a fresh connection
(`Connection: close`). `10000speedtest` opens `--connections` of these in
parallel for `--duration`, sums the bytes moved, and reports the throughput in
Mbps with a live readout.

The test server negotiates TLS 1.2 with the RSA cipher `AES256-GCM-SHA384`,
which Go does not offer by default; the client enables it explicitly so the
handshake succeeds. The same server also exposes an **unencrypted** mirror — see
[Plain HTTP](#plain-http-avoiding-tls-overhead).

## Install

### Quick install (prebuilt binary)

On Linux or macOS, install the latest release for your OS and CPU with one
command — it detects the platform, downloads the matching binary, verifies its
checksum, and installs it to `/usr/local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/ziyan/10000speedtest/main/install.sh | sh
```

The installer honors a couple of environment variables:

```sh
# install somewhere else (e.g. no root, or a UniFi device with a read-only /usr)
curl -fsSL https://raw.githubusercontent.com/ziyan/10000speedtest/main/install.sh | INSTALL_DIR=/tmp sh

# pin a specific release
curl -fsSL https://raw.githubusercontent.com/ziyan/10000speedtest/main/install.sh | VERSION=v0.6.0 sh
```

If it can't write to the target directory and `sudo` isn't available, it drops
the binary in the current directory instead.

Prefer to pick a file by hand? Grab it from the
[releases](https://github.com/ziyan/10000speedtest/releases) page. Windows ships
as a `.zip`; every release also has a `SHA256SUMS` to verify against.

### UniFi devices (gateways and access points)

The tool is a single static binary, so it runs on stock UniFi firmware. Check a
device's architecture first with `uname -m`:

| Device                                        | `uname -m` | Release asset          |
| --------------------------------------------- | ---------- | ---------------------- |
| **Gateway** (e.g. IPQ9574 / Dream Machine)    | `aarch64`  | `..._linux_arm64.tar.gz` |
| **Access point** (e.g. U6 / U7 series)        | `armv7l`   | `..._linux_arm.tar.gz`   |

If the device can reach GitHub, run the installer on it directly — UniFi's `/usr`
is read-only, so install to a writable path like `/tmp`:

```sh
curl -fsSL https://raw.githubusercontent.com/ziyan/10000speedtest/main/install.sh | INSTALL_DIR=/tmp sh
```

Otherwise download on another machine and copy it over:

```sh
url=$(curl -fsSL https://api.github.com/repos/ziyan/10000speedtest/releases/latest \
  | grep -o 'https://[^"]*_linux_arm\.tar\.gz' | head -1)   # arm64 for the gateway
curl -fsSL "$url" | tar -xzf - 10000speedtest
scp 10000speedtest <ssh-user>@<device-ip>:/tmp/
```

Two things to expect on an **access point**: it has no hardware AES, so the
default [plain HTTP](#plain-http-avoiding-tls-overhead) endpoint is important —
the HTTPS test would be CPU-bound (~200 Mbps). And running the test *on* the AP
measures the AP's own CPU, not the throughput of clients forwarded through it
(that path uses hardware offload); use the AP's built-in `iperf`/`iperf3`, or a
real client, to measure its actual link. The gateway (ARM64, hardware AES) runs
even the HTTPS test at line rate.

### From source

```sh
go install github.com/ziyan/10000speedtest@latest   # or: make build
```

## Usage

```sh
# Run both stages with defaults (8 connections, 10s each)
10000speedtest

# Download only, 16 connections, 15 seconds
10000speedtest --mode download --connections 16 --duration 15s

# Use the encrypted (HTTPS) server via its alias, or a full URL
10000speedtest --server gz-https
10000speedtest --server https://gz.10000gd.tech:12348
```

`--server` accepts a full base URL or a short alias for a well-known server:

| Alias      | URL                             |                                   |
| ---------- | ------------------------------- | --------------------------------- |
| `gz-http`  | `http://gz.10000gd.tech:12347`  | plain HTTP (**default**)          |
| `gz-https` | `https://gz.10000gd.tech:12348` | HTTPS (TLS 1.2, AES-GCM)          |

The default is **plain HTTP** so hardware without AES acceleration is not
crypto-bound — see [Plain HTTP](#plain-http-avoiding-tls-overhead).

On a terminal, each stage draws a live, colored bar chart of throughput over
time (bars shaded red → yellow → green by height), then prints the average. The
first second of each stage — the connection-ramp warmup — is left out of the
chart so the opening spike does not compress the steady state:

```
Download     590.87 Mbps   peak 853.02 Mbps
   853 │               ▂     ▃█
   746 │ ▃▁▇  ▃ ▃▂ ▅▃ ▁█  ▁  ██
   640 │ ███ ▅█ ██▅██▄██▁ █▂▄██▄
   533 │▂███▅██▃█████████▂██████
   427 │████████████████████████
   320 │████████████████████████
   213 │████████████████████████
   107 │████████████████████████
     0 └────────────────────────
Download:    635.04 Mbps   (455.55 MiB in 6.0s)
```

The chart sizes itself to the terminal width. When the output is piped or
redirected (not a terminal), the tool skips the live chart and progress
rewriting entirely and prints just the header and final per-stage summary:

```
Download:    671.64 Mbps   (640.83 MiB in 10.0s)
Upload:       65.30 Mbps   (62.28 MiB in 10.0s)
```

Pass `--chart=false` for the same plain output on a terminal.

Use `--json` for machine-readable results only (no header or live progress):

```sh
10000speedtest --json --duration 5s
```

```json
{
  "server": "https://gz.10000gd.tech:12348",
  "connections": 8,
  "durationSeconds": 5,
  "download": { "mbps": 672.93, "bytes": 422739968, "seconds": 5.03 },
  "upload": { "mbps": 68.50, "bytes": 42827776, "seconds": 5.00 }
}
```

## Plain HTTP (avoiding TLS overhead)

By **default** the tool uses the server's **unencrypted mirror** on port `12347`
(the `gz-http` alias), which serves the same `/shmfile/<N>` and `/upload`
endpoints as the HTTPS server on `12348` but skips TLS.

This matters on hardware **without AES acceleration**. The HTTPS endpoint only
negotiates AES-GCM (it rejects ChaCha20 and TLS 1.3), so a CPU that does AES in
software — for example a 32-bit ARM (`armv7l`) access point — becomes
crypto-bound and caps far below the link speed (e.g. ~200 Mbps while `iperf3`
shows multi-gigabit). Plain HTTP removes the encryption, so you measure the
network instead of the CPU. Defaulting to it means the tool is not
crypto-bound anywhere out of the box.

To use the encrypted endpoint instead (e.g. to test through a proxy that only
allows HTTPS), pass `--server gz-https`:

```sh
10000speedtest --server gz-https
```

The default traffic is unencrypted, which is fine for a throughput test. Note
that some gateways with deep-packet-inspection / IPS may filter plaintext HTTP
on non-standard ports; if the default `12347` times out from a device but
`gz-https` works, check the firewall on its path.

## Flags

| Flag              | Default                          | Description                                        |
| ----------------- | -------------------------------- | -------------------------------------------------- |
| `--server`        | `gz-http`                        | base URL or alias (`gz-http`, `gz-https`)          |
| `--mode`          | `both`                           | which test to run: `download`, `upload`, or `both` |
| `--connections`   | `8`                              | number of parallel connections                     |
| `--duration`      | `10s`                            | duration of each test stage                        |
| `--download-size` | `20`                             | MiB requested per download connection              |
| `--upload-size`   | `20`                             | MiB posted per upload connection                   |
| `--insecure`      | `true`                           | skip TLS certificate verification                  |
| `--chart`         | `true`                           | live colored bar chart (auto-off when not a TTY)   |
| `--json`          | `false`                          | print only the final results as JSON               |
| `--interface`     | (none)                           | bind connections to an interface name or source IP |
| `--log-level`     | `info`                           | log level (`debug`, `info`, `notice`, …)           |

Set `NO_COLOR` in the environment to draw the chart without ANSI colors.

## Exit status

The exit code reflects whether the test succeeded:

- `0` — every requested stage transferred data.
- non-zero — a stage that ran moved no data (the server was unreachable or every
  request failed); the failing stage is named on stderr. In `--json` mode the
  JSON is still written to stdout before the process exits non-zero.

This makes the tool safe to use in scripts:

```sh
if 10000speedtest --json --duration 5s > result.json; then
  echo "ok"
else
  echo "speed test failed" >&2
fi
```

## Multiple interfaces

`--interface` binds the test's connections to a specific NIC, given either an
interface name or a source IP:

```sh
10000speedtest --interface eno1
10000speedtest --interface 192.168.1.3
```

Repeat `--interface` to test several NICs **at once** — each runs
`--connections` connections and the results are aggregated, with a per-interface
breakdown:

```sh
10000speedtest --interface eno1 --interface eno2
```

```
Download:   1089.10 Mbps   (1.27 GiB in 10.0s)
  via eno1:   639.70 Mbps
  via eno2:   449.40 Mbps
```

For this to actually use each NIC, the host must **source-route** the NIC's IP
out that NIC. A non-default interface typically has no internet route, so add a
policy-routing table for it (Linux example, for `eno2` = `192.168.1.3/24`,
gateway `192.168.1.1`):

```sh
ip route replace 192.168.1.0/24 dev eno2 proto kernel scope link src 192.168.1.3 table 200
ip route replace default via 192.168.1.1 dev eno2 table 200
ip rule add from 192.168.1.3 lookup 200
```

Both the connected route **and** the default route are needed in the table, or
name resolution / on-link delivery for that source can fail.

Note: running two NICs only beats one when they have independent bandwidth up to
a shared uplink that is faster than a single NIC. If the interfaces share the
same uplink (same public IP) and that uplink is the bottleneck, the combined
figure is just the one pipe split in two.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code conventions,
and how to cut a release.

## License

[MIT](LICENSE)
