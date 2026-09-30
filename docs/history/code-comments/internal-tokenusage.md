# Comment history: `internal/tokenusage`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/tokenusage/apicover_named_test.go:27` — above `func TestResultPeakPromptTokensNamed(t *testing.T) {`

```text
// TestResultPeakPromptTokensNamed pins Result.PeakPromptTokens (cycle-1455) and
// the rule that separates it from Usage: it is ONE turn's prompt-side
// occupancy, so it must not move when a second turn adds to the summed spend.
```

### `go/internal/tokenusage/defaultresolver.go:3` — above `import (`

```text
// defaultresolver.go — the single shared resolver both production
// composition roots (internal/adapters/bridge.Adapter and internal/subagent's
// defaultExecAdapter) wire into their gobridge.Deps.TokenResolver field. See
// defaultresolver_test.go for the RED contract this satisfies.
//
// Cycle-779 (token-telemetry-input-cache-fidelity): the chain is dispatched
// per driver. Claude drivers get the full fidelity chain (their transcript
// JSONL is the format ScanConfigRoot parses); every other driver — agy,
// codex, and anything unknown — fails OPEN onto the driver-agnostic tiers
// (events-log result envelope, pane scrollback). A resolve where no tier has
// data carries an explicit per-driver Warn so an uncovered driver surfaces as
// "unmeasured", never as a silent zero recorded as if covered.
```

### `go/internal/tokenusage/defaultresolver.go:31` — above `r.FillPct = FillPctUnmeasured`

```text
// An uncovered launch has zero prompt tokens because nothing
// observed it, not because its context was empty (cycle-1444).
```

### `go/internal/tokenusage/defaultresolver_test.go:3` — above `import (`

```text
// defaultresolver_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96;
// scout-report.md hypothesis 1: "Wiring tokenusage.Chain(...) into both
// composition roots via one shared tokenusage.DefaultResolver(configRoot)
// helper").
//
// DefaultResolver is the SINGLE shared helper both production composition
// roots (internal/adapters/bridge.Adapter and internal/subagent's
// defaultExecAdapter) must call to build their Deps.TokenResolver — the fix
// for the confirmed bug (grep: 0 non-test hits for TokenResolver in either
// composition root) that has made token telemetry silently all-zero since at
// least cycle 612. DefaultResolver is undefined today, so this package fails
// to compile — the intended RED signal. Builder implements:
//
//	func DefaultResolver(configRoot string) func(Window) (Result, error) {
//	    return func(w Window) (Result, error) {
//	        return Chain(TranscriptCollector(configRoot, w)), nil
//	    }
//	}
//
// (S4/S5 tiers — EventsResultCollector, ScrollbackPeakCollector — are out of
// scope: Window carries no logPath/pane, only Worktree/ArtifactPath/Start/
// End, so only the transcript tier is derivable generically from a Window
// alone. See test-report.md Coverage Map for this scope decision.)
```

### `go/internal/tokenusage/driver_coverage_test.go:3` — above `import (`

```text
// driver_coverage_test.go — cycle-779 AC2 contract (named by ACS predicates
// C779_003/C779_004): per-driver fail-open extraction with an explicit
// uncovered/WARN signal. The 2026-07-13 baseline recorded agy/codex-driven
// launches as zero-usage-as-if-covered; these tests pin the fix — a driver
// whose sources carry no usage yields Source==SourceNone WITH a per-driver
// Warn (unmeasured, not free), a driver whose driver-agnostic tiers do carry
// data is extracted normally, and an unknown driver never errors a launch.
```

### `go/internal/tokenusage/fallbackchain_test.go:3` — above `import (`

