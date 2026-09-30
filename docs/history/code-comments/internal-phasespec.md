# Comment history: `internal/phasespec`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phasespec/activating_fields_test.go:3` — above `import (`

```text
// activating_fields_test.go — PA-BIG S4 (ADR-0058): the load-time validator for
// the transition-activating fields. ValidateActivatingFields enforces
// well-formedness (a known branching_strategy; on_pass/on_fail declared as a
// pair), and Load rejects a malformed registry — the registry is a contract, so
// a half-declared verdict branch or an unknown strategy fails loudly at load
// rather than silently degrading to the literal kernel.
```

### `go/internal/phasespec/activating_fields_test.go:65` — above `func TestDiscoverUserSpecs_StripsActivatingFields(t *testing.T) {`

```text
// TestDiscoverUserSpecs_StripsActivatingFields enforces the ADR-0058 trust
// boundary at the real user-file ingestion point: a user phase.json may NOT
// inject a transition branch into the kernel. Activating fields on a discovered
// user spec are stripped (with a warning) so the verdict/history/signal
// vocabulary stays built-in-only — a user phase that is a `current` in the flow
// can never route via injected on_pass/on_fail.
```

### `go/internal/phasespec/apicover_named_test.go:11` — above `func TestApplyArchetypeDefaults_EvaluateFillsDefaults(t *testing.T) {`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - func ApplyArchetypeDefaults (phasespec.go)
//   - func Roots (mergedcatalog.go)
//   - type IO (phasespec.go)
// Each test asserts a real contract (Rule 9), not a no-op reference.
```

### `go/internal/phasespec/apicover_rootswithpolicy_test.go:27` — above `func TestRoots(t *testing.T) {`

```text
// TestRoots covers the policy-loading wrapper Roots (cycle-17 made it delegate to
// RootsWithPolicy). With no policy.json under the temp root it falls back to the
// default root joined under projectRoot — a non-empty result.
```

### `go/internal/phasespec/branching_test.go:3` — above `import (`

```text
// branching_test.go — PA-BIG S2 (ADR-0058): wire-contract lock for the
// branching_strategy vocabulary. Pins the strategy const values and the JSON
// key so a registry entry's branching_strategy survives load into the catalog
// (the orchestrator's successorStrategy reads PhaseSpec.BranchingStrategy).
```

### `go/internal/phasespec/bug_repro_cycle229_test.go:10` — above `func TestBugRepro_Cycle229_TwoTierNamingMissing(t *testing.T) {`

```text
// TestBugRepro_Cycle229_TwoTierNamingMissing reproduces the bug described in
// scout-report.md finding #2: ValidateUserSpec accepts single-word user phase
// names (e.g. "scanner") because it only checks nameRE (^[a-z][a-z0-9-]*$)
// and lacks the two-tier enforcement gate (^[a-z]+(-[a-z]+)+$).
//
// FAIL on main tree (pre-fix): no violation returned for "scanner".
// PASS on worktree cycle-229 (post-fix): violation contains "multi-word".
```

### `go/internal/phasespec/catalog_membership_test.go:3` — above `import "testing"`

```text
// catalog_membership_test.go — PhaseSpec.Catalog decides advisor-MENU membership,
// never whether the phase is installed.
//
// Measured 2026-08-23: 65 non-control phases were projected as advisor SELECT
// cards against 12 enriched slots, so 53 rendered degraded — and 47 of the 65
// had never been selected in 120 cycles. Declining a slot fixes the crowding
// without removing capability; the declined set is still indexed by name in the
// prompt.
```

### `go/internal/phasespec/catalog_metadata_test.go:10` — above `var metadataAllowlist = map[string]bool{`

```text
// metadataAllowlist is the SHRINKING set of OPTIONAL catalog phases that still
// lack advisor-facing SELECT metadata (when_to_use / description). ADR-0052
// WS5-S3: it makes the metadata backlog VISIBLE and only lets it shrink — a NEW
// optional phase with no metadata fails the test (add metadata, do not pad this
// list), and a phase that GAINS metadata or is removed must be deleted here (the
// test rejects stale entries). Seeded against the live catalog at slice time
// (2026-06-17, 74 phases). Closing gap #3: the catalog metadata can no longer
// silently rot — each name here is a unit of backlog to retire, not a license.
```

### `go/internal/phasespec/catalog_metadata_test.go:55` — above `trackedUser := phasespec.TrackedUserPhaseNames(t, repoRoot(t))`

```text
// Built-in registry entries are always bound; user/on-disk overlay entries
// (cat.IsUser) bind only when their .evolve/phases/<dir>/phase.json is
// git-tracked — untracked dirs are runtime/local state that can never
// reach a CI checkout (cd49274beab2 class). Nil = no git context = bind all.
```

### `go/internal/phasespec/clamp.go:3` — above `import (`

```text
// clamp.go — the load-time registrar-equivalent clamp for DISCOVERED user
// specs (ADR-0073 security review, Finding 1 downstream trace; inbox
// loadtime-userspec-registrar-clamp).
//
// Two paths admit a user phase into the catalog. The registrar mint path
// (phaseregistrar.Register) normalizes and clamps: Optional forced true, and
// writes_source forces a sandbox-ENABLED dispatch profile into existence —
// the invariant `writes_source ⟹ sandboxed dispatch profile` holds by
// CONSTRUCTION. The discovery path (DiscoverUserSpecsFromRoots ← any on-disk
// .evolve/phases/*/phase.json: smuggled residue, git merge, operator typo)
// ran NO clamp: a spec claiming writes_source:true got worktree-write
// eligibility (core.worktreePhase reads spec.WritesSource) with no sandbox
// anywhere. This clamp closes that path by VERIFICATION: writes_source
// survives only when the spec's dispatch profile — the SAME on-disk profile
// the runner resolves at dispatch — exists with sandbox enabled.
//
// The two enforcement styles are pinned against each other by
// phaseregistrar's cross-binding test: a Register-minted writes_source phase
// must pass SandboxedProfilePredicate over its own persisted profile.
```

### `go/internal/phasespec/clamp_test.go:3` — above `import (`

```text
// clamp_test.go — RED contract for the load-time registrar-equivalent clamp
// (inbox loadtime-userspec-registrar-clamp 0.93; ADR-0073 security review,
// Finding 1 downstream trace). DiscoverUserSpecsFromRoots loads any on-disk
// .evolve/phases/*/phase.json into the catalog with NONE of the registrar's
// normalization: a smuggled/residue/typo spec claiming writes_source:true
// became a schedulable phase with worktree-write ELIGIBILITY
// (core.worktreePhase reads spec.WritesSource straight from the catalog) and
// no sandbox anywhere — specrunner has no sandbox plumbing; the registrar
// mint path enforces the invariant by CONSTRUCTION (it persists a
// sandbox-enabled profile). The discovery clamp enforces it by VERIFICATION:
// writes_source survives only when the spec's dispatch profile — the SAME
// on-disk profile the runner resolves at dispatch (TrimPrefix(AgentName,
// "evolve-")) — exists with sandbox enabled. On-disk reads are correct here
// (dispatch-parity, not a repo-contract scan — ADR-0084 I1 does not apply).
```

### `go/internal/phasespec/discover.go:54` — above `if s.OnPass != "" || s.OnFail != "" || s.BranchingStrategy != "" {`

```text
// ADR-0058 trust boundary: transition-activating fields are restricted to
// built-in phases. A user phase.json that declares one has it stripped (with
// a warning) so an overlay can never inject a verdict/history/signal branch
// into the kernel — the restriction is enforced at the real user-file
// ingestion point, where provenance is known.
```

### `go/internal/phasespec/discover.go:68` — above `func DiscoverUserSpecsFromRoots(roots []string) (specs []PhaseSpec, sources map[string]string, warnings []string) {`

```text
// DiscoverUserSpecsFromRoots reads phase definitions from each discovery root
// in order and concatenates them (ADR-0038 multi-root: .evolve/phases plus any
// plugin-bundle roots from EVOLVE_PHASE_ROOTS). On an inter-root name
// collision the LEFT-MOST root wins with a shadowing warning — so the local
// project root, conventionally first, can deliberately shadow a plugin phase.
// sources maps each kept phase name to the root it was loaded from
// (provenance for the inventory and `phases list`). Missing roots are
// fail-open, exactly like DiscoverUserSpecs.
```

### `go/internal/phasespec/discover.go:115` — above `if isOptionalBuiltinName(s.Name, c) {`

```text
// An overlay whose name matches an OPTIONAL built-in (e.g. `memo`,
// whose routing lives only in the operator overlay) is ADOPTED, not
// dropped — completing cycle-547's exemption (already wired into
// ValidateUserSpecWithCatalog/ApplyUserRouting via isOptionalBuiltinName)
// at this last unqualified call site. The name already sits in
// merged.order via the built-in, so replace the stub in place — no
// reorder. A NON-optional (mandatory spine) built-in still drops with
// the clash warning, so an operator can never hijack a spine phase's
// slot (the anti-hijack floor is the built-in's Optional flag, not the
// overlay's own).
```

### `go/internal/phasespec/merge_builtin_exempt_test.go:3` — above `import (`

```text
// merge_builtin_exempt_test.go — RED contract for cycle-554's
// memo-phase-routing-restore task.
//
// CORRECTED ROOT CAUSE (surfaces a Rule-3 pivot from the scout-report framing
// — documented in test-report.md): scout proposed renaming the built-in
// "memo" entry to a multi-word name. Investigation of the ACTUAL code found
// cycle-547 already shipped the real fix's foundation:
// ValidateUserSpecWithCatalog + ApplyUserRouting(3-arg) both exempt a
// single-word overlay name that matches an OPTIONAL built-in (see
// validate_builtin_exempt_test.go) — exactly the "memo" shape. That rollout
// missed ONE call site: Catalog.Merge (discover.go), which MergedCatalog
// (mergedcatalog.go — "the ONE merged-catalog loader", consumed by the CLI,
// self-check, host-contract-gate, and runner default) still calls unqualified.
// Verified empirically against the real repo today:
//
//	cat, _, warns, _ := MergedCatalog(repoRoot())
//	// warns == ["user phase memo clashes with a built-in — built-in kept, ..."]
//	// cat.Get("memo") == the bare built-in stub: Agent="", Classify=nil
//
// i.e. the overlay's agent/classify/routing fields never reach the merged
// Catalog (only cfg.Order/Triggers get them, via ApplyUserRouting operating
// on the raw discovered specs) — so any consumer resolving the phase THROUGH
// the catalog (CLI `phase list`, self-check, host-contract-gate) sees a
// hollow stub. Renaming the registry entry would ABANDON cycle-547's
// already-tested exemption machinery for a single-slice workaround
// (never_duplicate_centralize_via_design_patterns); completing the SAME
// exemption into Merge is the minimal, non-duplicative fix — reusing
// isOptionalBuiltinName (validate.go), not inventing a second mechanism.
// The reserved single-word floor is untouched: Merge's new adoption path is
// scoped exactly to isOptionalBuiltinName's existing safety scope (matches an
// OPTIONAL built-in only; a NON-optional built-in clash — e.g. "audit" —
// still drops with the existing warning, the anti-hijack negative case).
//
// No migration of .evolve/policy.json's pins.memo is needed: the phase keeps
// its name "memo" (this fix does not rename anything).
```

### `go/internal/phasespec/mergedcatalog.go:45` — above `func Roots(projectRoot string) []string {`

```text
// Roots returns the phase-spec discovery roots for a project, loading
// phase roots from policy.json (PathsConfig.PhaseRoots) or falling back
// to the default project-local .evolve/phases. Replaced EVOLVE_PHASE_ROOTS
// env read (cycle-17).
```

### `go/internal/phasespec/mergedcatalog.go:65` — above `user, clampWarns := ClampDiscoveredSpecs(user,`

```text
// Registrar-parity clamp (ADR-0073 — see clamp.go): EVERY admission seam
// applies it, so listing/lint consumers report the same writes_source the
// dispatch path enforces (they may never disagree about eligibility).
```

### `go/internal/phasespec/metadata_test.go:9` — above `const metadataRegistry = '{`

```text
// metadataRegistry exercises the advisor-facing metadata fields (ADR-0038):
// description, when_to_use, categories.
```

### `go/internal/phasespec/outputs_partition_test.go:8` — above `func TestValidateOutputsPartition(t *testing.T) {`

```text
// TestValidateOutputsPartition pins the ADR-0100 declaration rule at the seam
// both spec sources pass through, so an unclassified secondary is a load
// failure — never a phase that is silently ungated.
```

### `go/internal/phasespec/phasespec.go:36` — above `AgentOwed []string 'json:"agent_owed,omitempty"'`

```text
// AgentOwed names the secondary output files (basenames of Files[1:]) the
// phase AGENT must write itself; Files[0] is always owed. The declared-
// deliverables gate (ADR-0100) verifies these exist and parse after the
// phase, and a gap re-dispatches the agent with a correction naming them.
```

### `go/internal/phasespec/phasespec.go:58` — above `RequireFailureContext bool 'json:"require_failure_context,omitempty"'`

```text
// RequireFailureContext opts a verdict-emitting phase into the ADR-0039
// failure-signal contract: a FAIL/WARN sentinel must carry the structured
// failure block (class/defects/evidence_paths) or the contract gate
// re-dispatches with a correction.
```

### `go/internal/phasespec/phasespec.go:63` — above `VerdictFromSentinel string 'json:"verdict_from_sentinel,omitempty"'`

```text
// VerdictFromSentinel opts a JUDGMENT phase into having its own stated
// verdict decide, instead of having it discarded.
//
// A judgment phase (premise-challenge, adversarial-review) renders a
// conclusion and emits the canonical machine sentinel carrying it. Without
// this key the classifier reads STRUCTURE ONLY, so a well-formed report is
// PASS no matter what it concluded — cycle-1528 stated "FAIL (BLOCK). The
// cycle must not proceed as framed" and the cycle ran to completion.
//
// Stage word, not a bool, because the population this switches on is
// UNCALIBRATED — a verdict nothing ever consumed is a verdict nobody ever
// calibrated, so enforcing it without a measured soak would halt nearly
// every cycle at that phase. ADR-0091 owns the measured counts; they move
// every cycle, and a stale copy here would mislead a promotion decision.
// "" = off (legacy, byte-identical),
// "shadow" = record the disagreement and route as before, "enforce" = the
// stated verdict decides. Per-phase and not a global policy stage because
// the two phases are in very different states of calibration and must be
// promotable independently.
```

### `go/internal/phasespec/phasespec.go:102` — above `Description string   'json:"description,omitempty"'`

```text
// Advisor-facing metadata (ADR-0038): rendered into the phase inventory and
// the advisor's SELECT catalog so routing decisions are informed, not
// name-guessing. All optional; absence degrades to today's name-only card.
```

### `go/internal/phasespec/phasespec.go:108` — above `AllowedCLIs       []string                    'json:"allowed_clis,omitempty"'`

```text
// AllowedCLIs + ModelTierEnvelope (cycle-463 T1) carry this phase's OWN
// dispatch guardrails (phase-registry.json contracts) so the advisor's
// plan-prompt catalog can project them (router.PhaseCard mirrors these
// exact fields) instead of the advisor proposing a {cli,tier} blind. Both
// nil/empty in the common case (no per-phase guardrail configured).
```

### `go/internal/phasespec/phasespec.go:115` — above `Catalog   string 'json:"catalog,omitempty"'`

```text
// Catalog decides whether this phase occupies a slot on the ADVISOR'S
// SELECT MENU. It does not affect whether the phase is installed, routable,
// dispatchable by an explicit plan, or mintable — only whether the planner
// is offered it as a card.
//
// Measured 2026-08-23: 65 non-control phases were projected as SELECT cards
// against maxEnrichedCatalogCards=12, so 53 rendered in the degraded
// overflow form — and 47 of the 65 had never been selected in 120 cycles.
// The enriched slots were allocated by registry order, not usefulness, so
// phases the advisor actually uses lost their metadata to phases it has
// never once chosen. "If a human engineer can't definitively say which tool
// should be used, an AI agent can't be expected to do better."
//
// "" (absent) = CatalogSelect, today's behavior byte-for-byte.
// "on-demand" = keep it installed, take it off the menu. The declined set is
// still INDEXED by name in one line of the prompt, so nothing becomes
// undiscoverable — this hides phases from the menu, it does not remove them.
```

### `go/internal/phasespec/phasespec.go:137` — above `Effects       []string             'json:"effects,omitempty"'`

```text
// Effects names the lifecycle effects the phase's persona is instructed to
// perform outside its workspace (triage: "inbox-claim"). Each name binds
// to one deterministic check in the declared-deliverables gate; a user
// phase may declare them (they only ADD checks — nothing here loosens the
// primary contract, so the ADR-0058 stripping does not apply).
```

### `go/internal/phasespec/phasespec.go:151` — above `OnPass string 'json:"on_pass,omitempty"'`

```text
// Verdict branch targets (ADR-0058): the phase a verdict-branching phase
// transitions to on PASS/WARN (OnPass) and FAIL (OnFail). The state
// machine's Next consults these for the audit branch (S1); empty degrades
// to the literal table.
```

### `go/internal/phasespec/phasespec.go:157` — above `BranchingStrategy string 'json:"branching_strategy,omitempty"'`

```text
// BranchingStrategy (ADR-0058) selects how this phase's successor is chosen:
// "" / "verdict" = verdict-driven (the linear default); "history" = the
// failure-adapter consults cycle history rather than this phase's own verdict
// (retrospective); "signal" = a decision signal on PhaseResponse picks the
// successor (debugger). The orchestrator's successorStrategy reads it; empty —
// or an unset catalog — degrades to the literal phase-identity default
// (retro→history, debugger→signal), keeping the flow byte-identical.
```

### `go/internal/phasespec/phasespec.go:192` — above `type ArtifactGate struct {`

```text
// ArtifactGate is the declarative artifact-floor threshold for an anchor phase
// (PA-DDK DDK-4, ADR-0060). RequiresPresent demands a real on-disk handoff this
// cycle; VerdictIn (when non-empty) additionally requires the digested verdict
// to be one of the listed values — this is how audit's PASS/WARN soft-pass floor
// is expressed AS CONFIG. The digest that supplies present/verdict stays trusted
// Go; only these thresholds are operator-settable.
```

### `go/internal/phasespec/phasespec.go:203` — above `const (`

```text
// Branching strategy values for PhaseSpec.BranchingStrategy (ADR-0058). The
// empty value is treated as BranchingVerdict (the linear default), so a minimal
// phase.json need not declare one.
```

### `go/internal/phasespec/phasespec.go:391` — above `if viol := ValidateActivatingFields(s); len(viol) > 0 {`

```text
// Load-time validator (ADR-0058 S4): the registry is a contract — a
// malformed activating field fails loudly here, never silently degrades.
```

### `go/internal/phasespec/phasespec.go:396` — above `if viol := ValidateOutputsPartition(s); len(viol) > 0 {`

```text
// ADR-0100: a secondary output nobody classified would be silently
// ungated; the registry fails to load instead.
```

### `go/internal/phasespec/repo_phaseconfigs_test.go:3` — above `import (`

```text
// repo_phaseconfigs_test.go — authoring-time guard for the repo's tracked
// phase catalog (cycle-263 incident). The cycle-241 declared-semantics
// rejection deliberately FAILs any phase whose classify rules carry
// fail_if_signal (the Stage-3 signal bus does not exist, so the gate is
// inert — silently passing it would let an authoring mistake reach runtime
// undetected). Correct invariant, wrong enforcement boundary: 15 catalog
// phases shipped WITH the inert gate, so the rejection fired mid-cycle on
// first router insertion (adversarial-review in cycle-263 — a perfect PASS
// report recorded as FAIL, cycle dead, ~$ and ~30 min burned). This test
// moves the same invariant to CI: a mis-authored phase config fails the
// BUILD, never a production cycle. Delete this test when the Stage-3 signal
// bus lands and EvaluateClassify actually evaluates the gate.
```

### `go/internal/phasespec/repo_phaseconfigs_test.go:31` — above `tracked := TrackedPhaseDirs(t, root)`

```text
// Bind only git-TRACKED phase dirs: untracked dirs are runtime/local
// state that can never reach a CI checkout (cd49274beab2 class); nil
// tracked set = no usable git context = bind all (stricter fallback).
```

### `go/internal/phasespec/repo_phaseconfigs_test.go:66` — above `func TestRepoPhaseCatalog_VerdictFromSentinelStageIsKnown(t *testing.T) {`

```text
// TestRepoPhaseCatalog_VerdictFromSentinelStageIsKnown catches a typo'd rollout
// stage at AUTHORING time rather than mid-cycle.
//
// EvaluateClassify hard-FAILs an unknown verdict_from_sentinel word on purpose
// (an inert gate must fail loudly, cycle-241) — but discovering that from a
// dead cycle costs a dispatch and an operator's afternoon. This is the same
// belt-and-braces pairing the fail_if_signal guard above already uses: the
// runtime rejection is the floor, this is the tripwire.
```

### `go/internal/phasespec/routing.go:41` — above `for len(pending) > 0 {`

```text
// Placement is a FIXPOINT over the batch: a spec with a non-empty anchor
// waits for that anchor, which may itself be a later spec in the same batch
// — DiscoverUserSpecs sorts alphabetically, so input order never guarantees
// anchor-before-anchored (cycle-1550: bug-reproduction, anchored after
// fault-localization, was spliced first, missed its anchor, and silently
// took the before-audit fallback — executing a red-first Evaluate phase
// POST-build). Passes repeat while progress is made. When a pass strands,
// the stuck set splits: an anchor naming NO batch-mate will never appear —
// force-place that spec at the fallback, LOUDLY — while a spec anchored to
// a stuck batch-mate is only TRANSITIVELY blocked and is held, so the next
// pass places it after its just-placed anchor (declared order honored even
// through the escape). Only a pure anchor cycle, where no honorable order
// exists, falls back wholesale.
```

### `go/internal/phasespec/routing_anchor_fixpoint_test.go:11` — above `func TestApplyUserRouting_AnchorToLaterSpecInBatchIsHonored(t *testing.T) {`

```text
// Cycle-1550 (soak-20260824a): DiscoverUserSpecs hands ApplyUserRouting an
// alphabetically-sorted batch, and a spec anchored to an alphabetically-LATER
// spec found its anchor absent from cfg.Order at splice time — spliceAfter
// silently took the before-audit fallback, so bug-reproduction (a red-first
// Evaluate phase planned at index 3, pre-build) executed EIGHTH, post-build,
// where its deliberately-failing deliverable can only red the lane. Placement
// must therefore be a fixpoint over the batch: a spec waits for its anchor as
// long as any pass still makes progress.
```

### `go/internal/phasespec/routing_anchor_fixpoint_test.go:53` — above `func TestApplyUserRouting_UnresolvableAnchorFallsBackWithWarning(t *testing.T) {`

```text
// An anchor that never resolves must still place the phase (before audit, the
// long-standing fallback) but LOUDLY: silent fallback is what hid cycle-1550's
// mis-slotting for the life of the catalog.
```

### `go/internal/phasespec/tracked_realtree_test.go:3` — above `import (`

```text
// tracked_realtree_test.go — the ONE funnel for real-tree phase-catalog scans.
//
// The unit of the phase catalog is a SUBDIRECTORY carrying phase.json under
// .evolve/phases. The runtime can mint untracked phase dirs (and untracked
// profile stubs under .evolve/profiles) into the live tree; a scanner that
// binds everything on disk reds on state that can never reach a CI checkout —
// the 2026-08-09 zero-ship batch, fingerprint cd49274beab2
// (docs/incidents/2026-08-09-zero-ship-batch.md). Real-tree tests in this
// package (and in phasespec_test) must derive their iteration set from these
// helpers so future tests inherit the tracked-only filter.
//
// TrackedPhaseDirs / TrackedUserPhaseNames are EXPORTED although they live in
// a _test.go file (the export_test.go idiom): call sites span both package
// phasespec (repo_phaseconfigs_test.go, userphases_validate_test.go) and the
// external package phasespec_test (catalog_metadata_test.go,
// usercatalog_research_test.go), and the external test package compiles
// against the test-augmented package.
```

### `go/internal/phasespec/two_tier_naming_adversarial_cycle230_test.go:10` — above `func TestTwoTierNaming_DigitsRejected_Amp(t *testing.T) {`

```text
// Cycle-230 test-amplification adversarial tests for task phase-naming-lint.
// Written from spec only (no implementation read) — anti-bias isolation.
//
// Coverage gaps addressed:
//   - Digit-containing names: twoTierNameRE ^[a-z]+(-[a-z]+)+$ disallows digits
//     (documented in builder notes; tests deliberately NOT written by TDD-engineer)
//   - Three-or-more word names: ensures the (+) quantifier works for >2 segments
//   - Non-optional specs: ValidateUserSpec is called on non-optional specs in some
//     callers; built-in single-word names must not trip the two-tier gate if the
//     Optional flag guards the branch.
```

### `go/internal/phasespec/two_tier_naming_cycle230_test.go:10` — above `func TestTwoTierNaming_MultiWordAccepted(t *testing.T) {`

```text
// Cycle-230 companion tests to TestBugRepro_Cycle229_TwoTierNamingMissing
// (task phase-naming-lint). The anchor test covers the rejection criterion;
// these cover the acceptance + edge axes (adversarial-testing SKILL §6) so a
// fix that over-restricts (rejecting valid multi-word names) is also caught.
//
// DO NOT MODIFY (builder contract): make these pass by changing
// go/internal/phasespec/validate.go only.
```

### `go/internal/phasespec/usercatalog_research_test.go:41` — above `if tracked := phasespec.TrackedUserPhaseNames(t, root); tracked != nil {`

```text
// Bind only phases from git-TRACKED dirs: untracked dirs are runtime/local
// state that can never reach a CI checkout (cd49274beab2 class); nil
// tracked set = no usable git context = bind all (stricter fallback).
```

### `go/internal/phasespec/usercatalog_research_test.go:79` — above `"incident-postmortem": {`

```text
// Wave Ops (cycle 5) — domain-phase-catalog.md §3 Wave Ops table.
// incident-postmortem is the only evaluate phase (verdict vocabulary);
// runbook-draft (control) and capacity-plan (plan) carry
// verdict_on_pass but contract derivation leaves it inert (ADR-0035).
```

### `go/internal/phasespec/usercatalog_research_test.go:98` — above `"account-reconcile": {`

```text
// Wave Accounting (cycle-3 carry-forward) — domain-phase-catalog.md §3
// Wave Accounting table. Phase dirs were authored in cycle 3 but never
// committed; these cases pin the contract so the carry-forward commit
// is spec-covered.
```

### `go/internal/phasespec/usercatalog_research_test.go:117` — above `"risk-register": {`

```text
// Wave PM (cycle 6) — domain-phase-catalog.md §3 Wave PM table.
// dependency-map is the only evaluate phase (verdict vocabulary);
// risk-register and scope-baseline are plan phases — verdict_on_pass
// is carried for uniformity but contract derivation leaves it inert
// (ADR-0035, runbook-draft/capacity-plan precedent).
```

### `go/internal/phasespec/usercatalog_research_test.go:137` — above `"forces-analysis": {`

```text
// Wave Strategy (cycle 8) — domain-phase-catalog.md §3 Wave Strategy
// table. forces-analysis and market-sizing are evaluate phases
// (verdict vocabulary); okr-draft is a plan phase — verdict_on_pass
// is carried for uniformity but contract derivation leaves it inert
// (ADR-0035, risk-register/scope-baseline precedent).
```

### `go/internal/phasespec/usercatalog_research_test.go:157` — above `"opportunity-map": {`

```text
// Wave Product (cycle 10) — domain-phase-catalog.md §3 Wave Product
// table. metric-tree is the only evaluate phase (verdict vocabulary);
// opportunity-map and prd-draft are plan phases — verdict_on_pass
// is carried for uniformity but contract derivation leaves it inert
// (ADR-0035, okr-draft/risk-register precedent).
```

### `go/internal/phasespec/usercatalog_research_test.go:177` — above `"premise-challenge": {`

```text
// Wave 4 — adversarial-pipeline phases (2026-06-14, micro-phase-catalog.md §8).
// All 15 are evaluate gates → hasVerdict true; artifact <name>-report.md.
```

### `go/internal/phasespec/usercatalog_research_test.go:254` — above `"query-performance-scan": {`

```text
// Wave 5 — coverage expansion + plan/evaluate design pairing (2026-06-14,
// micro-phase-catalog.md §9). 9 evaluate gates (hasVerdict true, end in
// ## Verdict) + 5 plan design phases (hasVerdict false, no ## Verdict —
// ADR-0035 leaves plan verdict_on_pass inert, risk-register precedent).
```

### `go/internal/phasespec/userphases_validate_test.go:3` — above `import (`

```text
// userphases_validate_test.go — ONE table-driven port of the 8 LIVE bash
// phase-validation suites (Wave C of the bash→Go migration):
//
//	tests/test-implement-dependency-audit-phase.sh
//	tests/test-implement-security-scan-phase.sh
//	tests/test-phases-quality-gates.sh
//	tests/test-phases-release-and-memory.sh
//	tests/test-recover-wave2-phases.sh
//	tests/test-wave1-bugfix-phases.sh
//	tests/test-wave1-refactor-phases.sh
//	tests/test-wave1-router-config.sh
//
// The bash suites were authored as TDD RED contracts (cycles 214/217/246/247);
// the phases they encode have since SHIPPED, so the suites are now permanent
// regression guards. Each load-bearing bash check ran the real `evolve` binary
// (`phases list` / `phases validate`) — i.e. the DiscoverUserSpecs → Merge →
// ValidateUserSpec machinery in THIS package — plus jq/python/grep field reads
// on the same JSON the loader reads. This test reproduces that exact pipeline
// in-process: the phase.json rows go through MergedCatalog + ValidateUserSpec
// (the loader path the binary uses), and the profile / agent.md / router-doc /
// registry rows are file reads that mirror the bash jq/python/grep checks 1:1.
//
// FAITHFULNESS NOTE — stale `classify.fail_if_signal` assertions are NOT ported.
// Five bash rows assert classify.fail_if_signal keys (benchmark-gate
// perf.significant, fuzz-probe fuzz.crashers, rollback-plan rollback.ready,
// bug-reproduction repro.failing, behavior-compare behavior.preserved). The
// cycle-263 incident STRIPPED every fail_if_signal block from the shipped
// phase.json files (the Stage-3 signal bus does not exist; an inert gate makes
// the phase unconditionally FAIL at runtime — see repo_phaseconfigs_test.go,
// which asserts the ABSENCE of fail_if_signal). Those bash rows would FAIL
// against today's tree, so faking them green here would be a no-op lie. They are
// listed in the parity report (this file's package doc + the migration report)
// as "needs manual review — stale" and deliberately skipped. smell-scan's
// classify.fail_if_empty literal-JSON assertion is stale the same way (the field
// is applied by the archetype loader at discovery, not present in the shipped
// phase.json), so it is likewise not ported. The bash NEGATIVE corruption probes
// (kind:python / optional:false rejection) are covered by ValidateUserSpec unit
// tests in phasespec_test.go rather than duplicated here.
```

### `go/internal/phasespec/userphases_validate_test.go:193` — above `var profileRequiredKeys = []string{`

```text
// profileRequiredKeys are the keys the wave-3 profile schema check
// (test-phases-release-and-memory.sh AC3) requires. max_budget_usd is NOT a
// typed Profile field (it lives in the loader's Raw), so we read profiles as
// raw JSON exactly as the bash python3 snippet did.
```

### `go/internal/phasespec/userphases_validate_test.go:204` — above `if tracked := trackedRepoProfileNames(t, repoRoot()); tracked != nil && !tracked[name] {`

```text
// Only git-TRACKED profiles are repo config; an untracked same-named file
// is a runtime mint that can never reach a CI checkout (cd49274beab2
// class). Nil tracked set = no usable git context = bind all.
```

### `go/internal/phasespec/userphases_validate_test.go:386` — above `{"phases-quality-gates.sh/four-validate-OK", func(t *testing.T) {`

```text
// ============================================================
// test-phases-quality-gates.sh  (Wave-2:
// benchmark-gate, fuzz-probe, cleanup-sweep, rollback-plan)
// + test-recover-wave2-phases.sh (same 4 + mutation-gate)
// ============================================================
```

### `go/internal/phasespec/userphases_validate_test.go:476` — above `{"release-and-memory.sh/four-validate-OK", func(t *testing.T) {`

```text
// ============================================================
// test-phases-release-and-memory.sh  (Wave-3:
// changelog-sync, post-ship-monitor, api-contract-design,
// context-condense)
// ============================================================
```

### `go/internal/phasespec/userphases_validate_test.go:618` — above `var wave3 = []string{"changelog-sync", "post-ship-monitor", "api-contract-design", "context-condense"}`

```text
// wave3 is the Wave-3 release/feature/memory phase set
// (test-phases-release-and-memory.sh).
```

### `go/internal/phasespec/validate.go:40` — above `"database": true, "caching": true, "resilience": true,`

```text
// Wave 5 (skills-derived coverage expansion + plan/evaluate design pairing):
// data/query, cache, fault-tolerance, delivery-semantics, infra-config and
// stream/batch request classes the advisor classifies and routes.
```

### `go/internal/phasespec/validate.go:132` — above `func ValidateActivatingFields(s PhaseSpec) []string {`

```text
// ValidateActivatingFields returns well-formedness violations for the ADR-0058
// transition-activating fields on a spec, or nil when valid. It is the load-time
// validator (Load calls it): the registry is a contract, so a malformed
// activating field fails loudly rather than silently degrading to the literal
// kernel. It checks SHAPE, not presence — an empty field is valid (the byte-
// identical default); requiring a specific field is the registry-guard's job.
//
//   - branching_strategy must be a known strategy (verdict/history/signal) or empty.
//   - on_pass and on_fail are a verdict-branch PAIR: declare both or neither.
//     Next consults them only when both are set, so a half-set is dead config.
```

### `go/internal/phasespec/validate.go:158` — above `func ValidateOutputsPartition(s PhaseSpec) []string {`

```text
// ValidateOutputsPartition returns the violations of the ADR-0100 declaration
// rule for a spec's outputs: every secondary output (files[1:]) is classified
// exactly once — owed by the agent (agent_owed) or written by the harness
// (harness_produced) — and every classification names a basename that is
// declared. It runs for the registry (Load) and for user/overlay specs
// (validateUserSpec) alike, because an overlay REPLACES a built-in's spec
// wholesale (Catalog.Merge) and would otherwise ungate a phase silently.
```

### `go/internal/phasespec/validate_builtin_exempt_test.go:3` — above `import (`

```text
// validate_builtin_exempt_test.go — RED contract for cycle-547's
// memo-phase-routing-repair task.
//
// PROBLEM (scout Key Finding 1): the built-in optional `memo` phase can never
// route. Its activation overlay (.evolve/phases/memo/phase.json) is a
// single-word name, and twoTierNameRE (validate.go) rejects every
// single-word name for a discovered overlay with no exemption for names
// that already exist in the built-in catalog — so ApplyUserRouting
// (routing.go) always skips it with a warning and cfg.Order never gets
// "memo" spliced in. ~150 PASS cycles have produced zero memo.md /
// carryover-todos.json artifacts as a result.
//
// FIX CONTRACT (this cycle's new surface — undefined until Builder adds it,
// so this whole package fails to compile today; that compile failure IS the
// RED evidence, mirroring the cycle-465/507 precedent):
//
//   - ValidateUserSpecWithCatalog(s PhaseSpec, builtin Catalog) []string
//     behaves exactly like ValidateUserSpec, EXCEPT: the twoTierNameRE
//     single-word floor is skipped when s.Name matches an existing builtin
//     catalog entry AND that entry's Optional field is true. A name that
//     matches a NON-optional built-in (audit, build, ship, ...) keeps the
//     floor — the exemption is scoped to already-optional built-ins only, so
//     an operator can never hijack a mandatory spine phase's name/slot.
//   - ApplyUserRouting gains a third parameter, `builtin Catalog`, and
//     consults ValidateUserSpecWithCatalog instead of the bare
//     ValidateUserSpec. Every existing call site (cmd_cycle.go's builtinCat,
//     routing_dispatch.go's o.catalog, and this package's own
//     routing_test.go) already has a Catalog in scope at the call site.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestValidateUserSpecWithCatalog_ExemptsOptionalBuiltinName
//     (the memo case itself).
//   - Negative : TestValidateUserSpecWithCatalog_RejectsGenuineNewSingleWordName
//     (a name genuinely absent from the built-in catalog must still fail —
//     the exemption must not degenerate into "any single-word name passes").
//   - Negative (the critical anti-gaming case):
//     TestValidateUserSpecWithCatalog_RejectsNonOptionalBuiltinNameOverlay —
//     an overlay literally named "audit" (a real, non-optional built-in) with
//     Optional:true set on the OVERLAY itself must still be rejected: the
//     exemption looks at the BUILT-IN's Optional flag, not the overlay's,
//     or an operator could hijack the mandatory audit phase's routing slot.
//   - E2E      : TestApplyUserRouting_RoutesBuiltinNameOverlayWithoutWarning
//     drives the real routing splice (cfg.Order/Triggers/PhaseEnable) end to
//     end and asserts zero warnings — the actual PASS-cycle-unblocking
//     behavior, not just the validator in isolation.
//
// ADR-0058 field-stripping (the 4th AC clause: "ADR-0058 field-stripping
// unaffected") is existing, untouched code (DiscoverUserSpecs strips
// on_pass/on_fail/branching_strategy at the real user-file ingestion point,
// unrelated to this fix's seam) — already covered by
// discover_test.go/activating_fields_test.go; no new predicate needed here
// (Step 5: regression coverage, not new work).
```

### `go/internal/phasespec/writes_source_declared_test.go:11` — above `func TestUserSpecs_SourceWritersDeclareWritesSource(t *testing.T) {`

```text
// TestUserSpecs_SourceWritersDeclareWritesSource pins the operator-overlay
// phases whose personas author files into the worktree. Since ADR-0097 a
// phase without writes_source is FENCED — its worktree writes are undone and
// reported — so a writer that forgets the declaration loses its deliverable.
// The two known writers are pinned here (the architecture review found both
// undeclared); add a name when a new persona writes source.
```
