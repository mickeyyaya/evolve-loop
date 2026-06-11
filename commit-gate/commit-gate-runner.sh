#!/usr/bin/env bash
# commit-gate-runner.sh — the pre-commit quality gate (self-contained).
#
# Detects the languages of changed files, auto-installs any missing linter/test
# tool, runs lint + TARGETED tests (changed packages/files only), and on full
# pass writes <repo>/.commit-gate/attestation.json bound to sha256(git diff HEAD).
#
# Enforcement of the attestation happens at the commit chokepoint:
#   - this repo:  go/internal/phases/ship/commitgate.go verifies it in
#                 `evolve ship --class manual` (bare `git commit` is ship-gated).
#   - vendored:   add a thin PreToolUse hook that re-checks the same attestation
#                 against sha256(git diff HEAD); no `evolve` binary required.
#
# Invoked by the /commit skill. Runs NO agents (a script can't) — instead it
# REQUIRES the skill to declare, via --reviewers, that a simplifier AND one
# reviewer (general code-reviewer OR the matching language reviewer) ran, and
# refuses to attest otherwise. Targets bash 3.2 (atomic mv writes, no declare -A).
#
# Usage:
#   commit-gate-runner.sh --reviewers "code-simplifier,go-reviewer" [--files "p1 p2"] [--no-install]
# Exit codes:
#   0 pass (+attestation)  1 lint/test/precondition fail  2 git/SHA fatal
#   3 tool auto-install failed   10 bad args
#
# Test seam: CG_TEST_INSTALL=fail|ok and CG_TEST_FORCE_MISSING="tool ..." make
# the auto-install path hermetic without PATH hacks.

set -uo pipefail

# ── inlined primitives ──────────────────────────────────────────────────────
cg_have() { command -v "$1" >/dev/null 2>&1; }
cg_log()  { printf '%s\n' "$*" >&2; }
cg_repo_root() { git rev-parse --show-toplevel 2>/dev/null; }

cg_sha256() {
  if cg_have shasum; then shasum -a 256 | awk '{print $1}'
  elif cg_have sha256sum; then sha256sum | awk '{print $1}'
  else return 3; fi
}

# sha256 of the raw bytes of `git diff HEAD` — byte-for-byte mirror of
# go/internal/phases/ship/audit.go:computeTreeStateSHA, the gate that verifies
# it. A tempfile (not "$(...)") avoids command-substitution newline-stripping.
cg_tree_sha() {
  local tmp rc
  tmp="$(mktemp 2>/dev/null)" || return 2
  git diff HEAD >"$tmp" 2>/dev/null; rc=$?
  if [ "$rc" -gt 1 ]; then rm -f "$tmp"; return 2; fi   # rc 0/1 normal, 128 fatal
  cg_sha256 <"$tmp"; rc=$?
  rm -f "$tmp"; return "$rc"
}

cg_changed_files() { git diff --name-only HEAD 2>/dev/null; }

cg_detect_langs() {  # file paths on stdin → langs (sorted-unique)
  local f ext lang
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    ext="${f##*.}"
    case "$ext" in
      go)             lang=go ;;
      py)             lang=python ;;
      ts|tsx)         lang=ts ;;
      js|jsx|mjs|cjs) lang=js ;;
      rs)             lang=rust ;;
      *) continue ;;
    esac
    printf '%s\n' "$lang"
  done | sort -u
}

cg_atomic_write() {  # <path>, content on stdin, crash-safe via mv
  local path="$1" tmp; tmp="${path}.tmp.$$"
  cat >"$tmp" && mv "$tmp" "$path"
}

# ── args ────────────────────────────────────────────────────────────────────
REVIEWERS=""
FILES_OVERRIDE=""
NO_INSTALL=0
while [ $# -gt 0 ]; do
  case "$1" in
    --reviewers)  REVIEWERS="${2:-}"; shift 2 ;;
    --files)      FILES_OVERRIDE="${2:-}"; shift 2 ;;
    --no-install) NO_INSTALL=1; shift ;;
    *) cg_log "[commit-gate] unknown arg: $1"; exit 10 ;;
  esac
done

