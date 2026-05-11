#!/usr/bin/env bash
# builder-simplify-advisory.sh — opt-in code-simplifier advisory pass (v9.2.0+, Cycle 16)
#
# Invoked by phase-gate.sh gate_build_to_audit AFTER builder exits and BEFORE auditor starts.
# Writes an advisory artifact to $WORKSPACE/code-simplifier-report.md via subagent-run.sh.
# Returns exit 0 always — advisory; never blocks the cycle.
# Gated by EVOLVE_SIMPLIFY_ENABLED=1 (default 0, opt-in only).
#
# Bash 3.2 portable (no declare -A, mapfile, ${var^^}, GNU sed/date).

set -uo pipefail

CYCLE="${1:?Usage: builder-simplify-advisory.sh <cycle> <workspace>}"
WORKSPACE="${2:?Missing workspace path}"

# No-op unless opt-in flag is explicitly set
if [ "${EVOLVE_SIMPLIFY_ENABLED:-0}" != "1" ]; then
    exit 0
fi

# Invoke the code-simplifier subagent via subagent-run.sh.
# The subagent writes its report to $WORKSPACE/code-simplifier-report.md.
# We use || true to ensure advisory failures never block the pipeline.
subagent-run.sh code-simplifier "$CYCLE" "$WORKSPACE" 2>/dev/null || true

exit 0
