#!/usr/bin/env bash
# ACS predicate: verify EVOLVE_BUILDER_ISOLATION_CHECK and EVOLVE_BUILDER_ISOLATION_STRICT default to 1
# cycle: 41
# ac: AC2 — EVOLVE_BUILDER_ISOLATION_CHECK defaults to 1; AC3 — EVOLVE_BUILDER_ISOLATION_STRICT defaults to 1; AC4 — opt-out via =0 is documented; AC1 — git diff HEAD check is present
# metadata: {"id":"002","slug":"worktree-isolation-default-on","cycle":41,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
GATE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
[ -f "$GATE" ] || { echo "ERR: $GATE not found"; exit 1; }

rc=0

# AC2: EVOLVE_BUILDER_ISOLATION_CHECK defaults to 1 (:-1 pattern)
if ! grep -q 'EVOLVE_BUILDER_ISOLATION_CHECK:-1' "$GATE"; then
    echo "FAIL AC2: EVOLVE_BUILDER_ISOLATION_CHECK does not default to 1 (missing ':-1' pattern)"
    rc=1
else
    echo "PASS AC2: EVOLVE_BUILDER_ISOLATION_CHECK defaults to 1"
fi

# AC3: EVOLVE_BUILDER_ISOLATION_STRICT defaults to 1 (:-1 pattern)
if ! grep -q 'EVOLVE_BUILDER_ISOLATION_STRICT:-1' "$GATE"; then
    echo "FAIL AC3: EVOLVE_BUILDER_ISOLATION_STRICT does not default to 1 (missing ':-1' pattern)"
    rc=1
else
    echo "PASS AC3: EVOLVE_BUILDER_ISOLATION_STRICT defaults to 1"
fi

# AC1: git diff HEAD check is the primary detection mechanism
if ! grep -q 'git.*diff.*HEAD' "$GATE"; then
    echo "FAIL AC1: git diff HEAD check not found in phase-gate.sh"
    rc=1
else
    echo "PASS AC1: git diff HEAD check present in _check_builder_isolation_breach()"
fi

# AC4: opt-out documentation (EVOLVE_BUILDER_ISOLATION_CHECK=0 or EVOLVE_BUILDER_ISOLATION_STRICT=0 mentioned)
if ! grep -q 'ISOLATION_CHECK=0\|ISOLATION_STRICT=0' "$GATE"; then
    echo "FAIL AC4: opt-out (=0) not documented in phase-gate.sh"
    rc=1
else
    echo "PASS AC4: opt-out (=0) documented"
fi

exit $rc
