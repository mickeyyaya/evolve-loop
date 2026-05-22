#!/bin/bash
# cycle-health-check.sh — Deterministic health fingerprint for any evolve-loop cycle
#
# Usage: bash scripts/observability/cycle-health-check.sh <cycle-number> <workspace-path>
#
# Produces a structured JSON report with per-signal PASS/WARN/ANOMALY status.
# This script is READ-ONLY — it examines artifacts and reports anomalies, never blocks.
# Can be run retroactively on any past cycle for forensic auditing.
#
# Research basis:
#   - Process supervision ("Let's Verify Step by Step", Lightman et al. 2023)
#   - SLSA provenance verification (supply chain integrity)
#   - Behavioral anomaly detection (SentinelAgent, He et al. 2025)

set -euo pipefail

CYCLE="${1:?Usage: cycle-health-check.sh <cycle> <workspace-path>}"
WORKSPACE="${2:?Usage: cycle-health-check.sh <cycle> <workspace-path>}"
LEDGER=".evolve/ledger.jsonl"
EVALS_DIR=".evolve/evals"

# Counters
ANOMALIES=0
WARNS=0
PASSES=0

# Output accumulator (JSON lines, assembled at end)
SIGNALS=""

add_signal() {
  local name="$1" status="$2" detail="$3"
  SIGNALS="${SIGNALS}$(printf '    "%s": {"status": "%s", "detail": "%s"}' "$name" "$status" "$detail"),"
  case "$status" in
    ANOMALY) ANOMALIES=$((ANOMALIES + 1)) ;;
    WARN)    WARNS=$((WARNS + 1)) ;;
    PASS)    PASSES=$((PASSES + 1)) ;;
  esac
}

# ─── Signal 1: Ledger Role Completeness ───────────────────────────────
# Every cycle must have scout, builder, and auditor entries.
# Missing entries indicate the orchestrator skipped agents.

MISSING_ROLES=""
for ROLE in scout builder auditor; do
  if [ -f "$LEDGER" ]; then
    # Match exact cycle number (with optional space after colon) and exact role
    if grep -q "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" 2>/dev/null && \
       grep "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" 2>/dev/null | grep -q "\"role\"[: ]*\"${ROLE}\""; then
      : # role found
    else
      MISSING_ROLES="${MISSING_ROLES}${ROLE} "
    fi
  else
    MISSING_ROLES="scout builder auditor"
    break
  fi
done

if [ -n "$MISSING_ROLES" ]; then
  add_signal "ledgerRoleCompleteness" "ANOMALY" "Missing role entries: ${MISSING_ROLES}"
else
  add_signal "ledgerRoleCompleteness" "PASS" "All roles present (scout, builder, auditor)"
fi

# ─── Signal 2: Ledger Timestamp Spacing ───────────────────────────────
# Role entries for the same cycle must have realistic time gaps.
# Research: bulk forgery produces sub-second timestamps (cycles 102-111 incident).

if [ -f "$LEDGER" ]; then
  CYCLE_ENTRIES=$(grep "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" 2>/dev/null || true)
  if [ -n "$CYCLE_ENTRIES" ]; then
    # Extract timestamps and compute min/max delta
    TIMESTAMPS=$(echo "$CYCLE_ENTRIES" | grep -o '"ts"[: ]*"[^"]*"' | sed 's/"ts"[: ]*"//;s/"//' | sort)
    TS_COUNT=$(echo "$TIMESTAMPS" | wc -l | tr -d '[:space:]')

    if [ "$TS_COUNT" -ge 2 ]; then
      FIRST_TS=$(echo "$TIMESTAMPS" | head -1)
      LAST_TS=$(echo "$TIMESTAMPS" | tail -1)

      # Convert ISO timestamps to epoch seconds (portable)
      FIRST_EPOCH=$(date -jf "%Y-%m-%dT%H:%M:%S" "${FIRST_TS%%.*}" "+%s" 2>/dev/null || date -d "${FIRST_TS}" "+%s" 2>/dev/null || echo 0)
      LAST_EPOCH=$(date -jf "%Y-%m-%dT%H:%M:%S" "${LAST_TS%%.*}" "+%s" 2>/dev/null || date -d "${LAST_TS}" "+%s" 2>/dev/null || echo 0)

      if [ "$FIRST_EPOCH" != "0" ] && [ "$LAST_EPOCH" != "0" ]; then
        SPAN=$((LAST_EPOCH - FIRST_EPOCH))
        if [ "$SPAN" -lt 15 ]; then
          add_signal "timestampSpacing" "ANOMALY" "All ${TS_COUNT} entries span only ${SPAN}s (minimum 15s expected)"
        elif [ "$SPAN" -lt 30 ]; then
          add_signal "timestampSpacing" "WARN" "Entries span ${SPAN}s (tight but possible)"
        else
          add_signal "timestampSpacing" "PASS" "Entries span ${SPAN}s across ${TS_COUNT} entries"
        fi
      else
        add_signal "timestampSpacing" "WARN" "Could not parse timestamps for delta analysis"
      fi
    else
      add_signal "timestampSpacing" "WARN" "Only ${TS_COUNT} ledger entry for cycle — insufficient for spacing analysis"
    fi
  else
    add_signal "timestampSpacing" "ANOMALY" "No ledger entries found for cycle ${CYCLE}"
  fi
