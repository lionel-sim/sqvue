#!/bin/sh

set -eu

project_directory=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
temporary_directory=$(mktemp -d)
trap 'rm -rf "$temporary_directory"' 0 HUP INT TERM
version=v0.1.0-test

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) echo "unsupported operating system" >&2; exit 1 ;;
esac

case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) echo "unsupported CPU architecture" >&2; exit 1 ;;
esac

asset_directory="$temporary_directory/releases/download/$version"
mkdir -p "$asset_directory"
printf '#!/bin/sh\nprintf "sqvue version %s\\n"\n' "$version" > "$temporary_directory/sqvue"
chmod 0755 "$temporary_directory/sqvue"
archive_version=${version#v}
archive="sqvue_${archive_version}_${os}_${arch}.tar.gz"
tar -czf "$asset_directory/$archive" -C "$temporary_directory" sqvue

if command -v sha256sum >/dev/null 2>&1; then
	checksum=$(sha256sum "$asset_directory/$archive" | awk '{ print $1 }')
else
	checksum=$(shasum -a 256 "$asset_directory/$archive" | awk '{ print $1 }')
fi
printf '%s  %s\n' "$checksum" "$archive" > "$asset_directory/checksums.txt"

install_directory="$temporary_directory/bin"
SQVUE_DOWNLOAD_BASE_URL="file://$temporary_directory/releases/download" \
	SQVUE_VERSION="$version" \
	INSTALL_DIR="$install_directory" \
	"$project_directory/scripts/install.sh"

[ -x "$install_directory/sqvue" ]
[ "$("$install_directory/sqvue")" = "sqvue version $version" ]

printf '%s  %s\n' "incorrect" "$archive" > "$asset_directory/checksums.txt"
if SQVUE_DOWNLOAD_BASE_URL="file://$temporary_directory/releases/download" \
	SQVUE_VERSION="$version" \
	INSTALL_DIR="$temporary_directory/invalid-checksum-bin" \
	"$project_directory/scripts/install.sh" >/dev/null 2>&1; then
	echo "installer accepted an invalid checksum" >&2
	exit 1
fi

if HOME= INSTALL_DIR= \
	SQVUE_DOWNLOAD_BASE_URL="file://$temporary_directory/releases/download" \
	SQVUE_VERSION="$version" \
	"$project_directory/scripts/install.sh" >/dev/null 2>&1; then
	echo "installer accepted an unset HOME without INSTALL_DIR" >&2
	exit 1
fi
