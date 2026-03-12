#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Installer
#
# Preferred: Install as a Claude Code plugin:
#   /plugin marketplace add mickeyyaya/evolve-loop
#   /plugin install evolve-loop@evolve-loop
#
# Manual install (copies to ~/.claude/):
#   ./install.sh
#
# CI validation mode:
#   CI=true ./install.sh
#   ./install.sh --ci

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Detect CI mode
CI_MODE="${CI:-false}"
if [[ "${1:-}" == "--ci" ]]; then
  CI_MODE="true"
fi

# In CI mode, validate structure only (no copying)
if [[ "$CI_MODE" == "true" ]]; then
  ERRORS=0

  # Validate plugin manifest
  if [ ! -f "$SCRIPT_DIR/.claude-plugin/plugin.json" ]; then
    echo "FAIL: .claude-plugin/plugin.json not found"
    ERRORS=$((ERRORS + 1))
  else
    echo "OK: plugin.json exists"
    python3 -c "import json; json.load(open('$SCRIPT_DIR/.claude-plugin/plugin.json'))" 2>/dev/null \
      && echo "OK: plugin.json is valid JSON" \
      || echo "WARN: could not validate JSON"
  fi

  # Validate agents
  for agent in evolve-scout evolve-builder evolve-auditor evolve-operator; do
    if [ ! -f "$SCRIPT_DIR/agents/${agent}.md" ]; then
      echo "FAIL: agents/${agent}.md not found"
      ERRORS=$((ERRORS + 1))
    else
      if ! head -1 "$SCRIPT_DIR/agents/${agent}.md" | grep -q "^---$"; then
        echo "FAIL: agents/${agent}.md missing YAML frontmatter"
        ERRORS=$((ERRORS + 1))
      else
        echo "OK: agents/${agent}.md"
      fi
    fi
  done

  # Validate skill files
  for skill in SKILL.md phases.md memory-protocol.md eval-runner.md; do
    if [ ! -f "$SCRIPT_DIR/skills/evolve-loop/${skill}" ]; then
      echo "FAIL: skills/evolve-loop/${skill} not found"
      ERRORS=$((ERRORS + 1))
    else
      echo "OK: skills/evolve-loop/${skill}"
    fi
  done

  # Machine-readable summary
  AGENT_COUNT=$(ls "$SCRIPT_DIR"/agents/evolve-*.md 2>/dev/null | wc -l | tr -d ' ')
  SKILL_COUNT=$(ls "$SCRIPT_DIR"/skills/evolve-loop/*.md 2>/dev/null | wc -l | tr -d ' ')
  echo "EVOLVE_LOOP_VALIDATED=true"
  echo "EVOLVE_LOOP_AGENTS=${AGENT_COUNT}"
  echo "EVOLVE_LOOP_SKILLS=${SKILL_COUNT}"
  echo "EVOLVE_LOOP_ERRORS=${ERRORS}"

  if [ "$ERRORS" -gt 0 ]; then
    exit 1
  fi
  exit 0
fi

# Manual install mode — copy to ~/.claude/
AGENTS_DIR="$HOME/.claude/agents"
SKILLS_DIR="$HOME/.claude/skills/evolve-loop"

echo "Installing Evolve Loop v4..."
echo ""
echo "NOTE: Preferred method is plugin install:"
echo "  /plugin marketplace add mickeyyaya/evolve-loop"
echo "  /plugin install evolve-loop@evolve-loop"
echo ""

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

AGENT_COUNT=$(ls "$SCRIPT_DIR"/agents/evolve-*.md | wc -l | tr -d ' ')
SKILL_COUNT=$(ls "$SCRIPT_DIR"/skills/evolve-loop/*.md | wc -l | tr -d ' ')

echo ""
echo "Installation complete!"
echo ""
echo "Installed:"
echo "  - ${AGENT_COUNT} agents (Scout, Builder, Auditor, Operator)"
echo "  - ${SKILL_COUNT} skill files"
echo ""
echo "Usage: /evolve-loop [cycles] [goal]"
