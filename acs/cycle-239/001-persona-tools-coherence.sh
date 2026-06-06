#!/usr/bin/env bash
# ACS — cycle 239 / task persona-tools-coherence-gate (retry of cycle 238)
#
# Classification: BEHAVIORAL — runs the phasecoherence unit suite (exit-code
# authoritative), then invokes the actual CLI (`evolve phases check-coherence`)
# against a planted-drift fixture (persona declares "Edit", profile only allows
# "Read") asserting the WARN surfaces and --strict exits 2 EXACTLY (negative
# case), and against the real tree asserting advisory exit 0. Adding a magic
# string to any source file cannot satisfy this — the drift detection logic
# must run.
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

# 1. Behavioral: unit suite (positive, negative-disallowed, negative-undeclared,
#    Bash/Skill normalization, skip rules).
assert_go_test_pass ./internal/phasecoherence/... 'TestCoherence' || exit 1

# 2. Negative behavioral: planted drift must surface as WARN naming the tool.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/agents" "$tmp/.evolve/profiles"
cat > "$tmp/agents/evolve-widget.md" <<'MD'
---
name: evolve-widget
description: acs fixture
tools: ["Read", "Edit"]
---

# widget
MD
printf '{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}\n' \
  > "$tmp/.evolve/profiles/widget.json"

out=""
rc=0
out=$(EVOLVE_PROJECT_ROOT="$tmp" EVOLVE_PLUGIN_ROOT="$tmp" EVOLVE_PROFILES_DIR_OVERRIDE= EVOLVE_PERSONA_OVERRIDE= EVOLVE_PROMPTS_DIR= \
  "$BIN" phases check-coherence 2>&1) || rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: check-coherence on drift fixture exited $rc, want 0 (advisory default)" >&2
  echo "$out" >&2
  exit 1
fi
if ! echo "$out" | grep -q "WARN"; then
  echo "RED: planted Edit drift produced no WARN; output: $out" >&2
  exit 1
fi
if ! echo "$out" | grep -q "Edit"; then
  echo "RED: WARN does not name the drifting tool Edit; output: $out" >&2
  exit 1
fi

# 3. Negative behavioral (strict branch): --strict must block with exit 2
#    EXACTLY on the same drift — exit 1 means the exit code was rewritten
#    (cycle-238 defect class).
rc=0
EVOLVE_PROJECT_ROOT="$tmp" EVOLVE_PLUGIN_ROOT="$tmp" EVOLVE_PROFILES_DIR_OVERRIDE= EVOLVE_PERSONA_OVERRIDE= EVOLVE_PROMPTS_DIR= \
  "$BIN" phases check-coherence --strict >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 2 ]; then
  echo "RED: check-coherence --strict on drift fixture exited $rc, want exactly 2" >&2
  exit 1
fi

# 4. Behavioral: real tree — advisory mode must exit 0 (WARNs allowed; scout F6
#    documents live builder drift the gate is expected to surface, not block).
rc=0
EVOLVE_PROJECT_ROOT="$top" EVOLVE_PLUGIN_ROOT="$top" EVOLVE_PROFILES_DIR_OVERRIDE= EVOLVE_PERSONA_OVERRIDE= EVOLVE_PROMPTS_DIR= \
  "$BIN" phases check-coherence >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: check-coherence on real tree exited $rc, want 0" >&2
  exit 1
fi

echo "GREEN: tools-coherence gate behaves (unit suite, WARN surfacing, strict exit-2 block, real tree)" >&2
exit 0
