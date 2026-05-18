#!/usr/bin/env bash
# AC4: README.md has Auth modes section and CLAUDE.md has doctor pointer.
set -uo pipefail
ROOT="${WORKTREE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"

# README Auth modes table with 4 mode names
README="$ROOT/README.md"
grep -q '^## Auth modes' "$README" || { echo "RED AC4.1: README Auth modes heading missing"; exit 1; }
for mode in CUSTOM_PROXY API_KEY SUBSCRIPTION_OAUTH MISCONFIGURED; do
    grep -q "\`$mode\`" "$README" || { echo "RED AC4.2: README missing mode $mode"; exit 1; }
done
grep -q 'doctor-subscription-auth.sh' "$README" || { echo "RED AC4.3: README missing doctor pointer"; exit 1; }

# CLAUDE.md has doctor pointer on Subscription proxy row
CLM="$ROOT/CLAUDE.md"
grep -q 'doctor-subscription-auth.sh' "$CLM" || { echo "RED AC4.4: CLAUDE.md missing doctor pointer"; exit 1; }

echo "GREEN AC4: README Auth modes section + CLAUDE.md doctor pointer present"
exit 0
