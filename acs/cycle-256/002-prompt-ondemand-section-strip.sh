#!/usr/bin/env bash
# ACS — cycle-256 task `prompt-ondemand-section-strip`
# Behavioral: `go test` EXIT CODE on both halves (assert.sh; cycle-137 lesson).
#   - prompts.StripOnDemandSections: line-anchored strip, no-heading unchanged,
#     inline-mention safe, empty body.
#   - runner gate: EVOLVE_COMPACT_PROMPTS=1 strips the disk body before
#     ComposePrompt; default/0 byte-identical; inline body preserved (R7).
# Contract source: go/internal/prompts/strip_test.go,
#                  go/internal/phases/runner/compact_prompts_test.go.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/prompts/ 'TestStripOnDemandSections' || exit 1
assert_go_test_pass ./internal/phases/runner/ 'TestRun_CompactPrompts' || exit 1

echo "GREEN: opt-in prompt on-demand section stripping behavioral suite passes"
exit 0
