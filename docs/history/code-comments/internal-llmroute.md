# Comment history: `internal/llmroute`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/llmroute/candidates_unified_test.go:10` — above `type unifiedCase struct {`

```text
// Cycle-1265 RED contract for task `unify-llmroute-candidate-chain-builders`.
//
// Before this cycle the "primary first, then deduped profile.cli_fallback"
// chain was implemented TWICE: candidatesFrom (llmroute.go:216, used by
// Resolve) and chainCandidates (dispatch.go:77, used by ChainFor). The only
// difference is that ChainFor's copy also excludes prof.CLI — the original
// primary the composition root deliberately swapped away from (dispatch.go
// doc comment) — while Resolve's copy intentionally KEEPS the profile chain
// intact after a policy pin (llmroute.go Resolve doc comment).
//
// The contract frozen here: ONE builder with that difference as an explicit
// parameter.
//
//	func buildCandidates(primary string, prof *profiles.Profile, excludeProfileCLI bool) []string
//
// living in dispatch.go (already the Plan/Dispatch home), with candidatesFrom
// and chainCandidates both DELETED, Resolve calling
// buildCandidates(primary, prof, false) and ChainFor calling
// buildCandidates(primary, prof, true). Both documented behaviours are
// preserved verbatim — this is a dedup, not a behaviour change.
//
// DO NOT MODIFY (TDD handoff, doNotModifyTests:true).
```

### `go/internal/llmroute/chain_policy_test.go:8` — above `func TestExcludeFamilies_DropsTheOperatorBannedFamilies(t *testing.T) {`

```text
// TestExcludeFamilies_DropsTheOperatorBannedFamilies — the last-resort tail
// honours the operator's family ban (policy workflow.universal_fallback_exclude,
// default ["agy"]: the 2026-06-07 judgment that an error-prone model is the
// worst rescue choice). A banned family may still be a configured primary.
```

### `go/internal/llmroute/chain_shared.go:44` — above `func ExcludeFamilies(discovered, families []string) []string {`

```text
// ExcludeFamilies drops every driver whose family is in families — the
// operator's ban on a family as a LAST RESORT (policy
// workflow.universal_fallback_exclude, default ["agy"]: the 2026-06-07
// judgment that an error-prone model is the worst rescue choice). A banned
// family may still be a configured primary; only the discovered tail is filtered.
```

### `go/internal/llmroute/dispatch_amplify_test.go:11` — above `func TestDispatch_EmptyCandidatesNeverCallsLaunchOrClaimsSuccess(t *testing.T) {`

```text
// Test Amplification (cycle 435, black-box adversarial pass on top of the
// TDD-authored dispatch_test.go). These tests were designed from the
// contract only (Plan/DispatchResult shapes + Dispatch/ChainFor signatures,
// pinned by the pre-existing RED-turned-GREEN suite) without reading
// dispatch.go's actual walk implementation, per the amplifier's black-box
// mandate. They target basic/edge/null/negative/large-scale inputs the
// original suite didn't cover: the full default trigger set, chain length
// != 2, empty input, and concurrent independent use.
```

### `go/internal/llmroute/dispatch_amplify_test.go:41` — above `func TestDispatch_AllDefaultTriggerCodesAdvanceChain(t *testing.T) {`

```text
// TestDispatch_AllDefaultTriggerCodesAdvanceChain (basic, table-driven): the
// cycle-435 goal names five standard fallback triggers [80 81 85 124 127].
// The pre-existing suite only exercises 81 end-to-end; this pins the other
// four so a future edit to the trigger set (or a partial implementation that
// special-cased 81) can't silently regress the other codes.
```

### `go/internal/llmroute/dispatch_amplify_test.go:153` — above `func TestDispatch_ConcurrentIndependentCallsStayIsolated(t *testing.T) {`

```text
// TestDispatch_ConcurrentIndependentCallsStayIsolated (concurrency, the
// "go test -race green" requirement the cycle-435 goal names explicitly):
// many goroutines call Dispatch concurrently, each with its own Plan and
// scriptedLaunch closure. Dispatch must not share any mutable state across
// calls (e.g. a package-level counter or cache) -- every goroutine's result
// must reflect only its own scripted sequence.
```

### `go/internal/llmroute/dispatch_test.go:12` — above `var _ DispatchResult`

