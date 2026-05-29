#!/usr/bin/env bash
# ACS predicate 042 — cycle 60
# Verifies that when no llm_config.json is present, subagent-run.sh --validate-profile
# resolves phases using the profile default (cli_resolution_source="profile").
# This is the backward-compat E2E: cycles without llm_config.json continue to work.
#
# AC-ID: cycle-60-042
# Description: legacy-no-llm-config: profile fallback resolves correctly without llm_config.json
# Evidence: subagent-run.sh --validate-profile dispatch plan log
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: intent.md AC-042
#
# metadata:
#   id: 042-legacy-no-llm-config-cycle-completes
#   cycle: 60
#   task: predicate-042-legacy-backward-compat
#   severity: MEDIUM

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

if [ ! -f "$SUBAGENT_RUN" ]; then
    echo "RED: subagent-run.sh not found at $SUBAGENT_RUN"
    exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

rc=0

# ── AC1: --validate-profile scout exits 0 without llm_config.json ─────────────
PLAN_SCOUT="$TMP/plan-scout.json"
if EVOLVE_PROJECT_ROOT="$REPO_ROOT" \
   EVOLVE_LLM_CONFIG_PATH="/nonexistent/llm_config.json" \
   EVOLVE_DISPATCH_PLAN_LOG="$PLAN_SCOUT" \
   bash "$SUBAGENT_RUN" --validate-profile scout >/dev/null 2>&1; then
    echo "GREEN AC1: --validate-profile scout exits 0 without llm_config.json"
else
    echo "RED AC1: --validate-profile scout failed without llm_config.json"
    rc=1
fi

# ── AC2: dispatch plan shows cli_resolution_source=profile ────────────────────
if [ -f "$PLAN_SCOUT" ]; then
    src=$(jq -r '.cli_resolution_source // empty' "$PLAN_SCOUT" 2>/dev/null)
    if [ "$src" = "profile" ]; then
        echo "GREEN AC2: cli_resolution_source=profile (no llm_config.json)"
    else
        echo "RED AC2: cli_resolution_source='$src' (expected 'profile')"
        rc=1
    fi
else
    echo "RED AC2: dispatch plan log not written to $PLAN_SCOUT"
    rc=1
fi

# ── AC3: multi-role — builder, auditor, triage all resolve via profile ─────────
for role in builder auditor triage; do
    PLAN_ROLE="$TMP/plan-${role}.json"
    if EVOLVE_PROJECT_ROOT="$REPO_ROOT" \
       EVOLVE_LLM_CONFIG_PATH="/nonexistent/llm_config.json" \
       EVOLVE_DISPATCH_PLAN_LOG="$PLAN_ROLE" \
       bash "$SUBAGENT_RUN" --validate-profile "$role" >/dev/null 2>&1; then
        role_src=$(jq -r '.cli_resolution_source // empty' "$PLAN_ROLE" 2>/dev/null)
        if [ "$role_src" = "profile" ]; then
            echo "GREEN AC3[$role]: cli_resolution_source=profile"
        else
            echo "RED AC3[$role]: cli_resolution_source='$role_src' (expected profile)"
            rc=1
        fi
    else
        echo "RED AC3[$role]: --validate-profile $role failed"
        rc=1
    fi
done

# ── AC4 (anti-tautology): WITH fixture llm_config, source changes to llm_config ─
FIXTURE_CONFIG="$TMP/llm_config.json"
cat > "$FIXTURE_CONFIG" << 'FIXTURE_EOF'
{
  "phases": {
    "scout":   {"cli": "gemini", "model_tier": "sonnet"},
    "builder": {"cli": "claude", "model_tier": "sonnet"},
    "auditor": {"cli": "codex",  "model_tier": "opus"}
  },
  "_meta": {"purpose": "predicate-042-antitautology-fixture", "cycle": 60}
}
FIXTURE_EOF

PLAN_FIXTURE="$TMP/plan-fixture.json"
if EVOLVE_PROJECT_ROOT="$REPO_ROOT" \
   EVOLVE_LLM_CONFIG_PATH="$FIXTURE_CONFIG" \
   EVOLVE_DISPATCH_PLAN_LOG="$PLAN_FIXTURE" \
   bash "$SUBAGENT_RUN" --validate-profile scout >/dev/null 2>&1; then
    fixture_src=$(jq -r '.cli_resolution_source // empty' "$PLAN_FIXTURE" 2>/dev/null)
    fixture_cli=$(jq -r '.cli // empty' "$PLAN_FIXTURE" 2>/dev/null)
    if [ "$fixture_src" = "llm_config" ] && [ "$fixture_cli" = "gemini" ]; then
        echo "GREEN AC4 (anti-tautology): WITH fixture, cli=gemini source=llm_config"
    else
        echo "RED AC4 (anti-tautology): WITH fixture, got cli=$fixture_cli source=$fixture_src (expected gemini/llm_config)"
        rc=1
    fi
else
    echo "RED AC4 (anti-tautology): --validate-profile scout with fixture failed"
    rc=1
fi

exit "$rc"
