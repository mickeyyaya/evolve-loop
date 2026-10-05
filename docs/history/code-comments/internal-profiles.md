# Comment history: `internal/profiles`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/profiles/deep_tier_family_arrangement_test.go:3` — above `import (`

```text
// deep_tier_family_arrangement_test.go — the 2026-08-26 operator directive:
// deep/top-tier task types run on codex (gpt-5.6-sol — the 2026-09-09 gpt-6-astra cutover was withdrawn 2026-09-10 for token cost, at the directed rung — high since 2026-09-01; the model is pinned by bridge/codex_tier_map_test.go and effort by effort_defaults_test.go, not here), EXCEPT the two
// adversarial checks whose independence from the codex builder is the
// pipeline's anti-gaming core (cross-family floor: builder=codex ⇒ its graders
// are another family) and the advisor brain. Pins the WHOLE arrangement so a
// single-profile drift — either direction — is loud: a mover slipping back to
// claude silently sheds sol leverage; auditor/adversarial-review slipping to
// codex silently puts codex in judgment of codex.
```

### `go/internal/profiles/deep_tier_family_arrangement_test.go:23` — above `loader, names := RealTreeProfiles(t)`

```text
// TrackedRealProfileNames is the package's ONE funnel over the live
// profiles dir: the runtime mints untracked stubs into the same directory,
// and a raw ReadDir scanner reds on state no CI checkout can see (the
// 2026-08-09 zero-ship batch, fingerprint cd49274beab2) — exactly the
// shape this test's first draft reintroduced.
```

### `go/internal/profiles/driver_agnostic_test.go:94` — above `loader, names := RealTreeProfiles(t)`

```text
// RealTreeProfiles binds only git-tracked profiles (untracked = runtime
// mints, the cd49274beab2 false-RED class) and List() already excludes
// non-profile JSON like tool-policy.json.
```

### `go/internal/profiles/effort_defaults_test.go:3` — above `import (`

```text
// effort_defaults_test.go — cycle-566 RED test for the per-phase EFFORT default
// matrix (inbox `per-phase-effort-routing`). Loads the REAL shipped profiles (not
// a fixture) and asserts each phase pins the committed effort level. Every value
// is config-sourced — read through the loader from .evolve/profiles/*.json — so
// the production defaults carry ZERO Go literals (acceptance: "all config").
//
// Evidence for the matrix (inbox summary): Opus 4.5 at medium effort matches
// Sonnet 4.5's best SWE-bench score at 76% fewer output tokens; max effort buys
// single-digit gains at ~4x cost. Cheap survey/classification phases run low;
// generative/judgement phases run medium.
//
// RED now: scout/triage currently pin "medium", auditor pins "high", and
// tdd-engineer/adversarial-review pin nothing. GREEN once the config is aligned.
```

### `go/internal/profiles/effort_defaults_test.go:41` — above `want := map[string]string{`

```text
// 2026-09-01 operator directive: the CODEX-routed deep/top phases
// (gpt-5.6-sol — the 2026-09-09 gpt-6-astra cutover was withdrawn 2026-09-10 for token cost) run at HIGH — superseding 2026-08-28's max rung (which
// had superseded 2026-08-24's xhigh). Max is codex's most quota-hungry
// rung ("Max and Ultra consume usage limits faster"); the operator traded
// one rung of reasoning for quota headroom. Note the deliberate inversion
// this creates: the CLAUDE-routed deep/top graders stay at xhigh (above
// codex's high) — Anthropic's Opus guidance (docs/research/
// fable-simulation-2026/model-profiles.md) recommends xhigh for agentic
// work, and the graders are the adversarial quality floor. The abstract
// effort dial is realized per family; the split is by design, not drift.
// The fast/balanced rows keep the cycle-566 cost matrix.
```

### `go/internal/profiles/effort_defaults_test.go:74` — above `const codexDeepTopRung = "high"`

```text
// codexDeepTopRung is the single source for the codex deep/top effort rung —
// the one edit a directive change requires (proven: 2026-08-28 max, 2026-09-01
// high). Read by the matrix pin's codex rows and the class guard.
```

