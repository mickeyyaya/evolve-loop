#!/usr/bin/env bash
# release.sh — Version consistency checker for evolve-loop releases.
# Run before committing a version bump to ensure all files are in sync.
#
# Usage:
#   ./scripts/release.sh           # check current state
#   ./scripts/release.sh 8.4.0     # check + verify all files match 8.4.0
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET_VERSION="${1:-}"
ERRORS=0

# --- Helpers ---

# Extract version from a file using sed (macOS-compatible, no PCRE needed)
extract_json_version() {
  sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -1
}

check_json_version() {
  local file="$1"
  local description="$2"
  local full_path="$REPO_ROOT/$file"

  if [[ ! -f "$full_path" ]]; then
    printf "${RED}MISSING${NC}  %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
    return
  fi

  local match
  match=$(extract_json_version "$full_path")

  if [[ -z "$match" ]]; then
    printf "${RED}NO MATCH${NC} %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
  elif [[ -n "$TARGET_VERSION" && "$match" != "$TARGET_VERSION" ]]; then
    printf "${RED}MISMATCH${NC} %-45s found: %s, expected: %s\n" "$file" "$match" "$TARGET_VERSION"
    ERRORS=$((ERRORS + 1))
  else
    printf "${GREEN}OK${NC}       %-45s %s (%s)\n" "$file" "$description" "$match"
  fi
}

check_heading_version() {
  local file="$1"
  local description="$2"
  local full_path="$REPO_ROOT/$file"

  if [[ ! -f "$full_path" ]]; then
    printf "${RED}MISSING${NC}  %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
    return
  fi

  local match
  match=$(sed -n 's/^# Evolve Loop v\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/p' "$full_path" | head -1)

  if [[ -z "$match" ]]; then
    printf "${RED}NO MATCH${NC} %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
  elif [[ -n "$MAJOR_MINOR" && "$match" != "$MAJOR_MINOR" ]]; then
    printf "${RED}MISMATCH${NC} %-45s found: %s, expected: %s\n" "$file" "$match" "$MAJOR_MINOR"
    ERRORS=$((ERRORS + 1))
  else
    printf "${GREEN}OK${NC}       %-45s %s (v%s)\n" "$file" "$description" "$match"
  fi
}

check_readme_current() {
  local file="$1"
  local description="$2"
  local full_path="$REPO_ROOT/$file"

  if [[ ! -f "$full_path" ]]; then
    printf "${RED}MISSING${NC}  %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
    return
  fi

  local match
  match=$(sed -n 's/.*Current (v\([0-9][0-9]*\.[0-9][0-9]*\)).*/\1/p' "$full_path" | head -1)

  if [[ -z "$match" ]]; then
    printf "${RED}NO MATCH${NC} %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
  elif [[ -n "$MAJOR_MINOR" && "$match" != "$MAJOR_MINOR" ]]; then
    printf "${RED}MISMATCH${NC} %-45s found: %s, expected: %s\n" "$file" "$match" "$MAJOR_MINOR"
    ERRORS=$((ERRORS + 1))
  else
    printf "${GREEN}OK${NC}       %-45s %s (v%s)\n" "$file" "$description" "$match"
  fi
}

check_contains() {
  local file="$1"
  local pattern="$2"
  local description="$3"
  local full_path="$REPO_ROOT/$file"

  if [[ ! -f "$full_path" ]]; then
    printf "${RED}MISSING${NC}  %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
    return
  fi

  if grep -q "$pattern" "$full_path"; then
    printf "${GREEN}OK${NC}       %-45s %s\n" "$file" "$description"
  else
    printf "${RED}MISSING${NC}  %-45s %s\n" "$file" "$description"
    ERRORS=$((ERRORS + 1))
  fi
}

# --- Header ---

echo ""
echo "=== evolve-loop release checklist ==="
echo ""

# Read canonical version from plugin.json
CANONICAL=$(extract_json_version "$REPO_ROOT/.claude-plugin/plugin.json")
MAJOR_MINOR=$(echo "$CANONICAL" | sed 's/\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/')

if [[ -n "$TARGET_VERSION" ]]; then
  echo "Target version:    $TARGET_VERSION"
  echo "Canonical version: $CANONICAL (plugin.json)"
  if [[ "$TARGET_VERSION" != "$CANONICAL" ]]; then
    printf "${RED}WARNING: target version differs from plugin.json${NC}\n"
  fi
else
  echo "Canonical version: $CANONICAL (plugin.json)"
  echo "Tip: pass a version arg to verify all files match it"
  TARGET_VERSION="$CANONICAL"
fi

MAJOR_MINOR=$(echo "$TARGET_VERSION" | sed 's/\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/')

echo ""
echo "--- Version strings ---"

# 1. plugin.json — source of truth
check_json_version ".claude-plugin/plugin.json" "plugin.json version"

# 2. marketplace.json
check_json_version ".claude-plugin/marketplace.json" "marketplace.json version"

# 3. SKILL.md heading
check_heading_version "skills/evolve-loop/SKILL.md" "SKILL.md heading (major.minor)"

# 4. README current version in table
check_readme_current "README.md" "README.md current version table"

echo ""
echo "--- Required content ---"

# 5. CHANGELOG entry for this version
check_contains "CHANGELOG.md" "\[${TARGET_VERSION}\]" "CHANGELOG.md entry for ${TARGET_VERSION}"

# 6. README version history row for this major.minor
check_contains "README.md" "v${MAJOR_MINOR}" "README.md version history row for v${MAJOR_MINOR}"

