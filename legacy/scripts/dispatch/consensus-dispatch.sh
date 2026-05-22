#!/usr/bin/env bash
#
# consensus-dispatch.sh — Cross-CLI consensus auditor dispatch (v8.54.0+).
#
# Reads a profile's consensus block (e.g., .evolve/profiles/auditor.json:consensus)
# and dispatches N parallel audit invocations, each under a DIFFERENT CLI.
# Aggregates results via aggregator.sh's cross-cli-vote merge mode (MAJORITY-
# PASS with FAIL-VETO). Defeats same-vendor sycophancy at the cost of N x audit
# budget.
#
# Why this exists separately from cmd_dispatch_parallel:
# Standard fan-out invokes the SAME profile N times for SEMANTIC dimensions
# (auditor: eval-replay, lint, regression, build-quality). Consensus fan-out
# invokes the SAME audit N times under N DIFFERENT CLIs. Both use parallel
# execution + the aggregator, but the dispatch logic (which CLI for which
# worker) is fundamentally different.
#
# Inputs (env vars):
#   CYCLE              cycle number
#   WORKSPACE_PATH     .evolve/runs/cycle-N/
#   PROFILE_PATH       absolute path to profile JSON (must have .consensus block)
#   PROMPT_FILE        path to audit prompt (shared across all voters)
#
# Optional:
#   EVOLVE_CONSENSUS_AUDIT  if "0", refuse to dispatch (reaffirms operator opt-in)
#
# Output:
#   Aggregated cross-cli-vote audit at $WORKSPACE_PATH/audit-report.md
#   Per-CLI worker artifacts at $WORKSPACE_PATH/consensus-workers/<cli>.md
#
# Exit codes:
#   0   — consensus reached: PASS or WARN. Workspace contains artifact.
#   1   — consensus FAIL (any CLI returned FAIL — veto active)
#   2   — runtime error (insufficient voters, capability constraint failure)
#  10   — profile validation error
#
# Bash 3.2 compatible.

set -uo pipefail

ADAPTERS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../cli_adapters" && pwd)"
DISPATCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CAP_CHECK="$ADAPTERS_DIR/_capability-check.sh"
AGG="$DISPATCH_DIR/aggregator.sh"
FANOUT="$DISPATCH_DIR/fanout-dispatch.sh"

log() { echo "[consensus-dispatch] $*" >&2; }
fail() { log "FAIL: $*"; exit 2; }
profile_fail() { log "PROFILE-ERROR: $*"; exit 10; }

CYCLE="${CYCLE:?usage: CYCLE/WORKSPACE_PATH/PROFILE_PATH/PROMPT_FILE env vars required}"
WORKSPACE_PATH="${WORKSPACE_PATH:?WORKSPACE_PATH required}"
PROFILE_PATH="${PROFILE_PATH:?PROFILE_PATH required}"
PROMPT_FILE="${PROMPT_FILE:?PROMPT_FILE required}"

[ -f "$PROFILE_PATH" ] || profile_fail "profile not found: $PROFILE_PATH"
[ -f "$PROMPT_FILE" ]  || fail "prompt file missing: $PROMPT_FILE"
[ -d "$WORKSPACE_PATH" ] || mkdir -p "$WORKSPACE_PATH"

# Operator-side sanity: refuse if consensus opt-in is explicitly off
if [ "${EVOLVE_CONSENSUS_AUDIT:-1}" = "0" ]; then
    fail "EVOLVE_CONSENSUS_AUDIT=0 — refusing to run consensus dispatch (operator opt-out)"
fi

# Read consensus block
ENABLED=$(jq -r '.consensus.enabled // false' "$PROFILE_PATH")
CLI_VOTERS=$(jq -r '.consensus.cli_voters // [] | .[]' "$PROFILE_PATH")
QUORUM=$(jq -r '.consensus.quorum // 0' "$PROFILE_PATH")
MIN_TIER=$(jq -r '.consensus.require_min_tier // "hybrid"' "$PROFILE_PATH")

if [ "$ENABLED" = "false" ]; then
    profile_fail "profile.consensus.enabled is false; set to true and re-run, or invoke standard dispatch_parallel"
fi
[ -n "$CLI_VOTERS" ] || profile_fail "profile.consensus.cli_voters is empty"
[[ "$QUORUM" =~ ^[0-9]+$ ]] || profile_fail "profile.consensus.quorum must be integer"

