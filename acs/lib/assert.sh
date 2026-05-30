#!/usr/bin/env bash
# acs/lib/assert.sh — shared ACS predicate assertion helpers.
#
# WHY THIS EXISTS (cycle-137 lesson): ACS predicates are authored fresh by
# the TDD-Engineer every cycle, and they keep hand-rolling "did this Go
# test pass?" subtly wrong a new way each time:
#   - cycle-131: `^--- PASS:` anchor missed INDENTED subtests (undercount).
#   - cycle-137: `grep -q 'PASS'` on NON-verbose `go test` output — a
#     passing package prints `ok <pkg>`, with no `PASS` token at all →
#     false RED on predicates 004/005.
#
# The durable fix is a single source of truth: source this lib and call
# assert_go_test_pass / assert_go_coverage_ge instead of grepping output.
# "Did a Go test pass?" is then implemented correctly ONCE, here, with its
# own tests (acs/lib/assert_test.sh), rather than re-derived per cycle.
#
# Design: assertions key off `go test`'s EXIT CODE (the authoritative
# signal), never on scraping `PASS`/`ok` strings. Coverage parsing is
# isolated into a pure function (acs_coverage_pct) that the tests cover
# directly without invoking the Go toolchain.
#
# Bash 3.2 compatible. Source it; do not execute it:
#   . "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
#
# Convention: every function echoes a one-line GREEN:/RED: diagnostic to
# stderr and returns 0 (pass) or 1 (fail), so a predicate is simply:
#   assert_go_test_pass ./internal/cyclehealth/... 'TestCountFieldDuplicates'

set -uo pipefail

# acs_coverage_pct: extract the coverage percentage from `go test -cover`
# output passed on stdin. Echoes the bare number (e.g. "93.8"); echoes
# nothing and returns 1 when no coverage line is present. PURE: no
# subprocess, directly unit-tested. Handles both shapes Go emits:
#   "coverage: 93.8% of statements"
#   "ok  pkg 1.4s  coverage: 93.8% of statements"
acs_coverage_pct() {
  # sed extracts the digits between "coverage: " and "%". BSD/GNU portable.
  sed -n 's/.*coverage: \([0-9][0-9]*\(\.[0-9][0-9]*\)\{0,1\}\)%.*/\1/p' | head -1
}

# acs_pct_ge A B: pure numeric "A >= B" for percentages with one decimal,
# without bc/awk floating point (multiply by 10, integer compare). Echoes
# nothing; returns 0 if A >= B, else 1. Empty A is treated as 0.
acs_pct_ge() {
  local a="${1:-0}" b="${2:-0}"
  # Strip a trailing "%" if a caller passed one.
  a="${a%\%}"; b="${b%\%}"
  local ai bi
  ai=$(printf '%.0f' "$(echo "$a" | awk '{print $1*10}')" 2>/dev/null) || ai=0
  bi=$(printf '%.0f' "$(echo "$b" | awk '{print $1*10}')" 2>/dev/null) || bi=0
  [ "$ai" -ge "$bi" ]
}

# acs_go_module_dir: echo the directory `go test` should run in. ACS
# predicates are sourced from the repo root (git toplevel), but the Go
# module lives in <toplevel>/go (no go.mod at the repo root), so a bare
# `go test ./internal/...` from the predicate's cwd fails with "no
# required module" — which would itself be a false RED, the exact footgun
# this lib exists to kill. Resolve <toplevel>/go when it has a go.mod,
# else fall back to the toplevel (for repos whose module IS the root).
acs_go_module_dir() {
  local top
  top=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "."; return; }
  if [ -f "$top/go/go.mod" ]; then
    echo "$top/go"
  else
    echo "$top"
  fi
}

# assert_go_test_pass <pkg> [run-regex]: run `go test -race` for the
# package (optionally a single -run regex) and assert it EXITS 0. The exit
# code is the authoritative pass/fail signal — never scrape stdout. Runs in
# the resolved module dir (acs_go_module_dir) so it works regardless of the
# predicate's cwd; <pkg> is module-relative (e.g. ./internal/cyclehealth/...).
assert_go_test_pass() {
  local pkg="${1:?assert_go_test_pass: pkg required}" re="${2:-}"
  local out rc dir
  dir=$(acs_go_module_dir)
  if [ -n "$re" ]; then
    out=$(cd "$dir" && go test -race -count=1 -run "$re" "$pkg" 2>&1); rc=$?
  else
    out=$(cd "$dir" && go test -race -count=1 "$pkg" 2>&1); rc=$?
  fi
  if [ "$rc" -eq 0 ]; then
    echo "GREEN: go test ${re:+-run $re }$pkg exited 0" >&2
    return 0
  fi
  echo "RED: go test ${re:+-run $re }$pkg exited $rc" >&2
  echo "$out" | tail -5 >&2
  return 1
}

# assert_go_build [pkg]: run `go build` for the package (default ./...) and
# assert it EXITS 0 — the build is broken otherwise. The exit code is the
# authoritative signal — never scrape stdout. Runs in the resolved module dir
# (acs_go_module_dir) so it works regardless of the predicate's cwd; <pkg> is
# module-relative (default ./...). Companion to assert_go_test_pass for
# predicates that need to pin "the package still compiles".
assert_go_build() {
  local pkg="${1:-./...}"
  local out rc dir
  dir=$(acs_go_module_dir)
  out=$(cd "$dir" && go build "$pkg" 2>&1); rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "GREEN: go build $pkg exited 0" >&2
    return 0
  fi
  echo "RED: go build $pkg exited $rc" >&2
  echo "$out" | tail -5 >&2
  return 1
}

# assert_go_coverage_ge <pkg> <min-pct>: run `go test -cover` for the
# package and assert measured coverage >= min. Uses acs_coverage_pct +
# acs_pct_ge so the brittle field-extraction is the tested pure function,
# not inline grep/awk in each predicate (the cycle-137 008 footgun).
assert_go_coverage_ge() {
  local pkg="${1:?assert_go_coverage_ge: pkg required}" min="${2:?min pct required}"
  local out pct dir
  dir=$(acs_go_module_dir)
  out=$(cd "$dir" && go test -cover -count=1 "$pkg" 2>&1)
  pct=$(echo "$out" | acs_coverage_pct)
  if [ -z "$pct" ]; then
    echo "RED: no coverage line in output for $pkg" >&2
    echo "$out" | tail -5 >&2
    return 1
  fi
  if acs_pct_ge "$pct" "$min"; then
    echo "GREEN: $pkg coverage ${pct}% >= ${min}%" >&2
    return 0
  fi
  echo "RED: $pkg coverage ${pct}% < ${min}%" >&2
  return 1
}
