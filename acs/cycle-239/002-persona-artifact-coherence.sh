#!/usr/bin/env bash
# ACS — cycle 239 / task persona-output-artifact-coherence (retry of cycle 238)
#
# Classification: BEHAVIORAL — runs the artifact-coherence unit suite
# (exit-code authoritative), then invokes the actual CLI
# (`evolve phases check-artifact-coherence`) against a planted I-3(d)-replica
# fixture (persona says plan-review.md, profile says plan-review-report.md)
# asserting the WARN carries BOTH names and --strict exits 2 EXACTLY (negative
# case), and against the real tree asserting exit 0 (scout H3: current tree is
# clean).
#
# cycle-238 HIGH fix: compiled go/evolve binary, never `go run` (exit-code
# rewrite). See acs/cycle-239/000-profile-provenance.sh header.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
top=$(git rev-parse --show-toplevel)
BIN="$top/go/evolve"

# 0. Compile fresh — never test a stale binary.
if ! (cd "$top/go" && go build -o evolve ./cmd/evolve); then
  echo "RED: go build -o evolve ./cmd/evolve failed — cannot test CLI behavior" >&2
  exit 1
fi

# 1. Behavioral: unit suite (match, mismatch, first-token rule, skip rules).
assert_go_test_pass ./internal/phasecoherence/... 'TestArtifactCoherence' || exit 1

# 2. Negative behavioral: planted mismatch must WARN with both artifact names.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/agents" "$tmp/.evolve/profiles"
cat > "$tmp/agents/evolve-plan-review.md" <<'MD'
---
name: evolve-plan-review
description: acs fixture
output-format: "plan-review.md — ## Findings, ## Verdict"
---

# plan-review
MD
printf '{"name":"plan-review","role":"plan-review","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/plan-review-report.md"}\n' \
  > "$tmp/.evolve/profiles/plan-review.json"

out=""
rc=0
out=$(EVOLVE_PROJECT_ROOT="$tmp" EVOLVE_PLUGIN_ROOT="$tmp" EVOLVE_PROFILES_DIR_OVERRIDE= EVOLVE_PROMPTS_DIR= \
  "$BIN" phases check-artifact-coherence 2>&1) || rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: check-artifact-coherence on mismatch fixture exited $rc, want 0 (advisory default)" >&2
  echo "$out" >&2
  exit 1
fi
if ! echo "$out" | grep -q "WARN"; then
  echo "RED: planted artifact mismatch produced no WARN; output: $out" >&2
  exit 1
fi
if ! echo "$out" | grep -q "plan-review.md"; then
  echo "RED: WARN missing persona artifact plan-review.md; output: $out" >&2
  exit 1
fi
if ! echo "$out" | grep -q "plan-review-report.md"; then
  echo "RED: WARN missing profile artifact plan-review-report.md; output: $out" >&2
  exit 1
fi

# 3. Negative behavioral (strict branch): --strict must block with exit 2
#    EXACTLY on the same mismatch (cycle-238 exit-code-propagation fix).
rc=0
EVOLVE_PROJECT_ROOT="$tmp" EVOLVE_PLUGIN_ROOT="$tmp" EVOLVE_PROFILES_DIR_OVERRIDE= EVOLVE_PROMPTS_DIR= \
  "$BIN" phases check-artifact-coherence --strict >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 2 ]; then
  echo "RED: check-artifact-coherence --strict on mismatch fixture exited $rc, want exactly 2" >&2
  exit 1
fi

# 4. Behavioral: real tree — must exit 0 (scout H3: no current mismatch).
rc=0
EVOLVE_PROJECT_ROOT="$top" EVOLVE_PLUGIN_ROOT="$top" EVOLVE_PROFILES_DIR_OVERRIDE= EVOLVE_PROMPTS_DIR= \
  "$BIN" phases check-artifact-coherence >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: check-artifact-coherence on real tree exited $rc, want 0" >&2
  exit 1
fi

echo "GREEN: artifact-coherence gate behaves (unit suite, dual-name WARN, strict exit-2 block, real tree)" >&2
exit 0
