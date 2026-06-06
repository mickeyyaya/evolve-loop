#!/usr/bin/env bash
# ACS cycle-218 / task fix-stale-acs-predicates AC3 — TestTierModelsFor must be
# HERMETIC against the live model catalog.
#
# Root cause being fixed: tierModelsFor → bridge.LoadManifest →
# catalog_overlay.go merges any LIVE catalog entry over the manifest's
# ModelTierMap; the overlay's test seam is EVOLVE_MODEL_CATALOG_DIR
# (go/internal/bridge/catalog_overlay.go). The loop environment exports that
# var pointing at the real .evolve (with live agy entries like
# "Gemini 3.5 Flash (Low)"), so the un-isolated test fails in-loop.
#
# Adversarial design (anti-no-op): we fabricate a BOGUS live catalog and
# point the seam at it for the test subprocess. Only a test that isolates
# itself (t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir()) inside
# TestTierModelsFor) can pass — reverting the fix makes this predicate RED
# deterministically, in any environment.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

BOGUS_DIR=$(mktemp -d) || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -rf "$BOGUS_DIR"' EXIT
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

# 1. hostile environment: bogus live catalog injected via the test seam
export EVOLVE_MODEL_CATALOG_DIR="$BOGUS_DIR"
assert_go_test_pass ./internal/setup/... 'TestTierModelsFor' || {
  echo "RED: TestTierModelsFor polluted by injected live catalog — missing per-test EVOLVE_MODEL_CATALOG_DIR isolation" >&2
  exit 1
}

# 2. clean environment: same test must also pass with the seam unset
unset EVOLVE_MODEL_CATALOG_DIR
assert_go_test_pass ./internal/setup/... 'TestTierModelsFor' || exit 1

echo "GREEN: TestTierModelsFor hermetic under both bogus-catalog and clean environments" >&2
exit 0
