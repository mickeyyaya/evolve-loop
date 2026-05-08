#!/usr/bin/env bash
#
# mutate-eval.sh — Eval rigor verification via mutation testing.
#
# Mutation testing is the documented remedy for the cycle 102-111 reward-hacking
# class (tautological evals). The principle: if you mutate the source-under-test
# and the eval STILL passes, the eval isn't actually testing behavior — it's
# testing for the presence of strings. Reject those evals before they enter the
# build cycle.
#
# Pre-existing specifications:
#   - skills/evolve-loop/eval-runner.md § "Mutation Testing" (lines 237-285)
#   - docs/eval-grader-best-practices.md § "Mutation Resistance" (lines 175-197)
#     — target kill rate: ≥80%
#
# This script implements that spec for the first time. Bash-native (no mutmut
# or stryker dependency) because evolve-loop evals target many file types
# beyond Python.
#
# Usage:
#   bash scripts/verification/mutate-eval.sh <eval-file.md> [--threshold 0.8] [--mutations 3]
#   bash scripts/verification/mutate-eval.sh --version
#
# Exit codes:
#   0 — eval is rigorous (kill rate ≥ threshold)
#   1 — eval is tautological (kill rate < threshold) — RECOMMEND rejection
#   2 — anomaly: no source files inferable, or eval malformed
# 127 — required binary missing

set -uo pipefail   # No -e: we expect some commands to fail

# --- Argument parsing --------------------------------------------------------
THRESHOLD=0.8
MUTATIONS_PER_FILE=3
EVAL_FILE=""

while [ $# -gt 0 ]; do
    case "$1" in
        --threshold) THRESHOLD="$2"; shift 2 ;;
        --mutations) MUTATIONS_PER_FILE="$2"; shift 2 ;;
        --version)   echo "mutate-eval.sh v1.0"; exit 0 ;;
        --help|-h)
            sed -n '2,30p' "$0"
            exit 0 ;;
        *)
            if [ -z "$EVAL_FILE" ]; then
                EVAL_FILE="$1"
            else
                echo "[mutate-eval] unexpected arg: $1" >&2
                exit 2
            fi
            shift ;;
    esac
done

[ -n "$EVAL_FILE" ] || { echo "[mutate-eval] usage: mutate-eval.sh <eval-file>" >&2; exit 2; }
[ -f "$EVAL_FILE" ] || { echo "[mutate-eval] eval file not found: $EVAL_FILE" >&2; exit 2; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERIFY_EVAL="$REPO_ROOT/scripts/verification/verify-eval.sh"

[ -x "$VERIFY_EVAL" ] || { echo "[mutate-eval] verify-eval.sh missing or not executable" >&2; exit 127; }

log() { echo "[mutate-eval] $*" >&2; }

# --- 1. Extract eval commands -----------------------------------------------
# Reuse same parsing as verify-eval.sh for consistency.
COMMANDS=$(grep -E '^\s*-\s*`[^`]+`' "$EVAL_FILE" 2>/dev/null | sed 's/.*`\(.*\)`.*/\1/' || true)
if [ -z "$COMMANDS" ]; then
    COMMANDS=$(grep -E '^\s*-\s*command:\s*' "$EVAL_FILE" 2>/dev/null | sed 's/.*command:\s*//' | tr -d '"' || true)
fi
# Also try fenced blocks (matches the patched eval-quality-check.sh behaviour).
if [ -z "$COMMANDS" ]; then
    COMMANDS=$(awk '
        /^[[:space:]]*```/ {
            if (in_block) { in_block = 0; next }
            if ($0 ~ /^[[:space:]]*```(bash|sh|shell)?[[:space:]]*$/) { in_block = 1; next }
        }
        in_block && NF > 0 && $0 !~ /^[[:space:]]*#/ { print }
    ' "$EVAL_FILE" 2>/dev/null || true)
fi
if [ -z "$COMMANDS" ]; then
    log "ANOMALY: no eval commands found in $EVAL_FILE"
    exit 2
fi

# --- 2. Infer source files from commands ------------------------------------
# Match path patterns ending in known extensions. Repeat across all commands.
# Note: do NOT use \b word boundaries; they break on `/` since `/` is non-word.
# Pattern: capture longest run of path-safe chars ending in a known extension.
SOURCE_FILES=$(echo "$COMMANDS" | grep -oE '[a-zA-Z0-9_./-]+\.(py|sh|ts|js|md|json|yaml|yml|toml|go|rs|java|rb)' | sort -u || true)

