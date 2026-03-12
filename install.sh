#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Installer
# Copies agents to ~/.claude/agents/ and skill to ~/.claude/skills/evolve-loop/

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$HOME/.claude/agents"
SKILLS_DIR="$HOME/.claude/skills/evolve-loop"

echo "Installing Evolve Loop v4..."
echo ""

# Create directories
mkdir -p "$AGENTS_DIR"
mkdir -p "$SKILLS_DIR"

# Copy agents
echo "Copying agents to $AGENTS_DIR/"
for agent in "$SCRIPT_DIR"/agents/evolve-*.md; do
  filename=$(basename "$agent")
  if [ -f "$AGENTS_DIR/$filename" ]; then
    echo "  Overwriting: $filename"
  else
    echo "  Installing:  $filename"
  fi
  cp "$agent" "$AGENTS_DIR/$filename"
done

# Copy skill files
echo ""
echo "Copying skill to $SKILLS_DIR/"
for skill in "$SCRIPT_DIR"/skills/evolve-loop/*.md; do
  filename=$(basename "$skill")
  if [ -f "$SKILLS_DIR/$filename" ]; then
    echo "  Overwriting: $filename"
  else
    echo "  Installing:  $filename"
  fi
  cp "$skill" "$SKILLS_DIR/$filename"
done

echo ""
echo "Installation complete!"
echo ""
echo "Installed:"
echo "  - $(ls "$SCRIPT_DIR"/agents/evolve-*.md | wc -l | tr -d ' ') agents (Scout, Builder, Auditor, Operator)"
echo "  - $(ls "$SCRIPT_DIR"/skills/evolve-loop/*.md | wc -l | tr -d ' ') skill files"
echo ""
echo "No external dependencies required."
echo ""
echo "Usage: /evolve-loop [cycles] [goal]"
echo ""
echo "Examples:"
echo "  /evolve-loop              # 2 autonomous cycles"
echo "  /evolve-loop 1 add auth   # 1 goal-directed cycle"
echo "  /evolve-loop 5            # 5 autonomous cycles"
