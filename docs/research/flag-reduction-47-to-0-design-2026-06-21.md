# Flag-Reduction Campaign: 47 → 0 Execution Design (2026-06-21)

> **Provenance:** ultracode deep-dive workflow (29 analysis units covering all 47 flags +
> 29 adversarial verifiers + synthesis). Verdicts: 12 CONFIRM, 14 REVISE, 3 BLOCK — all
> folded in below. Design-only; no code edited. Every `file:line` verified against `main`
> (`b264dcb3`), `registry_table.go` rows 11–57, `FlagCeiling=47` (`registry_ceiling_test.go:27`).
> This is the design-of-record that becomes `campaign-plan.json` for `evolve campaign run`.
>
> **Status note:** the broader goal is loop large-scale-readiness (this campaign is the
> *proving run*). This doc is the flag plan; launch follows the loop-hardening fixes.

---

## 1. Executive summary

47 registry rows → **0**. The `flagreaders` guard scans production Go (standalone
`^EVOLVE(_[A-Z0-9]+)+$` `ast.BasicLit`, skips `_test.go`) **plus** text surfaces under
`textSurfaceRoots = {skills, agents, .github}` and all `*.sh`. It is blind to split-const
(`"EVOLVE_"+"…"`). **There is no allowlist mechanism today** beyond `skipDirs` /
`rootProseExclusions` — so reaching zero **requires adding a `protocolAllowlist`** for the
irreducible IPC/bootstrap/prompt-IPC literals that carry no registry row.

