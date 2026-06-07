#!/usr/bin/env bash
# ACS — cycle-249 task `phase-classify-declarative`
# Behavioral: specrunner.EvaluateClassify is exported and honors the
# declarative ClassifyRules contract, including the loud-failure row for
# malformed verdict_on_pass (intent AC3: explicit errors, no silent
# fallback). The go-test invocation is the load-bearing check; the
# delegation-count grep is the structural half of scout gate
# "≥2 built-in phases delegating" (≥2 non-specrunner production files
# reference EvaluateClassify).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
top=$(git rev-parse --show-toplevel)

assert_go_test_pass ./internal/phases/specrunner/... 'TestEvaluateClassifyExported' || exit 1

delegating=$(grep -rl 'EvaluateClassify' "$top/go/internal/phases/" --include='*.go' \
  | grep -v '/specrunner/' | grep -v '_test.go' | wc -l | tr -d ' ')
if [ "$delegating" -lt 2 ]; then
  echo "RED: only $delegating built-in phase file(s) delegate to EvaluateClassify — scout gate requires >= 2" >&2
  exit 1
fi

echo "GREEN: EvaluateClassify exported, contract holds, $delegating built-in phases delegate"
exit 0
