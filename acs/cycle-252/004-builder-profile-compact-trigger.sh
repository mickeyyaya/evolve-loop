#!/usr/bin/env bash
# ACS — cycle-252 task `taco-trajectory-compression-builder`
# Behavioral (parse, not grep): the declarative compaction signal in
# .evolve/profiles/builder.json must be machine-consumable — the predicate
# PARSES the profile with python3 json (subprocess) and asserts the field
# is the integer 15. A corrupt profile or a string "15" fails: this is the
# negative/edge axis (malformed config must be REJECTED), which a grep for
# the key name could never see. Dual-check (cycle-93+): disk + git
# tracking, so a gitignore shadow can't silently drop the profile at ship.
set -uo pipefail

top=$(git rev-parse --show-toplevel)
profile="$top/.evolve/profiles/builder.json"

# Dual-check 1: disk presence.
[ -f "$profile" ] || { echo "RED: $profile missing on disk" >&2; exit 1; }
# Dual-check 2: git tracking (gitignore-shadow guard).
git -C "$top" ls-files --error-unmatch .evolve/profiles/builder.json >/dev/null 2>&1 \
    || { echo "RED: .evolve/profiles/builder.json untracked — may be gitignored" >&2; exit 1; }

# Parse + typed assertion: context_compact_trigger_turns must be int 15.
python3 - "$profile" <<'PY' || exit 1
import json, sys
try:
    with open(sys.argv[1]) as fh:
        doc = json.load(fh)
except (ValueError, OSError) as e:
    print(f"RED: builder.json unparseable: {e}", file=sys.stderr)
    sys.exit(1)
v = doc.get("context_compact_trigger_turns")
if not isinstance(v, int) or isinstance(v, bool) or v != 15:
    print(f"RED: context_compact_trigger_turns = {v!r}, want integer 15", file=sys.stderr)
    sys.exit(1)
PY

echo "GREEN: builder.json parses and context_compact_trigger_turns == 15 (int)"
exit 0