Net: ~24 flags → `policy.json` typed fields; ~7 → DI seams (read moves into `_test.go`);
~8 → cobra flags / split-const IPC (reclassified OUT of the registry); ~5 dead (row-delete).
The terminal `protocolAllowlist` addition lets the last prompt-IPC literal
(`EVOLVE_REFLECTION_JOURNAL`, 18 `agents/*.md` sites — markdown can't split-const) survive
with zero registry rows.

**Anti-gaming (cycle-8 lesson, HARD):** "removing a config flag" = deleting its
`os.Getenv`/`envchain` read so the value flows from a Config struct loaded once. Split-const
is ONLY for genuine cross-process/bootstrap/prompt IPC, with a `// SSOT IPC-protocol-allowed:`
comment and EVERY reader through the const (no bare literal). The audit BLOCKs hidden reads.

## 2. Per-flag disposition (all 47, none duplicated)

| # | Flag (line) | Bucket | Concrete target | Risk | Wave |
|---|---|---|---|---|---|
| 1 | ACS_GO_TIMEOUT_S (11) | config-policyjson | `policy.ACSConfig().GoTimeoutSeconds`; del `acssuite.go:236`; thread `Options.GoTimeout` `cmd_acs.go:107`+`audit.go:362` | LOW | 2 |
| 2 | ADAPTERS_DIR_OVERRIDE (12) | test-seam-DI | DELETE override branch `paths.Resolve` (`paths.go:107`); standalone `cmd_subagent.go` → `--adapters-dir` | MED | 4 |
| 3 | ADVISOR_DEPTH (13) | test-seam-DI | `PhaseAdvisor.depthGuard` func field; read → `phase_advisor_guard_test.go` | LOW | 1 |
| 4 | ANTHROPIC_BASE_URL (14) | config-policyjson | `policy.ClaudeProxyBaseURL()`+`bridge.Config.ProxyMode`; del `driver_claudetmux.go:40`,`setup.go:281` | LOW | 1 |
| 5 | CLI (15) | config-policyjson (global) | `policy.DefaultCLI`+`DefaultCLIConfig()`; rewrite `llmroute.resolvePrimary`; per-agent `EVOLVE_<AGENT>_CLI` stays IPC | MED | 3 |
| 6 | CLI_HEALTH (16) | config-policyjson | `policy.WorkflowPolicy.CLIHealthEnabled *bool`; thread incl. specrunner/registrar minted path | MED | 3 |
| 7 | CLI_MAX_CONCURRENT_CODEX (17) | config-policyjson | `policy.ConcurrencyConfig().CLIMaxConcurrent map[string]int`+`Deps.CLIAdmitMax`; del `driver_tmux_repl.go:192`; rewrite acs cycle3/cycle5 | MED | 2 |
| 8 | COMMIT_EVIDENCE (18) | config-policyjson | `policy.GatesConfig().CommitEvidence`; del `config.go:477-478` (DTO) | LOW | 4 |
| 9 | COMPOSE_PHASES (19) | dead | del row + `cmd_compose.go:92-99` + test | LOW | 1 |
| 10 | CONDITIONAL_MANDATORY (20) | config-policyjson | `policy.OperatorRoutingConfig().ConditionalMandatory`; del `config.go:506-512` (DTO) | MED | 4 |
| 11 | DISABLE_WORKSPACE_GUARD (21) | test-seam-DI | `Orchestrator.disableWorkspaceGuard`+`WithWorkspaceGuardDisabled`; del `cyclerun.go:273` | LOW | 1 |
| 12 | DYNAMIC_ROUTING (22) | config-policyjson | `policy.OperatorRoutingConfig().DynamicRouting`; del `config.go:471-472` (DTO) | MED | 4 |
| 13 | FLEET (23) | ipc-split-const | split-const `fleet.go:20` + route ALL 5 readers (`orchestrator.go:433`,`cyclerun.go`,`driver_tmux_repl.go:107`,`recipe_adapter.go:156`,`gitops.go:461`) | MED | 2 |
| 14 | FLEET_SCOPE (24) | ipc-split-const | split-const `fleet.go:28` + route `cyclerun.go:441` | MED | 2 |
| 15 | GO_BIN (25) | bootstrap-cli-flag | `internal/binpath.Resolve`+`--go-bin`; re-inject split-const into child `cmd.Env` (IPC) | MED | 3 |
| 16 | HANG_CLASSIFIER (26) | config-policyjson | `policy.ClassifierConfig().HangClassifier`; `ClassifyOptions`; del `classify.go:162` | LOW | 1 |
| 17 | INTENT_DELTA (27) | config-policyjson | `policy.WorkflowConfig().IncrementalIntent`; `PhaseRequest.IncrementalIntent`; thread dispatch AND `resume.go:251` | MED | 3 |
| 18 | KB_SEARCH_PATHS (28) | config-policyjson | `policy.PathsConfig().KBSearchPaths`; `research.SearchPaths(cfg)`; del `kb.go:67` | MED | 2 |
| 19 | LEDGER_OVERRIDE (29) | test-seam-DI | DELETE override branch `paths.Resolve` (`paths.go:116`); `CheckProvenance(…, layout)` param | MED | 4 |
| 20 | MANDATORY_PHASES (30) | config-policyjson | `policy.OperatorRoutingConfig().MandatoryPhases`; del `config.go:503` (DTO) | MED | 4 |
| 21 | MARKETPLACE_DIR (31) | bootstrap-cli-flag | `--marketplace-dir` + param `runMarketplacePollLib`; del `bridges.go:74`,`cmd_marketplace_poll.go:28` | LOW | 1 |
| 22 | MAX_OPTIONAL_INSERTIONS (32) | config-policyjson | `policy.OperatorRoutingConfig().MaxOptionalInsertions`; del `config.go:516-520` (DTO) | MED | 4 |
| 23 | MODELCATALOG_AUTOREFRESH (33) | config-policyjson | `policy.ModelCatalogConfig().AutoRefresh *bool`; param `makeCatalogRefresher`; del `cmd_models_live.go:42` | LOW | 1 |
| 24 | MODEL_CATALOG_DIR (34) | test-seam-DI | `bridge.SetCatalogDir`; del `os.Setenv cmd_cycle.go:247`+`os.Getenv catalog_overlay.go:27` | LOW | 2 |
| 25 | PERSONA_OVERRIDE (35) | transient-cli-flag | `--persona-override`; `ship.Options.PersonaOverride`; cycle-path `ship.go:110` sets from `req.Env` | MED | 2 |
| 26 | PHASE_IO (36) | config-policyjson | `policy.RouterConfig().PhaseIO` (4-value) via `parseRouterStage`; del `config.go:486-492` (DTO) | LOW | 4 |
| 27 | PHASE_RECOVERY (37) | config-policyjson | `policy.RecoveryConfig().Stage`; both `NewEngine` sites (`bridge.go:91,161`)+standalone+observer; del `config.go:480` (DTO) | MED | 4 |
| 28 | PHASE_ROOTS (38) | config-policyjson | `policy.PathsConfig().PhaseRoots`; `phasespec.Roots(root,cfg)`; del `mergedcatalog.go:30` | MED | 2 |
| 29 | PLATFORM (39) | bootstrap-cli-flag | `--platform` on `detect-cli`; thread `Options.Env`; del `osgetenv.go` default | LOW | 1 |
| 30 | PLUGIN_ROOT (40) | bootstrap-cli-flag | `--plugin-root` (exists on ship)+split-const fallback; `req.Env` snapshot stays IPC | MED | 3 |
| 31 | POLICY_BYPASS (41) | transient-bypass-cli | `--bypass-policy` BoolVar→`runner.Options.BypassPolicy` (incl. specrunner); del `runner.go:367` | LOW | 1 |
| 32 | PROFILES_DIR_OVERRIDE (42) | test-seam-DI | DELETE override branch `paths.Resolve` (`paths.go:103`); standalone → `--profiles-dir` | MED | 4 |
| 33 | PROFILE_DIR (43) | bootstrap-cli-flag | `--profile-dir` on `evolve phases`; `ship.Options.ProfilesDir` keeps `req.Env` precedence | LOW | 2 |
| 34 | PROJECT_ROOT (44) | bootstrap-cli-flag | `--project-root` (exists)+split-const `ProjectRootEnvKey` SSOT; fix `catalog_overlay.go:30` bare read → ctor param; complete reader inventory | MED | 3 |
| 35 | PROMPTS_DIR (45) | ipc-split-const | split-const SSOT `prompts.go`/`cmd_phase.go` + IPC comment (load-bearing parent→child for e2e subprocess prompts root); NOT deleted | MED | 3 |
| 36 | PROMPT_MAX_TOKENS (46) | dead | del row only (`subagent/run.go:147` is deferred-work prose) | LOW | 1 |
| 37 | REFLECTION_JOURNAL (47) | ipc-allowlist | `policy.WorkflowConfig().ReflectionJournal *bool` gates env-injection; KEEP literal in `agents/*.md`; ADD guard `protocolAllowlist`; no row, no split-const | MED | 5 |
| 38 | ROUTING_MODE (48) | config-policyjson | `policy.OperatorRoutingConfig().RoutingMode`; del `config.go:474-475` (DTO) | MED | 4 |
| 39 | SANDBOX (49) | config-policyjson | `policy.SandboxConfig().Mode` → `Deps.SandboxMode` both NewEngine sites; del `config.go:494`+`sandbox_wrap.go:57` | MED | 4 |
| 40 | SANDBOX_FALLBACK_ON_EPERM (50) | dead | del row + `failureadapter.go:231,235`+`preflight.AutoConfig` field | LOW | 1 |
| 41 | STRICT_AUDIT (51) | config-policyjson | `policy.WorkflowConfig().StrictAudit *bool`; thread via `cr.workflowConfig`; e2e writes policy.json | MED | 3 |
| 42 | SYSTEM_PROMPT (52) | per-phase-profile | profile `system_prompt`/`_file` SSOT; keep req.Env per-dispatch tier; del global `systemprompt.go:29` | LOW | 2 |
| 43 | TESTING (53) | dead | del row + `docs_contract_test.go:40` exemption | LOW | 1 |
| 44 | USE_PHASE_REGISTRY (54) | transient-cli + config | `--use-registry` (`cmd_phase_order.go:28`); make `config.Load` registry read unconditional (del `config.go:319` gate) | MED | 4 |
| 45 | WORKTREE_BASE (55) | bootstrap-cli-flag | `--worktree-base`+split-const fallback; field on provisioner/preflight; del `worktree.go:40`+`provision.go:82`+`preflight.go:288` | MED | 3 |
| 46 | WORKTREE_PATH (56) | dead | del row + `registry_test.go:79`+`agents/evolve-tester.md` | LOW | 1 |
| 47 | WORKTREE_ROOT (57) | ipc-split-const | split-const SSOT shared by `acssuite.go:190` (setter)+`cmd_subagent.go:486` (reader); cross-process dual-root handoff | LOW | 2 |

## 3. Waves (FlagCeiling 47 → 35 → 24 → 15 → 3 → 0)

- **Wave 1 → ceiling 35** — dead (COMPOSE_PHASES, PROMPT_MAX_TOKENS, SANDBOX_FALLBACK_ON_EPERM, TESTING, WORKTREE_PATH) + self-contained DI/config (ADVISOR_DEPTH, DISABLE_WORKSPACE_GUARD, ANTHROPIC_BASE_URL, HANG_CLASSIFIER, MODELCATALOG_AUTOREFRESH, MARKETPLACE_DIR, PLATFORM, POLICY_BYPASS). No cross-flag deps.
- **Wave 2 → ceiling 24** — IPC reclassification (FLEET, FLEET_SCOPE, WORKTREE_ROOT) + leaf config (ACS_GO_TIMEOUT_S, CLI_MAX_CONCURRENT_CODEX, KB_SEARCH_PATHS, PHASE_ROOTS, MODEL_CATALOG_DIR) + PERSONA_OVERRIDE + PROFILE_DIR + SYSTEM_PROMPT. `PathsConfig` shared by KB+PHASE_ROOTS (add first); every FLEET reader via the const before row deletion.
- **Wave 3 → ceiling 15** — cross-package config (CLI, CLI_HEALTH, INTENT_DELTA, STRICT_AUDIT) + bootstrap (GO_BIN, PLUGIN_ROOT, PROJECT_ROOT, WORKTREE_BASE) + PROMPTS_DIR (ipc-split-const). CLI_HEALTH minted-path threading is the capability gate.
- **Wave 4 → ceiling 3** — the `config.Load` env→`RolloutInput` DTO inversion (§4); subsumes DYNAMIC_ROUTING, ROUTING_MODE, MANDATORY_PHASES, CONDITIONAL_MANDATORY, MAX_OPTIONAL_INSERTIONS, COMMIT_EVIDENCE, PHASE_IO, PHASE_RECOVERY, SANDBOX, USE_PHASE_REGISTRY + the override-seams `paths.Resolve` deletions (ADAPTERS/LEDGER/PROFILES_DIR_OVERRIDE).
- **Wave 5 → ceiling 0** — terminal: add `protocolAllowlist` for REFLECTION_JOURNAL (markdown prompt-IPC) + any reclassified literal still on a production text surface; wire `policy.WorkflowConfig().ReflectionJournal`; `FlagCeiling = 0`, `len(All) == 0`.

## 4. The load-bearing change: `config.Load` env → `RolloutInput` DTO inversion (wave 4)

`config.Load` accepts a stdlib-only `RolloutInput` DTO of RAW values built at the composition
root (`cmd_cycle.go wireOrchestratorDeps`) from `pol.OperatorRoutingConfig()`/`GatesConfig()`/
`RouterConfig()`/`RecoveryConfig()`/`SandboxConfig()`. `config` stays a **leaf** (no `policy`
import); the unexported parsers (`parseStage`/`parseEvidenceStage`/`parseMode`/`parseCondRule`/
`parseGateStage`/`parseRouterStage`) are reused **inside** config (package `main` must not call
them). One atomic diff — partial deletion = two sources of truth (forbidden). **Parity contract
test** `config_rollout_parity_test.go`: for the golden corpus (reuse `internal/routingeval` 7
cases + the 8-flag matrix) assert byte-identical `RoutingConfig` vs the old env path, for BOTH
callers (`cmd_cycle.go:276`, `router/policy.go:88`) and the two e2e harnesses (now write
`.evolve/policy.json`, not `ExtraEnv`).

## 5. Loop SSOT goal template (one wave per cycle)

```
GOAL: Flag-reduction campaign wave <N>/5. Convert EXACTLY these flags to their pre-classified
pattern and DELETE the os.Getenv/os.LookupEnv/envchain read so value flows from a Config struct
loaded once (config-policyjson) / DI field (test-seam-DI) / cobra flag (bootstrap/transient) /
documented split-const IPC const. NEVER use "EVOLVE_"+"..." to hide a CONFIG read while the
override stays live — GAMING, audit MUST reject (cycle-8).
FLAGS THIS CYCLE: <wave list §2>   PATTERN PER FLAG: <§2 target file:line>
DEPENDENCIES: <§3>; config.Load DTO inversion is wave 4 only.
DEFINITION OF DONE (strict TDD red→green): test first (param test per
flag-parameter-conversion-standard.md / DI-seam / cobra) → delete prod read → delete registry
row → lower FlagCeiling to new len(All) = <§3 ceiling> → update acs/cycleN predicates + docs same
diff → preserve capability on ALL paths (cycle/resume/minted-spec/standalone).
ANTI-GAMING GATE (audit BLOCKs if violated): rg the prod env read is EMPTY for every config/DI/
bootstrap flag; any surviving split-const is IPC-only with // SSOT comment + every reader via the
const; flagreaders 0 orphans; FlagCeiling == len(All).
OUT OF SCOPE: any unlisted flag; raising the ceiling; deferring a deletion (row+read leave atomically).
```

## 6. Decisions (recommended defaults ADOPTED unless the operator overrides)

1. **REFLECTION_JOURNAL terminal mechanism → (a) `protocolAllowlist`** [adopted]: markdown literal
   stays in `agents/*.md` with no registry row, operator-gated by `policy.json workflow.reflection_journal`.
   (Alt (b) embed the skip in prompt-rendering — larger blast radius.)
2. **Env-deprecation window → one-release `evolve doctor` WARN** [adopted] for the 7 operator-set
   dials (DYNAMIC_ROUTING, ROUTING_MODE, STRICT_AUDIT, SANDBOX, PHASE_IO, PHASE_RECOVERY, CLI):
   "set in env but no longer read; move to .evolve/policy.json.<field>" + CHANGELOG migration table.
3. **config.Load DTO inversion runs as ONE larger cycle (wave 4)** [adopted] — splitting creates a
   two-source-of-truth window (disallowed).
4. **GO_BIN re-injects into child `cmd.Env` via split-const** [adopted] — preserves CI behavior.

The other 40 flags convert with byte-identical defaults; no breaking change.

## 7. Verification per wave

Gate sequence (from `go/`): `go test ./...`; `go test -tags acs ./acs/regression/flagreaders/...`
(0 orphans incl. skills/agents/.github/*.sh); `go test ./internal/flagregistry/...`
(`FlagCeiling == len(All)` == the §3 value); `apicover -enforce` (every new exported policy symbol
named in its own package's test); `go test -tags acs ./acs/regression/apicover/...` (completeness).
Per-wave anti-gaming greps: the production env read is GONE for every config/DI/bootstrap flag;
`"EVOLVE_"+` appears ONLY in IPC consts each with `// SSOT IPC-protocol-allowed:`. Wave 4 also:
`env["EVOLVE_` empty in `config.go`, `paths.Resolve` names no `*_OVERRIDE`, parity test green.
Wave 5: `len(All)==0`, `FlagCeiling==0`, flagreaders green with the allowlist.
Report format: `tests N/N PASS, flagreaders 0 orphans, FlagCeiling==len(All)==<n>, anti-gaming greps empty, no regression`.