else
  add_signal "timestampSpacing" "ANOMALY" "Ledger file missing: ${LEDGER}"
fi

# ─── Signal 3: Workspace Artifact Existence ───────────────────────────
# A legitimate cycle produces scout-report.md, build-report.md, audit-report.md.

MISSING_ARTIFACTS=""
for FILE in scout-report.md build-report.md audit-report.md; do
  if [ ! -f "${WORKSPACE}/${FILE}" ]; then
    MISSING_ARTIFACTS="${MISSING_ARTIFACTS}${FILE} "
  fi
done

if [ -n "$MISSING_ARTIFACTS" ]; then
  add_signal "artifactExistence" "ANOMALY" "Missing workspace files: ${MISSING_ARTIFACTS}"
else
  add_signal "artifactExistence" "PASS" "All required workspace artifacts present"
fi

# ─── Signal 4: Eval File Existence (per-cycle) ───────────────────────
# Scout must create eval definitions for the tasks in this cycle.
# Cross-reference task slugs from ledger entries to verify matching eval files exist.

if [ -d "$EVALS_DIR" ]; then
  # Extract task slugs from this cycle's ledger entries (scout "tasksSelected" or builder "task" field)
  CYCLE_TASK_SLUGS=""
  if [ -f "$LEDGER" ]; then
    CYCLE_TASK_SLUGS=$(grep "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" 2>/dev/null \
      | grep -o '"task"[: ]*"[^"]*"' \
      | sed 's/"task"[: ]*"//;s/"//' \
      | sort -u || true)
  fi

  if [ -n "$CYCLE_TASK_SLUGS" ]; then
    # Check that each task slug has a corresponding eval file
    MISSING_EVALS=""
    FOUND_EVALS=0
    while IFS= read -r SLUG; do
      [ -z "$SLUG" ] && continue
      if [ -f "${EVALS_DIR}/${SLUG}.md" ]; then
        FOUND_EVALS=$((FOUND_EVALS + 1))
      else
        MISSING_EVALS="${MISSING_EVALS}${SLUG}.md "
      fi
    done <<< "$CYCLE_TASK_SLUGS"

    if [ -n "$MISSING_EVALS" ]; then
      add_signal "evalFileExistence" "ANOMALY" "Missing eval definitions for cycle tasks: ${MISSING_EVALS}"
    elif [ "$FOUND_EVALS" -gt 0 ]; then
      add_signal "evalFileExistence" "PASS" "${FOUND_EVALS} eval definition(s) found for cycle tasks"
    else
      add_signal "evalFileExistence" "WARN" "No task slugs found in ledger to verify evals against"
    fi
  else
    # Fallback: no task slugs in ledger, check if any non-canary evals exist at all
    EVAL_COUNT=$(find "$EVALS_DIR" -name "*.md" -not -name "_canary*" 2>/dev/null | wc -l | tr -d '[:space:]')
    if [ "$EVAL_COUNT" = "0" ]; then
      add_signal "evalFileExistence" "ANOMALY" "No eval definitions found in ${EVALS_DIR}"
    else
      add_signal "evalFileExistence" "WARN" "Cannot verify per-cycle evals (no task slugs in ledger); ${EVAL_COUNT} total eval(s) exist"
    fi
  fi
else
  add_signal "evalFileExistence" "ANOMALY" "Evals directory missing: ${EVALS_DIR}"
fi

# ─── Signal 5: Eval Checksum Integrity ────────────────────────────────
# Checksums captured after Scout must still match.