ROOT="$(cg_repo_root)" || { cg_log "[commit-gate] not in a git repo"; exit 2; }
ATTEST_DIR="${CG_ATTEST_DIR:-$ROOT/.commit-gate}"
ATTEST="$ATTEST_DIR/attestation.json"

if [ -n "$FILES_OVERRIDE" ]; then
  FILES="$(printf '%s\n' $FILES_OVERRIDE)"
else
  FILES="$(cg_changed_files)"
fi
FILES="$(printf '%s\n' "$FILES" | sed '/^$/d')"
if [ -z "$FILES" ]; then
  cg_log "[commit-gate] no changed tracked files vs HEAD — nothing to gate (stage your changes first)."
  exit 1
fi
LANGS="$(printf '%s\n' "$FILES" | cg_detect_langs)"

# ── reviewer precondition: simplify + ONE review (general OR language) ───────
#
# Capability-based and ECC-aware (namespace prefix stripped, so ecc:go-reviewer
# counts as go-reviewer). Only ONE of {code-reviewer, <lang>-reviewer} is
# required — the language reviewer is the richer choice but the general one
# suffices. code-review-simplify satisfies both simplify and review at once.
REV_NORM=""
oldIFS="$IFS"; IFS=','
for r in $REVIEWERS; do
  r="${r##*:}"; r="$(printf '%s' "$r" | tr -d '[:space:]')"
  [ -n "$r" ] && REV_NORM="$REV_NORM $r "
done
IFS="$oldIFS"

cg_cap_satisfied() {  # $1 = space-separated synonyms; 0 if any present in REV_NORM
  local syn
  for syn in $1; do
    case "$REV_NORM" in *" $syn "*) return 0 ;; esac
  done
  return 1
}

# Acceptable "review" reviewers = general + the matching language reviewer(s).
REVIEW_SYN="code-reviewer code-review code-review-simplify"
for l in $LANGS; do
  case "$l" in
    go)     REVIEW_SYN="$REVIEW_SYN go-reviewer go-review" ;;
    python) REVIEW_SYN="$REVIEW_SYN python-reviewer python-review" ;;
    ts|js)  REVIEW_SYN="$REVIEW_SYN typescript-reviewer typescript-review" ;;
    rust)   REVIEW_SYN="$REVIEW_SYN rust-reviewer rust-review" ;;
  esac
done

MISSING=""
cg_cap_satisfied "code-simplifier code-review-simplify refactor" || MISSING="$MISSING simplify"
cg_cap_satisfied "$REVIEW_SYN"                                   || MISSING="$MISSING review"
if [ -n "$MISSING" ]; then
  cg_log "[commit-gate] DENY: missing required review capability:${MISSING}"
  cg_log "[commit-gate]   simplify ← code-simplifier | code-review-simplify | refactor"
  cg_log "[commit-gate]   review   ← code-reviewer | code-review | a matching <lang>-reviewer (ECC variants OK)"
  cg_log "[commit-gate] run them, then pass --reviewers (use the /commit skill)."
  exit 1
fi

# ── tool ensure (auto-install; hard-block on failure) ───────────────────────
CHECKS="$(mktemp)" || exit 2
trap 'rm -f "$CHECKS"' EXIT
pass() { printf '%s\n' "$1" >>"$CHECKS"; }

cg_ensure_tool() {  # $1 tool  $2 install-cmd  $3 manual-hint   (install-cmd="" → not auto-installable)
  local tool="$1" install="$2" manual="$3"
  case " ${CG_TEST_FORCE_MISSING:-} " in
    *" $tool "*) ;;                       # test seam: forced missing
    *) cg_have "$tool" && return 0 ;;
  esac
  if [ -z "$install" ] || [ "$NO_INSTALL" = "1" ]; then
    cg_log "[commit-gate] missing '$tool' (not auto-installable here). Install manually: $manual"; return 3
  fi
  if [ -n "${CG_TEST_INSTALL:-}" ]; then
    [ "$CG_TEST_INSTALL" = "ok" ] && return 0
    cg_log "[commit-gate] auto-install of '$tool' FAILED. Install manually: $manual"; return 3
  fi
  cg_log "[commit-gate] auto-installing missing tool: $tool"
  if eval "$install" >/dev/null 2>&1 && cg_have "$tool"; then return 0; fi
  cg_log "[commit-gate] auto-install of '$tool' FAILED. Install manually: $manual"
  return 3
}