```text
// TDD RED (cycle 435, task advisor-cli-fallback-chain / A1 + runner-dispatch-
// dedup / A2): Dispatch is the single home for the "walk the CLI chain,
// advance on a trigger exit, stop on success or a real failure" loop —
// extracted from runner.go's inline WS-G1 for-loop (runner.go:485-537) so the
// advisor (which today does a single un-fallback-able Launch,
// phase_advisor.go:248-288) and the runner consume ONE implementation
// ([[never_duplicate_centralize_via_design_patterns]]). These tests exercise
// Dispatch directly against a scripted launch closure — no bridge, no I/O —
// pinning the walk semantics independent of either caller.
//
// AC1 (primary exit=81 → falls back → succeeds) is exercised at this layer by
// TestDispatch_FallsBackOnTriggerExit; the advisor-level equivalent lives in
// core/phase_advisor_fallback_test.go (the advisor must actually WIRE this
// walk in — a green Dispatch alone doesn't prove the advisor calls it).
```

### `go/internal/llmroute/dispatch_test.go:27` — above `var _ DispatchResult`

```text
// apicover naming pin: every test above exercises DispatchResult's fields via
// a type-inferred `got := Dispatch(...)`, which never spells the type name in
// the test AST (apicover's naming check is a bare-identifier scan, not a type
// resolver — cycle-413/426/430 CI-break class). This explicit reference keeps
// the exported type named without changing any test's behavior.
```

### `go/internal/llmroute/dispatch_test.go:80` — above `func TestDispatch_FallsBackOnTriggerExit(t *testing.T) {`

```text
// TestDispatch_FallsBackOnTriggerExit (AC1): primary exits 81 (a trigger),
// fallback succeeds — Dispatch must advance to and return the fallback's
// result, having launched BOTH candidates in order. This is the exact
// cycle-435 live failure (router-launch-error.txt: agy-tmux exit 81) replayed
// at the Dispatch layer.
// TestDispatch_FallsBackOnTriggerExit covers EVERY code in the default trigger
// set, not one representative. The end-to-end proof (cmd/evolve
// TestE2ECLIFallbackChain) walks the whole spine and therefore costs ~10
// minutes per code; per-code trigger semantics belong here, where they cost
// microseconds. If a code is added to defaultFallbackOnExit this table grows
// automatically — it is derived from the package default, never re-typed.
```

### `go/internal/llmroute/llmroute.go:40` — above `var defaultFallbackOnExit = []int{80, 81, 85, 124, 127}`

```text
// defaultFallbackOnExit is the conservative trigger set covering all known
// CLI-side stall + missing-binary signals (mirror of bridge/exitcodes.go;
// kept as integer literals so this leaf package doesn't depend on bridge):
//
//   - 80  ExitREPLBootTimeout    (the *-tmux REPL never showed its prompt)
//   - 81  ExitArtifactTimeout    (bridge artifact-timeout; cycle-122 codex stall)
//   - 85  ExitUnknownPrompt      (pane stuck on an unhandled interactive prompt,
//     incl. provider rate-limit escalations — cycle-267: codex's usage quota
//     exhausted mid-batch, the rate_limit pattern escalated with 85, and the
//     codex→claude chain never fired because 85 wasn't a trigger; a
//     quota-blocked/stuck primary is exactly when a different CLI family can
//     serve. The escalation report is still written before the chain advances.)
//   - 124 coreutils timeout(1)   (defensive — if any wrapper uses `timeout`)
//   - 127 ExitMissingBinary      (the CLI binary isn't on PATH)
//
// Operators extend per-agent via profile.cli_fallback_on_exit (e.g. add 2
// ExitSafetyGate) or shrink to [80,127] for the production-strict posture. A
// CLI failure NOT in this list still hard-fails — a legitimate FAIL verdict
// never silently routes to a different CLI.
```

### `go/internal/llmroute/llmroute.go:182` — above `func defaultDriverForFamily(cli string) string {`

```text
// defaultDriverForFamily normalizes a bare CLI family (e.g. "codex") to its
// default interactive driver ("codex-tmux") when one is registered. Policy pins
// and `evolve setup apply` emit bare base families (Assignment.CLI is the base
// family), but the dispatch default is the tmux driver (CLAUDE.md: "Default
// execution = tmux-LLM drivers"). The headless "<family>" driver lacks the
// manifest model_tier_map and the codex ChatGPT model clamp, so a bare-family
// pin previously selected it and codex exited rc=1 every cycle (cycle-378). A
// name that is already driver-qualified, or whose "<family>-tmux" form is not a
// registered driver (e.g. the explicit headless "claude-p"), is returned
// unchanged. cliBinaryFor is the single source of registered driver names.
```

