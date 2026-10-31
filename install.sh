#!/bin/sh
# Installs the latest glance release, or GLANCE_VERSION, to GLANCE_DIR (default ~/.local/bin),
# after checking it against the release checksums.
#   curl -fsSL https://raw.githubusercontent.com/nyactl/glance/main/install.sh | sh
set -eu

repo=nyactl/glance
dir=${GLANCE_DIR:-$HOME/.local/bin}
version=${GLANCE_VERSION:-}

fail() { echo "glance install: $*" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "unsupported system $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac

if [ -z "$version" ]; then
  # The latest release redirects to its tag; no API call, so no rate limit.
  version=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
  version=${version##*/}
fi
version=${version#v}
[ -n "$version" ] || fail "no release found"

archive="glance_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/v$version"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "no $archive in v$version"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"
want=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
else
  got=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
fi
[ -n "$want" ] && [ "$want" = "$got" ] || fail "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$dir"
# A new file renamed over the old one: macOS kills a signed binary overwritten in place.
install -m 755 "$tmp/glance" "$dir/glance.new"
mv -f "$dir/glance.new" "$dir/glance"
echo "glance $version installed to $dir/glance"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "add $dir to PATH to run it" ;;
esac