if [ -z "$SOURCE_FILES" ]; then
    log "ANOMALY: no source file references inferable from eval commands"
    log "  commands were:"
    echo "$COMMANDS" | sed 's/^/    /' >&2
    exit 2
fi

log "candidate source files for mutation:"
echo "$SOURCE_FILES" | sed 's/^/  - /' >&2

# Filter to files that actually exist in the repo.
EXISTING_SOURCES=""
while IFS= read -r src; do
    [ -z "$src" ] && continue
    # Try as-is and prefixed with REPO_ROOT.
    if [ -f "$src" ]; then
        EXISTING_SOURCES+="$src"$'\n'
    elif [ -f "$REPO_ROOT/$src" ]; then
        EXISTING_SOURCES+="$REPO_ROOT/$src"$'\n'
    else
        log "  (skip: $src not found on disk)"
    fi
done <<< "$SOURCE_FILES"

if [ -z "$EXISTING_SOURCES" ]; then
    log "ANOMALY: none of the inferred source files exist on disk"
    exit 2
fi

# --- 3. Mutation strategies ------------------------------------------------
# Apply one of N mutation strategies to a copy of the source. Returns 0 on success,
# 1 if the mutation was a no-op (file unchanged, mutation impossible).
apply_mutation() {
    local target="$1"
    local strategy="$2"
    local backup="${target}.mutbak.$$"
    cp "$target" "$backup"

    local sed_inplace=("-i")
    if [[ "$OSTYPE" == "darwin"* ]]; then sed_inplace=("-i" ""); fi

    case "$strategy" in
        # Python
        py_flip_eq)        sed "${sed_inplace[@]}" 's/==/!=/' "$target" 2>/dev/null ;;
        py_return_none)    sed "${sed_inplace[@]}" 's/^\([[:space:]]*\)return [^N].*/\1return None/' "$target" 2>/dev/null ;;
        py_comment_if)     sed "${sed_inplace[@]}" 's/^\([[:space:]]*\)\(if\|raise\) /\1# REMOVED: \2 /' "$target" 2>/dev/null ;;
        # Shell
        sh_flip_test)      sed "${sed_inplace[@]}" 's/\[ /[ ! /' "$target" 2>/dev/null ;;
        sh_invert_exit)    sed "${sed_inplace[@]}" 's/exit 0/exit 1/' "$target" 2>/dev/null ;;
        sh_comment_set_e)  sed "${sed_inplace[@]}" 's/^set -e/# REMOVED: set -e/' "$target" 2>/dev/null ;;
        # TypeScript / JavaScript
        ts_flip_eq)        sed "${sed_inplace[@]}" 's/===/!==/' "$target" 2>/dev/null ;;
        ts_return_null)    sed "${sed_inplace[@]}" 's/^\([[:space:]]*\)return [^n;].*/\1return null;/' "$target" 2>/dev/null ;;
        ts_comment_throw)  sed "${sed_inplace[@]}" 's/^\([[:space:]]*\)throw /\1\/\/ REMOVED: throw /' "$target" 2>/dev/null ;;
        # Markdown
        md_delete_heading) sed "${sed_inplace[@]}" '/^#/d' "$target" 2>/dev/null ;;
        md_truncate)       awk -v n="$(($(wc -l < "$target") / 2))" 'NR<=n' "$target" > "${target}.tmp" && mv "${target}.tmp" "$target" ;;
        md_remove_first_word)
            sed "${sed_inplace[@]}" '0,/[A-Za-z]/{s/[A-Za-z][A-Za-z]*//}' "$target" 2>/dev/null ;;
        # JSON
        json_flip_bool)    sed "${sed_inplace[@]}" 's/: *true/: false/; s/: *false/: true/' "$target" 2>/dev/null ;;
        json_corrupt)      printf ',' >> "$target" ;;
        json_drop_first)
            python3 -c "import json,sys; d=json.load(open('$target'));