### `go/internal/llmroute/llmroute.go:263` — above `func ApplyUniversalFallback(p Plan, discovered []string, lookPath func(string) (string, error)) Plan {`

```text
// ApplyUniversalFallback appends the discovered CLIs (installed + authed on
// this host, already filtered by the profile's allowed_clis and the operator's
// universal_fallback_exclude) AFTER the configured chain, deduped against it.
// The configured chain keeps precedence — it runs first, in order — and the
// appended tail is the last resort every launch walks before a phase gives up.
//
// Until 2026-09-14 the tail was added only when EVERY configured CLI's binary
// was absent; a configured CLI that was present but walled (quota, a rejected
// model, a boot timeout) ended the walk with the phase and, at the last phase,
// the cycle. Operator policy since wave 2: try every available CLI before
// giving up. Empty discovered ⇒ untouched (fail-loud preserved: the classifier
// still sees a real ExitMissingBinary on an absent chain). lookPath is kept as
// the seam for callers that probe. Non-Candidates Plan fields are carried through.
```

### `go/internal/llmroute/llmroute.go:341` — above `func ApplyBench(p Plan, benched map[string]time.Time) Plan {`

```text
// ApplyBench demotes candidates whose family is benched (cycle-283: a walled
// codex re-burned its 5-15min boot on every dispatch) to the chain end,
// mirroring Probe's demote-not-drop reorder. benched maps family → BenchedAt.
// Bench is advice, never a veto: when EVERY candidate is benched the chain is
// instead ordered least-recently-benched first — the caller logs loudly and
// dispatch proceeds. Copy-struct convention carries non-Candidates fields.
```

### `go/internal/llmroute/llmroute_bench_test.go:3` — above `import (`

```text
// RED contract for ApplyBench (cycle-283): the dispatch chain must START at a
// healthy CLI when the primary's family is benched, mirroring Probe's
// demote-not-drop reorder. Bench is advice, never a veto: with every family
// benched, the chain runs least-recently-benched first.
```

### `go/internal/llmroute/llmroute_test.go:91` — above `want := []int{80, 81, 85, 124, 127}`

```text
// 85 (ExitUnknownPrompt, incl. provider rate-limit escalations) joined the
// defaults after cycle-267: a quota-blocked codex never chained to claude.
```

### `go/internal/llmroute/llmroute_test.go:331` — above `func TestResolve_PinBaseFamilyNormalizesToDefaultDriver(t *testing.T) {`

```text
// TestResolve_PinBaseFamilyNormalizesToDefaultDriver guards the cycle-378
// incident: a policy pin written with a BARE family ("codex", which is exactly
// what `evolve setup apply` emits — Assignment.CLI is the base family) selected
// the headless `codex` driver instead of the default tmux driver `codex-tmux`.
// At the time the headless driver had neither the manifest model_tier_map
// (since 2026-09-09 codex.json adopts the family table via model_tier_map_from)
// nor the ChatGPT-account model clamp (still true), so codex exited rc=1 every
// cycle and the loop spun. A base family that has a registered "<family>-tmux" driver MUST
// normalize to that default driver; an already-qualified or explicit-headless
// (e.g. "claude-p") name is left untouched.
```

### `go/internal/llmroute/model_routing_overlay.go:9` — above `type Overlay struct {`

```text
// Overlay is the cycle-440 MR4 SOFT dispatch adjustment: unlike a policy.Pin
// (ABSOLUTE — can collapse the chain to a single candidate), an Overlay only
// reorders the EXISTING chain. CLI (if non-empty) is promoted to primary but
// every prior candidate, including the old primary, survives — so a benched
// or failing overlay CLI still falls back via the ordinary cli-health chain
// (model_routing=auto "proposes", it never "pins"). Tier (if non-empty)
// replaces Plan.Model outright; concrete-model translation still happens
// later at bridge dispatch via the manifest's ModelTierMap. A zero-value
// Overlay is a noop.
```

### `go/internal/llmroute/model_routing_overlay.go:23` — above `func ApplySoftOverlay(in Plan, ov Overlay, prof *profiles.Profile) Plan {`

