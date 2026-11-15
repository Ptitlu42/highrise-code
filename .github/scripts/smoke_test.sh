#!/bin/sh
set -eu

bin="$1"
project="$2"
work="$(mktemp -d)"
server_pid=""

cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

exit_code() {
  set +e
  "$@" >/dev/null 2>&1
  code=$?
  set -e
  echo "$code"
}

"$bin" --version | grep -Eq '^highrise-code [^ ]+$' || fail "version output"
"$bin" --help | grep -q -- '--pick' || fail "help does not mention --pick"

"$bin" -q --stats "$project" >"$work/stats.txt"
grep -qi 'lines' "$work/stats.txt" || fail "text statistics"

"$bin" -q --stats --format json "$project" >"$work/stats.json"
grep -q '"languages"' "$work/stats.json" || fail "json statistics"

"$bin" -q --out "$work/city.html" "$project"
[ -s "$work/city.html" ] || fail "standalone html is empty"
grep -q 'hr_layout' "$work/city.html" || fail "standalone html has no layout"

[ "$(exit_code "$bin" -q --stats "$work/does_not_exist")" = "1" ] || fail "missing root must exit 1"
[ "$(exit_code "$bin" --bogus)" = "2" ] || fail "unknown option must exit 2"

if [ "$(uname -s)" = "Linux" ] && ! command -v zenity >/dev/null 2>&1 && ! command -v kdialog >/dev/null 2>&1 && ! command -v yad >/dev/null 2>&1; then
  code="$(exit_code env -u DISPLAY -u WAYLAND_DISPLAY "$bin" </dev/null)"
  [ "$code" = "2" ] || fail "headless start without arguments must exit 2, got $code"
  set +e
  timeout 20 "$bin" --pick >"$work/pick.out" 2>"$work/pick.err"
  code=$?
  set -e
  [ "$code" = "1" ] || fail "--pick without a picker must exit 1, got $code"
  grep -q 'no folder picker' "$work/pick.err" || fail "--pick error message"
fi

log="$work/server.log"
"$bin" --no-open "$project" 2>"$log" &
server_pid=$!

url=""
i=0
while [ "$i" -lt 30 ]; do
  url="$(grep -o 'http://127.0.0.1:[0-9]*/?t=[0-9a-f]*' "$log" 2>/dev/null | head -n 1 || true)"
  if [ -n "$url" ]; then
    break
  fi
  i=$((i + 1))
  sleep 1
done
[ -n "$url" ] || fail "server did not start: $(cat "$log")"
base="${url%%/?t=*}"

status="$(curl -s -o /dev/null -w '%{http_code}' "$base/api/summary")"
[ "$status" = "403" ] || fail "unauthenticated request returned $status"

curl -fsS -L -c "$work/jar" -b "$work/jar" -o /dev/null "$url"
curl -fsS -b "$work/jar" "$base/api/summary" | grep -q '"root"' || fail "summary endpoint"
curl -fsS -b "$work/jar" "$base/api/names" | grep -q '\[' || fail "names endpoint"
curl -fsS -b "$work/jar" -o "$work/layout.bin" "$base/api/layout"
[ -s "$work/layout.bin" ] || fail "layout endpoint"

echo "smoke test passed: $bin"
