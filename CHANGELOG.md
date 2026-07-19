# Changelog

All notable changes to 10000speedtest will be documented in this file.

The format is based loosely on Keep a Changelog, and versions are recorded using repository tags.

## [0.1.0]

### Added

- Command-line download/upload speed test against the China Telecom Guangdong servers (`https://gz.10000gd.tech:12348`), reproducing the web client at `https://10000.gd.cn/#/speed`.
- Parallel multi-connection download (`GET /shmfile/<N>`) and upload (`POST /upload`) stages with a live Mbps readout.
- Flags for server, mode, connection count, per-stage duration, per-connection payload size, TLS verification, and log level.
