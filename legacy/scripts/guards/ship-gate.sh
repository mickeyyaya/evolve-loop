#!/usr/bin/env bash
#
# ship-gate.sh — PreToolUse hook for Claude Code Bash tool calls.
#
# v8.13.0 architectural simplification: instead of trying to detect
# ship-class commands (git commit / git push / gh release create) inside
# arbitrary bash command strings via a parser (which kept losing the arms
# race in cycles 8121-8122 audits), this gate allowlists exactly ONE
# canonical script: legacy/scripts/lifecycle/ship.sh.
#
# Logic:
#   1. Extract command from JSON payload (Claude Code passes one per Bash call).
#   2. If command is empty / not parseable → allow (passthrough).
#   3. If command's first executable, resolved via realpath, equals
#      $REPO_ROOT/legacy/scripts/lifecycle/ship.sh → allow (ship.sh enforces audit contract internally).
#   4. If command contains ship verbs (git commit/push, gh release create/edit) at
#      a tokenizable boundary → deny.
#   5. Otherwise → allow.
#
# The realpath check is the canonical decision; the regex is belt-and-suspenders
# against weird shell forms (chained commands, subshells, etc.). ship.sh's
# own self-SHA verification (TOFU pattern) defends against in-place modification.
#
# Bypass: EVOLVE_BYPASS_SHIP_GATE=1 — emergency only, logged with WARN.
#
# Exit codes:
#   0 — allow
#   2 — deny (Claude Code surfaces our stderr to the LLM)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
GUARDS_LOG="$REPO_ROOT/.evolve/guards.log"
SHIP_SH="$REPO_ROOT/legacy/scripts/lifecycle/ship.sh"

mkdir -p "$(dirname "$GUARDS_LOG")"

log() {
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$ts] [ship-gate] $*" >> "$GUARDS_LOG"
}

# Read JSON payload from stdin. Claude Code passes:
#   {"tool_input":{"command":"...","description":"..."},"session_id":"...","cwd":"..."}
PAYLOAD="$(cat || true)"
if [ -z "$PAYLOAD" ]; then
    log "no-payload (manual invocation?); ALLOW"
    exit 0
fi

# Extract the command. Use jq if available; fall back to a regex parser.
COMMAND=""
if command -v jq >/dev/null 2>&1; then
    COMMAND=$(echo "$PAYLOAD" | jq -r '.tool_input.command // empty' 2>/dev/null || true)
fi
if [ -z "$COMMAND" ]; then
    COMMAND=$(echo "$PAYLOAD" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
fi

if [ -z "$COMMAND" ]; then
    log "no command in payload; ALLOW"
    exit 0
fi

# Bypass switch (emergency rollback only).
if [ "${EVOLVE_BYPASS_SHIP_GATE:-0}" = "1" ]; then
    log "WARN: EVOLVE_BYPASS_SHIP_GATE=1 — bypassing for: ${COMMAND:0:80}"
    echo "[ship-gate] WARN: bypass active; gate not enforcing" >&2
    exit 0
fi

# --- Step 1: canonical-path check (the PRIMARY decision) ---------------------
#
# Strip leading whitespace, extract the first non-whitespace token (or the
# second if it's `bash`), resolve via realpath / cd-pwd fallback. If it
# resolves to legacy/scripts/lifecycle/ship.sh, allow.

# Determine which token to resolve.
TRIMMED="${COMMAND#"${COMMAND%%[![:space:]]*}"}"   # left-trim
FIRST_TOKEN=$(echo "$TRIMMED" | awk '{print $1}')
TARGET_TOKEN=""
case "$FIRST_TOKEN" in
    bash|sh|/bin/bash|/bin/sh|/usr/bin/env)
        # If the first token is a shell, the SECOND token is the script path.
        # `env bash legacy/scripts/lifecycle/ship.sh` → resolve "legacy/scripts/lifecycle/ship.sh".
        # Take whichever token is the first non-flag, non-shell token.
        TARGET_TOKEN=$(echo "$TRIMMED" | awk '
            {
                start = 2
                if ($1 == "/usr/bin/env") start = 3   # env <interp> <script>
                for (i = start; i <= NF; i++) {
                    # Skip flags
                    if ($i ~ /^-/) continue
                    print $i
                    exit
                }
            }
        ')
        ;;
    *)
        TARGET_TOKEN="$FIRST_TOKEN"
        ;;
esac

