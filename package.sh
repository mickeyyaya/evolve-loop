#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Packager
# Generates the evolve-loop.skill file for GitHub releases.
# Requires the Gemini CLI to be installed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Find package_skill.cjs from the global Gemini CLI installation
# We look for the skill-creator plugin inside the global npm directory
PACKAGE_SCRIPT=$(find /opt/homebrew/Cellar/gemini-cli -name package_skill.cjs -path "*/skill-creator/scripts/*" | head -n 1)

if [[ -z "$PACKAGE_SCRIPT" ]]; then
  # Fallback: try finding via npm root -g
  NPM_GLOBAL=$(npm root -g)
  PACKAGE_SCRIPT=$(find "$NPM_GLOBAL" -name package_skill.cjs -path "*/skill-creator/scripts/*" 2>/dev/null | head -n 1)
fi

if [[ -z "$PACKAGE_SCRIPT" ]]; then
  echo "Error: Could not find package_skill.cjs from Gemini CLI."
  echo "Please ensure Gemini CLI is installed."
  exit 1
fi

echo "Found package_skill.cjs at: $PACKAGE_SCRIPT"

# Ensure the skill is published locally first so the structure is correct
echo "Running publish.sh to prepare the skill structure..."
"$SCRIPT_DIR/publish.sh"

GEMINI_SKILL_DIR="$HOME/.gemini/skills/evolve-loop"

echo "Packaging skill..."
node "$PACKAGE_SCRIPT" "$GEMINI_SKILL_DIR" "$SCRIPT_DIR"

echo "Done! The evolve-loop.skill package is ready for release in the project root."
