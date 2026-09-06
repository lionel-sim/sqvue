#!/bin/sh

set -eu

repository=${SQVUE_REPOSITORY:-lionel-sim/sqvue}
download_base=${SQVUE_DOWNLOAD_BASE_URL:-"https://github.com/$repository/releases/download"}

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	bold=$(printf '\033[1m')
	blue=$(printf '\033[34m')
	green=$(printf '\033[32m')
	red=$(printf '\033[31m')
	reset=$(printf '\033[0m')
else
	bold=
	blue=
	green=
	red=
	reset=
fi

header() {
	printf '\n%s==>%s %ssqvue%s installer\n\n' "$blue" "$reset" "$bold" "$reset"
}

step() {
	printf '%s==>%s %s\n' "$blue" "$reset" "$*"
}

fail() {
	printf '%serror:%s %s\n' "$red" "$reset" "$*" >&2
	exit 1
}

download() {
	url=$1
	destination=$2
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$destination"
		return
	fi
	if command -v wget >/dev/null 2>&1; then
		wget -qO "$destination" "$url"
		return
	fi
	fail "curl or wget is required"
}

header

if [ -n "${INSTALL_DIR:-}" ]; then
	install_dir=$INSTALL_DIR
else
	[ -n "${HOME:-}" ] || fail "HOME is not set; set INSTALL_DIR explicitly"
	install_dir="$HOME/.local/bin"
fi

if [ -n "${SQVUE_VERSION:-}" ]; then
	version=$SQVUE_VERSION
else
	step "Resolving the latest stable release"
	temporary_metadata=$(mktemp)
	trap 'rm -f "$temporary_metadata"' 0
	download "https://api.github.com/repos/$repository/releases/latest" "$temporary_metadata"
	version=$(sed -n 's/^[[:space:]]*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$temporary_metadata")
	rm -f "$temporary_metadata"
	trap - 0
	[ -n "$version" ] || fail "could not determine the latest release version"
fi

case "$version" in
	v[0-9]*.[0-9]*.[0-9]*) ;;
	*) fail "version must be a tag such as v0.1.0 or v0.1.0-beta.1" ;;
esac

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) fail "unsupported operating system; download a release archive manually" ;;
esac

case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) fail "unsupported CPU architecture; download a release archive manually" ;;
esac

archive="sqvue_${version}_${os}_${arch}.tar.gz"
temporary_directory=$(mktemp -d)
trap 'rm -rf "$temporary_directory"' 0 HUP INT TERM

step "Downloading sqvue $version for $os/$arch"
download "$download_base/$version/$archive" "$temporary_directory/$archive"
download "$download_base/$version/checksums.txt" "$temporary_directory/checksums.txt"

step "Verifying SHA-256 checksum"
expected_checksum=$(awk -v filename="$archive" '$NF == filename { print $1; exit }' "$temporary_directory/checksums.txt")
[ -n "$expected_checksum" ] || fail "checksum for $archive was not found"

if command -v sha256sum >/dev/null 2>&1; then
	actual_checksum=$(sha256sum "$temporary_directory/$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	actual_checksum=$(shasum -a 256 "$temporary_directory/$archive" | awk '{ print $1 }')
else
	fail "sha256sum or shasum is required to verify the download"
fi

[ "$expected_checksum" = "$actual_checksum" ] || fail "checksum verification failed"

tar -xzf "$temporary_directory/$archive" -C "$temporary_directory"
[ -f "$temporary_directory/sqvue" ] || fail "release archive did not contain sqvue"

step "Installing to $install_dir"
mkdir -p "$install_dir"
install -m 0755 "$temporary_directory/sqvue" "$install_dir/sqvue"
printf '%sInstalled%s sqvue %s at %s/sqvue\n' "$green" "$reset" "$version" "$install_dir"

case ":$PATH:" in
	*":$install_dir:"*) ;;
	*) printf 'Add %s to your PATH to run sqvue from any directory.\n' "$install_dir" ;;
esac
