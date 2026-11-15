#!/bin/sh
set -eu

format="$1"
dist="$2"
arch="$3"
project="$4"
here="$(cd "$(dirname "$0")" && pwd)"

ensure_curl() {
  if command -v curl >/dev/null 2>&1; then
    return 0
  fi
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl ca-certificates
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q curl
  elif command -v zypper >/dev/null 2>&1; then
    zypper --non-interactive install curl
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache curl
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm curl
  fi
}

pick_file() {
  set -- "$dist"/highrise-code_*_linux_"$arch"."$1"
  [ -f "$1" ] || { echo "package not found: $1" >&2; exit 1; }
  echo "$1"
}

installed_files() {
  case "$1" in
    deb) dpkg -L highrise-code ;;
    rpm) rpm -ql highrise-code ;;
    apk) apk info -L highrise-code ;;
    pacman) pacman -Ql highrise-code | cut -d' ' -f2 ;;
  esac
}

check_installed_files() {
  command -v highrise-code >/dev/null 2>&1 || { echo "highrise-code is not in PATH" >&2; exit 1; }
  listing="$(installed_files "$1")"
  for f in \
    /usr/bin/highrise-code \
    /usr/share/man/man1/highrise-code.1.gz \
    /usr/share/bash-completion/completions/highrise-code \
    /usr/share/zsh/site-functions/_highrise-code \
    /usr/share/fish/vendor_completions.d/highrise-code.fish \
    /usr/share/applications/highrise-code.desktop \
    /usr/share/icons/hicolor/scalable/apps/highrise-code.svg; do
    echo "$listing" | grep -qx "$f" || { echo "package does not contain: $f" >&2; echo "$listing" >&2; exit 1; }
  done
}

ensure_curl

case "$format" in
  deb)
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$(pick_file deb)"
    check_installed_files "$format"
    binary="highrise-code"
    ;;
  rpm)
    file="$(pick_file rpm)"
    if command -v dnf >/dev/null 2>&1; then
      dnf install -y -q "$file"
    else
      zypper --non-interactive --no-gpg-checks install --allow-unsigned-rpm "$file"
    fi
    check_installed_files "$format"
    binary="highrise-code"
    ;;
  apk)
    apk add --no-cache --allow-untrusted "$(pick_file apk)"
    check_installed_files "$format"
    binary="highrise-code"
    ;;
  pacman)
    pacman -U --noconfirm "$(pick_file pkg.tar.zst)"
    check_installed_files "$format"
    binary="highrise-code"
    ;;
  tar)
    target="$(mktemp -d)"
    tar -xzf "$(pick_file tar.gz)" -C "$target"
    binary="$target/highrise-code"
    ;;
  *)
    echo "unknown format: $format" >&2
    exit 1
    ;;
esac

sh "$here/smoke_test.sh" "$binary" "$project"