keys=list(d.keys()) if isinstance(d,dict) else None
if keys: del d[keys[0]]
open('$target','w').write(json.dumps(d))" 2>/dev/null ;;
        # YAML / TOML
        yaml_flip_bool)    sed "${sed_inplace[@]}" 's/: *true/: false/; s/: *false/: true/' "$target" 2>/dev/null ;;
        yaml_truncate)     awk -v n="$(($(wc -l < "$target") / 2))" 'NR<=n' "$target" > "${target}.tmp" && mv "${target}.tmp" "$target" ;;
        # Go / Rust / Java / Ruby — generic strategies
        generic_flip_eq)   sed "${sed_inplace[@]}" 's/ == / != /' "$target" 2>/dev/null ;;
        generic_truncate)  awk -v n="$(($(wc -l < "$target") / 2))" 'NR<=n' "$target" > "${target}.tmp" && mv "${target}.tmp" "$target" ;;
        generic_corrupt)   echo "// MUTATION: appended garbage" >> "$target" ;;
        *)
            mv "$backup" "$target"
            return 1 ;;
    esac

    # Verify the mutation actually changed the file.
    if cmp -s "$target" "$backup"; then
        mv "$backup" "$target"
        return 1   # no-op mutation
    fi
    # Keep the backup — restore_from_backup will use it. Do NOT delete here;
    # losing the backup before restore is the source of the cycle-99001 file
    # corruption incident (untracked file + premature backup delete = no restore).
    return 0
}

# Restore a file from its prior state. STRONG ORDER:
# 1. If a fresh .mutbak.$$ exists (just-applied mutation), use it — most reliable.
# 2. Otherwise try git checkout HEAD -- if file is tracked.
# 3. Otherwise FAIL LOUDLY — refuse to leave an untracked file in mutated state.
restore_from_backup() {
    local target="$1"
    if [ -f "${target}.mutbak.$$" ]; then
        mv -f "${target}.mutbak.$$" "$target"
        return 0
    fi
    if git -C "$REPO_ROOT" ls-files --error-unmatch "$target" >/dev/null 2>&1; then
        git -C "$REPO_ROOT" checkout HEAD -- "$target" 2>/dev/null && return 0
    fi
    log "CRITICAL: cannot restore $target — no backup, file is untracked. FILE IS LEFT IN MUTATED STATE."
    return 1
}

# Refuse to mutate a file that we cannot restore. This is the single most
# important safety check — without it, mutating an untracked file leaves it
# permanently corrupted (the cycle-99001 incident).
ensure_restorable() {
    local target="$1"
    if git -C "$REPO_ROOT" ls-files --error-unmatch "$target" >/dev/null 2>&1; then
        return 0   # tracked — git checkout works
    fi
    # Untracked. Snapshot a pre-mutation backup we can restore from.
    cp "$target" "${target}.mutsafe.$$"
    log "  (untracked file: pre-mutation snapshot at ${target}.mutsafe.$$)"
    return 0
}

# Final-pass restore for untracked files using the .mutsafe.$$ snapshot.
restore_untracked_safety() {
    local target="$1"
    if [ -f "${target}.mutsafe.$$" ]; then
        cp -f "${target}.mutsafe.$$" "$target"
        rm -f "${target}.mutsafe.$$"
    fi
}

# Pick mutation strategies for a given file extension.
strategies_for_file() {
    local f="$1"
    case "$f" in
        *.py)              echo "py_flip_eq py_return_none py_comment_if" ;;
        *.sh)              echo "sh_flip_test sh_invert_exit sh_comment_set_e" ;;
        *.ts|*.js)         echo "ts_flip_eq ts_return_null ts_comment_throw" ;;
        *.md)              echo "md_delete_heading md_truncate md_remove_first_word" ;;
        *.json)            echo "json_flip_bool json_corrupt json_drop_first" ;;
        *.yaml|*.yml)      echo "yaml_flip_bool yaml_truncate generic_corrupt" ;;
        *.toml)            echo "yaml_flip_bool yaml_truncate generic_corrupt" ;;
        *.go|*.rs|*.java|*.rb)
                           echo "generic_flip_eq generic_truncate generic_corrupt" ;;
        *)                 echo "generic_truncate generic_corrupt" ;;
    esac
}

