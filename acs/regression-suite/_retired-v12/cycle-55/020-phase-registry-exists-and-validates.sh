#!/usr/bin/env bash
# ACS predicate 020 — cycle 55
# docs/architecture/phase-registry.json must exist, parse as valid JSON, have
# required schema fields, and all gate_in/gate_out references must resolve to
# declared functions in phase-gate.sh. Anti-tautology: a corrupt fixture with
# a nonexistent gate name must fail validation.
#
# AC-ID: cycle-55-020
# Description: phase-registry.json exists, validates schema + gate refs + role refs
# Evidence: docs/architecture/phase-registry.json, scripts/lifecycle/phase-gate.sh
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-5
#
# metadata:
#   id: 020-phase-registry-exists-and-validates
#   cycle: 55
#   task: slice-b-phase-registry
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
REGISTRY="$REPO_ROOT/docs/architecture/phase-registry.json"
PHASE_GATE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
PROFILES_DIR="$REPO_ROOT/.evolve/profiles"

rc=0

# ── AC1: registry file exists ─────────────────────────────────────────────────
if [ -f "$REGISTRY" ]; then
    echo "GREEN AC1: phase-registry.json exists at $REGISTRY"
else
    echo "RED AC1: phase-registry.json not found at $REGISTRY"
    exit 1
fi

# ── AC2: valid JSON ───────────────────────────────────────────────────────────
if jq -e '.' "$REGISTRY" >/dev/null 2>&1; then
    echo "GREEN AC2: phase-registry.json parses as valid JSON"
else
    echo "RED AC2: phase-registry.json is not valid JSON"
    rc=1
fi

# ── AC3: has schema_version field ────────────────────────────────────────────
if jq -e 'has("schema_version")' "$REGISTRY" >/dev/null 2>&1; then
    echo "GREEN AC3: schema_version field present"
else
    echo "RED AC3: schema_version field missing"
    rc=1
fi

# ── AC4: non-empty phases array ──────────────────────────────────────────────
phase_count=$(jq '.phases | length' "$REGISTRY" 2>/dev/null || echo "0")
if [ "$phase_count" -gt 0 ]; then
    echo "GREEN AC4: phases array has $phase_count entries"
else
    echo "RED AC4: phases array is empty or missing"
    rc=1
fi

# ── AC5: every phase has required fields ──────────────────────────────────────
missing_fields=0
while IFS= read -r phase_name; do
    for field in name role gate_in gate_out parallel_eligible; do
        # gate_in and gate_out may be null — presence (not value) is required
        has=$(jq -e --arg p "$phase_name" --arg f "$field" \
            '.phases[] | select(.name == $p) | has($f)' "$REGISTRY" 2>/dev/null || echo "false")
        if [ "$has" != "true" ]; then
            echo "RED AC5: phase '$phase_name' missing field '$field'"
            missing_fields=$((missing_fields + 1))
            rc=1
        fi
    done
done < <(jq -r '.phases[].name' "$REGISTRY" 2>/dev/null)

if [ "$missing_fields" -eq 0 ]; then
    echo "GREEN AC5: all phases have required fields (name, role, gate_in, gate_out, parallel_eligible)"
fi

# ── AC6: every non-null gate_in/gate_out resolves to a declared function ──────
if [ ! -f "$PHASE_GATE" ]; then
    echo "RED AC6: phase-gate.sh not found at $PHASE_GATE"
    rc=1
else
    bad_gates=0
    while IFS= read -r gate_val; do
        if [ -z "$gate_val" ] || [ "$gate_val" = "null" ]; then
            continue
        fi
        if grep -qE "^${gate_val}\(\)" "$PHASE_GATE"; then
            : # gate function found
        else
            echo "RED AC6: gate function '$gate_val' not declared in phase-gate.sh"
            bad_gates=$((bad_gates + 1))
            rc=1
        fi
    done < <(jq -r '.phases[] | (.gate_in, .gate_out) | select(. != null)' "$REGISTRY" 2>/dev/null)
    if [ "$bad_gates" -eq 0 ]; then
        echo "GREEN AC6: all non-null gate refs resolve to declared functions in phase-gate.sh"
    fi
fi

# ── AC7: every role maps to an existing .evolve/profiles/<role>.json ──────────
bad_roles=0
while IFS= read -r role_val; do
    if [ -z "$role_val" ] || [ "$role_val" = "null" ]; then
        continue
    fi
    if [ -f "$PROFILES_DIR/${role_val}.json" ]; then
        : # profile found
    else
        echo "RED AC7: role '$role_val' has no profile at $PROFILES_DIR/${role_val}.json"
        bad_roles=$((bad_roles + 1))
        rc=1
    fi
done < <(jq -r '.phases[].role' "$REGISTRY" 2>/dev/null)
if [ "$bad_roles" -eq 0 ]; then
    echo "GREEN AC7: all roles map to existing profile files"
fi

# ── AC8 (anti-tautology): corrupt fixture must fail AC6 validation ────────────
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

CORRUPT_REGISTRY="$TMP_DIR/registry-corrupt.json"
# Copy registry, replace first non-null gate_in with a nonexistent function name
jq '(.phases[] | select(.gate_in != null) | .gate_in) |= "gate_does_not_exist_zzz_test"' \
    "$REGISTRY" > "$CORRUPT_REGISTRY" 2>/dev/null

corrupt_valid=0
if [ -f "$CORRUPT_REGISTRY" ] && [ -s "$CORRUPT_REGISTRY" ]; then
    while IFS= read -r gate_val; do
        if [ -z "$gate_val" ] || [ "$gate_val" = "null" ]; then
            continue
        fi
        if ! grep -qE "^${gate_val}\(\)" "$PHASE_GATE"; then
            corrupt_valid=1  # found a bad gate → validation would fail
            break
        fi
    done < <(jq -r '.phases[] | (.gate_in, .gate_out) | select(. != null)' "$CORRUPT_REGISTRY" 2>/dev/null)
fi

if [ "$corrupt_valid" -eq 1 ]; then
    echo "GREEN AC8: corrupt fixture with nonexistent gate correctly fails validation (anti-tautology passed)"
else
    echo "RED AC8: corrupt fixture with nonexistent gate was not detected (anti-tautology failed)"
    rc=1
fi

exit "$rc"