# Verify each voter meets the minimum capability tier requirement
log "validating voter capabilities (require_min_tier=$MIN_TIER)..."
VOTER_COUNT=0
ELIGIBLE_VOTERS=""
for cli in $CLI_VOTERS; do
    VOTER_COUNT=$((VOTER_COUNT + 1))
    if [ ! -x "$CAP_CHECK" ]; then
        log "WARN: capability-check missing; cannot validate $cli — including anyway"
        ELIGIBLE_VOTERS="${ELIGIBLE_VOTERS}${ELIGIBLE_VOTERS:+ }$cli"
        continue
    fi
    tier=$(bash "$CAP_CHECK" "$cli" 2>/dev/null | jq -r '.quality_tier // "unknown"')
    case "$MIN_TIER" in
        full)
            [ "$tier" = "full" ] && ELIGIBLE_VOTERS="${ELIGIBLE_VOTERS}${ELIGIBLE_VOTERS:+ }$cli" \
                || log "  excluded $cli (tier=$tier, require=full)"
            ;;
        hybrid)
            case "$tier" in
                full|hybrid) ELIGIBLE_VOTERS="${ELIGIBLE_VOTERS}${ELIGIBLE_VOTERS:+ }$cli" ;;
                *) log "  excluded $cli (tier=$tier, require>=hybrid)" ;;
            esac
            ;;
        degraded)
            case "$tier" in
                full|hybrid|degraded) ELIGIBLE_VOTERS="${ELIGIBLE_VOTERS}${ELIGIBLE_VOTERS:+ }$cli" ;;
                *) log "  excluded $cli (tier=$tier, require>=degraded)" ;;
            esac
            ;;
        none|*)
            ELIGIBLE_VOTERS="${ELIGIBLE_VOTERS}${ELIGIBLE_VOTERS:+ }$cli"
            ;;
    esac
done

ELIGIBLE_COUNT=$(echo $ELIGIBLE_VOTERS | wc -w | tr -d ' ')
log "voters: $VOTER_COUNT declared, $ELIGIBLE_COUNT eligible (after tier filter)"
log "eligible: $ELIGIBLE_VOTERS"

# Adjust quorum if eligible count is less than declared quorum
if [ "$ELIGIBLE_COUNT" -lt "$QUORUM" ]; then
    log "WARN: eligible count ($ELIGIBLE_COUNT) < declared quorum ($QUORUM); reducing quorum to ceil($ELIGIBLE_COUNT / 2)"
    QUORUM=$(( (ELIGIBLE_COUNT + 1) / 2 ))
    log "  effective quorum: $QUORUM"
fi
if [ "$ELIGIBLE_COUNT" -lt 2 ]; then
    fail "consensus requires at least 2 eligible voters; got $ELIGIBLE_COUNT"
fi

# Build commands TSV for fanout-dispatch.sh
# Each line: <name>\t<command>
WORKERS_DIR="$WORKSPACE_PATH/consensus-workers"
mkdir -p "$WORKERS_DIR"
COMMANDS_TSV="$WORKERS_DIR/.commands.tsv"
RESULTS_TSV="$WORKERS_DIR/.results.tsv"
> "$COMMANDS_TSV"

for cli in $ELIGIBLE_VOTERS; do
    artifact="$WORKERS_DIR/${cli}-audit.md"
    # Each voter invokes the auditor under their CLI. We use subagent-run.sh's
    # approach: call the per-CLI adapter with the audit prompt and artifact path.
    # Simplification: the adapter will produce a Verdict line; we capture it.
    ADAPTER="$ADAPTERS_DIR/${cli}.sh"
    if [ ! -x "$ADAPTER" ]; then
        log "skipping $cli: adapter missing or not executable"
        continue
    fi
    # Worker command: env-prefixed adapter invocation.
    # Resolve model from profile
    model=$(jq -r '.model_tier_default // "sonnet"' "$PROFILE_PATH")
    cmd=$(cat <<CMDEOF
PROFILE_PATH='$PROFILE_PATH' RESOLVED_MODEL='$model' \
PROMPT_FILE='$PROMPT_FILE' CYCLE='$CYCLE' \
WORKSPACE_PATH='$WORKERS_DIR' STDOUT_LOG='$WORKERS_DIR/${cli}-stdout.log' \
STDERR_LOG='$WORKERS_DIR/${cli}-stderr.log' ARTIFACT_PATH='$artifact' \
bash '$ADAPTER'
CMDEOF
)
    printf '%s\t%s\n' "$cli" "$cmd" >> "$COMMANDS_TSV"
done

WORKER_COUNT=$(wc -l < "$COMMANDS_TSV" | tr -d ' ')
[ "$WORKER_COUNT" -ge 2 ] || fail "after filter, only $WORKER_COUNT workers ready (need ≥2)"

log "dispatching $WORKER_COUNT parallel cross-CLI workers..."
set +e
bash "$FANOUT" "$COMMANDS_TSV" "$RESULTS_TSV"
fanout_rc=$?
set -e
log "fanout completed: rc=$fanout_rc"

# Collect worker artifacts
worker_artifacts=""
for cli in $ELIGIBLE_VOTERS; do
    artifact="$WORKERS_DIR/${cli}-audit.md"
    if [ -f "$artifact" ] && [ -s "$artifact" ]; then
        worker_artifacts="${worker_artifacts} $artifact"
    else
        log "WARN: $cli produced no usable artifact; consensus may be reduced"
    fi
done

if [ -z "$worker_artifacts" ]; then
    fail "no worker artifacts produced; cannot aggregate"
fi

# Aggregate via cross-cli-vote merge mode
AGG_OUTPUT="$WORKSPACE_PATH/audit-report.md"
log "aggregating via cross-cli-vote..."
set +e
# shellcheck disable=SC2086
bash "$AGG" cross-cli-vote "$AGG_OUTPUT" $worker_artifacts
agg_rc=$?
set -e

log "DONE: consensus dispatch rc=$agg_rc; aggregate at $AGG_OUTPUT"
exit "$agg_rc"
