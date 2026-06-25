package flagregistry

// registry_table.go — the flag data. Seeded mechanically 2026-06-11 from the
// repo-wide inventory + control-flags.md tables; hand-maintained since.
// KEEP SORTED BY NAME (Lookup binary-searches; the test enforces order).

// All is the complete EVOLVE_* flag registry, sorted by Name. It is the SSOT
// projected into control-flags.md; Lookup binary-searches it, so it must stay
// sorted (TestAll_SortedByName enforces this).
var All = []Flag{
	{Name: "EVOLVE_ADAPTERS_DIR_OVERRIDE", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_CLI", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_CLI_HEALTH", Status: StatusActive, Cluster: "Readiness Gate (pre-batch)", Doc: "The one dial for the CLI-health bench layer (cycle-283: a quota-walled codex re-burned its boot on every dispatch all night because nothing remembered the wall). `0` disables ALL of it: the runner's bench-writer (exit-85 + classified `rate_limit` escalation → bench the CLI FAMILY in `.evolve/cli-health.json`, `benched_until` from the wall's own reset hint else a strike-doubled cooldown), the dispatch-chain demotion (benched families start at their fallback; bench is advice — all-benched dispatches least-recently-benched with a loud WARN; policy pins bypass entirely), the loop's per-cycle canary (one `bridge.LiveSmokeTest` per EXPIRED bench: recovered → cleared, walled again → strikes+1), and the advisor's environmental \"CLI health\" prompt section. Preflight's `cli-health` check (WARN-only) and `evolve doctor live <driver>` (the probe that can SEE a quota wall — boot smoke cannot, walls appear only after work is submitted) remain readable surfaces."},
	{Name: "EVOLVE_COMMIT_EVIDENCE", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_CONDITIONAL_MANDATORY", Status: StatusActive, Cluster: "Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)", Doc: "`phase:expr` conditional-mandatory predicate; op ∈ `!= == >= <= > <`"},
	{Name: "EVOLVE_DYNAMIC_ROUTING", Status: StatusActive, Cluster: "Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)", Doc: "Rollout stage: `off`/`0` (static state machine drives — operator escape hatch) / `shadow` (router computes + logs, static drives) / `advisory` (router drives optional surface, spine static; DEFAULT) / `enforce` (router drives, kernel-clamped). Unknown value → `off` + WARN"},
	{Name: "EVOLVE_GO_BIN", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_INTENT_DELTA", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_LANE", Status: StatusInternal, Cluster: "Worktree / Workspace", Doc: "Operator-set human-readable worktree lane (e.g. EVOLVE_LANE=campaign); --lane CLI flag is primary, env retained for script compatibility. Readability only — correctness never depends on it (runscope.go). Surfaced fold-aware by the envtaint read-set (ADR-0064)."},
	{Name: "EVOLVE_LEDGER_OVERRIDE", Status: StatusActive, Cluster: "Override / Test Seams", Doc: "Override ledger.jsonl path"},
	{Name: "EVOLVE_MANDATORY_PHASES", Status: StatusActive, Cluster: "Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)", Doc: "CSV ordered mandatory spine. Omitting `audit` or `ship` emits a `weak-spine` WARN"},
	{Name: "EVOLVE_MAX_OPTIONAL_INSERTIONS", Status: StatusActive, Cluster: "Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)", Doc: "Cap on optional phases the router may insert"},
	{Name: "EVOLVE_PERSONA_OVERRIDE", Status: StatusInternal, Doc: "cycle-16: migrated to --persona-override CLI flag in phasecmd/phases.go (os.Getenv removed from phasecmd surface). Cycle-path use in ship/commitgate.go (via req.Env) remains active."},
	{Name: "EVOLVE_PHASE_IO", Status: StatusActive, Cluster: "Phase I/O (ADR-0050)", Doc: "ADR-0050 Phase-3 unified-phase-I/O rollout dial. FULL off→shadow→advisory→enforce ladder (4-value, unlike the 3-value gate dials). off = dormant legacy dispatch, byte-identical (the rollback escape hatch); shadow = typed envelope assembled + compared against legacy disk reads (mismatch → ledger phaseio_shadow_mismatch); advisory = envelope populated + read alongside legacy (legacy still wins); enforce = the typed envelope is AUTHORITATIVE — phase readers consume it and the audit/ship verdict parse is sentinel-mandatory. DEFAULT enforce as of the 3.10 cutover (was off through 18.14.0); set EVOLVE_PHASE_IO=off to roll back. A typo falls back to off (fail-safe, never leaves the dial in an unintended state)."},
	{Name: "EVOLVE_PLUGIN_ROOT", Status: StatusActive, Cluster: "Core Infrastructure (never consolidate)", Doc: "Read-only plugin install location"},
	{Name: "EVOLVE_PROFILES_DIR_OVERRIDE", Status: StatusActive, Cluster: "Override / Test Seams", Doc: "Override profiles dir path"},
	{Name: "EVOLVE_PROFILE_DIR", Status: StatusInternal, Doc: "cycle-16: migrated to --profile-dir CLI flag in phasecmd/phases.go (os.Getenv removed from phasecmd surface). Cycle-path use in ship/commitgate.go (via req.Env) remains active."},
	{Name: "EVOLVE_PROJECT_ROOT", Status: StatusActive, Cluster: "Core Infrastructure (never consolidate)", Doc: "Writable project directory (dual-root pattern)"},
	{Name: "EVOLVE_PROMPTS_DIR", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_REFLECTION_JOURNAL", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_ROUTING_MODE", Status: StatusActive, Cluster: "Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)", Doc: "Routing brain: `llm`/`dynamic`/`dynamic-llm` (LLM proposes, kernel clamps) / `static`/`static-preset`/`preset` (triggers + spine only, no LLM). Unknown → `llm` + WARN"},
	{Name: "EVOLVE_SANDBOX", Status: StatusActive, Cluster: "Sandbox Cluster", Doc: "Enable outer sandbox-exec/bwrap wrapper"},
	{Name: "EVOLVE_TMUX_SOCKET", Status: StatusInternal, Cluster: "Bridge Cluster", Doc: "IPC channel (F6, not an operator dial): the loop exports the per-run bridge tmux socket name (evolve-bridge-p<looppid>) so its bridge subprocesses, the reaper, and the orphan GC all target the same isolated socket — so an external `tmux kill-server` on one run's socket can't tear down another. Unset ⇒ the shared `evolve-bridge` default. Set by the loop (cmd_loop), never by the operator."},
	{Name: "EVOLVE_USE_PHASE_REGISTRY", Status: StatusActive, Cluster: "Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)", Doc: "Set `0` to skip reading `phase-registry.json` (built-in defaults only)"},
}
