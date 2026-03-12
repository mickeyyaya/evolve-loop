#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Installer
# Copies agents to ~/.claude/agents/ and skill to ~/.claude/skills/evolve-loop/
#
# CI/non-interactive mode:
#   CI=true ./install.sh
#   ./install.sh --ci

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$HOME/.claude/agents"
SKILLS_DIR="$HOME/.claude/skills/evolve-loop"

# Detect CI mode
CI_MODE="${CI:-false}"
if [[ "${1:-}" == "--ci" ]]; then
  CI_MODE="true"
fi

log() {
  if [[ "$CI_MODE" != "true" ]]; then
    echo "$@"
  fi
}

log "Installing Evolve Loop v4..."
log ""

# Create directories
mkdir -p "$AGENTS_DIR"
mkdir -p "$SKILLS_DIR"

# Copy agents
log "Copying agents to $AGENTS_DIR/"
for agent in "$SCRIPT_DIR"/agents/evolve-*.md; do
  filename=$(basename "$agent")
  if [ -f "$AGENTS_DIR/$filename" ]; then
    log "  Overwriting: $filename"
  else
    log "  Installing:  $filename"
  fi
  cp "$agent" "$AGENTS_DIR/$filename"
done

# Copy skill files
log ""
log "Copying skill to $SKILLS_DIR/"
for skill in "$SCRIPT_DIR"/skills/evolve-loop/*.md; do
  filename=$(basename "$skill")
  if [ -f "$SKILLS_DIR/$filename" ]; then
    log "  Overwriting: $filename"
  else
    log "  Installing:  $filename"
  fi
  cp "$skill" "$SKILLS_DIR/$filename"
done

AGENT_COUNT=$(ls "$SCRIPT_DIR"/agents/evolve-*.md | wc -l | tr -d ' ')
SKILL_COUNT=$(ls "$SCRIPT_DIR"/skills/evolve-loop/*.md | wc -l | tr -d ' ')

log ""
log "Installation complete!"
log ""
log "Installed:"
log "  - ${AGENT_COUNT} agents (Scout, Builder, Auditor, Operator)"
log "  - ${SKILL_COUNT} skill files"
log ""
log "No external dependencies required."
log ""
log "Usage: /evolve-loop [cycles] [goal]"
log ""
log "Examples:"
log "  /evolve-loop              # 2 autonomous cycles"
log "  /evolve-loop 1 add auth   # 1 goal-directed cycle"
log "  /evolve-loop 5            # 5 autonomous cycles"

# In CI mode, output machine-readable summary
if [[ "$CI_MODE" == "true" ]]; then
  echo "EVOLVE_LOOP_INSTALLED=true"
  echo "EVOLVE_LOOP_AGENTS=${AGENT_COUNT}"
  echo "EVOLVE_LOOP_SKILLS=${SKILL_COUNT}"
fi
