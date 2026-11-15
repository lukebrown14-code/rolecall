#!/bin/sh
# Install rolecall and its bundled Plugin from a GitHub release; no Go required.
#
#   curl -fsSL https://raw.githubusercontent.com/lukebrown14-code/rolecall/main/scripts/install.sh | sh
#
# Environment:
#   ROLECALL_VERSION  release tag to install, such as v0.1.1 (default: latest)
#   ROLECALL_PREFIX   install prefix (default: ~/.local)
#   ROLECALL_BASE_URL release download URL to use instead of GitHub (mirrors and tests)
#
# Layout: $PREFIX/share/rolecall holds the executable and plugin/ side by side, and
# $PREFIX/bin/rolecall links to it, so rolecall finds the Plugin without configuration.
set -eu

repo="lukebrown14-code/rolecall"

fail() { echo "rolecall install: $*" >&2; exit 1; }

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    fail "curl or wget is required"
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    fail "sha256sum or shasum is required to verify the download"
  fi
}

main() {
  case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail "unsupported OS $(uname -s); see the README for other install options" ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) fail "unsupported architecture $(uname -m)" ;;
  esac

  version="${ROLECALL_VERSION:-latest}"
  if [ -n "${ROLECALL_BASE_URL:-}" ]; then
    base="$ROLECALL_BASE_URL"
  elif [ "$version" = latest ]; then
    base="https://github.com/$repo/releases/latest/download"
  else
    base="https://github.com/$repo/releases/download/$version"
  fi
  archive="rolecall_${os}_${arch}.tar.gz"

  prefix="${ROLECALL_PREFIX:-$HOME/.local}"
  home_dir="$prefix/share/rolecall"
  bin_dir="$prefix/bin"

  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT INT TERM

  echo "Downloading $archive ($version)..."
  download "$base/$archive" "$tmp/$archive" || fail "could not download $base/$archive"
  download "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download checksums.txt"

  expected=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
  [ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
  [ "$(sha256 "$tmp/$archive")" = "$expected" ] || fail "checksum mismatch for $archive"

  mkdir "$tmp/extract"
  tar -xzf "$tmp/$archive" -C "$tmp/extract"
  [ -f "$tmp/extract/rolecall" ] || fail "archive does not contain the rolecall executable"

  # Replace a previous install, but never delete a directory rolecall did not create.
  if [ -e "$home_dir" ] && [ ! -f "$home_dir/rolecall" ]; then
    fail "$home_dir exists and is not a rolecall install; remove it or set ROLECALL_PREFIX"
  fi
  mkdir -p "$prefix/share" "$bin_dir"
  rm -rf "$home_dir"
  mv "$tmp/extract" "$home_dir"
  chmod 755 "$home_dir/rolecall"
  ln -sf "$home_dir/rolecall" "$bin_dir/rolecall"

  echo "Installed rolecall to $bin_dir/rolecall (Plugin: $home_dir/plugin)"
  case ":$PATH:" in
    *":$bin_dir:"*) echo "Run 'rolecall init' to create a Workspace." ;;
    *) echo "Add $bin_dir to your PATH, then run 'rolecall init' to create a Workspace." ;;
  esac
}

main "$@"