CHECKSUM_FILE="${WORKSPACE}/eval-checksums.json"
if [ -f "$CHECKSUM_FILE" ]; then
  # Use sha256sum or shasum depending on platform
  if command -v sha256sum &>/dev/null; then
    SHA_CMD="sha256sum"
  elif command -v shasum &>/dev/null; then
    SHA_CMD="shasum -a 256"
  else
    SHA_CMD=""
  fi

  if [ -n "$SHA_CMD" ]; then
    if $SHA_CMD -c "$CHECKSUM_FILE" > /dev/null 2>&1; then
      add_signal "evalChecksumIntegrity" "PASS" "All eval checksums verified"
    else
      add_signal "evalChecksumIntegrity" "ANOMALY" "Eval checksum mismatch — possible tampering"
    fi
  else
    add_signal "evalChecksumIntegrity" "WARN" "No sha256sum or shasum available for verification"
  fi
else
  add_signal "evalChecksumIntegrity" "WARN" "No eval checksum file found at ${CHECKSUM_FILE}"
fi

# ─── Signal 6: Challenge Token Consistency ────────────────────────────
# If challenge tokens are in use, verify they match across all artifacts.

CHALLENGE_TOKEN=""
CHALLENGE_MISMATCHES=""

# Extract challenge from ledger (orchestrator phase-transition entry)
if [ -f "$LEDGER" ]; then
  CHALLENGE_TOKEN=$(grep "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" | grep -o '"challenge"[: ]*"[^"]*"' | head -1 | sed 's/"challenge"[: ]*"//;s/"//' || true)
fi

if [ -n "$CHALLENGE_TOKEN" ]; then
  for FILE in scout-report.md build-report.md audit-report.md; do
    FILEPATH="${WORKSPACE}/${FILE}"
    if [ -f "$FILEPATH" ]; then
      if ! grep -q "$CHALLENGE_TOKEN" "$FILEPATH" 2>/dev/null; then
        CHALLENGE_MISMATCHES="${CHALLENGE_MISMATCHES}${FILE} "
      fi
    fi
  done

  if [ -n "$CHALLENGE_MISMATCHES" ]; then
    add_signal "challengeTokenConsistency" "ANOMALY" "Challenge token missing from: ${CHALLENGE_MISMATCHES}"
  else
    add_signal "challengeTokenConsistency" "PASS" "Challenge token consistent across all artifacts"
  fi
else
  add_signal "challengeTokenConsistency" "WARN" "No challenge token found in ledger for cycle ${CYCLE}"
fi

# ─── Signal 7: Git Commit Velocity ────────────────────────────────────
# Time between consecutive commits near this cycle must be physically plausible.
# Uses ledger timestamps (cycle-specific) when available, falls back to git log.

VELOCITY_CHECKED=false

# Primary: use ledger timestamps for this cycle (cycle-specific, works retroactively)
if [ -f "$LEDGER" ]; then
  CYCLE_TS=$(grep "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" 2>/dev/null \
    | grep -o '"ts"[: ]*"[^"]*"' | sed 's/"ts"[: ]*"//;s/"//' | sort || true)

  if [ -n "$CYCLE_TS" ]; then
    TS_COUNT=$(echo "$CYCLE_TS" | wc -l | tr -d '[:space:]')
    if [ "$TS_COUNT" -ge 2 ]; then
      # Already covered by Signal 2 (timestamp spacing), but also check
      # against the PREVIOUS cycle's last entry for inter-cycle velocity
      PREV_CYCLE=$((CYCLE - 1))
      PREV_LAST_TS=$(grep "\"cycle\"[: ]*${PREV_CYCLE}[^0-9]" "$LEDGER" 2>/dev/null \
        | grep -o '"ts"[: ]*"[^"]*"' | sed 's/"ts"[: ]*"//;s/"//' | sort | tail -1 || true)

      if [ -n "$PREV_LAST_TS" ]; then
        CURR_FIRST_TS=$(echo "$CYCLE_TS" | head -1)
        PREV_EPOCH=$(date -jf "%Y-%m-%dT%H:%M:%S" "${PREV_LAST_TS%%.*}" "+%s" 2>/dev/null || date -d "${PREV_LAST_TS}" "+%s" 2>/dev/null || echo 0)
        CURR_EPOCH=$(date -jf "%Y-%m-%dT%H:%M:%S" "${CURR_FIRST_TS%%.*}" "+%s" 2>/dev/null || date -d "${CURR_FIRST_TS}" "+%s" 2>/dev/null || echo 0)

        if [ "$PREV_EPOCH" != "0" ] && [ "$CURR_EPOCH" != "0" ]; then
          DELTA=$((CURR_EPOCH - PREV_EPOCH))
          VELOCITY_CHECKED=true
          if [ "$DELTA" -lt 5 ]; then
            add_signal "gitCommitVelocity" "ANOMALY" "Only ${DELTA}s between cycle $((CYCLE-1)) and cycle ${CYCLE} (sub-5s = mass cycle)"
          elif [ "$DELTA" -lt 30 ]; then
            add_signal "gitCommitVelocity" "WARN" "${DELTA}s between cycles (tight but possible)"
          else
            add_signal "gitCommitVelocity" "PASS" "${DELTA}s between cycle $((CYCLE-1)) and cycle ${CYCLE}"
          fi
        fi
      fi
    fi
  fi
