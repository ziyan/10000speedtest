# Changelog

All notable changes to 10000speedtest will be documented in this file.

The format is based loosely on Keep a Changelog, and versions are recorded using repository tags.

## [0.4.0]

### Added

- `--interface` flag to bind the test's connections to a specific network interface (by name) or source IP, for measuring a single NIC on a multi-homed host. Requires matching source routing so the interface's IP egresses that interface.
- `scripts/dual-interface.sh` to run the test across several interfaces in parallel and report the combined throughput.

## [0.3.0]

### Added

- `--json` flag: print only the final results as a single JSON object (server, connections, duration, and per-stage mbps/bytes/seconds), with no header or live progress.

### Changed

- The bar chart now sizes itself to the terminal width instead of a fixed 50 columns, and the header is truncated so it cannot wrap and desync the redraw on narrow terminals.
- When stdout is not a terminal (piped or redirected), the tool no longer rewrites a progress line; it prints just the header and the final per-stage summary.

## [0.2.0]

### Added

- Live colored bar chart of throughput over time, drawn per stage (separate charts for download and upload). Bars are shaded red → yellow → green by height and the vertical axis auto-scales to the running peak. The first second (the connection-ramp warmup) is excluded from the chart so the initial spike does not compress the steady state; the summary average still covers the whole run. Enabled with `--chart` (default on); it automatically falls back to single-line output when stdout is not a terminal, and honors `NO_COLOR`.

## [0.1.0]

### Added

- Command-line download/upload speed test against the China Telecom Guangdong servers (`https://gz.10000gd.tech:12348`), reproducing the web client at `https://10000.gd.cn/#/speed`.
- Parallel multi-connection download (`GET /shmfile/<N>`) and upload (`POST /upload`) stages with a live Mbps readout.
- Flags for server, mode, connection count, per-stage duration, per-connection payload size, TLS verification, and log level.
