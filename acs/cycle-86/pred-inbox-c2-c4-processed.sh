#!/usr/bin/env bash
# AC-ID: cycle-86-inbox-c2-c4-processed
# Verify the 3 predicate-quality inbox items are in .evolve/inbox/processed/, not in root inbox.
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
INBOX="$REPO_ROOT/.evolve/inbox"
PROCESSED="$INBOX/processed"

expected_files=(
  "2026-05-19T03-48-14Z-6bfe8b89.json"
  "2026-05-19T03-48-30Z-06644b72.json"
  "2026-05-19T03-48-40Z-9953bfea.json"
)

fail=0
errors=""

for f in "${expected_files[@]}"; do
  if [ -f "$INBOX/$f" ]; then
    errors="${errors}\n  STILL IN ROOT INBOX: $f"
    fail=$((fail + 1))
  elif [ ! -f "$PROCESSED/$f" ]; then
    errors="${errors}\n  NOT FOUND IN PROCESSED: $f"
    fail=$((fail + 1))
  fi
done

if [ $fail -gt 0 ]; then
  echo "RED cycle-86-inbox-c2-c4-processed: $fail/3 inbox items not yet processed"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-86-inbox-c2-c4-processed: all 3 predicate-quality inbox items in processed/"
exit 0
