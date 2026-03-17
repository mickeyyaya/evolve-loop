#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Uninstaller
# Removes agents from ~/.claude/agents/ and skill from ~/.claude/skills/evolve-loop/
#
# CI validation mode (dry-run, no deletions):
#   CI=true ./uninstall.sh
#   ./uninstall.sh --ci

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Detect CI mode
CI_MODE="${CI:-false}"
if [[ "${1:-}" == "--ci" ]]; then
  CI_MODE="true"
fi

AGENTS_DIR="$HOME/.claude/agents"
SKILLS_DIR="$HOME/.claude/skills/evolve-loop"

# In CI mode, validate structure only (no deletions)
if [[ "$CI_MODE" == "true" ]]; then
  echo "Uninstall dry-run (CI mode) — validating targets only"
  echo ""

  AGENT_COUNT=0
  if ls "$AGENTS_DIR"/evolve-*.md 1>/dev/null 2>&1; then
    for agent in "$AGENTS_DIR"/evolve-*.md; do
      echo "  Would remove: $(basename "$agent")"
      AGENT_COUNT=$((AGENT_COUNT + 1))
    done
  else
    echo "  No agents found in $AGENTS_DIR/"
  fi

  SKILL_EXISTS="false"
  if [ -d "$SKILLS_DIR" ]; then
    echo "  Would remove: $SKILLS_DIR/"
    SKILL_EXISTS="true"
  else
    echo "  No skill found at $SKILLS_DIR/"
  fi

  echo ""
  echo "EVOLVE_LOOP_UNINSTALL_VALIDATED=true"
  echo "EVOLVE_LOOP_AGENTS_TO_REMOVE=${AGENT_COUNT}"
  echo "EVOLVE_LOOP_SKILL_DIR_EXISTS=${SKILL_EXISTS}"
  exit 0
fi

echo "Uninstalling Evolve Loop..."
echo ""

# Remove agents
if ls "$AGENTS_DIR"/evolve-*.md 1>/dev/null 2>&1; then
  echo "Removing agents from $AGENTS_DIR/"
  for agent in "$AGENTS_DIR"/evolve-*.md; do
    echo "  Removing: $(basename "$agent")"
    rm "$agent"
  done
else
  echo "No agents found in $AGENTS_DIR/"
fi

# Remove skill
if [ -d "$SKILLS_DIR" ]; then
  echo ""
  echo "Removing skill from $SKILLS_DIR/"
  rm -rf "$SKILLS_DIR"
else
  echo "No skill found at $SKILLS_DIR/"
fi

echo ""
echo "Uninstallation complete."
echo ""
echo "Note: Project workspace files (.evolve/) are NOT removed."
echo "Delete them manually if you no longer need cycle history."
