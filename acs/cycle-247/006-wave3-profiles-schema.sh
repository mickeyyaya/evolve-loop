#!/usr/bin/env bash
# ACS cycle-247 — phases-release-and-memory AC3 (behavioral via JSON parser).
# Each wave-3 profile must parse as JSON and carry the required fields of the
# shipped profile schema (reference: .evolve/profiles/mutation-gate.json).
# A malformed/missing-field profile is what the bridge dispatcher consumes —
# parsing with a real JSON parser is the consumption path, not a grep.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

rc=0
for p in changelog-sync post-ship-monitor api-contract-design context-condense; do
  f=".evolve/profiles/$p.json"
  [ -f "$f" ] || { echo "RED: $f missing" >&2; rc=1; continue; }
  if python3 - "$p" <<'PY'
import json, sys
p = sys.argv[1]
required = ["name", "cli", "model_tier_default", "role", "sandbox",
            "max_turns", "max_budget_usd", "allowed_tools", "output_artifact"]
try:
    d = json.load(open(f".evolve/profiles/{p}.json"))
except Exception as e:
    print(f"invalid JSON in {p}.json: {e}", file=sys.stderr); sys.exit(1)
missing = [k for k in required if k not in d]
if missing:
    print(f"missing keys in {p}.json: {missing}", file=sys.stderr); sys.exit(1)
if d["name"] != p:
    print(f"profile name {d['name']!r} != {p!r}", file=sys.stderr); sys.exit(1)
PY
  then
    echo "GREEN: $f JSON-valid with required fields" >&2
  else
    echo "RED: $f failed schema check" >&2
    rc=1
  fi
done
exit "$rc"
