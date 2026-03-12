#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Uninstaller
# Removes agents from ~/.claude/agents/ and skill from ~/.claude/skills/evolve-loop/

AGENTS_DIR="$HOME/.claude/agents"
SKILLS_DIR="$HOME/.claude/skills/evolve-loop"

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
echo "Note: Project workspace files (.claude/evolve/) are NOT removed."
echo "Delete them manually if you no longer need cycle history."