# Resolve via realpath if available, else fallback to cd-pwd. Both follow
# symlinks to a canonical absolute path.
RESOLVED=""
if [ -n "$TARGET_TOKEN" ]; then
    if command -v realpath >/dev/null 2>&1; then
        RESOLVED=$(realpath "$TARGET_TOKEN" 2>/dev/null || echo "")
    fi
    if [ -z "$RESOLVED" ]; then
        # Portable fallback for systems without realpath (older macOS without
        # coreutils). Use python3 if available; else perl; else manual.
        if command -v python3 >/dev/null 2>&1; then
            RESOLVED=$(python3 -c "import os, sys; print(os.path.realpath(sys.argv[1]))" "$TARGET_TOKEN" 2>/dev/null || echo "")
        fi
    fi
fi

# If resolved equals ship.sh's canonical path, run C1 pre-commit tree-SHA guard
# then allow. Guard is skipped for --class manual/release (no cycle audit binding).
if [ -n "$RESOLVED" ] && [ "$RESOLVED" = "$SHIP_SH" ]; then
    _skip_guard=0
    case "$COMMAND" in
        *--class\ manual*|*--class\ release*) _skip_guard=1 ;;
    esac
    if [ "$_skip_guard" = "0" ]; then
        _ledger="$REPO_ROOT/.evolve/ledger.jsonl"
        _state="$REPO_ROOT/.evolve/cycle-state.json"
        if [ -f "$_ledger" ] && [ -f "$_state" ] && command -v jq >/dev/null 2>&1; then
            _audit_report=$(grep '"kind":"agent_subprocess"' "$_ledger" 2>/dev/null \
                | jq -r 'select(.role=="auditor") | .artifact_path' 2>/dev/null \
                | tail -1)
            _worktree=$(jq -r '.active_worktree // empty' "$_state" 2>/dev/null || echo "")
            if [ -n "$_audit_report" ] && [ -f "$_audit_report" ] && [ -n "$_worktree" ]; then
                _audit_sha=$(grep -m1 'audit_bound_tree_sha:' "$_audit_report" \
                    | awk '{print $NF}' | tr -d '[:space:]')
                _wt_sha=$(git -C "$_worktree" rev-parse HEAD^{tree} 2>/dev/null || echo "")
                if [ -n "$_audit_sha" ] && [ -n "$_wt_sha" ] && [ "$_audit_sha" != "$_wt_sha" ]; then
                    log "DENY: C1 pre-commit tree-SHA mismatch: audit_bound=$_audit_sha worktree=$_wt_sha"
                    echo "[ship-gate] DENY: INTEGRITY BREACH (pre-commit): audit-bound tree SHA $_audit_sha != worktree tree SHA $_wt_sha — worktree was modified after audit. Re-run Auditor." >&2
                    exit 2
                fi
                log "C1 pre-commit tree-SHA guard: OK (audit=$_audit_sha wt=$_wt_sha)"
            fi
            # v10.15.0 Cycle A (trust-kernel hardening — cycle-93 breach fix):
            # Builder commit-SHA self-attestation check. If build-report.md
            # declares a `Commit:` field, it MUST equal the worktree HEAD.
            # Cycle-93 breach: Builder hit turn-overrun, sourced Commit: from
            # intent.md (literal) rather than $(git rev-parse HEAD), and the
            # fabricated SHA referenced a commit that didn't yet exist.
            # Conditional: absence of the field is allowed (most pre-cycle-93
            # build-reports don't have it). Presence + mismatch = DENY.
            _build_report=$(grep '"kind":"agent_subprocess"' "$_ledger" 2>/dev/null \
                | jq -r 'select(.role=="builder") | .artifact_path' 2>/dev/null \
                | tail -1)
            if [ -n "$_build_report" ] && [ -f "$_build_report" ] && [ -n "$_worktree" ]; then
                _build_commit=$(grep -m1 -iE '\*?\*?Commit:?\*?\*?[[:space:]]*`?[0-9a-f]{7,40}`?' "$_build_report" 2>/dev/null \
                    | grep -oE '[0-9a-f]{7,40}' | head -1)
                if [ -n "$_build_commit" ]; then
                    _wt_commit=$(git -C "$_worktree" rev-parse HEAD 2>/dev/null || echo "")
                    # Resolve build_commit via `git rev-parse` to get the full
                    # 40-char SHA AND reject ambiguous prefixes (rev-parse exits
                    # non-zero if the short SHA matches multiple objects). This
                    # fixes Finding 4 from code-review: prefix-match `case` was
                    # ambiguous under adversarial input — a Builder could emit
                    # a 7-char prefix that matched the real worktree HEAD by
                    # collision rather than by reference. rev-parse is the
                    # canonical resolver for both short→full normalization and
                    # ambiguity detection.
                    _build_commit_full=$(git -C "$_worktree" rev-parse --verify "$_build_commit" 2>/dev/null || echo "")
                    if [ -n "$_wt_commit" ] && [ -n "$_build_commit_full" ]; then
                        if [ "$_wt_commit" = "$_build_commit_full" ]; then
                            log "C1 builder commit-SHA self-attestation: OK (build=$_build_commit resolved=$_build_commit_full wt=$_wt_commit)"
                        else
                            log "DENY: builder commit-SHA mismatch: build-report=$_build_commit resolved=$_build_commit_full worktree=$_wt_commit"
                            echo "[ship-gate] DENY: INTEGRITY BREACH (commit-SHA): build-report.md declares Commit: $_build_commit (resolves to $_build_commit_full) but worktree HEAD is $_wt_commit — fabricated build-report (cycle-93 breach mode). Re-run Builder against actual worktree HEAD." >&2
                            exit 2
                        fi
                    elif [ -n "$_wt_commit" ] && [ -z "$_build_commit_full" ]; then
                        log "DENY: builder commit-SHA unresolvable: build-report=$_build_commit could not be resolved by git rev-parse (ambiguous prefix or unknown object)"
                        echo "[ship-gate] DENY: INTEGRITY BREACH (commit-SHA): build-report.md declares Commit: $_build_commit but git rev-parse cannot resolve it — ambiguous prefix or fabricated SHA (cycle-93 breach mode). Re-run Builder against actual worktree HEAD." >&2
                        exit 2
                    fi
                    unset _wt_commit _build_commit_full
                fi
                unset _build_commit
            fi
            unset _build_report
        fi
    fi
    unset _skip_guard _ledger _state _audit_report _worktree _audit_sha _wt_sha
    log "ALLOW: canonical ship.sh: ${COMMAND:0:80}"
    exit 0