# 7. GitHub release reminder
echo ""
echo "--- Manual checks ---"
printf "${YELLOW}REMIND${NC}   %-45s %s\n" "GitHub release" "Create release v${TARGET_VERSION} after push"

# --- Plugin Cache Refresh ---

echo ""
echo "--- Plugin cache refresh ---"

PLUGIN_CACHE_DIR="$HOME/.claude/plugins/cache/evolve-loop"
PLUGIN_MARKETPLACE_DIR="$HOME/.claude/plugins/marketplaces/evolve-loop"
PLUGIN_REGISTRY="$HOME/.claude/plugins/installed_plugins.json"
CURRENT_SHA=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

CACHE_REFRESHED=false

# Clear stale cache directory
if [[ -d "$PLUGIN_CACHE_DIR" ]]; then
  rm -rf "$PLUGIN_CACHE_DIR"
  printf "${GREEN}CLEANED${NC}  %-45s %s\n" "Plugin cache" "Removed stale cache at $PLUGIN_CACHE_DIR"
  CACHE_REFRESHED=true
else
  printf "${GREEN}OK${NC}       %-45s %s\n" "Plugin cache" "No stale cache found"
fi

# Update marketplace checkout
if [[ -d "$PLUGIN_MARKETPLACE_DIR/.git" ]]; then
  MARKETPLACE_SHA=$(git -C "$PLUGIN_MARKETPLACE_DIR" rev-parse HEAD 2>/dev/null || echo "unknown")
  if [[ "$MARKETPLACE_SHA" != "$CURRENT_SHA" ]]; then
    git -C "$PLUGIN_MARKETPLACE_DIR" pull origin main --ff-only 2>/dev/null
    if [[ $? -eq 0 ]]; then
      printf "${GREEN}UPDATED${NC}  %-45s %s\n" "Marketplace checkout" "Pulled latest ($(git -C "$PLUGIN_MARKETPLACE_DIR" rev-parse --short HEAD))"
      CACHE_REFRESHED=true
    else
      printf "${RED}FAILED${NC}   %-45s %s\n" "Marketplace checkout" "git pull failed — update manually"
      ERRORS=$((ERRORS + 1))
    fi
  else
    printf "${GREEN}OK${NC}       %-45s %s\n" "Marketplace checkout" "Already at latest ($CURRENT_SHA)"
  fi
elif [[ -d "$PLUGIN_MARKETPLACE_DIR" ]]; then
  printf "${YELLOW}SKIP${NC}     %-45s %s\n" "Marketplace checkout" "Not a git repo — cannot auto-update"
fi

# Update installed_plugins.json registry
if [[ -f "$PLUGIN_REGISTRY" ]]; then
  # Check if the registry still points to the old version
  REGISTRY_VERSION=$(sed -n '/"evolve-loop@evolve-loop"/,/\]/{ s/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p; }' "$PLUGIN_REGISTRY" | head -1)
  if [[ -n "$REGISTRY_VERSION" && "$REGISTRY_VERSION" != "$TARGET_VERSION" ]]; then
    # Update version
    sed -i '' "s|\"installPath\": \".*cache/evolve-loop[^\"]*\"|\"installPath\": \"$PLUGIN_MARKETPLACE_DIR\"|" "$PLUGIN_REGISTRY"
    sed -i '' "/"evolve-loop@evolve-loop"/,/\]/{
      s/\"version\": \"[^\"]*\"/\"version\": \"$TARGET_VERSION\"/
      s/\"gitCommitSha\": \"[^\"]*\"/\"gitCommitSha\": \"$CURRENT_SHA\"/
    }" "$PLUGIN_REGISTRY" 2>/dev/null
    # Simpler approach: use python for reliable JSON update
    python3 -c "
import json, sys
with open('$PLUGIN_REGISTRY', 'r') as f:
    data = json.load(f)
key = 'evolve-loop@evolve-loop'
if key in data.get('plugins', {}):
    for entry in data['plugins'][key]:
        entry['version'] = '$TARGET_VERSION'
        entry['installPath'] = '$PLUGIN_MARKETPLACE_DIR'
        entry['gitCommitSha'] = '$CURRENT_SHA'
        entry['lastUpdated'] = '$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'
    with open('$PLUGIN_REGISTRY', 'w') as f:
        json.dump(data, f, indent=2)
    print('updated')
else:
    print('not-found')
" 2>/dev/null
    RESULT=$?
    if [[ $RESULT -eq 0 ]]; then
      printf "${GREEN}UPDATED${NC}  %-45s %s\n" "Plugin registry" "Updated to v${TARGET_VERSION} (SHA: ${CURRENT_SHA:0:7})"
      CACHE_REFRESHED=true
    else
      printf "${RED}FAILED${NC}   %-45s %s\n" "Plugin registry" "Could not update installed_plugins.json"
      ERRORS=$((ERRORS + 1))
    fi
  else
    printf "${GREEN}OK${NC}       %-45s %s\n" "Plugin registry" "Already at v${TARGET_VERSION}"
  fi
fi

if $CACHE_REFRESHED; then
  printf "\n${GREEN}Plugin cache refreshed.${NC} New sessions will use v${TARGET_VERSION}.\n"
  printf "Run ${YELLOW}/reload-plugins${NC} in existing sessions to pick up the update.\n"
fi

# --- Summary ---

echo ""
if [[ $ERRORS -gt 0 ]]; then
  printf "${RED}FAILED: $ERRORS issue(s) found. Fix before releasing.${NC}\n"
  exit 1
else
  printf "${GREEN}PASSED: All version references are consistent.${NC}\n"
  exit 0
fi