fi

# Fallback: use git log (only valid when run immediately after cycle commits)
if ! $VELOCITY_CHECKED; then
  LAST_TWO_EPOCHS=$(git log --format="%at" -n 2 2>/dev/null || true)
  if [ -n "$LAST_TWO_EPOCHS" ]; then
    TS_CURRENT=$(echo "$LAST_TWO_EPOCHS" | head -1)
    TS_PREVIOUS=$(echo "$LAST_TWO_EPOCHS" | tail -1)

    if [ -n "$TS_CURRENT" ] && [ -n "$TS_PREVIOUS" ]; then
      DELTA=$((TS_CURRENT - TS_PREVIOUS))
      if [ "$DELTA" -lt 5 ]; then
        add_signal "gitCommitVelocity" "ANOMALY" "Only ${DELTA}s between last 2 commits (sub-5s = mass commit)"
      elif [ "$DELTA" -lt 30 ]; then
        add_signal "gitCommitVelocity" "WARN" "${DELTA}s between commits (tight but possible for S-complexity)"
      else
        add_signal "gitCommitVelocity" "PASS" "${DELTA}s between last 2 commits"
      fi
    else
      add_signal "gitCommitVelocity" "WARN" "Could not parse commit timestamps"
    fi
  else
    add_signal "gitCommitVelocity" "WARN" "Fewer than 2 commits in history"
  fi
fi

# ─── Signal 8: Diff Substance ─────────────────────────────────────────
# Classify changed files as boilerplate vs substantive.

BOILERPLATE_PATTERNS="\.prettierignore|\.editorconfig|\.gitignore|Makefile|pyproject\.toml|SECURITY\.md|SUPPORT\.md|LICENSE|\.npmignore|\.eslintignore"

DIFF_FILES=$(git diff HEAD~1 --name-only 2>/dev/null || true)
if [ -n "$DIFF_FILES" ]; then
  TOTAL_FILES=$(echo "$DIFF_FILES" | wc -l | tr -d '[:space:]')
  BOILERPLATE_COUNT=$(echo "$DIFF_FILES" | grep -cE "$BOILERPLATE_PATTERNS" 2>/dev/null || echo 0)
  BOILERPLATE_COUNT=$(echo "$BOILERPLATE_COUNT" | tr -d '[:space:]')

  if [ "$BOILERPLATE_COUNT" = "$TOTAL_FILES" ] && [ "$TOTAL_FILES" -gt 0 ]; then
    add_signal "diffSubstance" "WARN" "All ${TOTAL_FILES} changed files are boilerplate"
  elif [ "$TOTAL_FILES" -gt 0 ]; then
    SUBSTANTIVE=$((TOTAL_FILES - BOILERPLATE_COUNT))
    add_signal "diffSubstance" "PASS" "${SUBSTANTIVE}/${TOTAL_FILES} files are substantive"
  else
    add_signal "diffSubstance" "WARN" "No files in diff"
  fi
else
  add_signal "diffSubstance" "WARN" "Could not compute diff"
fi

# ─── Signal 9: Canary File Integrity ──────────────────────────────────
# Verify honeypot files are untouched.

CANARY_FILE="${WORKSPACE}/.integrity-canary"
CANARY_EXPECTED="${WORKSPACE}/.integrity-canary-expected"

if [ -f "$CANARY_EXPECTED" ]; then
  if [ -f "$CANARY_FILE" ]; then
    EXPECTED_VAL=$(cat "$CANARY_EXPECTED")
    ACTUAL_VAL=$(cat "$CANARY_FILE")
    if [ "$EXPECTED_VAL" = "$ACTUAL_VAL" ]; then
      add_signal "canaryIntegrity" "PASS" "Canary file untouched"
    else
      add_signal "canaryIntegrity" "ANOMALY" "Canary file tampered with (expected: ${EXPECTED_VAL:0:8}..., got: ${ACTUAL_VAL:0:8}...)"
    fi
  else
    add_signal "canaryIntegrity" "ANOMALY" "Canary file deleted"
  fi
else
  add_signal "canaryIntegrity" "WARN" "No canary expected-value file — canaries not planted this cycle"
fi

# ─── Signal 10: Hash Chain Integrity ──────────────────────────────────
# Verify ledger entries form a valid hash chain (each entry's prevHash matches).

