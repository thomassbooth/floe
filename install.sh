#!/bin/sh
# Installs the latest floe release on Linux or macOS:
#
#   curl -fsSL https://raw.githubusercontent.com/thomassbooth/floe/main/install.sh | sh
#
# FLOE_VERSION picks a release, such as v0.2.0, instead of the latest.
# FLOE_INSTALL_DIR picks where floe goes. By default that is /usr/local/bin,
# using sudo if needed, or ~/.local/bin when sudo is not available.

set -eu

repo=thomassbooth/floe

fail() {
	echo "floe install: $*" >&2
	exit 1
}

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) fail "floe runs on Linux and macOS, not $(uname -s)" ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "there is no floe release for $(uname -m) processors" ;;
esac

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q -O "$2" "$1"; }
else
	fail "curl or wget is needed to download floe"
fi

if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
	fail "sha256sum or shasum is needed to check the download"
fi

if [ -n "${FLOE_VERSION:-}" ]; then
	base="https://github.com/$repo/releases/download/$FLOE_VERSION"
else
	base="https://github.com/$repo/releases/latest/download"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The checksums name every archive, so they also say what this release's
# archive for this system is called.
fetch "$base/checksums.txt" "$tmp/checksums.txt" ||
	fail "could not download the release from $base"
line=$(grep "_${os}_${arch}\.tar\.gz\$" "$tmp/checksums.txt") ||
	fail "the release has no archive for $os $arch"
sum=${line%% *}
archive=${line##* }

echo "Downloading $archive"
fetch "$base/$archive" "$tmp/$archive" || fail "could not download $archive"
[ "$(sha256 "$tmp/$archive")" = "$sum" ] ||
	fail "$archive does not match its checksum; try again"
tar -xzf "$tmp/$archive" -C "$tmp" floe

dir=${FLOE_INSTALL_DIR:-/usr/local/bin}
sudo=
if ! mkdir -p "$dir" 2>/dev/null || [ ! -w "$dir" ]; then
	if [ -n "${FLOE_INSTALL_DIR:-}" ]; then
		fail "cannot write to $dir"
	elif command -v sudo >/dev/null 2>&1; then
		echo "Installing to $dir needs sudo"
		sudo=sudo
		$sudo mkdir -p "$dir"
	else
		dir="$HOME/.local/bin"
		mkdir -p "$dir"
	fi
fi
$sudo install -m 755 "$tmp/floe" "$dir/floe"

echo "Installed $("$dir/floe" version) to $dir/floe"
case ":$PATH:" in
*":$dir:"*) ;;
*) echo "Add $dir to your PATH to run floe" ;;
esac