```text
// fallbackchain_test.go — RED contract for cycle-754 task
// token-resolver-production-wiring (inbox id token-resolver-production-wiring,
// weight 0.96; scout-report.md Task 1 `token-resolver-fallback-chain`).
//
// Confirmed live gap: 124 llm-calls.ndjson files under .evolve/runs/ all show
// "source":"none" for tmux-driven launches. DefaultResolver chains ONLY the
// transcript tier (defaultresolver.go:14) even though EventsResultCollector and
// ScrollbackPeakCollector are fully implemented in chain.go — the S2 fallback
// chain was built but never connected end-to-end.
//
// This contract extends Window with the context the lower tiers need:
//
//	EventsLogPath string // path to the launch's *-events.ndjson (tier 2 input)
//	Scrollback    string // captured pane scrollback CONTENT (tier 3 input —
//	                     // ScrollbackPeakCollector takes content, not a pane id)
//
// and requires DefaultResolver(configRoot) to chain, in fidelity order:
//
//	TranscriptCollector(configRoot, w) > EventsResultCollector(w.EventsLogPath)
//	  > ScrollbackPeakCollector(w.Scrollback)
//
// Window.EventsLogPath / Window.Scrollback are undefined today, so package
// tokenusage fails to compile — the intended RED signal (the same strategy
// scanner_test.go's S1 and chain_test.go's S2 contracts used). Builder makes
// these compile AND pass; DO NOT modify these tests.
//
// Reuses same-package helpers: writeFile (chain_test.go), mustParse /
// launchWindowStart / launchWindowEnd (scanner_test.go).
```

### `go/internal/tokenusage/fillpct.go:3` — above `import (`

```text
// fillpct.go — context-fill telemetry (cycle-1444, task
// `context-fill-telemetry-record`). Fill% is a DERIVED reading off the usage
// the resolver already recovered — prompt-side tokens ÷ the driver family's
// effective window — never a second independent measurement path.
//
// Two invariants carry the whole file: an unmeasurable reading is an explicit
// negative sentinel (never 0%, never Inf/NaN, so a comparison downstream can
// never silently treat "we don't know" as "empty context"), and an unmapped CLI
// family reports window 0 rather than a guessed window (publishing a fabricated
// fill reading is worse than publishing none).
```

### `go/internal/tokenusage/fillpct.go:44` — above `func PromptTokens(u cyclestate.TokenUsage) int {`

```text
// PromptTokens returns the input-side token count that occupies the context
// window: fresh input plus both cache halves. Generated Output is excluded —
// it does not sit in the prompt, and counting it would make the fill WARN fire
// on long answers instead of on big prompts.
//
// The three counters are driver-controlled (they arrive as JSON off a CLI's own
// usage report), so the sum is guarded rather than trusted: any negative
// counter, or an addition that would wrap, returns a negative total. Wrapping
// silently is the worse failure — it publishes a fabricated percentage that
// FillWarn's "any negative is unmeasured" rule then swallows, so the launch
// whose telemetry is bogus is exactly the one that raises no warning
// (cycle-1444 audit M1).
```

### `go/internal/tokenusage/fillpct.go:67` — above `func windowOccupancy(r Result) int {`

```text
// windowOccupancy returns the prompt-side token count to measure a Result
// against its window: the fullest single observed turn when the tier broke the
// launch down per turn (transcript), otherwise that tier's whole-launch
// prompt-side total. The distinction is the cycle-1455 defect: a per-turn tier's
// summed Usage counts the same accumulated context once per turn and reads as
// multiples of 100% (566.9% observed live), while the events/scrollback tiers
// report ONE result envelope, whose total already is a single reading.
//
// A negative peak (turns were expected, none was observed) is passed through
// rather than repaired, so FillPct's negative guard turns it into the documented
// sentinel — nothing observed the context is not the same as the context being
// empty.
```

### `go/internal/tokenusage/fillpct_contributors_test.go:3` — above `import (`

```text
// fillpct_contributors_test.go — RED contract for cycle-1482 task
// `context-fill-warning-attribution` (scout-report.md Task 2).
//
// FillWarn today reports only a phase and a percentage — no contributor
// breakdown at all. The cycle-1458 audit (M1) flagged the failure mode a
// contributor breakdown WOULD fall into if it were ever added carelessly: a
// breakdown built off the whole-launch SUMMED usage would disagree with a
// percentage that fillpct.go's own windowOccupancy derives from a single PEAK
// turn (cycle-1455) — an operator would see a 75% reading annotated with
// components that total 1500% of the window.
//
// FillWarnWithContributors is undefined today, so this file fails to COMPILE
// until Builder adds it — the RED signal. The contract: whatever basis the
// caller hands in as `contributors` is what the message must show, verbatim
// and attributably, alongside the SAME phase/threshold/sentinel semantics
// FillWarn already promises. Basis SELECTION (peak turn vs whole-launch total)
// is the caller's job — proven by the internal/bridge wiring test in the sister
// task's RED contract — not this function's.
```

