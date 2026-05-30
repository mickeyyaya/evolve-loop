#!/usr/bin/env bash
#
# commit-gate-test.sh — exercises commit-gate/commit-gate-runner.sh over
# ephemeral git repos. Mirrors the tests/markdown-structure-test.sh harness
# (PASS/FAIL counters, mktemp -d, trap cleanup, prints N/N PASS).
#
# Scope: the RUNNER (lint + targeted tests + reviewer precondition + attestation
# + auto-install). The attestation's ENFORCEMENT at commit time is covered by
# go/internal/phases/ship/commitgate_test.go (the --class manual gate).

set -uo pipefail

# Script lives at go/test/commitgate/ — repo root is three levels up.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
RUNNER="$REPO_ROOT/commit-gate/commit-gate-runner.sh"

PASS=0; FAIL=0
ok()   { PASS=$((PASS + 1)); echo "PASS  $1"; }
fail() { FAIL=$((FAIL + 1)); echo "FAIL  $1: $2"; }

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

mk_repo() {
  local d; d="$(mktemp -d "$WORK/repo.XXXXXX")"
  git -C "$d" init -q
  git -C "$d" config user.email t@t.t; git -C "$d" config user.name t
  printf 'module ex\n\ngo 1.23\n' > "$d/go.mod"
  printf 'package ex\n\nfunc Add(a, b int) int { return a + b }\n' > "$d/calc.go"
  git -C "$d" add -A
  git -C "$d" -c commit.gpgsign=false commit -qm init
  printf '%s' "$d"
}

run_runner() {  # $1 repo  $2 reviewers  $3.. extra args  -> echoes exit code; stderr/stdout discarded
  local repo="$1" rev="$2"; shift 2
  ( cd "$repo" && bash "$RUNNER" --no-install --reviewers "$rev" "$@" >/dev/null 2>&1; echo $? )
}

att_sha() { grep -oE '[0-9a-f]{64}' "$1" 2>/dev/null | head -1; }   # only the tree_state_sha is 64-hex
manual_sha() { ( cd "$1" && git diff HEAD > "$WORK/d.$$" 2>/dev/null; shasum -a 256 < "$WORK/d.$$" | awk '{print $1}'; rm -f "$WORK/d.$$" ); }

CLEAN2='package ex\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Sub(a, b int) int { return a - b }\n'

# ── T1: clean pass → attestation written + tree_state_sha matches manual ────
r="$(mk_repo)"; printf "$CLEAN2" > "$r/calc.go"
rc="$(run_runner "$r" "code-simplifier,go-reviewer")"
att="$r/.commit-gate/attestation.json"
if [ "$rc" = "0" ] && [ -f "$att" ] && [ "$(att_sha "$att")" = "$(manual_sha "$r")" ]; then
  ok "T1 clean pass writes attestation bound to sha256(git diff HEAD)"
else fail "T1" "rc=$rc att=$(att_sha "$att") manual=$(manual_sha "$r")"; fi

# ── T2: simplifier only (no reviewer) → DENY, no attestation ────────────────
r="$(mk_repo)"; printf "$CLEAN2" > "$r/calc.go"
rc="$(run_runner "$r" "code-simplifier")"
[ "$rc" = "1" ] && [ ! -f "$r/.commit-gate/attestation.json" ] \
  && ok "T2 missing review capability → exit 1, no attestation" || fail "T2" "rc=$rc"

# ── T3: gofmt violation → exit 1, no attestation ────────────────────────────
r="$(mk_repo)"; printf 'package ex\nfunc Bad( x int )int{return x}\n' > "$r/calc.go"
rc="$(run_runner "$r" "code-simplifier,go-reviewer")"
[ "$rc" = "1" ] && [ ! -f "$r/.commit-gate/attestation.json" ] \
  && ok "T3 gofmt violation → exit 1, no attestation" || fail "T3" "rc=$rc"

# ── T4: ONE review via general code-reviewer (no language reviewer) → pass ──
r="$(mk_repo)"; printf "$CLEAN2" > "$r/calc.go"
rc="$(run_runner "$r" "code-simplifier,code-reviewer")"
[ "$rc" = "0" ] && ok "T4 general code-reviewer alone satisfies 'one review'" || fail "T4" "rc=$rc"

# ── T5: ECC-prefixed + language reviewer satisfies 'one review' → pass ──────
r="$(mk_repo)"; printf "$CLEAN2" > "$r/calc.go"
rc="$(run_runner "$r" "ecc:code-simplifier,ecc:go-reviewer")"
[ "$rc" = "0" ] && ok "T5 ecc:go-reviewer (prefix stripped) satisfies 'one review'" || fail "T5" "rc=$rc"

# ── T6: auto-install failure → exit 3 + manual command (hermetic seam) ──────
r="$(mk_repo)"; printf "$CLEAN2" > "$r/calc.go"
out="$( cd "$r" && CG_TEST_FORCE_MISSING="go" CG_TEST_INSTALL="fail" bash "$RUNNER" --reviewers "code-simplifier,go-reviewer" 2>&1 )"; rc=$?
if [ "$rc" -eq 3 ] && printf '%s' "$out" | grep -q "Install manually"; then
  ok "T6 forced-missing tool → exit 3 + manual command"
else fail "T6" "rc=$rc out=[$out]"; fi

echo
echo "commit-gate: $PASS/$((PASS + FAIL)) PASS"
[ "$FAIL" -eq 0 ]
