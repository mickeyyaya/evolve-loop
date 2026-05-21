#!/usr/bin/env bash
# AC-ID: cycle-102-001-profile-max-turns-ceilings
# AC-source: cycle-102/scout-report.md AC-1..AC-4 (carryover abnormal-turn-overrun-c99)
#
# Behavioral predicate: the four agent profiles whose ceilings were
# breached in cycles 99-100 must have their `max_turns` raised to at
# least the scout-recommended floor:
#
#   triage.json  : >= 18   (was 15; cycle-99 hit 17, cycle-100 hit ...)
#   intent.json  : >= 12   (was 10; cycle-100 hit 12)
#   scout.json   : >= 42   (was 30; cycle-100 hit 41)
#   builder.json : >= 36   (was 25; cycle-100 hit 35)
#
# Predicate is BEHAVIORAL: it invokes `python3` as a subprocess to
# parse each profile JSON and emit the value. Pure-grep predicates are
# forbidden by acs/AGENTS.md.
#
# Dual-check pattern: each profile must (a) exist on disk and (b) be
# tracked by git. A worktree-only edit must not satisfy this predicate
# — it would be silently dropped at ship (cycle-92 failure mode).
#
# Bash 3.2 compatible. Iterates a tuple stream via printf | while read.
#
# Exit codes:
#   0 = GREEN (all 4 ceilings meet or exceed floor)
#   1 = RED   (at least one ceiling below floor, missing, or untracked)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd to repo root failed" >&2; exit 1; }

if ! command -v python3 >/dev/null 2>&1; then
  echo "RED: python3 required for JSON parse (predicate is behavioral)" >&2
  exit 1
fi

# Stream of "path:min_value" tuples.
TUPLES="\
.evolve/profiles/triage.json:18
.evolve/profiles/intent.json:12
.evolve/profiles/scout.json:42
.evolve/profiles/builder.json:36"

TMP_FAIL="$(mktemp -t cycle102-001.XXXXXX)" || { echo "RED: mktemp failed" >&2; exit 1; }
: > "$TMP_FAIL"

printf '%s\n' "$TUPLES" | while IFS=: read -r path min_value; do
  [ -z "$path" ] && continue

  if [ ! -f "$path" ]; then
    printf 'MISSING:%s\n' "$path" >> "$TMP_FAIL"
    continue
  fi

  if ! git ls-files --error-unmatch "$path" >/dev/null 2>&1; then
    printf 'UNTRACKED:%s\n' "$path" >> "$TMP_FAIL"
    continue
  fi

  # Behavioral: invoke python3 to extract max_turns. A profile that
  # parses to JSON but has no max_turns key fails here.
  actual=$(python3 -c "
import json, sys
try:
    d = json.load(open('$path'))
    v = d.get('max_turns')
    if v is None:
        sys.exit(2)
    print(v)
except json.JSONDecodeError:
    sys.exit(3)
" 2>/dev/null)
  rc=$?
  if [ $rc -eq 2 ]; then
    printf 'NO_KEY:%s(max_turns missing)\n' "$path" >> "$TMP_FAIL"
    continue
  elif [ $rc -eq 3 ]; then
    printf 'BAD_JSON:%s\n' "$path" >> "$TMP_FAIL"
    continue
  elif [ $rc -ne 0 ]; then
    printf 'PARSE_FAIL:%s(rc=%s)\n' "$path" "$rc" >> "$TMP_FAIL"
    continue
  fi

  # Numeric comparison (bash arithmetic; rejects non-integers).
  if ! [ "$actual" -ge "$min_value" ] 2>/dev/null; then
    printf 'TOO_LOW:%s(%s<%s)\n' "$path" "$actual" "$min_value" >> "$TMP_FAIL"
    continue
  fi
done

if [ -s "$TMP_FAIL" ]; then
  fail_count=0
  fail_summary=""
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    fail_count=$(( fail_count + 1 ))
    fail_summary="$fail_summary  - $line"$'\n'
  done < "$TMP_FAIL"
  rm -f "$TMP_FAIL"

  printf 'RED: %s profile(s) failed max_turns ceiling check:\n' "$fail_count" >&2
  printf '%s' "$fail_summary" >&2
  exit 1
fi
rm -f "$TMP_FAIL"

echo "GREEN: all 4 profile max_turns ceilings meet/exceed scout-recommended floor (triage>=18, intent>=12, scout>=42, builder>=36)"
exit 0