### `go/internal/tokenusage/fillpct_driverwindow_test.go:3` — above `import (`

```text
// fillpct_driverwindow_test.go — RED contract for cycle-1482 task
// `context-fill-driver-window-coverage`.
//
// Today EffectiveWindow maps exactly ONE family (claude); every other driver —
// including the two whose real advertised windows are documented — returns 0 and
// degrades its fill reading to the sentinel. That is safe but blind: a codex or
// agy lane can run at 90% occupancy and the operator sees "unmeasured" forever,
// which is precisely the measurement gap the inbox item
// `context-fill-telemetry-and-cap` was filed to close ("a per-family constant,
// conservative — e.g. 200K for 1M-advertised").
//
// The contract has TWO halves and both must hold together:
//   - families with a documented advertised window get a CONSERVATIVE mapped
//     window (strictly below any advertised maximum, capped here at 400_000), and
//   - every family whose real window nobody has measured — ollama (the served
//     model, not the CLI, owns the window), an unknown CLI, whitespace, or a
//     name that merely LOOKS adjacent to a mapped family — stays at 0, because a
//     fabricated fill reading is worse than no reading.
```

### `go/internal/tokenusage/fillpct_multiturn_test.go:3` — above `import (`

```text
// fillpct_multiturn_test.go — RED contract for cycle-1455 task
// `contextfill-ratio-over-100pct` (inbox 2026-08-12, weight 0.75,
// pipeline-repair; observed twice in one monitored wave: scout 566.9%, triage
// 114.3%).
//
// The defect, in one line: ScanConfigRoot sums EVERY assistant turn's
// Input+CacheRead+CacheWrite into one grand total (scanner.go:147-153), and
// DefaultResolver feeds that summed total straight into FillPct against a
// SINGLE-turn 200K window (defaultresolver.go:38). Each turn's own
// cache_read_input_tokens already carries that turn's entire prior context, so
// summing turn N with turn N+1 re-counts the same context once per turn. A
// 12-turn phase near the ceiling therefore reports several hundred percent.
//
// The contract is BEHAVIOURAL and implementation-agnostic — it never names a
// new symbol. Every fixture below grows monotonically (real transcripts do:
// context only accumulates within a phase), so the terminal turn IS the peak
// turn and either extraction satisfies these tests. What the fixtures DO rule
// out is the sum, the first turn, and the mean.
//
// The two invariants the fix must not break: Result.Usage stays the SUM (it is
// the cost/spend number, and correct as-is), and an honest over-100% reading
// stays unclamped and legible (fillpct.go's own documented promise).
```

### `go/internal/tokenusage/fillpct_test.go:3` — above `import (`

```text
// fillpct_test.go — RED contract for cycle-1444 task `context-fill-telemetry-record`.
//
// RED: fillpct.go does not exist yet. PromptTokens / EffectiveWindow / FillPct /
// FillPctUnmeasured / FillWarn and the Result.FillPct field are all undefined, so
// this file fails to COMPILE until Builder adds them (compile-fail = RED evidence).
//
// The contract, in one line: context fill is a DERIVED reading off the usage the
// existing scanner already recovers (prompt-side tokens ÷ the driver family's
// effective window) — never a second independent measurement path — and an
// unmeasurable reading is an explicit sentinel, never 0%, never Inf/NaN.
```

### `go/internal/tokenusage/fillpct_test.go:24` — above `const claudeWindow = 200_000`

```text
// claudeWindow is the conservative per-family effective window the research
// update pins for the claude family (200K — deliberately below any advertised
// 1M, per the 2026-08-03 reliability finding embedded in the inbox item).
```

### `go/internal/tokenusage/fillpct_test.go:188` — above `func TestFillTelemetry_PromptTokenOverflowIsUnmeasured(t *testing.T) {`