### `go/internal/profiles/effort_defaults_test.go:79` — above `const maxEffortRung = "max"`

```text
// maxEffortRung names the literal top rung for the CLI-agnostic converse
// guard: "max" is the most expensive rung on every family that exposes it,
// and it may only ever appear on a deep/top profile. Since 2026-09-01 nothing
// runs at max (codex deep/top moved to high), so the converse polices a
// currently-empty set — kept armed for any future adoption.
```

### `go/internal/profiles/effort_defaults_test.go:86` — above `func TestCodexDeepTierProfilesAllRunAtDirectedRung(t *testing.T) {`

```text
// CLASS GUARD for the 2026-08-28 directive. The original change swept the 21
// codex deep/top profiles that existed at that moment — a point-in-time edit.
// failure-adjudicator.json landed hours later on a different branch (#508) and
// missed the sweep entirely, arriving on main at xhigh while every sibling ran
// max. Nothing caught it: the realizability guard passed (xhigh is still a
// mapped rung) and the matrix pin only names specific profiles.
//
// So the rule is pinned as a RULE rather than as a list. A new codex deep/top
// profile now has to state its effort deliberately instead of inheriting a
// stale default by accident.
//
// WHAT THIS DOES NOT COVER (adversarial review, stated so it is not over-read):
// the selector reads the DECLARED `model_tier_default`, not the tier a phase
// actually dispatches at. `subagent.applyModelTierOverride` floor-escalates a
// profile via `model_tier_overrides[situation]`, and one such situation is live
// today — `cycle_1_or_low_goal` fires whenever Cycle <= 1, so scout.json
// (codex, balanced, effort low) really does dispatch at DEEP tier on cycle 1
// while its effort stays low. Since ADR-0096 an ESCALATED tier CAN raise
// effort, but only through the profile's own `effort_overrides[tier]` (read by
// bridge.effortForTier at launch) — config decides, never a guard. The
// audit_retry_2plus situation is produced by core.repairRoundTier at the
// dispatch seam; TestEffortOverrides_PinnedToDirectiveRungs pins the rungs
// those overrides may name.
//
// RUN WITH -count=1 WHEN ONLY A PROFILE JSON CHANGED. Go's test cache does not
// track reads that escape the module root via "..", so a bare `go test` serves
// a stale PASS after a profile-only edit — reproduced: cached "ok" while the
// regression was live on disk. CI and `make test` already pass -count=1; the
// exposure is local verification.
```

### `go/internal/profiles/effort_defaults_test.go:116` — above `loader, names := RealTreeProfiles(t)`

```text
// Through RealTreeProfiles, NOT a raw directory scan. The runtime mints
// UNTRACKED profile stubs into .evolve/profiles; a scanner that binds
// everything on disk reds on state that can never reach a CI checkout
// (the 2026-08-09 zero-ship batch, fingerprint cd49274beab2).
//
// This is not hypothetical here: the first version of this guard DID scan
// raw, and it failed in the live runtime plane on two minted stubs
// (disposition-preflight, regression-predicate-precheck) that carry no
// effort_level at all — it would have red the test gate on every cycle.
```

### `go/internal/profiles/effort_defaults_test.go:152` — above `func TestMaxEffortOnlyOnDeepOrTopProfiles(t *testing.T) {`

```text
// The CONVERSE of the class guard above (placement law), per the 2026-08-29
// operator directive: "the max thinking level should only apply to deep/top
// model". Together the two make it a biconditional for codex — deep/top ⟺ max —
// and this half alone constrains EVERY family, so `max` can never drift onto a
// fast/balanced phase.
//
// Why pin a constraint nothing violates today: max is the most expensive rung
// on every CLI that has one (codex's own picker warns "Max and Ultra consume
// usage limits faster"). A fast/balanced phase is fast/balanced BECAUSE it was
// costed that way — scout and triage sit at low under the cycle-566 matrix.
// Silently promoting one to max would raise spend with nothing reporting it:
// the same shape as every other defect this file guards, a change nobody sees.
//
// CLI-AGNOSTIC on purpose. The directive is about thinking level vs model tier,
// not about codex — claude exposes max too, and the moment a claude profile
// adopts it the same rule must hold.
//
// NOTE on escalation: this reads the DECLARED tier, like its sibling. A phase
// floor-escalated at dispatch (scout.json → deep on cycle 1 via
// model_tier_overrides) keeps its declared effort, and that is COMPLIANT:
// scout runs low, and low is not max. The directive restricts where max may
// APPEAR; it does not require an escalated phase to adopt it.
```

