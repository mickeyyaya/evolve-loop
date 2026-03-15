#!/usr/bin/env bash
set -euo pipefail

# Evolve Loop Publisher
#
# Syncs the local plugin cache and registry so all new Claude Code sessions
# automatically load the latest version. Run after pushing changes.
#
# Usage:
#   ./publish.sh              # auto-detect version from plugin.json
#   ./publish.sh 6.8.0        # explicit version override
#
# What it does:
#   1. Reads version from .claude-plugin/plugin.json (or argument)
#   2. Pulls latest into the marketplace directory
#   3. Copies all files into the plugin cache at the new version
#   4. Updates installed_plugins.json registry to point to the new cache
#   5. Verifies the update

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_JSON="$SCRIPT_DIR/.claude-plugin/plugin.json"
MARKETPLACE_JSON="$SCRIPT_DIR/.claude-plugin/marketplace.json"
CACHE_BASE="$HOME/.claude/plugins/cache/evolve-loop/evolve-loop"
MARKETPLACE_DIR="$HOME/.claude/plugins/marketplaces/evolve-loop"
REGISTRY="$HOME/.claude/plugins/installed_plugins.json"

# --- 1. Determine version ---
if [[ -n "${1:-}" ]]; then
  VERSION="$1"
else
  VERSION=$(python3 -c "import json; print(json.load(open('$PLUGIN_JSON'))['version'])")
fi
echo "Publishing evolve-loop v${VERSION}"

# --- 2. Validate source files exist ---
ERRORS=0
for f in "$PLUGIN_JSON" "$MARKETPLACE_JSON"; do
  if [[ ! -f "$f" ]]; then
    echo "FAIL: $f not found"
    ERRORS=$((ERRORS + 1))
  fi
done
for agent in evolve-scout evolve-builder evolve-auditor evolve-operator; do
  if [[ ! -f "$SCRIPT_DIR/agents/${agent}.md" ]]; then
    echo "FAIL: agents/${agent}.md not found"
    ERRORS=$((ERRORS + 1))
  fi
done
for skill in SKILL.md phases.md memory-protocol.md eval-runner.md; do
  if [[ ! -f "$SCRIPT_DIR/skills/evolve-loop/${skill}" ]]; then
    echo "FAIL: skills/evolve-loop/${skill} not found"
    ERRORS=$((ERRORS + 1))
  fi
done
if [[ "$ERRORS" -gt 0 ]]; then
  echo "Aborting: $ERRORS validation errors"
  exit 1
fi

# --- 3. Verify versions are consistent ---
PLUGIN_VER=$(python3 -c "import json; print(json.load(open('$PLUGIN_JSON'))['version'])")
MARKET_VER=$(python3 -c "import json; d=json.load(open('$MARKETPLACE_JSON')); print(d['plugins'][0]['version'])")
if [[ "$PLUGIN_VER" != "$VERSION" ]]; then
  echo "WARN: plugin.json version ($PLUGIN_VER) != target ($VERSION), updating..."
  python3 -c "
import json
with open('$PLUGIN_JSON') as f: d = json.load(f)
d['version'] = '$VERSION'
with open('$PLUGIN_JSON', 'w') as f: json.dump(d, f, indent=2)
"
fi
if [[ "$MARKET_VER" != "$VERSION" ]]; then
  echo "WARN: marketplace.json version ($MARKET_VER) != target ($VERSION), updating..."
  python3 -c "
import json
with open('$MARKETPLACE_JSON') as f: d = json.load(f)
d['plugins'][0]['version'] = '$VERSION'
with open('$MARKETPLACE_JSON', 'w') as f: json.dump(d, f, indent=2)
"
fi

# --- 4. Update marketplace directory ---
if [[ -d "$MARKETPLACE_DIR/.git" ]]; then
  echo "Pulling latest into marketplace..."
  git -C "$MARKETPLACE_DIR" pull origin main --quiet 2>/dev/null || echo "WARN: marketplace pull failed (may need manual sync)"
fi

# --- 5. Populate plugin cache ---
CACHE_DIR="$CACHE_BASE/$VERSION"
echo "Caching to $CACHE_DIR"
mkdir -p "$CACHE_DIR"

# Copy all project files (excluding .git, .claude/evolve workspace data)
rsync -a --delete \
  --exclude='.git' \
  --exclude='.claude/evolve' \
  "$SCRIPT_DIR/" "$CACHE_DIR/"

echo "  Cached: $(ls "$CACHE_DIR" | wc -l | tr -d ' ') top-level items"

# --- 6. Update plugin registry ---
if [[ -f "$REGISTRY" ]]; then
  GIT_SHA=$(git -C "$SCRIPT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")
  NOW=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")

  python3 -c "
import json
with open('$REGISTRY') as f:
    reg = json.load(f)
key = 'evolve-loop@evolve-loop'
reg['plugins'][key] = [{
    'scope': 'user',
    'installPath': '$CACHE_DIR',
    'version': '$VERSION',
    'installedAt': reg['plugins'].get(key, [{}])[0].get('installedAt', '$NOW') if key in reg.get('plugins',{}) else '$NOW',
    'lastUpdated': '$NOW',
    'gitCommitSha': '$GIT_SHA'
}]
with open('$REGISTRY', 'w') as f:
    json.dump(reg, f, indent=2)
"
  echo "  Registry updated: evolve-loop@evolve-loop -> v${VERSION}"
else
  echo "WARN: $REGISTRY not found, skipping registry update"
fi

# --- 7. Verify ---
echo ""
echo "Verification:"
CACHED_VER=$(python3 -c "import json; print(json.load(open('$CACHE_DIR/.claude-plugin/plugin.json'))['version'])" 2>/dev/null || echo "FAIL")
REG_VER=$(python3 -c "import json; r=json.load(open('$REGISTRY')); print(r['plugins']['evolve-loop@evolve-loop'][0]['version'])" 2>/dev/null || echo "FAIL")
echo "  Cache version:    $CACHED_VER"
echo "  Registry version: $REG_VER"
echo "  Agents cached:    $(ls "$CACHE_DIR/agents"/evolve-*.md 2>/dev/null | wc -l | tr -d ' ')"
echo "  Skills cached:    $(ls "$CACHE_DIR/skills/evolve-loop"/*.md 2>/dev/null | wc -l | tr -d ' ')"

if [[ "$CACHED_VER" == "$VERSION" && "$REG_VER" == "$VERSION" ]]; then
  echo ""
  echo "Published evolve-loop v${VERSION} successfully."
  echo "All new Claude Code sessions will load v${VERSION} automatically."
else
  echo ""
  echo "WARN: Version mismatch detected. Manual check recommended."
  exit 1
fi