```text
// TestFillTelemetry_PromptTokenOverflowIsUnmeasured is the RED contract for
// cycle-1446 task `contextfill-promptTokens-overflow-guard` (cycle-1444 audit
// finding M1). The three counters are driver-controlled: each value below is
// individually a valid `int` that encoding/json will happily land in the
// TokenUsage fields, but the SUM wraps negative. Today PromptTokens is plain
// unguarded addition, so FillPct returns a fabricated negative percentage that
// is neither a real reading nor the documented sentinel — FillWarn then treats
// it as unmeasured and stays silent on exactly the launch whose telemetry is
// bogus, while the bogus number is still persisted to llm-calls.ndjson.
//
// The contract is behavioural and implementation-agnostic: whatever the guard
// looks like, the full PromptTokens→FillPct path must yield FillPctUnmeasured
// for a wrapped or otherwise negative prompt-side total.
```

### `go/internal/tokenusage/inputcache_fidelity_test.go:3` — above `import (`

```text
// inputcache_fidelity_test.go — cycle-779 TDD contract for the
// token-telemetry-input-cache-fidelity task (inbox weight 0.96,
// operator-boosted 2026-07-13). The 2026-07-13 live baseline showed
// input=0/cache_read=0/cache_write=0 across every phase: only OUTPUT tokens
// survive to `evolve tokens report`, hiding the dominant cost dimension
// (input outweighs output 2:1–100:1 per
// knowledge-base/research/token-optimization-2026).
//
// This file pins the AC1 claude-transcript half of the fix: usage blocks in
// the claude CLI transcript JSONL carry input_tokens /
// cache_read_input_tokens / cache_creation_input_tokens, and the scanner must
// surface ALL of them — not just output — with cache-dominated magnitudes
// (the realistic shape: cache_read >> input).
//
// The per-driver coverage half (agy/codex fail-open + WARN counters) needs a
// new seam (Window carries no driver today) and is bound by name in
// go/acs/cycle779/predicates_test.go: TestScanner_PerDriverCoverageWarnsNotZeros,
// TestScanner_UnknownDriverFailsOpenNoError (Builder authors test+seam; the
// ACS predicates stay RED until they exist and pass).
```

### `go/internal/tokenusage/inputcache_fidelity_test.go:30` — above `func TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks(t *testing.T) {`

```text
// TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks: a realistic
// cache-dominated claude transcript (cache_read ~25x input, input ~2x output)
// must yield non-zero Input/CacheRead/CacheWrite — the exact fields the
// 2026-07-13 baseline reported as all-zero. An output-only extraction fails
// three of the four field assertions.
```

### `go/internal/tokenusage/scanner.go:63` — above `type Result struct {`

```text
// Result is the outcome of a scan: the summed token usage and the Source that
// produced it. Warn carries an explicit per-driver coverage warning when no
// tier could observe the launch's usage (Source == SourceNone) — the signal
// that distinguishes "unmeasured" from "measured zero" so uncovered drivers
// never masquerade as free (the 2026-07-13 all-zeros baseline defect).
// FillPct is the derived context-fill reading for the launch (percent of the
// driver family's effective window occupied by prompt-side tokens), stamped by
// DefaultResolver off the usage that same resolve recovered. It carries
// FillPctUnmeasured when the fill could not be derived — an uncovered launch
// or an unmapped driver family — so "unmeasured" never reads as "0% full".
// PeakPromptTokens is the fullest any SINGLE observed turn's prompt side got —
// the numerator the fill reading is measured against, and deliberately not the
// summed Usage: each turn's cache_read already carries that turn's whole prior
// context, so summing turns re-counts the same context once per turn (a 12-turn
// phase then reports several hundred percent — cycle-1455). Zero means the tier
// observed no per-turn breakdown (events/scrollback report one whole-launch
// envelope, which is already a single reading); negative means turns were
// expected but none was observed, which degrades the fill to the sentinel.
// PeakUsage carries that same turn's component counters so a fill warning can
// name contributors without mixing a single-turn percentage with launch totals.
```

### `go/internal/tokenusage/scanner.go:212` — above `var artifactAnchors = []string{`