fi

# --- Step 1.5: bash -c / sh -c shell-string extraction (D-NEW-1 fix) --------
#
# `bash -c "git commit -m foo"` was a HIGH bypass in cycle 8130 RC1: the
# canonical-path resolver extracted `"git` (the literal quoted token) as
# TARGET_TOKEN, realpath failed, and the regex below missed the match
# because the character preceding `git` was `"` (not in the boundary class).
# Fix: when the first token is a shell interpreter AND -c is present,
# extract the quoted command string and inspect it directly.
INNER_CMD=""
# Use bash glob-pattern case to match ANY path ending in a shell interpreter
# name OR being `env`/`/usr/bin/env`/etc. v8.13.0 RC1-RC3 audits found that
# enumerating individual shell paths (bash, /bin/bash, ...) loses an arms race
# — D-NEW-6 used `/usr/bin/env bash -c` to slip past the enumerated list.
# The glob pattern catches: bash, /bin/bash, /usr/local/bin/bash,
# /opt/homebrew/bin/zsh, env, /usr/bin/env, env-style wrappers, etc.
case "$FIRST_TOKEN" in
    bash|sh|zsh|dash|ksh|ash|*/bash|*/sh|*/zsh|*/dash|*/ksh|*/ash|env|*/env|nice|nohup|time|xargs|stdbuf|timeout|*/nice|*/nohup|*/time|*/xargs|*/stdbuf|*/timeout)
        # Walk ALL tokens looking for -c (or short flag combination ending in c
        # like -ec, -xc). When found, treat the rest of the line as the inner
        # snippet. The walk handles all forms uniformly:
        #   bash -c "..."                    (i=2)
        #   bash -x -c "..."                 (i=3, flag at $2 skipped)
        #   env bash -c "..."                (i=3, env at $1 + bash at $2 skipped)
        #   nice bash -c "..."               (i=3, nice + bash skipped)
        #   /usr/bin/env -i bash -c "..."    (i=4 or 5)
        #   xargs -I{} bash -c "..."         (i=4)
        # No static `start` offset needed — the walk just looks for -c at any
        # token position and ignores anything before it.
        # v8.13.0 RC4 audit (D-NEW-7) added utility wrappers (nice, nohup,
        # time, xargs, stdbuf, timeout) to the case statement so they don't
        # fall through to Step 2 where the quote-boundary design rule
        # intentionally allows quoted ship-verb strings (Test 8: grep "git commit").
        INNER_CMD=$(echo "$TRIMMED" | awk '
            {
                i = 2
                while (i <= NF) {
                    tok = $i
                    # Match -c exactly OR -<letters>c (e.g., -ec, -xc, -ic)
                    if (tok == "-c" || tok ~ /^-[A-Za-z]+c$/) {
                        # Inner command is everything from $(i+1) onward.
                        n = 0
                        for (j = i+1; j <= NF; j++) {
                            if (n > 0) printf " "
                            printf "%s", $j
                            n++
                        }
                        exit
                    }
                    i++
                }
            }
        ')
        # Strip surrounding quotes (single or double) from the snippet
        INNER_CMD=$(echo "$INNER_CMD" | sed -E 's/^"//;s/"$//;s/^\x27//;s/\x27$//')
        ;;
    eval|*/eval)
        # eval "git commit ..." — same recursion target. Strip the first
        # whitespace-separated arg's quotes.
        INNER_CMD=$(echo "$TRIMMED" | sed -E 's/^[[:space:]]*[^[:space:]]+[[:space:]]+//;s/^"//;s/"$//;s/^\x27//;s/\x27$//')
        ;;