### `go/internal/profiles/effort_defaults_test.go:205` — above `t.Logf("no tracked profile at effort %q — expected since the 2026-09-01 directive; placement law armed for future adopti…`

```text
// Since 2026-09-01 this is the EXPECTED state: codex deep/top moved to
// high, so no tracked profile carries max. The guard stays armed for
// any future max adoption. Decode-health delegation, precisely: the
// sibling class guard's per-profile comparison reads the same
// EffortLevel field through the same loader and fails loudly (Errorf)
// on a decode regression; its zero-match Fatal is a separate
// CLI/tier-selector check, not an EffortLevel guard.
```

### `go/internal/profiles/effort_defaults_test.go:216` — above `func TestEffortOverrides_PinnedToDirectiveRungs(t *testing.T) {`

```text
// TestEffortOverrides_PinnedToDirectiveRungs — a per-tier effort override is a
// second statement of the directive rung for that tier (ADR-0096: the repair
// round escalates builder/tdd-engineer to deep and the effort follows). Pin it
// beside the matrix so the next rung change edits ONE constant and the
// override cannot silently keep the old rung. Run with -count=1 after a
// profile-only edit (see the class-guard note above).
```

### `go/internal/profiles/fallback_chain_test.go:22` — above `func TestEveryAgentProfileHasAFallbackChain(t *testing.T) {`

```text
// TestEveryAgentProfileHasAFallbackChain — operator policy (2026-09-14): a
// phase must try every available CLI before giving up, so no agent that names
// a primary CLI may leave cli_fallback empty. Each entry must be a registered
// driver, must be a family the profile's allowed_clis permits, and must not be
// the agy family (the 2026-06-07 ban on agy as a rescue). The two audit-spine
// agents that had no chain at all (auditor, tdd-engineer) are the ones this
// pins hardest: with an empty chain, one Claude wall failed the whole cycle.
// The Claude-family floor (family_floor_test.go) keeps those five agents'
// chains INSIDE the claude family on purpose; this guard only requires a chain.
```

### `go/internal/profiles/family_floor_test.go:8` — above `var claudeFamilyFloor = map[string]string{`

```text
// family_floor_test.go — the claude-family FLOOR (2026-09-02 operator
// directive: rebalance claude/codex usage; claude is the quota-constrained
// family). claudeFamilyFloor is the ONE home of "which phases stay claude and
// why" — the deep-tier arrangement guard projects its exceptions from this
// map rather than restating it (2026-09-02 architecture review: the belief
// briefly had three homes, one self-contradicting).
//
// THE HONEST PREDICATE (stated because the tree falsifies the tempting one):
// this floor is NOT "everything that feeds a blocking verdict". The residual
// claude set is: the two ADVERSARIAL graders whose judgment of build CONTENT
// must be cross-family (anti-gaming core), the test author (anti-cooperative
// -bias family split from the builder), and the two spec verifiers
// (audit-side verification of build output). Verdict-DECLARING mechanical
// gates — merge-to-main-gate, coverage-gate — are deliberately on codex:
// their verdicts are deterministic measurements the host re-verifies, not
// adversarial judgment. The classify.require_sections("Verdict") discriminator
// exists in phase.json but deliberately does NOT map onto this floor.
//
// Every floor entry keeps its cli_fallback INSIDE the floor family ON PURPOSE
// (claude-p behind claude-tmux — a driver-level rescue for a boot or artifact
// timeout, 2026-09-14 operator policy "try every available CLI before giving
// up"): a claude-quota halt on a floored phase still fails loudly, because the
// "obvious" operator remedy — adding a codex fallback to the auditor — silently
// puts codex in judgment of codex on every fallback dispatch. The guard binds
// cli, cli_fallback, AND allowed_clis (the policy-pin validator's enforcement
// surface, mirrored per the tdd-engineer precedent) so no plane can breach the
// floor quietly.
//
// Tier and effort facts live with their own guards
// (deep_tier_family_arrangement_test.go, effort_defaults_test.go), never
// restated here. Runtime-minted audit-side stubs
// (pre-audit-evidence-check, production-path-wiring-proof,
// defect-disposition-*, inherited-defect-reconcile, ship-stage-hygiene-check)
// are UNTRACKED runtime-plane state — RealTreeProfiles filters them by design
// (the 2026-08-09 zero-ship class); their family is the minting registrar's
// contract, not this guard's.
```