# files_ext lists changed files with extension $1 that STILL EXIST in the working
# tree — deleted files (mass refactors, file moves) carry no lintable/testable
# content and their (possibly deleted) parent dir breaks cg_find_up's go.mod walk.
files_ext() {
  printf '%s\n' "$FILES" | grep -E "\\.$1\$" | while IFS= read -r f; do
    [ -n "$f" ] && [ -e "$ROOT/$f" ] && printf '%s\n' "$f"
  done || true
}

# Nearest ancestor dir of file $1 containing $2 (e.g. go.mod / Cargo.toml).
cg_find_up() {
  local d; d="$(cd "$(dirname "$ROOT/$1")" 2>/dev/null && pwd)" || return 1
  while [ -n "$d" ] && [ "$d" != "/" ]; do
    [ -f "$d/$2" ] && { printf '%s' "$d"; return 0; }
    d="$(dirname "$d")"
  done
  return 1
}

# ── Go lane ─────────────────────────────────────────────────────────────────
lane_go() {
  local gofiles; gofiles="$(files_ext go)"
  [ -n "$gofiles" ] || return 0
  cg_ensure_tool go "" "install Go from https://go.dev/dl" || return 3

  local unformatted="" f
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    # -s (simplify) matches CI's `gofmt -d -s` — plain gofmt passes code CI then
    # rejects (inbox cycle-gate-gofmt-not-simplify; recurring since 2026-06-05).
    [ -n "$(gofmt -s -l "$ROOT/$f" 2>/dev/null)" ] && unformatted="$unformatted $f"
  done <<EOF2
$gofiles
EOF2
  if [ -n "$unformatted" ]; then cg_log "[commit-gate] go: gofmt -s needs:$unformatted"; return 1; fi
  pass "go:gofmt"

  local map; map="$(mktemp)"; local fdir rel
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    local md; md="$(cg_find_up "$f" go.mod)" || { cg_log "[commit-gate] go: no go.mod above $f"; rm -f "$map"; return 1; }
    fdir="$(cd "$(dirname "$ROOT/$f")" && pwd)"
    rel=".${fdir#$md}"; [ "$rel" = "." ] && rel="./."
    printf '%s\t%s\n' "$md" "$rel" >>"$map"
  done <<EOF3
$gofiles
EOF3
  sort -u "$map" -o "$map"

  # EGPS predicate packages under acs/ are build-tagged (//go:build acs) state
  # assertions, NOT unit tests. Vetting/testing them in the default (untagged)
  # config fails with "build constraints exclude all Go files", and running them
  # would false-fail on historical bit-rot. They are gated by `evolve acs suite`
  # (current-cycle scope, at audit) + the tagguard test in internal/acssuite.
  # Exclude them here, mirroring CI's `go list ./... | grep -v '/acs/'`.
  awk -F'\t' '$2 !~ /^\.\/acs\//' "$map" > "$map.f" && mv "$map.f" "$map"

  local moddir glc=0
  for moddir in $(cut -f1 "$map" | sort -u); do
    local pkgs; pkgs="$(awk -F'\t' -v m="$moddir" '$1==m{print $2}' "$map" | tr '\n' ' ')"
    ( cd "$moddir" && go vet $pkgs ) || { cg_log "[commit-gate] go vet failed in $moddir"; rm -f "$map"; return 1; }
    if cg_have golangci-lint; then
      ( cd "$moddir" && golangci-lint run $pkgs ) || { cg_log "[commit-gate] golangci-lint failed"; rm -f "$map"; return 1; }
      glc=1
    fi
    ( cd "$moddir" && go test $pkgs ) || { cg_log "[commit-gate] go test failed in $moddir"; rm -f "$map"; return 1; }
  done
  rm -f "$map"
  # Record once, in execution order (gofmt already recorded before the loop).
  pass "go:vet"; [ "$glc" = 1 ] && pass "go:golangci-lint"; pass "go:test"
}