```text
// artifactAnchors are every literal label a dispatched prompt puts immediately
// ahead of the launch's deliverable path. Attribution keys on anchor+path rather
// than on a bare path substring so that a transcript which merely CITES another
// launch's artifact in prose — e.g. the retrospective profile's "Read
// .evolve/runs/cycle-{cycle}/build-report.md" — is not billed to that launch's
// Window.
//
// All three disclosure forms must stay listed: the subagent assemblers stamp
// artifactMarker, while the bridge dispatch path every loop phase takes stamps
// the contract footer (phasecontract.FooterMarker, see render.go:86) and the
// contract tail's <artifact-path> element (render.go:117) — a real cycle-1457
// build prompt carries the footer form and no artifactMarker at all. Dropping a
// form here does not narrow attribution loudly; it silently degrades those
// launches to the scrollback tier (input:0, cache_read:0), the cycle-867 defect
// this scanner exists to close.
```

### `go/internal/tokenusage/scanner_artifactpath_test.go:3` — above `import (`

```text
// scanner_artifactpath_test.go — RED contract for the production token-telemetry
// attribution defect (2026-07-17). On the default tmux-LLM driver path a claude
// phase launch's transcript was NEVER attributed: attributes() hard-gated on an
// exact cwd == Window.Worktree match, but Worktree is lossy across the exec
// boundary (WORKTREE_PATH → ProjectRoot fallback ≠ the transcript's recorded
// worktree cwd). So the one tier carrying input/cache_read tokens (the
// transcript) fell through to scrollback_peak → input:0, cache_read:0, hiding
// the ~173K-token/turn context-window cost. The launch-unique ArtifactPath the
// general bridge Window already carries appears verbatim in exactly that
// launch's first user message, so it is the reliable attribution key.
```

### `go/internal/tokenusage/scanner_artifactpath_test.go:21` — above `func TestTranscriptScan_AttributesByArtifactPath_WhenCwdMispropagated(t *testing.T) {`

```text
// TestTranscriptScan_AttributesByArtifactPath_WhenCwdMispropagated reproduces
// the cycle-867 production failure: the Window.Worktree the resolver received
// (the repo root, from the WORKTREE_PATH→ProjectRoot fallback) does NOT equal
// the transcript's recorded cwd (the real cycle worktree, and its /go subdir).
// The launch-unique ArtifactPath in the first user message must attribute the
// transcript anyway, recovering the real input + cache_read the exact-cwd gate
// discarded.
```

### `go/internal/tokenusage/scanner_artifactpath_test.go:87` — above `func TestTranscriptScan_AttributesByArtifactPath_StringContent(t *testing.T) {`

```text
// TestTranscriptScan_AttributesByArtifactPath_StringContent — the first user
// message's content is a BARE JSON STRING (the common Claude Code transcript
// form for the phase prompt), not an array of {type,text} blocks. firstUserText
// must decode the string form; otherwise the ArtifactPath key never matches and
// attribution silently fails. This is the production blind spot that made the
// prior ArtifactPath-primary fix (c41fa94b) inert: unit fixtures used the block
// form, real transcripts use the string form, so cache_read read as zero.
```

### `go/internal/tokenusage/scanner_marker_test.go:3` — above `import (`

```text
// scanner_marker_test.go — regression contract for the over-attribution vector
// closed in cycle-1457. attributes() used to key on a BARE ArtifactPath
// substring anywhere in the first user message. Every production launch carries
// the path, but so does any prompt that merely cites it in prose:
// .evolve/profiles/retrospective.json instructs a retrospective launch to "Read
// .evolve/runs/cycle-{cycle}/build-report.md", which under the bare rule billed
// the whole retrospective launch to the BUILDER's Window. Both assemblers stamp
// a literal label — subagent.go:358 ("Artifact path: %s\n") and run.go:442
// ("- Artifact path: %s\n") — so the match anchors on artifactMarker+path.
```

### `go/internal/tokenusage/scanner_marker_test.go:58` — above `name:          "bridge contract footer form attributes",`

```text
// The shape a REAL loop-phase prompt carries: the bridge's contract
// footer, not the subagent assembler's marker. Verified against the
// cycle-1457 build prompt itself, whose only path disclosure is
// "DELIVERABLE PATH: <abs>" (phasecontract/render.go:86) — an anchor
// set that omitted this would silently zero every loop launch's
// transcript-tier token telemetry.
```