### `go/internal/profiles/family_floor_test.go:62` — above `func TestClaudeFamilyFloor(t *testing.T) {`

```text
// TestClaudeFamilyFloor holds both directions, value-free against the live
// builder so a future builder flip cannot make the floor self-contradictory:
//
//	forward: a claude-family profile outside the floor is unjustified spend on
//	         the quota-constrained family (2026-09-02: 24 such moved to codex,
//	         triage included — the every-cycle spine win);
//	reverse: every floor entry must resolve, must NOT share the builder's
//	         family (on cli, on every cli_fallback entry, and on every
//	         allowed_clis entry — the policy-pin plane), and must keep the
//	         loud-failure empty fallback.
```

### `go/internal/profiles/loop_unblock_contract_test.go:12` — above `for _, name := range []string{"router"} {`

```text
// Two phases have LEFT this set, both for the same reason — agy-tmux does
// not honor the structured-sentinel sub-contract even under explicit
// correction, and a phase whose corrections cannot converge either demotes
// the contract gate or burns the cycle:
//   - adversarial-review, 2026-07-29: 7/7 contract failures across
//     corrections demoted the gate enforce→advisory in BOTH batch-18
//     wave-1 lanes (see TestAdversarialReviewRoutesToClaudeDeep).
//   - triage, 2026-07-30: emitted a v1 FAIL sentinel with no
//     schema_version-2 failure block, corrections exhausted, and the CYCLE
//     failed on the top-priority queue item's lane (batch-21 cycle-1215).
//     See TestTriageRoutesToCodexForQuotaBalance (2026-09-02 supersede:
//     that incident was agy-specific; triage now codex for quota balance).
//   - retrospective, 2026-08-14: operator-directed model-strength reroute —
//     agy deep resolves Gemini 3.1 Pro, now the weakest deep-tier model in
//     the fleet (vs opus and gpt-5.6-sol); retro post-mortems are
//     reasoning-heavy and move to claude/deep (opus). See
//     TestRetrospectiveRoutesToClaudeDeep.
// The durable fix LANDED: second consecutive contract-gate block escalates
// the re-dispatch CLI (internal/core/contract_escalation.go, soft overlay)
// — a phase-wide reroute is no longer the only remedy, which is what makes
// the 2026-09-02 triage supersede below safe to carry.
```

### `go/internal/profiles/loop_unblock_contract_test.go:49` — above `func TestRetrospectiveRoutesToCodexDeep(t *testing.T) {`

```text
// TestRetrospectiveRoutesToCodexDeep pins the 2026-08-26 operator-directed
// reroute (supersedes the 2026-08-14 claude/deep pin, whose own comment named
// this move: "gpt-5.6-sol is the alternative once codex returns from quota
// bench" — codex returned, live-verified at 44% dispatch share with zero
// quota halts). Retro post-mortems are the single biggest deep-tier consumer
// (~40% of deep dispatches) and are NOT adversarial-vs-builder work, so they
// lead the deep→sol arrangement: codex/deep (gpt-5.6-sol at the directed
// rung — see effort_defaults_test.go), claude as the explicit fallback
// (universal-fallback rule; agy stays banned from fallback chains).
```

### `go/internal/profiles/loop_unblock_contract_test.go:75` — above `func TestAdversarialReviewRoutesToClaudeDeep(t *testing.T) {`

