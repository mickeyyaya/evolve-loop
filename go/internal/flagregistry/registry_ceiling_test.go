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
// cycle-5: +1 for EVOLVE_REAP_ORPHANS (pre-existing flag, previously unregistered; required
// by ACS cycle-5 predicate); -3 active readers (HANG_CLASSIFIER/MODELCATALOG_AUTOREFRESH/
// ANTHROPIC_BASE_URL migrated to policy.json) → net active reduction.
// flag-campaign-8 wave-1 (salvaged): deleted 13 rows — 5 dead (PROMPT_MAX_TOKENS,
// SANDBOX_FALLBACK_ON_EPERM, TESTING, WORKTREE_PATH, REAP_ORPHANS), COMPOSE_PHASES
// (converted to policy then row-deleted), and 7 of the 8 campaign-7 tombstones
// (readers fully removed) → 48 -> 35.
// cycle-15 (bypass-policy-flag): POLICY_BYPASS converted to --bypass-policy CLI flag,
// row deleted → 35 -> 34. FlagCeiling stays at 35 (upper bound, not exact).
// flag-campaign-10 wave-1 INTEGRATION: 5 rows (PHASE_RECOVERY, FLEET, FLEET_SCOPE,
// WORKTREE_ROOT, POLICY_BYPASS) → 35 -> 30.
// flag-campaign-10 wave-2 INTEGRATION: 6 rows (SYSTEM_PROMPT, ACS_GO_TIMEOUT_S,
// CLI_MAX_CONCURRENT_CODEX, KB_SEARCH_PATHS, PHASE_ROOTS, MODEL_CATALOG_DIR) → 30 -> 24.
// 2026-06-23 ADR-0064 Pillar 2 (S4a, envtaint fold-aware read-set): +1 completeness
// for EVOLVE_LANE — a pre-existing operator-set worktree dial read via split-const
// (runscope.go), invisible to the go/ast flagreaders scan and so previously
// unregistered. The new fold-aware gate (R_go ⊆ registry) surfaces it; a row is
// required for completeness. StatusInternal, so LiveFeatureFlagCeiling is unchanged.
// 2026-06-24: -1 — EVOLVE_WORKTREE_BASE legitimately removed (policy.json
// worktree.base + WithWorktreeBase DI to all 3 readers; ADR-0064). StatusActive,
// so LiveFeatureFlagCeiling also drops by 1.
const FlagCeiling = 24

// TestRegistry_FlagCeiling enforces a one-way bound on TOTAL registry rows.
//
// NOTE — this is no longer the campaign's progress metric, only a loose
// completeness backstop. len(All) is ALLOWED to rise when the flagreaders guard
// discovers a pre-existing unregistered live reader (a row must be added for
// completeness); the real campaign target is LiveFeatureFlagCeiling below, which
// counts only live operator dials. So this ceiling may be bumped for a genuine
// completeness addition, but never to mask a net-new feature flag.
func TestRegistry_FlagCeiling(t *testing.T) {
	if got := len(All); got > FlagCeiling {
		t.Errorf("len(flagregistry.All) = %d exceeds FlagCeiling=%d — "+
			"remove flags rather than raising the ceiling", got, FlagCeiling)
	}
}

// LiveFeatureFlagCeiling is the campaign's real monotonic-decrease ratchet:
// the count of live operator-facing feature flags (LiveFeatureFlags() =
// StatusActive minus core-infrastructure) may never exceed it. Every cycle that
// deprecates a flag (rewiring its env read to policy.json/DI) lowers the live
// count and must lower this constant in the same diff. The campaign target is 0
// — at which point only core-infra Active rows + internal/test-seam plumbing
// remain, i.e. zero operator feature dials (the no_feature_flags goal).
//
// Anti-regression teeth: the in-tree count <= ceiling check below is the fast,
// git-independent floor; go/acs/regression/flagceiling additionally fails the
// per-cycle gate if the live count rose versus the campaign baseline (main),
// which is what a same-metric unit test alone cannot enforce — a cycle could
// otherwise raise this const the way cycle-5 raised FlagCeiling 47->48.
//
// flag-campaign-7 (8 deprecations: ADVISOR_DEPTH/ANTHROPIC_BASE_URL/
// DISABLE_WORKSPACE_GUARD/HANG_CLASSIFIER/MARKETPLACE_DIR/MODELCATALOG_AUTOREFRESH/
// PLATFORM/POLICY_BYPASS → policy.json/DI/CLI) lowered the live count 23 -> 21.
// flag-campaign-8 wave-1 (salvaged): removed the 3 live dead dials
// (PROMPT_MAX_TOKENS, REAP_ORPHANS, SANDBOX_FALLBACK_ON_EPERM); 21 -> 18.
// cycle-15 (bypass-policy-flag): POLICY_BYPASS was already StatusDeprecated
// (not a live feature flag), so LiveFeatureFlagCeiling unchanged at 18.
// flag-campaign-10 wave-1 INTEGRATION: 4 live Active dials (PHASE_RECOVERY, FLEET,
// FLEET_SCOPE, WORKTREE_ROOT) → 18 -> 14.
// flag-campaign-10 wave-2 INTEGRATION: 1 live Active dial (CLI_MAX_CONCURRENT_CODEX,
// a dead Active row); the other 5 wave-2 deletions were StatusInternal → 14 -> 13.
// 2026-06-24: EVOLVE_WORKTREE_BASE (StatusActive operator dial) legitimately
// removed → policy.json worktree.base + WithWorktreeBase DI; 13 -> 12 (ADR-0064).
const LiveFeatureFlagCeiling = 12

// TestRegistry_LiveFeatureFlagCeiling enforces the live-feature-flag ratchet.
// Lowering LiveFeatureFlagCeiling is progress; raising it is a regression and is
// additionally blocked against the baseline by the ACS guard.
func TestRegistry_LiveFeatureFlagCeiling(t *testing.T) {
	if got := len(LiveFeatureFlags()); got > LiveFeatureFlagCeiling {
		t.Errorf("len(LiveFeatureFlags) = %d exceeds LiveFeatureFlagCeiling=%d — "+
			"deprecate a flag to its replacement rather than raising the ceiling", got, LiveFeatureFlagCeiling)
	}
}
