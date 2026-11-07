#!/usr/bin/env bash
# End-to-end check of `tokenfetch service` against the real systemd user
# manager. Runs in CI on ubuntu-latest: it installs and removes a real timer,
# so don't run it on a machine you care about.
#
# Usage: .github/scripts/service-e2e.sh [path/to/tokenfetch]
set -euo pipefail

tf=$(realpath "${1:-bin/tokenfetch}")
units=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user
work=$(mktemp -d)
trap '"$tf" service uninstall >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

fail() {
  echo "FAIL: $*" >&2
  journalctl --user -u tokenfetch.service --no-pager >&2 || true
  exit 1
}

# CI runs without a login session, so there is no user manager until linger
# starts one.
sudo loginctl enable-linger "$(id -un)"
export XDG_RUNTIME_DIR=/run/user/$(id -u)
export DBUS_SESSION_BUS_ADDRESS=unix:path=$XDG_RUNTIME_DIR/bus
for _ in $(seq 50); do
  [[ -S $XDG_RUNTIME_DIR/bus ]] && break
  sleep 0.2
done
systemctl --user is-system-running --wait || true # "degraded" is fine

# Isolated data and transcripts; the job must inherit both from install.
export TOKENFETCH_DATA_DIR=$work/data CLAUDE_CONFIG_DIR=$work/claude CODEX_HOME=$work/codex
mkdir -p "$CLAUDE_CONFIG_DIR/projects/-e2e"
cp internal/source/claudecode/testdata/session.jsonl "$CLAUDE_CONFIG_DIR/projects/-e2e/"
want=$(TOKENFETCH_DATA_DIR=$work/expected "$tf" --all --json | jq .messages)
[[ $want -gt 0 ]] || fail "fixture yields no messages"

echo "--- install"
"$tf" service install --interval=daily
systemd-analyze --user verify "$units/tokenfetch.service" "$units/tokenfetch.timer" ||
  fail "systemd rejects the units"
grep -qx 'OnCalendar=daily' "$units/tokenfetch.timer" || fail "--interval not applied"
systemctl --user is-active --quiet tokenfetch.timer || fail "timer not active"
systemctl --user list-timers tokenfetch.timer --no-pager

echo "--- status"
status=$("$tf" service status)
echo "$status"
grep -q '^active · systemd · every day$' <<<"$status" || fail "status header"
grep -q 'last run: ' <<<"$status" || fail "status has no last run"
grep -q 'next run: ' <<<"$status" || fail "status has no next run"
grep -q 'last exit: ok' <<<"$status" || fail "last run failed"

# Installing again replaces the job: one timer, new schedule live in systemd,
# not just in the file.
echo "--- reinstall with another interval"
"$tf" service install --interval=hourly
grep -qx 'OnCalendar=hourly' "$units/tokenfetch.timer" || fail "reinstall kept the old interval in the unit"
calendar=$(systemctl --user show tokenfetch.timer --property=TimersCalendar --value)
echo "$calendar"
[[ $calendar == *'*-*-* *:00:00'* ]] || fail "systemd still runs the old schedule: $calendar"
[[ $(systemctl --user list-timers 'tokenfetch*' --all --no-legend | wc -l) == 1 ]] || fail "more than one timer"
"$tf" service status | grep -q '^active · systemd · every hour$' || fail "status after reinstall"

# What the job is for: usage survives Claude Code deleting the transcript.
# The report below ingests too, but there is nothing left for it to read.
echo "--- usage recorded by the job"
rm -rf "$CLAUDE_CONFIG_DIR/projects"
got=$("$tf" --all --json | jq .messages)
[[ $got == "$want" ]] || fail "job recorded $got messages, want $want"
journalctl --user -u tokenfetch.service --no-pager || true # informational: runner journals may not persist

echo "--- uninstall"
"$tf" service uninstall
[[ ! -e $units/tokenfetch.service && ! -e $units/tokenfetch.timer ]] || fail "units left behind"
systemctl --user is-active --quiet tokenfetch.timer && fail "timer still active"
"$tf" service status | grep -q '^not installed' || fail "status after uninstall"
got=$("$tf" --all --json | jq .messages)
[[ $got == "$want" ]] || fail "uninstall lost usage: $got messages, want $want"

echo "PASS"