```text
// TestAdversarialReviewRoutesToClaudeDeep pins the 2026-07-29 reroute: the
// adversarial reviewer is a review-class phase (auditor / plan-reviewer /
// premise-challenge house pattern — claude at deep) and its deliverable
// contract requires exact section + machine-verdict compliance that agy did
// not honor under correction (cycles 1171/1172, 7/7 blocks, circuit opened).
// Contract-gate CLI-escalation LANDED (internal/core/contract_escalation.go,
// soft overlay): a second consecutive contract block escalates the
// re-dispatch CLI — the belt behind routing primaries by quota rather than
// by format-compliance-only (the 2026-09-02 triage supersede).
```

### `go/internal/profiles/loop_unblock_contract_test.go:102` — above `func TestTriageRoutesToCodexForQuotaBalance(t *testing.T) {`

```text
// TestTriageRoutesToCodexForQuotaBalance pins the 2026-09-02 reroute
// (supersedes the 2026-07-30 claude pin the way TestRetrospectiveRoutesToCodexDeep
// superseded its predecessor). The 2026-07-30 evidence was AGY-specific —
// triage-on-agy emitted a v1 FAIL sentinel with no schema_version-2 block and
// the fix promoted the declared fallback (claude) to primary; codex was never
// the offender, and its contract compliance is since live-proven at volume:
// the retro supersede below carries the quantified run (76 codex dispatches,
// 44% share, cycles 1530-1552, zero quota halts), and the v22.21.0 soak
// (cycles 1589-1594) shipped on codex scout/build handoffs with the contract
// gate green throughout. With claude
// the quota-constrained family and triage firing EVERY cycle, the single-
// writer decision phase moves to codex; claude becomes the fallback
// (universal-fallback rule), the tier stays balanced (this is a quota
// change, not a reasoning-budget change), and the contract-gate + correction
// ladder + second-block CLI escalation remain the format-compliance belts.
```

### `go/internal/profiles/profile_model_routing_adversarial_test.go:146` — above `func TestModelTierOverridesWithinEnvelope(t *testing.T) {`

```text
// TestModelTierOverridesWithinEnvelope is the permanent regression guard added
// in cycle-974 (envelope-floor-guard-model-tier-overrides). It asserts that
// every model_tier_overrides value ranks within its OWN profile's
// model_tier_envelope [min,max] on the canonical ladder fast<balanced<deep<top.
//
// The two pre-existing envelope tests leave a gap this closes:
// TestAllProfilesModelTierOverridesValuesAreCanonical checks an override value
// is a canonical tier name; TestEnvelopeTierHierarchyOrdering checks a
// profile's own min/default/max are non-decreasing. Neither cross-checks an
// override value against its own envelope bounds — the exact drift that let a
// below-floor ("fast" under min="balanced") and above-ceiling ("deep" over
// max="balanced") override ship undetected across six profiles.
//
// Profiles without an envelope (or with an empty/non-canonical min/max) are
// skipped, matching the conventions of the tests above.
```

### `go/internal/profiles/profiles.go:37` — above `CLIFallbackOnExit  []int              'json:"cli_fallback_on_exit,omitempty"'`

```text
// CLIFallbackOnExit enumerates the bridge exit codes that trigger
// fallback (Workstream G; default extended in cycle-122 Fix 2).
// Defaults to [80, 81, 124, 127] when nil/empty:
//   80  = ExitREPLBootTimeout (the *-tmux REPL never showed its prompt)
//   81  = ExitArtifactTimeout (bridge artifact-timeout — added in cycle-122)
//   124 = coreutils timeout(1) exit code (defensive; if a wrapper uses it)
//   127 = ExitMissingBinary  (the CLI binary isn't on PATH)
// Operators can extend per-agent (e.g. add 2 ExitSafetyGate) for an
// even more aggressive policy, OR shrink to [80, 127] for the
// production-strict posture where 81 should surface to the
// failure-adapter rather than retry. CLI failures NOT in this list
// still hard-fail — a legitimate FAIL verdict never silently routes
// to a different CLI. See bridge/exitcodes.go for the canonical exit
// numbers and runner/cli_chain.go:defaultFallbackOnExit for the live
// default per code.
```

