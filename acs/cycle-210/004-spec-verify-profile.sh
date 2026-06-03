#!/usr/bin/env bash
# ACS cycle-210 / Task-2 AC2 — spec-verifier profile is git-TRACKED and a VALID
# profile: parses as JSON (behavioral — runs the actual json parser the
# orchestrator uses), name == spec-verifier, declares output_artifact, and
# max_turns is within the sane [1,20] bound (edge axis).
#
# This is behavioral, not grep-only: python3 -c json.load actually deserializes
# the file. A malformed-JSON stub that merely contained the string
# "spec-verifier" would FAIL here.
#
# RED at baseline (file absent); GREEN once Builder writes a valid profile.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

FILE=".evolve/profiles/spec-verifier.json"

[ -f "$FILE" ] || { echo "RED: $FILE missing on disk" >&2; exit 1; }
git ls-files --error-unmatch "$FILE" >/dev/null 2>&1 \
  || { echo "RED: $FILE untracked — would be dropped at ship (cycle-209 mode)" >&2; exit 1; }

python3 - "$FILE" <<'PY' >&2 || exit 1
import json, sys
p = sys.argv[1]
try:
    d = json.load(open(p))
except Exception as e:
    print(f"RED: {p} is not valid JSON: {e}")
    sys.exit(1)
if d.get("name") != "spec-verifier":
    print(f"RED: profile name={d.get('name')!r}, expected 'spec-verifier'")
    sys.exit(1)
if "output_artifact" not in d:
    print("RED: profile missing required 'output_artifact' field")
    sys.exit(1)
t = d.get("max_turns", 0)
if not isinstance(t, int) or t <= 0 or t > 20:
    print(f"RED: max_turns={t!r} out of bound [1,20]")
    sys.exit(1)
print(f"GREEN: spec-verifier profile valid (output_artifact={d['output_artifact']}, max_turns={t})")
PY
exit 0
