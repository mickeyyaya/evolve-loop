#!/bin/bash
# apply-envelope-additions.sh — Atomically merge envelope additions into all profiles.
#
# Target path: scripts/routing/apply-envelope-additions.sh (intended for one-shot Phase 1 application)
# Bash 3.2 compatible.
#
# Reads: profile-envelope-additions.json (manifest of additions per phase)
# Writes: .evolve/profiles/<role>.json (jq-merged)
# Backup: .evolve/profiles/<role>.json.bak-<timestamp>
#
# Usage:
#   apply-envelope-additions.sh --manifest <path> --target <profile-dir> [--dry-run]
#
# Exit codes:
#   0 = success
#   2 = manifest or target invalid
#   3 = atomic merge failed (rollback succeeded)
#   4 = catastrophic (partial state — operator intervention required)

set -uo pipefail

MANIFEST=""
TARGET_DIR=""
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manifest) MANIFEST="$2"; shift 2 ;;
    --target)   TARGET_DIR="$2"; shift 2 ;;
    --dry-run)  DRY_RUN=1; shift ;;
    *) echo "Unknown flag: $1" >&2; exit 4 ;;
  esac
done

if [[ -z "$MANIFEST" || -z "$TARGET_DIR" ]]; then
  echo "Usage: apply-envelope-additions.sh --manifest <path> --target <profile-dir> [--dry-run]" >&2
  exit 4
fi

if [[ ! -f "$MANIFEST" ]]; then
  echo "ERROR: manifest not found: $MANIFEST" >&2
  exit 2
fi
if [[ ! -d "$TARGET_DIR" ]]; then
  echo "ERROR: target dir not found: $TARGET_DIR" >&2
  exit 2
fi

# Validate manifest schema
if ! jq -e '.additions | type == "object"' "$MANIFEST" > /dev/null; then
  echo "ERROR: manifest missing .additions object" >&2
  exit 2
fi

TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BACKUP_SUFFIX=".bak-$TIMESTAMP"

# Collect phase names from manifest
PHASES=$(jq -r '.additions | keys[]' "$MANIFEST")

echo "=== apply-envelope-additions.sh ==="
echo "Manifest:  $MANIFEST"
echo "Target:    $TARGET_DIR"
echo "Backup:    *.json$BACKUP_SUFFIX"
[[ $DRY_RUN -eq 1 ]] && echo "Mode:      DRY-RUN (no writes)"
echo ""

APPLIED=0
SKIPPED=0
FAILED=0
ROLLBACK_LIST=()

for phase in $PHASES; do
  profile="$TARGET_DIR/$phase.json"
  if [[ ! -f "$profile" ]]; then
    printf "  %-15s SKIP   (profile file not found)\n" "$phase"
    SKIPPED=$((SKIPPED + 1))
    continue
  fi

  # Check for already-applied (idempotent)
  existing_envelope=$(jq -r '.model_tier_envelope // empty' "$profile" 2>/dev/null)
  if [[ -n "$existing_envelope" ]]; then
    printf "  %-15s SKIP   (envelope already present)\n" "$phase"
    SKIPPED=$((SKIPPED + 1))
    continue
  fi

  if [[ $DRY_RUN -eq 1 ]]; then
    additions=$(jq -c --arg p "$phase" '.additions[$p]' "$MANIFEST")
    printf "  %-15s WOULD-APPLY  additions=%s\n" "$phase" "$additions"
    APPLIED=$((APPLIED + 1))
    continue
  fi

  # Atomic merge: profile + additions[phase]
  tmp="${profile}.tmp.$$"
  backup="${profile}${BACKUP_SUFFIX}"

  cp "$profile" "$backup" || { echo "FAIL: backup failed for $phase" >&2; FAILED=$((FAILED + 1)); continue; }

  if jq --slurpfile m "$MANIFEST" --arg p "$phase" \
       '. + ($m[0].additions[$p] // {})' "$profile" > "$tmp"; then
    if mv "$tmp" "$profile"; then
      printf "  %-15s APPLIED  (backup: %s)\n" "$phase" "$(basename "$backup")"
      ROLLBACK_LIST+=("$profile:$backup")
      APPLIED=$((APPLIED + 1))
    else
      echo "FAIL: atomic rename failed for $phase" >&2
      rm -f "$tmp"
      FAILED=$((FAILED + 1))
    fi
  else
    echo "FAIL: jq merge failed for $phase" >&2
    rm -f "$tmp"
    FAILED=$((FAILED + 1))
  fi
done

echo ""
echo "============================================"
echo "APPLIED: $APPLIED  SKIPPED: $SKIPPED  FAILED: $FAILED"

if [[ "$FAILED" -gt 0 ]]; then
  echo ""
  echo "PARTIAL FAILURE. To rollback applied profiles:"
  for entry in "${ROLLBACK_LIST[@]}"; do
    profile="${entry%%:*}"
    backup="${entry#*:}"
    echo "  mv '$backup' '$profile'"
  done
  exit 3
fi

if [[ "$APPLIED" -eq 0 && $DRY_RUN -eq 0 ]]; then
  echo "Nothing applied — likely already at target state. (Idempotent.)"
fi

exit 0
