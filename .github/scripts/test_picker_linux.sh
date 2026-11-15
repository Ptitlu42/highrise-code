#!/bin/sh
set -eu

bin="$1"
work="$(mktemp -d)"
project="$work/picked_project"
mkdir -p "$project"
printf 'package main\n\nfunc main() {}\n' >"$project/picked_marker.go"

Xvfb :99 -screen 0 1024x768x24 >/dev/null 2>&1 &
xvfb_pid=$!
export DISPLAY=:99

cleanup() {
  pkill zenity 2>/dev/null || true
  kill "$app_pid" 2>/dev/null || true
  kill "$xvfb_pid" 2>/dev/null || true
  rm -rf "$work"
}
app_pid=""
trap cleanup EXIT

sleep 2

"$bin" --pick -q --stats --format json >"$work/out.json" 2>"$work/err.log" &
app_pid=$!

found=""
i=0
while [ "$i" -lt 30 ]; do
  if xdotool search --name 'choose the project folder' >/dev/null 2>&1; then
    found=1
    break
  fi
  i=$((i + 1))
  sleep 1
done

if [ -z "$found" ]; then
  echo "FAIL: the folder dialog did not appear" >&2
  cat "$work/err.log" >&2 || true
  exit 1
fi
echo "folder dialog appeared"

xdotool key ctrl+l
sleep 1
xdotool type --delay 40 "$project"
sleep 1
xdotool key Return
sleep 2
xdotool key Return

i=0
while [ "$i" -lt 15 ]; do
  if ! kill -0 "$app_pid" 2>/dev/null; then
    break
  fi
  i=$((i + 1))
  sleep 1
done

if grep -q 'picked_marker.go' "$work/out.json" 2>/dev/null; then
  echo "folder dialog selection reached the scanner"
else
  echo "::warning::could not complete the dialog selection automatically"
fi
