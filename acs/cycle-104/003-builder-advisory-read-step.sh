#!/usr/bin/env bash
# AC-ID: cycle-104-003-builder-advisory-read-step
# AC-source: scout-report.md AC EG-104-03 (lines 220-223); intent.md acceptance_checks[2]
# Behavioral predicate (ADVISORY-READ instruction):
#   agents/evolve-builder.md must contain a NEW step (positioned BEFORE
#   Step 3) instructing Builder to read workspace/build-plan.md when the
#   build-planner phase produced it. The step must mention both:
#     - the literal token "build-plan.md"
#     - the word "advisory" (or "Advisory") to signal it's non-blocking
#   AND it must appear BEFORE the Step 3 heading (positional invariant).
#
#   Per scout-report.md line 93, the new step is labeled "Step 2.8" (the
#   intent.md "Step 2.5" name collides with an existing Step 2.5). This
#   predicate does NOT grep for a specific step number — it greps for the
#   semantic tokens, so naming flexibility is preserved.
#
# Mutation spec (cycle-104-003-MUT):
#   Mutant: build-plan.md mentioned only AFTER Step 3 heading   -> must FAIL.
#   Mutant: build-plan.md absent entirely                       -> must FAIL.
#   Mutant: build-plan.md present without "advisory" semantics  -> must FAIL.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

BUILDER="agents/evolve-builder.md"

if [ ! -f "$BUILDER" ]; then
  echo "RED: $BUILDER does not exist" >&2
  exit 1
fi

# Tokens that MUST appear.
if ! grep -qF 'build-plan.md' "$BUILDER"; then
  echo "RED: $BUILDER missing 'build-plan.md' (advisory-read step not added)" >&2
  exit 1
fi

if ! grep -qiE 'advisory' "$BUILDER"; then
  echo "RED: $BUILDER missing 'advisory' qualifier near build-plan.md reference" >&2
  exit 1
fi

# Positional invariant: at least one occurrence of "build-plan.md" must appear
# on a line BEFORE the Step 3 heading. Find line numbers and compare.
STEP3_LINE=$(grep -nF '### Step 3: Design (chain-of-thought required)' "$BUILDER" | head -1 | cut -d: -f1)
if [ -z "$STEP3_LINE" ]; then
  echo "RED: $BUILDER missing Step 3 heading (predicate 002 should also be RED)" >&2
  exit 1
fi

FIRST_BPM_LINE=$(grep -nF 'build-plan.md' "$BUILDER" | head -1 | cut -d: -f1)
if [ -z "$FIRST_BPM_LINE" ]; then
  echo "RED: $BUILDER no build-plan.md occurrence found (defense against grep race)" >&2
  exit 1
fi

if [ "$FIRST_BPM_LINE" -ge "$STEP3_LINE" ]; then
  echo "RED: $BUILDER first 'build-plan.md' (line $FIRST_BPM_LINE) appears AT/AFTER Step 3 (line $STEP3_LINE); advisory read must be inserted BEFORE Step 3" >&2
  exit 1
fi

echo "GREEN: $BUILDER contains advisory build-plan.md read step before Step 3 (line $FIRST_BPM_LINE < $STEP3_LINE)"
exit 0
