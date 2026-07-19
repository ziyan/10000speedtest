# 10000speedtest

A command-line download/upload speed test for the China Telecom Guangdong
network. It reproduces, on the terminal, the test that the web page at
[`https://10000.gd.cn/#/speed`](https://10000.gd.cn/#/speed) runs in the browser.

> The test servers (`gz.10000gd.tech`) are only reachable from within the China
> Telecom Guangdong network, so the tool must be run from a host on that network.

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
handshake succeeds.

## Install

```sh
go install github.com/ziyan/10000speedtest@latest
```

Or download a prebuilt binary from the [releases](https://github.com/ziyan/10000speedtest/releases) page, or build from source:

```sh
make build
```

## Usage

```sh
# Run both stages with defaults (8 connections, 10s each)
10000speedtest

# Download only, 16 connections, 15 seconds
10000speedtest --mode download --connections 16 --duration 15s

# Point at a different regional server
10000speedtest --server https://gz.10000gd.tech:12348
```

Example output:

```
Server:      https://gz.10000gd.tech:12348
Connections: 12
Duration:    10s per stage

Download:    671.64 Mbps   (640.83 MiB in 10.0s)
Upload:       65.30 Mbps   (62.28 MiB in 10.0s)
```

## Flags

| Flag              | Default                          | Description                                        |
| ----------------- | -------------------------------- | -------------------------------------------------- |
| `--server`        | `https://gz.10000gd.tech:12348`  | base URL of the speed-test server                  |
| `--mode`          | `both`                           | which test to run: `download`, `upload`, or `both` |
| `--connections`   | `8`                              | number of parallel connections                     |
| `--duration`      | `10s`                            | duration of each test stage                        |
| `--download-size` | `20`                             | MiB requested per download connection              |
| `--upload-size`   | `20`                             | MiB posted per upload connection                   |
| `--insecure`      | `true`                           | skip TLS certificate verification                  |
| `--log-level`     | `info`                           | log level (`debug`, `info`, `notice`, …)           |

## License

[MIT](LICENSE)