esac

if [ -n "$INNER_CMD" ]; then
    log "extracted inner shell snippet from $FIRST_TOKEN: ${INNER_CMD:0:80}"
    # Recurse the regex check against the inner snippet. Use word-boundary
    # detection that ALSO treats start-of-string and end-of-string as
    # boundaries — the inner snippet's first char IS the verb.
    if echo "$INNER_CMD" | grep -qE '(^|[[:space:];&|()`"\x27])(git[[:space:]]+(commit|push)|gh[[:space:]]+release[[:space:]]+(create|edit))([[:space:]]|$|[\x27"])'; then
        log "DENY: $FIRST_TOKEN with ship-class inner command: ${COMMAND:0:120}"
        {
            echo "[ship-gate] DENY: ship-class commands inside $FIRST_TOKEN -c \"...\" must go through legacy/scripts/lifecycle/ship.sh"
            echo "[ship-gate] Use: bash legacy/scripts/lifecycle/ship.sh \"<commit-message>\""
            echo "[ship-gate] To bypass (emergency only): EVOLVE_BYPASS_SHIP_GATE=1"
        } >&2
        exit 2
    fi
fi

# --- Step 2: ship-verb detection (defense in depth) -------------------------
#
# Reach for the regex only if the canonical path didn't match. Catches:
#   git commit ...
#   git push ...
#   gh release create ...
#   gh release edit ...
# at a tokenizable boundary (start, whitespace, ;, &, |, &&, ||, (, `, etc.).
#
# Pre-process: strip heredoc bodies first. A markdown build report passed via
# `cat > x.md <<EOF\n...git commit...\nEOF` contains "git commit" as data,
# not code — and that data sits at a newline boundary that the regex would
# wrongly flag. The awk pre-processor below removes content between heredoc
# start (<<MARKER, <<-MARKER, <<'MARKER', <<"MARKER") and the matching marker
# on its own line.

STRIPPED=$(printf '%s' "$COMMAND" | awk '
    BEGIN { in_heredoc = 0; marker = "" }
    {
        if (in_heredoc) {
            stripped = $0
            sub(/^[[:space:]]+/, "", stripped)
            if (stripped == marker) {
                in_heredoc = 0
                print $0
            }
            # else: skip (heredoc body content not emitted)
            next
        }
        # Detect heredoc start: <<MARKER or <<-MARKER (optionally quoted)
        if (match($0, /<<-?[[:space:]]*("|\x27)?[A-Za-z_][A-Za-z0-9_]*("|\x27)?/)) {
            decl = substr($0, RSTART, RLENGTH)
            tmp = decl
            gsub(/<<-?[[:space:]]*/, "", tmp)
            gsub(/("|\x27)/, "", tmp)
            marker = tmp
            in_heredoc = 1
        }
        print
    }
')

if echo "$STRIPPED" | grep -qE '(^|[[:space:];&|()`])(git[[:space:]]+(commit|push)|gh[[:space:]]+release[[:space:]]+(create|edit))([[:space:]]|$)'; then
    log "DENY: ship-class command outside legacy/scripts/lifecycle/ship.sh: ${COMMAND:0:120}"
    {
        echo "[ship-gate] DENY: ship-class commands (git commit, git push, gh release create) must go through legacy/scripts/lifecycle/ship.sh"
        echo "[ship-gate] Use: bash legacy/scripts/lifecycle/ship.sh \"<commit-message>\""
        echo "[ship-gate] To bypass (emergency only): EVOLVE_BYPASS_SHIP_GATE=1"
    } >&2
    exit 2
fi

# Passthrough for everything else.
log "ALLOW: non-ship command: ${COMMAND:0:80}"
exit 0