### `go/internal/profiles/profiles.go:81` — above `DigestFile string 'json:"digest_file,omitempty"'`

```text
// DigestFile names a pre-generated role-scoped digest (go/internal/digest
// output) resolved relative to the profile dir when not absolute, mirroring
// SystemPromptFile. When set AND the file exists on disk, systemprompt.Resolve
// prefers its content over SystemPromptFile (cycle-1391,
// tokenopt-role-scoped-instruction-digests Task 2). Unset, or set but the
// file absent, leaves the existing precedence chain unchanged.
```

### `go/internal/profiles/profiles_test.go:259` — above `func TestSmoke_RealProfiles(t *testing.T) {`

```text
// TestSmoke_RealProfiles — load every git-TRACKED profile under
// .evolve/profiles/ (via the RealTreeProfiles funnel; untracked files are
// runtime mints, not repo config — cd49274beab2 class) and verify each has
// Name + Role + CLI. Skipped if dir absent. This is the canary for any
// schema drift between bash JSON and Go types.
```

### `go/internal/profiles/provenance_test.go:1` — above `package profiles`

```text
// provenance_test.go — cycle-238 task `profile-provenance-field` (RED first).
//
// Pins the Invariant-1 foothold: every profile carries a `generated_from`
// provenance marker distinguishing hand-authored originals from generated
// projections (campaign retro §4, migration step 4; architecture-design R1,
// blueprint B1/B2). Contract for Builder:
//
//	Profile gains `GeneratedFrom string `json:"generated_from,omitempty"``
//
// Provenance vocabulary is free-form (architecture: "hand-authored" today,
// "phasespec:<name>@<sha>" later) — the validation logic lives at the CLI
// (`phases validate`, see cmd_phases_cycle238_test.go), not here.
```

### `go/internal/profiles/provenance_test.go:22` — above `const stampedProfile = '{`

```text
// stampedProfile mirrors a post-cycle-238 .evolve/profiles/*.json with the
// provenance stamp applied.
```

### `go/internal/profiles/tracked_realtree_test.go:3` — above `import (`

```text
// tracked_realtree_test.go — the ONE funnel for real-tree profile scans.
//
// Every test in this package (and in profiles_test) that iterates the LIVE
// .evolve/profiles directory must go through RealTreeProfiles /
// TrackedRealProfileNames so it binds only git-TRACKED profiles. The runtime
// mints untracked profile stubs into the same directory; a scanner that binds
// everything on disk reds on state that can never reach a CI checkout — the
// 2026-08-09 zero-ship batch, fingerprint cd49274beab2
// (docs/incidents/2026-08-09-zero-ship-batch.md).
//
// The helpers are EXPORTED although they live in a _test.go file (the
// export_test.go idiom): call sites span both package profiles
// (profiles_test.go, driver_agnostic_test.go) and the external package
// profiles_test (profile_model_routing_*_test.go), and the external test
// package compiles against the test-augmented package.
```

### `go/internal/profiles/tracked_realtree_test.go:62` — above `func RealTreeProfiles(t *testing.T) (*Loader, []string) {`

```text
// RealTreeProfiles returns a Loader over the live .evolve/profiles directory
// plus its List() names filtered to git-tracked profiles. Untracked names are
// runtime-minted state, logged and NOT bound (cd49274beab2 class); when git
// context is unusable the full unfiltered list is returned (bind-all
// fallback). New real-tree tests must iterate via this helper.
```

### `go/internal/profiles/tracked_realtree_test.go:112` — above `root := mirrorTrackedProfiles(t, filepath.Join(realProfilesDir(t), "..", ".."), tracked)`

```text
// The live tree is never mutated (a phase sandbox denies writes under
// .evolve/profiles — cycles 1676/1679 red on EPERM here): the real tracked
// profiles are mirrored into a temp git repo and the decoy is planted THERE.
```

## cli-routing table L1a (2026-10-05)

### `go/internal/profiles/family_floor_test.go:8` — above `var claudeFamilyFloor = map[string]string{`

```text
// claudeFamilyFloor lists the phases that must stay off the builder's CLI
// family, each with its reason. See ADR-0104.
```
