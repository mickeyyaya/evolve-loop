#!/usr/bin/env bash
# ACS cycle-210 / Task-3 AC3 — phase-registry.json contains a doc-sync entry
# that is optional=true AND positioned AFTER build (higher array index). The
# ordering is the whole point of doc-sync (it documents what build produced),
# so an entry inserted before build would be semantically wrong even if
# present — this predicate asserts the relationship, not mere presence.
#
# Behavioral: python3 deserializes the registry and compares the parsed
# positions of build and doc-sync in the phases array.
#
# RED at baseline (entry absent); GREEN once Builder inserts doc-sync after
# build.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

REG="docs/architecture/phase-registry.json"
[ -f "$REG" ] || { echo "RED: $REG missing" >&2; exit 1; }

python3 - "$REG" <<'PY' >&2 || exit 1
import json, sys
d = json.load(open(sys.argv[1]))
phases = d.get("phases", [])
names = [p.get("name") for p in phases]
build_idx = names.index("build") if "build" in names else None
doc_idx = names.index("doc-sync") if "doc-sync" in names else None
if build_idx is None:
    print("RED: 'build' phase not found in registry")
    sys.exit(1)
if doc_idx is None:
    print(f"RED: 'doc-sync' not in registry. names={names}")
    sys.exit(1)
phase = phases[doc_idx]
if phase.get("optional") is not True:
    print(f"RED: doc-sync optional={phase.get('optional')!r}, must be true")
    sys.exit(1)
if doc_idx <= build_idx:
    print(f"RED: doc-sync (idx={doc_idx}) must come AFTER build (idx={build_idx})")
    sys.exit(1)
print(f"GREEN: doc-sync (idx={doc_idx}) after build (idx={build_idx}), optional=true")
PY
exit 0
