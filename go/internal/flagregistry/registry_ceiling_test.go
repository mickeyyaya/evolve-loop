package flagregistry

import "testing"

// FlagCeiling is the monotonic ratchet value for the cluster-consolidation
// campaign. Every cycle that removes flags must lower this constant in the
// same diff — the test below fails if a net addition pushes count above the
// current ceiling.
//
// v20 consolidation history (branch flag-reduction-v20, cycles 39–52): migrated
// legacyFlags (REQUIRE_INTENT, TRIAGE_DISABLE, PLAN_REVIEW, TEST_PHASE_ENABLED,
// BUILD_PLANNER, SWARM_PLANNER, CONSENSUS_AUDIT) + bridge-timing (SCROLLBACK_LINES,
// BOOT_TIMEOUT_S, ARTIFACT_TIMEOUT_S, ARTIFACT_MAX_EXTENDS, PSMAS_SKIP) into policy
// structs; removed STRATEGY/RESET/SHIP_RELEASE_NOTES (dead env writes / IPC split-const),
// GO_BIN_TEST/CODEX_VERSION_PATH/STDOUT_FILTER (DI), PLAN_WORKSPACE/FORCE_FRESH/
// RELEASE_STRICT_PASS/SKIP_PREFLIGHT[_BOOT] (CLI flags), RETRO_MODEL/CACHE_PREFIX_V2/
// CODEX_CONFIG_PATH/MODELCATALOG_CLASSIFIER_CLI/GUARDS_LOG (Config Object/DI),
// LANE (split-const bootstrap), RELEASE_REQUIRE_PREFLIGHT/OLLAMA_BASE/SHIP_AUTO_CONFIRM.
//
// 2026-06-20 v20→main integration: v20's consolidation (47 rows) was verified to
// cover EVERY live reader in the merged tree — the flagreaders guard passes, and
// all 79 main-only flags (advisor-maximization EVOLVE_ROUTER_*, TRIAGE_CAP_GATE,
// EVOLVE_CYCLE_BUDGET, …) have zero remaining production readers because v20
// deleted their os.Getenv reads and rewired the consumers to policy.json structs
// (RouterPolicy / GatesPolicy / WorkflowPolicy), which this merge brings in. The
// ceiling records the post-integration floor; the campaign resumes toward <30.
const FlagCeiling = 47

// TestRegistry_FlagCeiling enforces the one-way ratchet: the registry may
// never exceed FlagCeiling rows. Raising this constant without a matching
// removal is intentionally hard — do not bump it; remove a flag instead.
func TestRegistry_FlagCeiling(t *testing.T) {
	if got := len(All); got > FlagCeiling {
		t.Errorf("len(flagregistry.All) = %d exceeds FlagCeiling=%d — "+
			"remove flags rather than raising the ceiling", got, FlagCeiling)
	}
}
