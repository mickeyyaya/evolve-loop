#!/usr/bin/env bash
# ACS — cycle-252 task `psmas-phase-skip-wire-go-router`
# acs-predicate: config-check (grep waiver) — the criterion "orchestrator
# wires PSMASEnabled from EVOLVE_PSMAS_SKIP=1" is a composition-root
# wiring-presence check: the router behavioral suite (001-) cannot reach
# os.Getenv in core, and no seam exists to subprocess-test the orchestrator
# env read in isolation. The behavioral anchor lives in 001- (Route()
# consumes the gate); this predicate adds the compile proof plus the
# wiring sweep. Auditor: review waiver validity.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

top=$(git rev-parse --show-toplevel)

# Compile proof — the wiring must build (subprocess, exit-code authoritative).
assert_go_build ./... || exit 1

# Wiring sweep: EVOLVE_PSMAS_SKIP must be read in non-test core/cmd source
# AND the same source must thread PSMASEnabled (the RouteInput gate).
hits=$(grep -rln 'EVOLVE_PSMAS_SKIP' "$top/go/internal/core" "$top/go/cmd" 2>/dev/null | grep -v '_test\.go' || true)
if [ -z "$hits" ]; then
    echo "RED: EVOLVE_PSMAS_SKIP not read anywhere in non-test go/internal/core or go/cmd source" >&2
    exit 1
fi
if ! echo "$hits" | xargs grep -l 'PSMASEnabled' >/dev/null 2>&1; then
    echo "RED: file(s) reading EVOLVE_PSMAS_SKIP never thread PSMASEnabled into RouteInput: $hits" >&2
    exit 1
fi

echo "GREEN: orchestrator reads EVOLVE_PSMAS_SKIP and threads PSMASEnabled (compile-proven)"
exit 0