# --- 4. Run eval against current state (sanity check: must pass first) ------
log "sanity check: running eval against unmodified source..."
SANITY_RESULT=$(bash "$VERIFY_EVAL" "$EVAL_FILE" 2>/dev/null)
SANITY_VERDICT=$(echo "$SANITY_RESULT" | grep -oE '"verdict": *"[A-Z]*"' | head -1 | sed 's/.*"\([A-Z]*\)"/\1/')
if [ "$SANITY_VERDICT" != "PASS" ]; then
    log "ANOMALY: eval does not pass against unmodified source (verdict=$SANITY_VERDICT)"
    log "  cannot meaningfully mutation-test an eval that is already failing"
    exit 2
fi
log "OK: eval passes against unmodified source"

# --- 5. Apply mutations and measure kill rate -------------------------------
TOTAL_MUTANTS=0
KILLED_MUTANTS=0
SURVIVED_DETAIL=""

while IFS= read -r src; do
    [ -z "$src" ] && continue
    log "mutating $src..."

    # SAFETY: ensure we can restore this file before touching it.
    ensure_restorable "$src"

    strategies=$(strategies_for_file "$src")
    n=0
    for strategy in $strategies; do
        n=$((n + 1))
        if [ "$n" -gt "$MUTATIONS_PER_FILE" ]; then break; fi

        # Apply mutation in-place; restore after each iteration.
        if ! apply_mutation "$src" "$strategy"; then
            log "  skip: $strategy was a no-op for $src"
            continue
        fi

        TOTAL_MUTANTS=$((TOTAL_MUTANTS + 1))
        # Re-run the eval against the mutated source.
        MUT_RESULT=$(bash "$VERIFY_EVAL" "$EVAL_FILE" 2>/dev/null)
        MUT_VERDICT=$(echo "$MUT_RESULT" | grep -oE '"verdict": *"[A-Z]*"' | head -1 | sed 's/.*"\([A-Z]*\)"/\1/')

        if [ "$MUT_VERDICT" = "PASS" ]; then
            # Eval STILL PASSED despite mutation → eval is not testing behavior.
            log "  SURVIVED: $strategy on $src (eval still passed)"
            SURVIVED_DETAIL+="    - mutation $strategy on $src — survived"$'\n'
        else
            log "  KILLED:   $strategy on $src (verdict $MUT_VERDICT)"
            KILLED_MUTANTS=$((KILLED_MUTANTS + 1))
        fi

        # Restore source. If this fails, abort — leaving files mutated is unacceptable.
        if ! restore_from_backup "$src"; then
            log "ABORT: cannot restore $src; further mutations on this file refused"
            break
        fi
    done

    # Final safety pass for untracked files.
    restore_untracked_safety "$src"
done <<< "$EXISTING_SOURCES"

# --- 6. Compute kill rate and verdict ---------------------------------------
if [ "$TOTAL_MUTANTS" -eq 0 ]; then
    log "ANOMALY: zero applicable mutations generated (all strategies were no-ops)"
    exit 2
fi

KILL_RATE=$(awk -v k="$KILLED_MUTANTS" -v t="$TOTAL_MUTANTS" 'BEGIN { printf "%.2f", k/t }')
PASS_THRESHOLD=$(awk -v r="$KILL_RATE" -v th="$THRESHOLD" 'BEGIN { print (r >= th) ? "PASS" : "FAIL" }')

cat <<EOF
{
  "eval_file": "$EVAL_FILE",
  "total_mutants": $TOTAL_MUTANTS,
  "killed_mutants": $KILLED_MUTANTS,
  "survived_mutants": $((TOTAL_MUTANTS - KILLED_MUTANTS)),
  "kill_rate": $KILL_RATE,
  "threshold": $THRESHOLD,
  "verdict": "$PASS_THRESHOLD"
}
EOF

if [ -n "$SURVIVED_DETAIL" ]; then
    echo
    echo "Survived mutations (eval did not detect):"
    printf '%s' "$SURVIVED_DETAIL"
fi

if [ "$PASS_THRESHOLD" = "PASS" ]; then
    log "OK: eval is rigorous (kill rate $KILL_RATE >= $THRESHOLD)"
    exit 0
else
    log "FAIL: eval is tautological (kill rate $KILL_RATE < $THRESHOLD)"
    log "  Recommend: rewrite eval to assert behavioral properties, not source-string presence."
    exit 1
fi