# ── Python lane (best-effort targeted) ──────────────────────────────────────
lane_python() {
  local pyfiles; pyfiles="$(files_ext py)"
  [ -n "$pyfiles" ] || return 0
  cg_ensure_tool ruff "python3 -m pip install --user ruff" "pip install ruff" || return 3
  ( cd "$ROOT" && printf '%s\n' "$pyfiles" | sed "s#^#$ROOT/#" | xargs ruff check ) \
    || { cg_log "[commit-gate] ruff failed"; return 1; }
  pass "python:ruff"
  if printf '%s\n' "$pyfiles" | grep -Eq '(^|/)(test_.*|.*_test)\.py$'; then
    cg_ensure_tool pytest "python3 -m pip install --user pytest" "pip install pytest" || return 3
    local tests; tests="$(printf '%s\n' "$pyfiles" | grep -E '(^|/)(test_.*|.*_test)\.py$' | sed "s#^#$ROOT/#" | tr '\n' ' ')"
    ( cd "$ROOT" && pytest -q $tests ) || { cg_log "[commit-gate] pytest failed"; return 1; }
    pass "python:pytest"
  fi
}

# ── TS/JS lane (best-effort) ────────────────────────────────────────────────
lane_node() {
  local nfiles; nfiles="$(printf '%s\n' "$FILES" | grep -E '\.(ts|tsx|js|jsx|mjs|cjs)$' || true)"
  [ -n "$nfiles" ] || return 0
  if cg_have eslint || cg_have npx; then
    local bin="eslint"; cg_have eslint || bin="npx eslint"
    ( cd "$ROOT" && $bin $(printf '%s ' $nfiles) ) || { cg_log "[commit-gate] eslint failed"; return 1; }
    pass "node:eslint"
  else
    cg_log "[commit-gate] eslint/npx absent. Install manually: npm install"; return 3
  fi
}

# ── Rust lane (best-effort) ─────────────────────────────────────────────────
lane_rust() {
  local rfiles; rfiles="$(files_ext rs)"
  [ -n "$rfiles" ] || return 0
  cg_ensure_tool cargo "" "install Rust via https://rustup.rs" || return 3
  local f md seen=""
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    md="$(cg_find_up "$f" Cargo.toml)" || continue
    case " $seen " in *" $md "*) continue ;; esac
    seen="$seen $md"
    ( cd "$md" && cargo fmt --check && cargo clippy -- -D warnings && cargo test ) \
      || { cg_log "[commit-gate] cargo checks failed in $md"; return 1; }
  done <<EOF4
$rfiles
EOF4
  pass "rust:fmt"; pass "rust:clippy"; pass "rust:test"
}

# ── run lanes ───────────────────────────────────────────────────────────────
for l in $LANGS; do
  case "$l" in
    go)     lane_go ;;
    python) lane_python ;;
    ts|js)  lane_node ;;
    rust)   lane_rust ;;
  esac
  rc=$?
  [ "$rc" -eq 0 ] || exit "$rc"
done

# ── write attestation ───────────────────────────────────────────────────────
TREE_SHA="$(cg_tree_sha)" || { cg_log "[commit-gate] cannot compute tree SHA"; exit 2; }
SHATOOL="shasum"; cg_have shasum || SHATOOL="sha256sum"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
checks_json="$(awk 'BEGIN{p=""} {printf "%s\"%s\"",p,$0; p=","}' "$CHECKS")"
rev_json="$(printf '%s' "$REVIEWERS" | awk -F',' '{p="";for(i=1;i<=NF;i++){if($i!=""){printf "%s\"%s\"",p,$i;p=","}}}')"

mkdir -p "$ATTEST_DIR"
cg_atomic_write "$ATTEST" <<JSON
{
  "tree_state_sha": "$TREE_SHA",
  "ts": "$TS",
  "checks_passed": [$checks_json],
  "reviewers_run": [$rev_json],
  "tool": "$SHATOOL"
}
JSON

cg_log "[commit-gate] PASS — attestation written ($TREE_SHA)"
exit 0
