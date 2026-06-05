#!/usr/bin/env bash
# tests/test-fix-stale-acs-predicates.sh — cycle 218, task fix-stale-acs-predicates
#
# RED contract for Builder: 3 stale tests broke after the CLAUDE.md split
# (d8ac721) and the live model catalog landed. Builder fixes the TESTS
# (they are the production artifact for this task):
#   1. go/acs/cycle89/predicates_test.go  — env-var check CLAUDE.md → docs/operations/runtime-reference.md
#   2. go/acs/cycle100/predicates_test.go — same relocation for EVOLVE_OBSERVER_ENFORCE
#   3. go/internal/setup/setup_test.go    — TestTierModelsFor must isolate from the
#      live catalog via t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir())
#
# Behavioral: every load-bearing assertion runs `go test` as a subprocess and
# keys off its EXIT CODE (cycle-137 lesson). Greps are auxiliary anti-deletion
# guards only (the fix must MOVE the check, not delete it).
set -uo pipefail

TOP=$(git rev-parse --show-toplevel) || { echo "FAIL: not a git repo"; exit 1; }
GO_DIR="$TOP/go"
PASS=0; FAIL=0

ok()  { echo "PASS: $1"; PASS=$((PASS+1)); }
bad() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

# --- 1. behavioral: cycle89 env-var predicate passes -------------------------
if (cd "$GO_DIR" && go test -count=1 -run TestC89_ClaudeMdResearchEnvVars ./acs/cycle89/ >/dev/null 2>&1); then
  ok "cycle89 TestC89_ClaudeMdResearchEnvVars passes"
else
  bad "cycle89 TestC89_ClaudeMdResearchEnvVars fails (still checks CLAUDE.md?)"
fi

# --- 2. behavioral: cycle100 observer-enforce predicate passes ---------------
if (cd "$GO_DIR" && go test -count=1 -run TestC100_001_ObserverEnforceDefaultOn ./acs/cycle100/ >/dev/null 2>&1); then
  ok "cycle100 TestC100_001_ObserverEnforceDefaultOn passes"
else
  bad "cycle100 TestC100_001_ObserverEnforceDefaultOn fails (still checks CLAUDE.md?)"
fi

# --- 3. behavioral: TestTierModelsFor passes in the current (loop) env -------
if (cd "$GO_DIR" && go test -count=1 -run TestTierModelsFor ./internal/setup/ >/dev/null 2>&1); then
  ok "setup TestTierModelsFor passes in current environment"
else
  bad "setup TestTierModelsFor fails in current environment"
fi

# --- 4. NEGATIVE / adversarial: TestTierModelsFor must survive a HOSTILE live
#        catalog. We fabricate a bogus live agy entry and point the overlay's
#        test seam at it; only a hermetic test (t.Setenv inside the test) can
#        pass. This is the anti-no-op check: deleting the isolation fix makes
#        this assertion fail whenever any live catalog is present.
BOGUS_DIR=$(mktemp -d)
cat > "$BOGUS_DIR/model-catalog.json" <<'JSON'
{
  "fetched_at": "2026-06-05T00:00:00Z",
  "clis": {
    "agy": {
      "tier_models": {
        "fast": "BOGUS-POLLUTION-fast",
        "balanced": "BOGUS-POLLUTION-balanced",
        "deep": "BOGUS-POLLUTION-deep"
      },
      "source": "live"
    }
  }
}
JSON
if (cd "$GO_DIR" && EVOLVE_MODEL_CATALOG_DIR="$BOGUS_DIR" go test -count=1 -run TestTierModelsFor ./internal/setup/ >/dev/null 2>&1); then
  ok "TestTierModelsFor is hermetic under a bogus live catalog"
else
  bad "TestTierModelsFor polluted by bogus live catalog — missing t.Setenv isolation"
fi
rm -rf "$BOGUS_DIR"

# --- 5-7. auxiliary anti-deletion guards (NOT load-bearing): the fix must
#          relocate the env-var checks, not delete them.
if grep -qF 'runtime-reference.md' "$GO_DIR/acs/cycle89/predicates_test.go" \
   && grep -qF 'EVOLVE_RESEARCH_CACHE_ENABLED' "$GO_DIR/acs/cycle89/predicates_test.go"; then
  ok "cycle89 test still asserts EVOLVE_RESEARCH_CACHE_ENABLED (now against runtime-reference.md)"
else
  bad "cycle89 test lost its env-var assertion or does not target runtime-reference.md"
fi

if grep -qF 'runtime-reference.md' "$GO_DIR/acs/cycle100/predicates_test.go" \
   && grep -qF 'EVOLVE_OBSERVER_ENFORCE' "$GO_DIR/acs/cycle100/predicates_test.go"; then
  ok "cycle100 test still asserts EVOLVE_OBSERVER_ENFORCE (now against runtime-reference.md)"
else
  bad "cycle100 test lost its env-var assertion or does not target runtime-reference.md"
fi

if grep -qF 'EVOLVE_MODEL_CATALOG_DIR' "$GO_DIR/internal/setup/setup_test.go"; then
  ok "setup_test.go contains EVOLVE_MODEL_CATALOG_DIR isolation"
else
  bad "setup_test.go missing EVOLVE_MODEL_CATALOG_DIR isolation seam"
fi

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
