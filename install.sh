#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Installer
# Copies agents to ~/.claude/agents/ and skill to ~/.claude/skills/evolve-loop/

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$HOME/.claude/agents"
SKILLS_DIR="$HOME/.claude/skills/evolve-loop"

echo "Installing Evolve Loop v3..."
echo ""

# Check ECC dependency
ECC_MISSING=()
for agent_type in architect tdd-guide code-reviewer e2e-runner security-reviewer; do
  # Check if ECC agent files exist (common install locations)
  if ! ls "$HOME/.claude/agents/${agent_type}.md" 1>/dev/null 2>&1 && \
     ! ls "$HOME/.claude/agents/"*"${agent_type}"* 1>/dev/null 2>&1; then
    ECC_MISSING+=("$agent_type")
  fi
done

# Also check if ECC plugin is registered (look for the plugin marker)
if [ ${#ECC_MISSING[@]} -gt 0 ]; then
  echo "WARNING: Everything Claude Code (ECC) may not be installed."
  echo ""
  echo "Evolve Loop delegates to these ECC agents at runtime:"
  echo "  - everything-claude-code:architect"
  echo "  - everything-claude-code:tdd-guide"
  echo "  - everything-claude-code:code-reviewer"
  echo "  - everything-claude-code:e2e-runner"
  echo "  - everything-claude-code:security-reviewer"
  echo ""
  echo "Install ECC first: https://github.com/anthropics/everything-claude-code"
  echo ""
  read -p "Continue anyway? [y/N] " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 1
  fi
  echo ""
fi

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
echo "  - $(ls "$SCRIPT_DIR"/agents/evolve-*.md | wc -l | tr -d ' ') agents (6 custom + 5 ECC context overlays)"
echo "  - $(ls "$SCRIPT_DIR"/skills/evolve-loop/*.md | wc -l | tr -d ' ') skill files"
echo ""
echo "Usage: /evolve-loop [cycles] [goal]"
echo ""
echo "Examples:"
echo "  /evolve-loop              # 2 autonomous cycles"
echo "  /evolve-loop 1 add auth   # 1 goal-directed cycle"
echo "  /evolve-loop 5            # 5 autonomous cycles"
