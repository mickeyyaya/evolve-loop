# Comment history: `internal/flagregistry`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/flagregistry/registry.go:1` — above `package flagregistry`

```text
// Package flagregistry is the declarative SSOT for every EVOLVE_* control
// flag across all reader surfaces (Go production code, Go test seams, and
// the bash skill/agent/commit-gate surface). It exists to end the
// 252-actual-vs-93-documented drift (L2, concurrency-factory plan):
// `evolve flags generate` projects the registry into the marker region of
// docs/architecture/control-flags.md and `evolve flags check` fails on
// drift, so a flag can no longer ship undocumented.
//
// Metadata ONLY — the registry never funnels env reads through config.Load:
// subprocess-reads-env is a deliberate architecture property (the bridge
// subprocess and bash adapters read their own env).
//
// registry_table.go (the data) was seeded mechanically from the 2026-06-11
// inventory (grep over go/ + agents/ + skills/ + commit-gate/ + legacy/ +
// control-flags.md) and is maintained by hand from then on — add a row when
// you add a flag; the drift test (L2.3) catches omissions.
```

### `go/internal/flagregistry/registry_budget_cluster_test.go:5` — above `func TestFlagRegistry_NoBudgetClusterDeadFlags(t *testing.T) {`

```text
// TestFlagRegistry_NoBudgetClusterDeadFlags is the cycle-356 regression guard.
// It asserts that none of the 12 dead Budget Cluster flags (StatusDead,
// DEPRECATED no-op since PR #96) remain in the registry after removal.
//
// This test is authored RED by TDD-engineer (flags still present) and turns
// GREEN when Builder removes all 12 rows from registry_table.go.
//
// The 12 flags were confirmed dead by cycle-356 grep sweep — zero behavioral
// readers; only help text, test setenv, and documentation references remain.
```

### `go/internal/flagregistry/registry_ceiling_test.go:5` — above `const FlagCeiling = 26`

```text
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
// 24 -> 25 (2026-07-11): EVOLVE_WORKTREE_PATH re-registered by cycle 664 to
// re-green the FORBIDDEN-REPEAT guard acs/cycle29 TestC29_007 (cycles 17/18
// fail history pins that row's registry membership). StatusInternal row, not
// an operator dial - LiveFeatureFlagCeiling (the real ratchet) is unchanged.
// 25 -> 26 (2026-09-15, cycle-1684): EVOLVE_OPERATOR_CONFIRM registered — the
// non-interactive spelling of `evolve continuation release -operator`
// (ADR-0089). This is the completeness bump this ceiling explicitly allows, NOT
// a net-new feature flag masked by a raise: the row is StatusInternal, so
// LiveFeatureFlagCeiling (the campaign's real ratchet) and the ACS baseline
// guard are both UNCHANGED at 11. A per-invocation consent token configures no
// behavior and cannot be consolidated into policy.json — persisting "the
// operator agrees" would permanently disarm the gate it guards — so it is
// outside the campaign metric by that metric's own exclusion criterion
// ("driving them to zero is neither possible nor desirable").
```

### `go/internal/flagregistry/registry_ceiling_test.go:79` — above `const LiveFeatureFlagCeiling = 11`

```text
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
// 2026-06-24: EVOLVE_STRICT_AUDIT (StatusActive operator dial) legitimately removed
// → policy.json workflow.strict_audit + DI (cyclerun via WorkflowConfig; audit &
// ship phases via StrictAuditFor); 12 -> 11 (ADR-0064).
```

### `go/internal/flagregistry/registry_deadflags_test.go:5` — above `func TestDeadFlagsSweep_Gone(t *testing.T) {`

```text
// TestDeadFlagsSweep_Gone is the regression test for cycle-2 dead-flag-sweep.
// It asserts that every flag retired from registry_table.go in this cycle is
// truly absent from the registry — Lookup must return ok=false for each name.
//
// RED: all 18 entries are still registered in registry_table.go (Lookup returns
// ok=true), so each sub-test fails until Builder removes the rows.
//
// This test lives in the flagregistry package (not go/acs/cycle2/) so it runs
// under `go test ./internal/flagregistry/...` without the `acs` build tag —
// providing a fast, permanent regression guard in normal CI.
```

### `go/internal/flagregistry/registry_shell_reader_test.go:5` — above `func TestFlagRegistry_MigratedShellReadFlagsAreDeprecated(t *testing.T) {`

```text
// TestFlagRegistry_MigratedShellReadFlagsAreDeprecated is the cycle-360
// regression guard, updated for the cycle-7 retirement.
//
// Cycle 360 tried to remove four flags as "dead", trusting a stale 2026-06-11
// "no reader on any surface" inventory — but at the time they had LIVE readers
// in adapters/claude.sh, so the remediation pinned them as live.
//
// The script→Go migration (TASK A5) then DELETED adapters/claude.sh: the four
// flags lost their ONLY reader and were reclassified to StatusDeprecated.
//
// Cycle 7 completes the journey: all four rows are REMOVED from the registry.
// Operators who still set these env vars get a clean no-op (unrecognized vars
// are silently ignored by the Go runtime); the Go bridge has superseded every
// former use. This guard now asserts ABSENCE — any accidental re-introduction
// of these rows will fail CI immediately.
```

### `go/internal/flagregistry/registry_table.go:3` — above `var All = []Flag{`

```text
// registry_table.go — the flag data. Seeded mechanically 2026-06-11 from the
// repo-wide inventory + control-flags.md tables; hand-maintained since.
// KEEP SORTED BY NAME (Lookup binary-searches; the test enforces order).
```
