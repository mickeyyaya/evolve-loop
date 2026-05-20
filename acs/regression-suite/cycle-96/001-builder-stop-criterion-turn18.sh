#!/usr/bin/env bash
# AC-ID: cycle-96-001
# Description: Builder STOP CRITERION has turn-18 hard exit + checkpoint commit rule; builder.json turn_budget_hint==20
# Evidence: agents/evolve-builder.md L205-228 (STOP CRITERION section); .evolve/profiles/builder.json L68 (turn_budget_hint)
# Author: tdd-engineer (cycle-96 RED phase)
# Created: 2026-05-20
# Acceptance-of: triage-decision.md T1 AC1+AC2 — STOP CRITERION strengthening
#
# Verifies T1 acceptance criteria from triage-decision.md:
#   AC1: agents/evolve-builder.md STOP CRITERION section contains a
#        checkpoint commit rule AND a "turn 18" hard exit trigger.
#   AC2: .evolve/profiles/builder.json turn_budget_hint == 20 (no regression).
#
# Why two anchors in one predicate: AC1 (persona prompt) and AC2 (profile
# field) are coupled — the persona's "turn 18" exit is meaningless if the
# profile's turn_budget_hint regresses. Bundling catches the "edited persona,
# forgot profile" failure mode that cycle-95 hit (mastery gate shipped but
# state.json increment path broken).
#
# Behavioral, not grep-only: invokes jq subprocess against the profile JSON
# (real parsing, not text scan) AND greps the persona file for the textual
# anchor pair. The jq invocation is the behavioral leg; the persona grep is
# the mixed sanity leg (see acs/AGENTS.md "Mixed: ACCEPTABLE").
#
# Bash 3.2 compatible. Hermetic (no network, no sleeps).
#
# Exit codes:
#   0 = GREEN (STOP CRITERION strengthened + turn_budget_hint preserved)
#   1 = RED   (any anchor missing)

set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
[ -d "$REPO_ROOT" ] || { echo "RED: REPO_ROOT not a directory: $REPO_ROOT" >&2; exit 1; }
cd "$REPO_ROOT" || { echo "RED: cd failed: $REPO_ROOT" >&2; exit 1; }

BUILDER_MD="agents/evolve-builder.md"
BUILDER_PROFILE=".evolve/profiles/builder.json"

# ── Existence + git-tracking dual-check (cycle-93 pattern) ─────────────────
for f in "$BUILDER_MD" "$BUILDER_PROFILE"; do
  if [ ! -f "$f" ]; then
    echo "RED: $f missing on disk" >&2
    exit 1
  fi
  if ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    echo "RED: $f untracked by git (would be silently dropped at ship)" >&2
    exit 1
  fi
done

# ── AC1a: locate STOP CRITERION section heading ────────────────────────────
stop_line=$(grep -n -i -E '^##[[:space:]]+STOP CRITERION' "$BUILDER_MD" | head -n 1 | cut -d: -f1)
if [ -z "$stop_line" ]; then
  echo "RED: $BUILDER_MD has no '## STOP CRITERION' heading" >&2
  exit 1
fi

# Find the next ## heading after STOP CRITERION to bound the section.
section_end=$(awk -v start="$stop_line" '
  NR > start && /^## / { print NR; exit }
' "$BUILDER_MD")
if [ -z "$section_end" ]; then
  section_end=$(wc -l < "$BUILDER_MD" | tr -d ' ')
fi

# Slice the STOP CRITERION section into a temp file for sub-checks.
section_tmp="$(mktemp -t stop-section.XXXXXX)"
trap 'rm -f "$section_tmp"' EXIT
sed -n "${stop_line},${section_end}p" "$BUILDER_MD" > "$section_tmp"

# ── AC1b: section must mention "turn 18" hard exit ─────────────────────────
# Builder must hard-exit at turn 18 (down from previous 20) per scout F1 fix.
# Accept variants: "turn 18", "at turn 18", "turn-18", "Turn 18".
if ! grep -q -i -E '\bturn[- ]?18\b' "$section_tmp"; then
  echo "RED: STOP CRITERION section (L${stop_line}-${section_end}) does not mention 'turn 18' hard exit" >&2
  echo "RED: section content:" >&2
  cat "$section_tmp" >&2
  exit 1
fi

# ── AC1c: section must mention a checkpoint commit rule ────────────────────
# The cycle-95 35-turn overrun on 2 simple tasks revealed that the builder
# never commits mid-stream, so on overrun all work is lost. The fix is to
# checkpoint-commit work as it completes so a turn-18 exit drops only the
# in-flight task, not everything.
#
# Accept: "checkpoint" within 3 lines of "commit", OR a "CHECKPOINT RULE"
# heading/bullet, OR "commit completed work".
if ! grep -q -i -E '(checkpoint.{0,200}commit|commit.{0,200}checkpoint|CHECKPOINT RULE|commit completed work)' "$section_tmp"; then
  echo "RED: STOP CRITERION section lacks a checkpoint-commit rule" >&2
  echo "RED: expected one of: 'checkpoint...commit', 'CHECKPOINT RULE', 'commit completed work'" >&2
  exit 1
fi

# ── AC2: builder.json turn_budget_hint == 20 (behavioral via jq) ───────────
if ! command -v jq >/dev/null 2>&1; then
  echo "RED: jq not available — cannot validate $BUILDER_PROFILE structurally" >&2
  exit 1
fi

hint=$(jq -r '.turn_budget_hint // empty' "$BUILDER_PROFILE" 2>/dev/null)
if [ -z "$hint" ]; then
  echo "RED: $BUILDER_PROFILE has no turn_budget_hint field" >&2
  exit 1
fi
if [ "$hint" != "20" ]; then
  echo "RED: $BUILDER_PROFILE turn_budget_hint=$hint (expected 20 — regression from cycle-95 ceiling)" >&2
  exit 1
fi

# Also verify max_turns is still 25 (the hard ceiling). turn_budget_hint=20
# without max_turns=25 means the builder gets no recovery margin.
mx=$(jq -r '.max_turns // empty' "$BUILDER_PROFILE" 2>/dev/null)
if [ "$mx" != "25" ]; then
  echo "RED: $BUILDER_PROFILE max_turns=$mx (expected 25 — hard ceiling regression)" >&2
  exit 1
fi

# Sanity: turn_budget_guidance.hard_exit_at_turn should be 18 to match persona.
hard_exit=$(jq -r '.turn_budget_guidance.hard_exit_at_turn // empty' "$BUILDER_PROFILE" 2>/dev/null)
if [ -n "$hard_exit" ] && [ "$hard_exit" != "18" ]; then
  echo "RED: $BUILDER_PROFILE turn_budget_guidance.hard_exit_at_turn=$hard_exit (expected 18 to match persona STOP CRITERION)" >&2
  exit 1
fi

echo "GREEN: STOP CRITERION L${stop_line}-${section_end} has turn-18 + checkpoint-commit; turn_budget_hint=$hint, max_turns=$mx, hard_exit_at_turn=${hard_exit:-(unset)}"
exit 0
