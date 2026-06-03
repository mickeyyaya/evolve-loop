#!/usr/bin/env bash
# ACS cycle-210 / Task-2 AC3 — phase-registry.json contains a spec-verify entry
# that is optional=true AND absent from config.mandatory_phases (NEGATIVE axis:
# adding it must NOT silently make it mandatory, which would alter the spine).
#
# Behavioral: python3 deserializes the registry and inspects the parsed data
# structure (the same JSON the orchestrator's config.Load consumes), not a
# grep for the literal "spec-verify".
#
# RED at baseline (entry absent); GREEN once Builder adds an optional entry.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

REG="docs/architecture/phase-registry.json"
[ -f "$REG" ] || { echo "RED: $REG missing" >&2; exit 1; }

python3 - "$REG" <<'PY' >&2 || exit 1
import json, sys
d = json.load(open(sys.argv[1]))
phases = d.get("phases", [])
phase = next((p for p in phases if p.get("name") == "spec-verify"), None)
if phase is None:
    print(f"RED: 'spec-verify' not in registry. names={[p.get('name') for p in phases]}")
    sys.exit(1)
if phase.get("optional") is not True:
    print(f"RED: spec-verify optional={phase.get('optional')!r}, must be true")
    sys.exit(1)
mandatory = d.get("config", {}).get("mandatory_phases", [])
if "spec-verify" in mandatory:
    print(f"RED: spec-verify must NOT be in mandatory_phases ({mandatory})")
    sys.exit(1)
print("GREEN: spec-verify present, optional=true, not mandatory")
PY
exit 0
