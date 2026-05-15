#!/usr/bin/env bash
# ACS predicate 040 — cycle 60
# Verifies that the LLM router (resolve-llm.sh) correctly resolves 3 phases to
# 3 distinct CLIs when given a mixed-CLI fixture llm_config.json:
#   scout→gemini, builder→claude, auditor→codex
#
# AC-ID: cycle-60-040
# Description: mixed-CLI routing: 3 distinct CLIs (gemini, claude, codex) across scout/builder/auditor
# Evidence: resolve-llm.sh output with fixture llm_config.json
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: intent.md AC-040
#
# metadata:
#   id: 040-e2e-mixed-cli-cycle
#   cycle: 60
#   task: predicate-040-mixed-cli-e2e
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
PLUGIN_ROOT="${EVOLVE_PLUGIN_ROOT:-$REPO_ROOT}"
RESOLV=""
for candidate in \
    "$PLUGIN_ROOT/scripts/dispatch/resolve-llm.sh" \
    "$REPO_ROOT/scripts/dispatch/resolve-llm.sh"; do
    [ -f "$candidate" ] && RESOLV="$candidate" && break
done

if [ -z "$RESOLV" ]; then
    echo "RED: resolve-llm.sh not found under PLUGIN_ROOT=$PLUGIN_ROOT or REPO_ROOT=$REPO_ROOT"
    exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

rc=0

# ── Write the mixed-CLI fixture ────────────────────────────────────────────────
FIXTURE="$TMP/llm_config.json"
cat > "$FIXTURE" << 'FIXTURE_EOF'
{
  "phases": {
    "scout":   {"cli": "gemini", "model_tier": "sonnet"},
    "builder": {"cli": "claude", "model_tier": "sonnet"},
    "auditor": {"cli": "codex",  "model_tier": "opus"}
  },
  "_meta": {"purpose": "predicate-040-mixed-cli-fixture", "cycle": 60}
}
FIXTURE_EOF

# ── Resolve all three phases ────────────────────────────────────────────────────
SCOUT_JSON=$(bash "$RESOLV" scout "$FIXTURE" 2>/dev/null)
BUILDER_JSON=$(bash "$RESOLV" builder "$FIXTURE" 2>/dev/null)
AUDITOR_JSON=$(bash "$RESOLV" auditor "$FIXTURE" 2>/dev/null)

# ── AC1: scout resolves to gemini ─────────────────────────────────────────────
scout_cli=$(echo "$SCOUT_JSON" | jq -r '.cli // empty' 2>/dev/null)
scout_src=$(echo "$SCOUT_JSON" | jq -r '.source // empty' 2>/dev/null)
if [ "$scout_cli" = "gemini" ] && [ "$scout_src" = "llm_config" ]; then
    echo "GREEN AC1: scout→gemini (source=llm_config)"
else
    echo "RED AC1: scout resolved to cli=$scout_cli source=$scout_src (expected gemini/llm_config)"
    rc=1
fi

# ── AC2: builder resolves to claude ───────────────────────────────────────────
builder_cli=$(echo "$BUILDER_JSON" | jq -r '.cli // empty' 2>/dev/null)
builder_src=$(echo "$BUILDER_JSON" | jq -r '.source // empty' 2>/dev/null)
if [ "$builder_cli" = "claude" ] && [ "$builder_src" = "llm_config" ]; then
    echo "GREEN AC2: builder→claude (source=llm_config)"
else
    echo "RED AC2: builder resolved to cli=$builder_cli source=$builder_src (expected claude/llm_config)"
    rc=1
fi

# ── AC3: auditor resolves to codex ────────────────────────────────────────────
auditor_cli=$(echo "$AUDITOR_JSON" | jq -r '.cli // empty' 2>/dev/null)
auditor_src=$(echo "$AUDITOR_JSON" | jq -r '.source // empty' 2>/dev/null)
if [ "$auditor_cli" = "codex" ] && [ "$auditor_src" = "llm_config" ]; then
    echo "GREEN AC3: auditor→codex (source=llm_config)"
else
    echo "RED AC3: auditor resolved to cli=$auditor_cli source=$auditor_src (expected codex/llm_config)"
    rc=1
fi

# ── AC4: 3 distinct CLIs ──────────────────────────────────────────────────────
distinct=$(printf '%s\n%s\n%s\n' "$scout_cli" "$builder_cli" "$auditor_cli" | sort -u | wc -l | tr -d ' ')
if [ "$distinct" -ge 3 ]; then
    echo "GREEN AC4: $distinct distinct CLIs across 3 phases (gemini, claude, codex)"
else
    echo "RED AC4: only $distinct distinct CLI(s) — expected 3 (gemini, claude, codex)"
    rc=1
fi

# ── AC5 (anti-tautology): without fixture, profile default (claude) is used ───
# Verifies the predicate is NOT just checking for "claude" everywhere.
# If this AC passed trivially, a predicate that always returns "claude" would FAIL here.
NOFIXTURE_JSON=$(bash "$RESOLV" scout /nonexistent/llm_config.json 2>/dev/null)
nofixture_cli=$(echo "$NOFIXTURE_JSON" | jq -r '.cli // empty' 2>/dev/null)
nofixture_src=$(echo "$NOFIXTURE_JSON" | jq -r '.source // empty' 2>/dev/null)
if [ "$nofixture_cli" = "claude" ] && [ "$nofixture_src" = "profile" ]; then
    echo "GREEN AC5 (anti-tautology): without fixture, scout falls back to cli=claude source=profile"
else
    echo "RED AC5 (anti-tautology): without fixture, got cli=$nofixture_cli source=$nofixture_src (expected claude/profile)"
    rc=1
fi

exit "$rc"