```text
// ApplySoftOverlay returns a NEW Plan with ov applied over in; in is never
// mutated.
//
// ov.CLI resolves in three rungs, and the order matters — the DECIDED
// semantics of the family/driver name ambiguity (overlay-family-name-
// transport-ambiguity): a BARE name (no hyphen) is a FAMILY selector, a
// hyphen-QUALIFIED name is a DRIVER selector, and an exact chain entry
// outranks both. (1) If the plan's chain ALREADY contains ov.CLI, that exact
// entry is promoted — the chain was resolved for this phase and its entries
// are concrete drivers the phase can actually run. (2) Otherwise a bare
// FAMILY name is satisfied by promoting the chain's existing same-family
// entry, whatever its transport — chain [claude-p …] + overlay "claude"
// stays on claude-p, never rewritten onto claude-tmux. (3) Only then is the
// name normalized like a pin primary (defaultDriverForFamily): a bare family
// the chain does not hold promotes to its default driver; a driver-qualified
// name passes through unchanged — an EXPLICIT "claude-tmux" against a chain
// of [claude-p] wins as written, because an explicit transport request must
// never be satisfied by promoting its opposite.
//
// Promoting-in-place is what keeps an overlay from crossing TRANSPORT. Found on
// CI macOS (PR #390): a headless phase with chain [claude-p codex] escalated its
// contract-blocked re-dispatch to "codex" and was sent to codex-TMUX, because
// the bare-family rule fired on a name the chain already held. The same string
// then resolved two ways inside one phase's own chain — the fallback ladder ran
// driver "codex", the escalation ran "codex-tmux" — which is a hard exit=10 on a
// host without tmux, and a silent transport change (different cost, cadence and
// quota behaviour) on a host with it.
```

### `go/internal/llmroute/model_routing_overlay_test.go:118` — above `func TestApplySoftOverlay_PromotesAnExistingCandidateWithoutRewritingItsTransport(t *testing.T) {`

```text
// TestApplySoftOverlay_PromotesAnExistingCandidateWithoutRewritingItsTransport
// is the regression pin for the contract-escalation transport crossing found on
// CI macOS (PR #390): a headless phase (chain [claude-p codex]) whose contract
// block escalated to "codex" was dispatched to codex-TMUX, because the overlay
// normalized the bare family to its default driver even though the chain already
// held a concrete, correct entry for it. On a host without tmux that is a hard
// exit=10 cycle failure; on a host WITH tmux it silently moves a headless-
// configured phase onto a different transport with different cost and quota
// behaviour. The same string resolved two ways in one phase's own chain: the
// fallback ladder ran driver "codex", the escalation ran "codex-tmux".
//
// Rule: an overlay naming something the chain ALREADY contains promotes that
// exact entry; only a CLI the chain lacks is normalized to its family default.
```

### `go/internal/llmroute/overlay_family_transport_test.go:3` — above `import "testing"`

```text
// overlay_family_transport_test.go — RED contract for the family-name
// transport ambiguity (inbox overlay-family-name-transport-ambiguity 0.87;
// hotter since #430 made escalation overlays live-fire). PR #390's
// exact-chain-match rung closed the observed instance; a bare FAMILY the
// chain holds only under a NON-default driver still crossed transport:
// chain [claude-p codex] + overlay "claude" found no exact match, fell to
// defaultDriverForFamily("claude") = claude-tmux, and moved a
// headless-configured phase onto tmux. The decided semantics (documented in
// ApplySoftOverlay): a BARE name (no hyphen) is a FAMILY selector satisfied
// by promoting the chain's existing same-family entry (transport preserved);
// a hyphen-QUALIFIED name is a DRIVER selector that wins even over a
// same-family chain entry (an explicit transport request is never rewritten).
```

### `go/internal/llmroute/overlay_family_transport_test.go:62` — above `in := Plan{Candidates: []string{"claude-p", "codex"}}`

```text
// "codex" is both a family name and a registered driver; the #390 rung
// (exact chain entry) outranks everything.
```

### `go/internal/llmroute/tier_fallback.go:16` — above `const universalTierFloorMin = "balanced"`

```text
// universalTierFloorMin is the lowest tier DispatchTiered will ever step down
// to when the phase's ModelTierEnvelope.Min is empty or unclassifiable —
// mirror of the router's cycle-480 universalTierFloor{Min:"balanced"}.
```

### `go/internal/llmroute/universal_fallback_test.go:41` — above `func TestApplyUniversalFallback_AConfiguredCLIAvailable_AppendsTheRestAsLastResort(t *testing.T) {`

```text
// TestApplyUniversalFallback_AConfiguredCLIAvailable_AppendsTheRestAsLastResort
// — operator policy (2026-09-14, wave 2): a phase must try EVERY available CLI
// before it gives up, because one CLI's quota wall must never fail a cycle at
// its last phase. The configured chain keeps precedence (it runs first, in
// order); the discovered CLIs the profile allows are appended after it even
// when a configured CLI is present — that tail is what the walk reaches when
// the configured chain is present but walled.
```