if [ -f "$LEDGER" ]; then
  CYCLE_ENTRIES=$(grep "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" 2>/dev/null || true)
  if [ -n "$CYCLE_ENTRIES" ]; then
    # Check if prevHash field exists in entries (may be inside "data" object)
    HAS_HASH_CHAIN=false
    if echo "$CYCLE_ENTRIES" | grep -q '"prevHash"' 2>/dev/null; then
      HAS_HASH_CHAIN=true
    fi

    if $HAS_HASH_CHAIN; then
      # Verify chain: each entry's prevHash should match sha256 of the previous line in the full ledger
      # We need to check entries in order, including the entry BEFORE this cycle
      CHAIN_VALID=true
      BROKEN_AT=""

      # Get the line just before the first cycle entry as the seed
      FIRST_CYCLE_LINE=$(grep -n "\"cycle\"[: ]*${CYCLE}[^0-9]" "$LEDGER" 2>/dev/null | head -1 | cut -d: -f1)
      if [ -n "$FIRST_CYCLE_LINE" ] && [ "$FIRST_CYCLE_LINE" -gt 1 ]; then
        PREV_LINE_NUM=$((FIRST_CYCLE_LINE - 1))
        SEED_LINE=$(sed -n "${PREV_LINE_NUM}p" "$LEDGER")
        if command -v sha256sum &>/dev/null; then
          EXPECTED_HASH=$(echo -n "$SEED_LINE" | sha256sum | cut -d' ' -f1)
        elif command -v shasum &>/dev/null; then
          EXPECTED_HASH=$(echo -n "$SEED_LINE" | shasum -a 256 | cut -d' ' -f1)
        else
          EXPECTED_HASH=""
        fi
      else
        EXPECTED_HASH=""
      fi

      # Verify each cycle entry's prevHash against the computed hash
      while IFS= read -r line; do
        ENTRY_PREV=$(echo "$line" | grep -o '"prevHash"[: ]*"[^"]*"' | sed 's/"prevHash"[: ]*"//;s/"//')
        if [ -n "$ENTRY_PREV" ] && [ -n "$EXPECTED_HASH" ]; then
          if [ "$ENTRY_PREV" != "$EXPECTED_HASH" ]; then
            CHAIN_VALID=false
            BROKEN_AT="expected ${EXPECTED_HASH:0:12}... got ${ENTRY_PREV:0:12}..."
            break
          fi
        fi
        # Compute hash of this line for the next entry
        if command -v sha256sum &>/dev/null; then
          EXPECTED_HASH=$(echo -n "$line" | sha256sum | cut -d' ' -f1)
        elif command -v shasum &>/dev/null; then
          EXPECTED_HASH=$(echo -n "$line" | shasum -a 256 | cut -d' ' -f1)
        fi
      done <<< "$CYCLE_ENTRIES"

      if $CHAIN_VALID; then
        add_signal "hashChainIntegrity" "PASS" "Ledger hash chain verified for cycle ${CYCLE}"
      else
        add_signal "hashChainIntegrity" "ANOMALY" "Hash chain broken — ${BROKEN_AT}"
      fi
    else
      add_signal "hashChainIntegrity" "WARN" "No prevHash fields in ledger — hash chain not yet active for cycle ${CYCLE}"
    fi
  else
    add_signal "hashChainIntegrity" "WARN" "No entries for cycle ${CYCLE} to verify"
  fi
else
  add_signal "hashChainIntegrity" "ANOMALY" "Ledger file missing"
fi

# ─── Signal 11: Health Check Ran (self-reference) ─────────────────────
# The existence of this output file proves the health check ran.
# The Operator checks for this file — if missing, the orchestrator skipped the check.
add_signal "healthCheckRan" "PASS" "This health check executed successfully"

# ─── Assemble JSON Output ─────────────────────────────────────────────

# Remove trailing comma from signals
SIGNALS="${SIGNALS%,}"

HEALTHY="true"
if [ "$ANOMALIES" -gt 0 ]; then
  HEALTHY="false"
fi

OUTPUT=$(cat <<EOF
{
  "cycle": ${CYCLE},
  "healthy": ${HEALTHY},
  "anomalyCount": ${ANOMALIES},
  "warnCount": ${WARNS},
  "passCount": ${PASSES},
  "signals": {
${SIGNALS}
  }
}
EOF
)

echo "$OUTPUT"

# Write to workspace for Operator consumption
HEALTH_FILE="${WORKSPACE}/cycle-health.json"
echo "$OUTPUT" > "$HEALTH_FILE" 2>/dev/null || true

# Exit code reflects health
if [ "$ANOMALIES" -gt 0 ]; then
  exit 1
fi
exit 0
