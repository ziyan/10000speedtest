#!/bin/sh
# Install the latest 10000speedtest release binary for this host's OS and CPU.
#
#   curl -fsSL https://raw.githubusercontent.com/ziyan/10000speedtest/main/install.sh | sh
#
# Environment overrides:
#   INSTALL_DIR   where to install (default: /usr/local/bin; falls back to the
#                 current directory if that is not writable and sudo is absent)
#   VERSION       release tag to install, e.g. v0.6.0 (default: latest)
#
# Windows is not supported by this script; download the .zip from the releases
# page instead.
set -eu

REPO="ziyan/10000speedtest"
BINARY="10000speedtest"
VERSION="${VERSION:-latest}"

info() { printf '%s\n' "$*" >&2; }
fail() { printf 'error: %s\n' "$*" >&2; exit 1; }

# --- pick a downloader -------------------------------------------------------
if command -v curl >/dev/null 2>&1; then
	fetch()     { curl -fsSL "$1"; }
	fetch_to()  { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch()     { wget -qO- "$1"; }
	fetch_to()  { wget -qO "$2" "$1"; }
else
	fail "need curl or wget"
fi

# --- detect OS and architecture ---------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
	linux | darwin) ;;
	*) fail "unsupported OS '$os' (on Windows, download the .zip from the releases page)" ;;
esac

case "$(uname -m)" in
	x86_64 | amd64)          arch=amd64 ;;
	aarch64 | arm64)         arch=arm64 ;;
	armv7l | armv7 | armhf)  arch=arm ;;
	*) fail "unsupported architecture '$(uname -m)'" ;;
esac

# --- resolve the release asset ----------------------------------------------
if [ "$VERSION" = latest ]; then
	api="https://api.github.com/repos/$REPO/releases/latest"
else
	api="https://api.github.com/repos/$REPO/releases/tags/$VERSION"
fi
info "querying $VERSION release for ${os}/${arch}..."
release=$(fetch "$api") || fail "could not query the release API"

suffix="_${os}_${arch}.tar.gz"
url=$(printf '%s' "$release" | grep -o "https://[^\"]*${suffix}" | head -1)
[ -n "$url" ] || fail "no release binary for ${os}/${arch}"
sums_url=$(printf '%s' "$release" | grep -o 'https://[^"]*_SHA256SUMS' | head -1)

# --- download, verify, extract ----------------------------------------------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
tarball="$tmp/$(basename "$url")"

info "downloading $(basename "$url")..."
fetch_to "$url" "$tarball"

if [ -n "${sums_url:-}" ] && fetch_to "$sums_url" "$tmp/SHA256SUMS" 2>/dev/null; then
	if command -v sha256sum >/dev/null 2>&1; then
		got=$(sha256sum "$tarball" | awk '{print $1}')
	elif command -v shasum >/dev/null 2>&1; then
		got=$(shasum -a 256 "$tarball" | awk '{print $1}')
	else
		got=""
	fi
	if [ -n "$got" ]; then
		want=$(grep "$(basename "$tarball")" "$tmp/SHA256SUMS" | awk '{print $1}')
		[ "$got" = "$want" ] || fail "checksum mismatch for $(basename "$tarball")"
		info "checksum verified"
	fi
fi

tar -xzf "$tarball" -C "$tmp" "$BINARY"
chmod +x "$tmp/$BINARY"

# --- install ----------------------------------------------------------------
dir="${INSTALL_DIR:-/usr/local/bin}"
if mkdir -p "$dir" 2>/dev/null && [ -w "$dir" ]; then
	mv "$tmp/$BINARY" "$dir/$BINARY"
elif command -v sudo >/dev/null 2>&1; then
	info "installing to $dir (using sudo)..."
	sudo install -m 0755 "$tmp/$BINARY" "$dir/$BINARY"
else
	dir=$(pwd)
	mv "$tmp/$BINARY" "$dir/$BINARY"
	info "no write access to ${INSTALL_DIR:-/usr/local/bin}; installed to $dir instead"
fi

info "installed $BINARY to $dir"
"$dir/$BINARY" --version
