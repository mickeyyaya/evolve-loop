#!/usr/bin/env bash
# ACS cycle-210 / Task-3 AC2+AC4 — doc-sync profile is git-TRACKED, valid JSON,
# name == doc-sync, declares output_artifact, AND grants write access to either
# the cycle workspace or docs/ paths (a doc-generation phase that cannot write
# anywhere is inert — this is the behavioral teeth of the task).
#
# Behavioral: python3 json.load deserializes the profile and inspects the
# allowed_tools + sandbox.write_subpaths it actually declares.
#
# RED at baseline (file absent); GREEN once Builder writes a valid writable
# profile.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

FILE=".evolve/profiles/doc-sync.json"

[ -f "$FILE" ] || { echo "RED: $FILE missing on disk" >&2; exit 1; }
git ls-files --error-unmatch "$FILE" >/dev/null 2>&1 \
  || { echo "RED: $FILE untracked — would be dropped at ship (cycle-209 mode)" >&2; exit 1; }

python3 - "$FILE" <<'PY' >&2 || exit 1
import json, sys
try:
    d = json.load(open(sys.argv[1]))
except Exception as e:
    print(f"RED: {sys.argv[1]} is not valid JSON: {e}")
    sys.exit(1)
if d.get("name") != "doc-sync":
    print(f"RED: profile name={d.get('name')!r}, expected 'doc-sync'")
    sys.exit(1)
if "output_artifact" not in d:
    print("RED: profile missing required 'output_artifact' field")
    sys.exit(1)
allowed = d.get("allowed_tools", [])
sandbox_write = d.get("sandbox", {}).get("write_subpaths", [])
combined = list(allowed) + list(sandbox_write)
can_write_cycle = any(".evolve/runs/cycle-" in s or "cycle-*" in s for s in combined)
can_write_docs = any("docs/" in s or "knowledge-base" in s for s in combined)
if not (can_write_cycle or can_write_docs):
    print(f"RED: doc-sync profile grants no write access. paths={combined}")
    sys.exit(1)
print(f"GREEN: doc-sync profile valid, writable (cycle={can_write_cycle}, docs={can_write_docs})")
PY
exit 0
