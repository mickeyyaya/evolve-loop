# Comment history: `internal/bridge/panestream`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/bridge/panestream/agy_liveness_amplify_test.go:8` — above `const (`

```text
// agy_liveness_amplify_test.go — adversarial amplification for AgyDetector (cycle-425).
// Written by the Test Amplifier — black-box view, spec-only, no implementation reading.
//
// Coverage gaps targeted beyond C425_004–C425_006 and the ACS predicates:
//
//	A1  Prime call on generating frame returns base behavior (not Converging uplift)
//	A2  Partial spinner text ("⣯ Generat") is NOT a convergence signal
//	A3  Spinner embedded mid-line (not standalone) is NOT detected
//	A4  "esc to cancel" alone (no spinner) is NOT a convergence signal
//	A5  Spinner→answer→spinner oscillation: Converging correctly re-fires on spinner return
//	A6  Multiple consecutive generating frames each return Converging (signal persists)
//	A7  Malformed edge cases: no panic, confidence never meaningfully above DefaultDetector
//	A8  AgyDetector with non-agy profile: no panic, graceful fallback
//	A9  Spinner overrides high stallThreshold — Converging is independent of stall config
//	A10 Confidence strictly > DefaultDetector on generating frame (direct construction path)
//	A11 Extended answer-frame parity across 10+ iterations (long-run composition stability)
```

### `go/internal/bridge/panestream/classify.go:8` — above `type Layer int`

```text
// classify.go — the SINGLE channel separator for a tmux pane (ADR-0047). A pane
// co-mingles two logically distinct channels with no structural delimiter: the
// agent's transcript CONTENT and the CLI's volatile CHROME. Every pane detector
// — PaneBusy (liveness), cleanPane/PaneHasSubstantiveChange (progress),
// trimVolatileTail (delta extraction) — is a PROJECTION of ClassifyLine, so the
// chrome vocabulary can never again diverge between detectors. Before this, the
// claude `· Schlepping… (50s · ↑ 3.1k tokens)` line was CHROME to PaneBusy but
// CONTENT to cleanPane — the divergence that let a busy agent's ticking clock
// read as "progress" and a stalled one's read as "stuck" (the 8-instance
// content-vs-chrome disease, ADR-0047).
```

### `go/internal/bridge/panestream/classify.go:42` — above `chromeAsciiSpinnerRE = regexp.MustCompile('^[-/|\\]\s*$')`

```text
// chromeAsciiSpinnerRE matches a line that is ONLY an ascii spinner frame
// (`-` `\` `|` `/` alone, optional trailing space) — NOT a markdown bullet
// "- real text", which is agent CONTENT. The earlier `^[-/|\\]\s` prefix
// match trimmed every bullet as chrome (ADR-0047: a closed structural shape,
// never an open prefix into the content channel).
```

### `go/internal/bridge/panestream/classify_test.go:5` — above `func TestClassifyLine_Layers(t *testing.T) {`

```text
// TestClassifyLine_Layers pins the channel each representative pane line belongs
// to. The corpus seed is the REAL cycle-312 claude frame (`· Schlepping… (50s ·
// ↑ 3.1k tokens)`) that killed two soak cycles: it is the live-turn affordance —
// busy, but NOT progress.
```

### `go/internal/bridge/panestream/classify_test.go:41` — above `func TestClassifyLine_DetectorsAgree(t *testing.T) {`

```text
// TestClassifyLine_DetectorsAgree is the load-bearing anti-divergence property
// (ADR-0047): the SAME spinner-stats line must read consistently across all
// three pane detectors — busy (liveness), NOT content (progress), and volatile
// (trim). Before the single classifier, `· Schlepping…` was busy to PaneBusy but
// content to cleanPane, so a working agent's ticking clock counted as progress
// and a stalled one's didn't — the exact bridge-busy / bench-evidence bug class.
```

### `go/internal/bridge/panestream/liveness.go:9` — above `type LivenessState int`

```text
// liveness.go — Strategy-based liveness detection over a sequence of rendered
// pane snapshots (ADR-0047 projection seam). All detectors are projections of
// ClassifyLine / PaneDelta — the single channel separator — so chrome/affordance
// vocabulary cannot drift between liveness and progress paths.
//
// LivenessProbe is the per-run interface. DefaultDetector uses stable-content
// growth velocity (new content lines / interval via PaneDelta), closing the
// codex weak-signal gap (PaneBusy=false on codex → Idle or Converging from
// growth alone). ClaudeDetector composes over DefaultDetector, layering a
// monotonic ↓ token counter as higher-confidence Converging for claude.
//
// Registry: DetectorFor(profile) is co-located with Profiles (ADR-0047
// single-source-with-projection); the only per-CLI branch is here, never in
// the reviewer (stopreview.go).
```

### `go/internal/bridge/panestream/liveness.go:169` — above `func ExtractResponseTokens(pane string) int {`

```text
// ExtractResponseTokens returns the peak ↓ response-token count from a rendered
// pane. It is the single-source token extractor for the signal-center campaign
// (S1, ADR-0047): both ClaudeDetector and the stopreview callsite use this
// instead of maintaining separate per-package extractors.
```

### `go/internal/bridge/panestream/liveness.go:349` — above `func DetectorFor(p PaneProfile) LivenessProbe {`

```text
// DetectorFor returns a new LivenessProbe for the given pane profile.
// Co-located with Profiles (ADR-0047 single-source-with-projection): the
// per-CLI strategy selection lives here, never in the reviewer.
// claude routes to ClaudeDetector; ollama routes to OllamaDetector;
// agy routes to AgyDetector; all others receive DefaultDetector.
```

### `go/internal/bridge/panestream/liveness.go:367` — above `func (s LivenessState) String() string {`

```text
// String is the one spelling of a liveness state — the word pane.liveness
// signals carry in fields.state (ADR-0101 S3).
```

### `go/internal/bridge/panestream/liveness_amplify_test.go:9` — above `func TestAmp_DefaultDetector_StallCounterResetsOnContent(t *testing.T) {`

```text
// Adversarial amplification tests for the LivenessDetector strategy layer (cycle-423).
// Written by the Test Amplifier — black-box view, spec only, no implementation reading.
//
// Coverage gaps targeted (not reached by ACS predicates C423_001–C423_014):
//   A1  Stall counter RESET on content growth (AC5 only tests monotonic accumulation)
//   A2  Confidence ∈ [0,1] for ALL state paths (AC6 only checks Converging via DetectorFor)
//   A3  stallThreshold=1 boundary (AC5 uses threshold=2 and tests N-1/N)
//   A4  ClaudeDetector: DECREASING token → falls back to default (AC11=increasing, AC12=static)
//   A5  ClaudeDetector: very large token value — no overflow, valid confidence
//   A6  DetectorFor: unknown/empty profile — no panic, returns valid probe
//   A7  Many quiet (Idle) frames: never reaches Hung (stall counter must not increment on Idle)
//   A8  Upload-direction (↑) token counter: not a Converging signal for ClaudeDetector
```

### `go/internal/bridge/panestream/liveness_amplify_test.go:220` — above `func TestAmp_ClaudeDetector_UploadArrowNotConvergenceSignal(t *testing.T) {`

```text
// TestAmp_ClaudeDetector_UploadArrowNotConvergenceSignal verifies that a token
// counter using the ↑ (upload/prompt) direction does NOT elevate the ClaudeDetector
// above the default. ADR-0047 classifies BOTH ↑ and ↓ as chrome affordance;
// only ↓ (response generation) is a download-growth signal. A monotonically
// increasing ↑ counter must not be misread as response generation evidence.
```

### `go/internal/bridge/panestream/liveness_c429_amplify_test.go:10` — above `func TestExtractResponseTokens_Concurrent(t *testing.T) {`

```text
// Adversarial amplification tests for ExtractResponseTokens (cycle-429).
// Written by Test Amplifier — black-box, spec only: no implementation reading.
//
// Contract under test:
//   ExtractResponseTokens(pane string) int
//   - k-form:  "↓ 5.2k tokens" → 5200
//   - plain:   "↓ 200 tokens"  → 200
//   - peak:    returns max across all matches in pane
//   - invalid: returns 0
```

### `go/internal/bridge/panestream/liveness_conformance_test.go:5` — above `var (`

```text
// liveness_conformance_test.go — names the LivenessProbe Strategy abstraction by
// IDENTIFIER for every per-CLI detector (cycles 423-425, ADR-0047). The prior
// behavioral tests only referenced the New* constructors, leaving the
// LivenessProbe interface and the four detector types unnamed in any test AST;
// apicover -enforce (Phase 5) flagged them as "no test names it". This test
// names them through the interface AND pins the contract they share.
```

### `go/internal/bridge/panestream/liveness_state_string_test.go:5` — above `func TestLivenessState_StringNamesEveryState(t *testing.T) {`

```text
// LivenessState renders as the word the Signal Center's pane.liveness event
// carries in fields.state (ADR-0101 S3): one spelling, owned by the vocabulary.
```

### `go/internal/bridge/panestream/liveness_test.go:242` — above `func TestExtractResponseTokens(t *testing.T) {`

```text
// TestExtractResponseTokens pins the exported ExtractResponseTokens contract:
// the single-source token extractor (S1, cycle-429). Named so apicover -enforce
// can locate the exported symbol. Table covers k-form, plain-integer (superset
// behavior vs. old stopreview k-only extractor), peak-across-matches, malformed
// inputs, and empty — the adversarial negative (malformed→0) prevents a no-op
// that returns a constant from passing the k-form cases.
```

### `go/internal/bridge/panestream/livenesscenter.go:5` — above `type LivenessCenter struct {`

```text
// LivenessCenter is a Facade that owns per-session liveness signal state
// (ADR-0068). It aggregates all active session signals into one LivenessState,
// and exposes a handler-registration API so that adding a CLI = register a
// strategy + add a profile entry (OCP — no switch edits required).
//
// Concurrency model (S5, measured — see ADR-0068 "Consequences"): a global
// RWMutex (mu) guards ONLY the sessions/registry map STRUCTURE (insert,
// lookup, RegisterHandler); it is never held across the stateful, per-CLI
// probe.Assess() call. Each sessionSignals carries its OWN sync.Mutex guarding
// its probe/last/busy/clean/changed fields — this is what lets independent
// sessions' Observe calls run without funneling through one lock.
//
// Lock ordering (invariant, enforced by code shape, not just convention): the
// global structural lock is always acquired AND FULLY RELEASED before any
// per-session lock is acquired. No code path holds both locks at once, and no
// code path acquires a per-session lock before the global lock.
//
// Aggregation rule: any Exhausted session ⇒ Exhausted (a quota/rate-limit wall
// dominates — the artifact will never come); else Converging if any session is
// Converging; else Hung; else BusyButStagnant; else Idle; else 0 (empty center).
// The rule is documented and testable, not implicit.
```

### `go/internal/bridge/panestream/livenesscenter.go:103` — above `func (sc *LivenessCenter) Observe(sessionKey, rendered string, profile PaneProfile) {`

```text
// Observe records one liveness observation for sessionKey. On the first call
// for a key, the probe is created via the registry (if profile.Name is
// registered) or DetectorFor (fallback). Subsequent calls reuse the same
// stateful probe. Observe is safe for concurrent use.
//
// S5: the global lock (sc.mu) is held ONLY for the map lookup/insert below —
// never across probe.Assess(), which runs under the session's OWN lock
// (ss.mu). This is what lets Observe calls on distinct session keys proceed
// without serializing on one process-global mutex (ADR-0068, measured).
```

### `go/internal/bridge/panestream/livenesscenter.go:198` — above `func (sc *LivenessCenter) BusyOf(rendered string, profile PaneProfile) bool {`

```text
// BusyOf reports the live-turn busy affordance for rendered/profile directly,
// with NO session key and NO Observe call (cycle-434 S4 completion). It
// delegates to the SAME standalone PaneBusy definition Observe folds into
// Busy(sessionKey), so it can never drift from that projection, and it never
// touches sc.sessions — repeated calls create no session entry and leave
// Aggregate/Busy/Changed at their unobserved defaults. This is what lets
// callers that fire at different loop points than the checkpoint's own
// Observe (autorespond.go's tick busy-gate, driver_tmux_repl.go's idle_reached
// bracket) read the same busy signal without polluting the checkpoint's
// Observe/Aggregate baseline.
//
// Safe on a nil *LivenessCenter — it reads no receiver state — so a caller
// holding an optional (possibly-nil) center reference never needs a nil guard.
```

### `go/internal/bridge/panestream/livenesscenter_amplify_test.go:10` — above `type fixedProbe struct {`

```text
// livenesscenter_amplify_test.go — Adversarial amplification for LivenessCenter (cycle 430).
// Written by the Test Amplifier — black-box view, spec + ADR-0068 only.
// No implementation files read; fixedProbe injects controlled LivenessState values.
//
// Coverage gaps targeted (not reached by livenesscenter_test.go AC1–AC12):
//   AMP1  Aggregation priority: Hung > BusyButStagnant (two sessions, different states)
//   AMP2  Aggregation priority: Converging > Hung (two sessions)
//   AMP3  Aggregation priority: Converging wins over all four states simultaneously
//   AMP4  Factory call frequency: called exactly once per session key (not per-Observe)
//   AMP5  Session isolation: two keys drive independent probe state machines
//   AMP6  Empty session key: no panic on Observe or Aggregate
//   AMP7  Empty rendered string: no panic (boundary for regex/string-scan detectors)
//   AMP8  Large N sessions (200): no panic, valid aggregate
//   AMP9  Multiple concurrent readers: -race clean (existing test has only 1 reader)
//   AMP10 Single observation per session: Aggregate doesn't panic (half-primed probe)
//   AMP11 RegisterHandler overrides built-in "claude" profile (registry-first OCP seam)
//   AMP12 All-Idle sessions: Aggregate returns LivenessIdle, not zero
```

### `go/internal/bridge/panestream/livenesscenter_amplify_test.go:39` — above `func TestAmp_SignalCenter_AggregationPriority_HungBeatsBusyStagnant(t *testing.T) {`

```text
// TestAmp_SignalCenter_AggregationPriority_HungBeatsBusyStagnant (AMP1):
// When one session is Hung and another is BusyButStagnant, Aggregate must return
// LivenessHung. Validates priority level 2 > level 3 from ADR-0068 aggregation rule.
```

### `go/internal/bridge/panestream/livenesscenter_amplify_test.go:67` — above `func TestAmp_SignalCenter_AggregationPriority_ConvergingBeatsHung(t *testing.T) {`

```text
// TestAmp_SignalCenter_AggregationPriority_ConvergingBeatsHung (AMP2):
// When one session is Converging and another is Hung, Aggregate must return
// LivenessConverging. Validates priority level 1 > level 2 from ADR-0068.
```

### `go/internal/bridge/panestream/livenesscenter_amplify_test.go:91` — above `func TestAmp_SignalCenter_AggregationPriority_ConvergingWinsAll(t *testing.T) {`

```text
// TestAmp_SignalCenter_AggregationPriority_ConvergingWinsAll (AMP3):
// Four sessions cover all four LivenessState values; Converging must win.
// Validates the full priority ordering (Converging > Hung > BusyStagnant > Idle)
// in a single assertion, guarding the complete ADR-0068 aggregation rule.
```

### `go/internal/bridge/panestream/livenesscenter_amplify_test.go:334` — above `func TestAmp_SignalCenter_AllIdleSessionsAggregateIdle(t *testing.T) {`

```text
// TestAmp_SignalCenter_AllIdleSessionsAggregateIdle (AMP12):
// When all sessions are at LivenessIdle, Aggregate must return LivenessIdle —
// not 0 (zero is reserved for the empty-center case per ADR-0068 rule 5).
// This validates the distinction between "no sessions" (zero) and "all-idle sessions"
// (LivenessIdle), which an off-by-one in the priority sweep could collapse.
```

### `go/internal/bridge/panestream/livenesscenter_apicover_test.go:3` — above `var (`

```text
// livenesscenter_apicover_test.go — names the exported LivenessCenter type (and,
// as of cycle-432 S4, its Busy/Changed methods; as of cycle-434 S4-completion,
// its BusyOf method) as AST identifiers so apicover -enforce (Phase 5) tracks
// them.
//
// The behavioral suite in livenesscenter_test.go / livenesscenter_busychange_test.go /
// livenesscenter_busyof_test.go exercises the Facade only through the
// NewLivenessCenter constructor (and names the type/methods only in comments in
// this file), which leaves the exported symbol *tokens* unreferenced in any
// test AST — apicover flags them "UNCOVERED (no test names it)" and
// hard-fails repo-wide CI (the recurring warnship_apicover_ci_gap class,
// panestream is enrolled in go/.apicover-enforce). The full behavioral
// contract (observe/aggregate/register/empty/concurrency, busy/changed/busyOf
// projections) is already covered in the sibling test files; this
// declaration adds only the missing symbol references.
```

### `go/internal/bridge/panestream/livenesscenter_bench_test.go:3` — above `import (`

```text
// livenesscenter_bench_test.go — cycle-433 slice S5, Task 2
// (s5-resolve-sharding-decision): measures concurrent Observe throughput on
// DISTINCT session keys — the metric that exposes whether sc.mu's
// write-lock-across-Assess() serializes independent sessions (ADR-0068 KF2/H1).
// Its measured result (see docs/architecture/adr/0068-*.md "Consequences")
// drives the resolution of the ADR's "Deferred (S5)" per-session-sharding
// decision.
```

### `go/internal/bridge/panestream/livenesscenter_busychange_test.go:3` — above `import (`

```text
// livenesscenter_busychange_test.go — RED tests for cycle-432 slice S4, Task 1
// (s4-center-busy-change-projection): fold panestream.PaneBusy and
// bridge.PaneHasSubstantiveChange into panestream.LivenessCenter as per-session
// projections Busy(sessionKey) bool and Changed(sessionKey) bool, so the
// driver checkpoint (Task 2) stops parsing CLI chrome a second time itself.
// TDD contract: these tests are written BEFORE Busy/Changed exist. They
// compile-fail (Busy/Changed undefined) until Builder implements them.
// DO NOT MODIFY THESE TESTS — Builder implements to make them GREEN.
```

### `go/internal/bridge/panestream/livenesscenter_busyof_amplify_test.go:10` — above `func TestSignalCenter_BusyOf_AllProfilesMatrix(t *testing.T) {`

```text
// livenesscenter_busyof_amplify_test.go — Test-Amplification pass for cycle 434
// slice S4 (s4-complete-residual-busy-callsites, ADR-0068). Written black-box
// against the TDD contract (test-report.md) and the eval acceptance criteria
// (s4-complete-residual-busy-callsites.md) WITHOUT reading livenesscenter.go's
// diff — only the documented signature is used:
//
//	func (sc *LivenessCenter) BusyOf(rendered string, profile PaneProfile) bool
//
// Contract pinned here (from the spec, not the implementation):
//  1. BusyOf delegates to the SAME PaneBusy(rendered, profile) definition
//     (test-report.md: "delegates to the same PaneBusy definition; no
//     Observe, no per-session state") — every case below is a DIFFERENTIAL
//     oracle against the real, pre-existing panedelta.go:PaneBusy, so any
//     future divergence (e.g. a "helpful" special case added only to BusyOf)
//     fails here even though it might look like an improvement.
//  2. Nil-receiver-safe (build-report.md: "safe on a nil *LivenessCenter
//     receiver ... never dereferences sc").
//  3. Stateless: never mutates sc.sessions (build-report.md).
//
// These tests target gaps NOT already named in build-report.md's Self-Verify
// Evidence list (TestSignalCenter_BusyOf_MatchesStandalonePaneBusy,
// _EmptyPaneUnknownProfileNoPanic, _StatelessNoSessionMutation,
// _NilReceiverSafe) — breadth across all four shipped CLI profiles, the
// ANSI/large-input/ollama-placeholder edge cases those single-fixture names
// suggest are not yet covered, and NEW-method safety under the established
// ParallelEvaluate mixed-op concurrency stress (livenesscenter_parallelevaluate_test.go,
// cycle 433) which predates BusyOf and so never exercised it.
```

### `go/internal/bridge/panestream/livenesscenter_busyof_amplify_test.go:296` — above `func TestSignalCenter_BusyOf_ConcurrentWithObserveAggregateRegisterHandler(t *testing.T) {`

```text
// TestSignalCenter_BusyOf_ConcurrentWithObserveAggregateRegisterHandler
// (-race, S5-forward-looking): BusyOf is the newest method on LivenessCenter
// and postdates the established ParallelEvaluate mixed-op stress harness
// (livenesscenter_parallelevaluate_test.go, cycle 433's s5-parallelevaluate-stress-race,
// "written against the ALREADY-SHIPPED LivenessCenter") — that harness never
// exercised BusyOf because it did not exist yet. The cycle-434 goal's own S5
// slice ("concurrency hardening ... under ParallelEvaluate") is still
// upcoming; this pins BusyOf's concurrency safety now, before S5 formally
// starts, using the same mixed-op shape (concurrent Observe producers +
// concurrent RegisterHandler + concurrent Aggregate/Busy/Changed readers)
// plus concurrent BusyOf calls interleaved throughout.
```

### `go/internal/bridge/panestream/livenesscenter_busyof_test.go:3` — above `import "testing"`

```text
// livenesscenter_busyof_test.go — RED tests for cycle-434 slice S4 completion
// (s4-complete-residual-busy-callsites, Task 1): a STATELESS busy projection
// on LivenessCenter — BusyOf(rendered, profile) bool — that the two surviving
// direct panestream.PaneBusy consumers (autorespond.go's tick busy-gate,
// driver_tmux_repl.go's idle_reached busy/idle bracket) route through instead
// of calling PaneBusy directly. Unlike Busy(sessionKey) (S4/cycle-432), BusyOf
// takes NO session key and touches NO per-session state — Observe is never
// called, so routing these two call sites through it cannot pollute the
// checkpoint's Observe/Aggregate baseline (F3, scout finding — these sites
// fire at different loop points than the checkpoint's Observe).
//
// TDD contract: written BEFORE BusyOf exists. Compile-fails (BusyOf
// undefined) until Builder implements it. DO NOT MODIFY THESE TESTS —
// Builder implements to make them GREEN.
```

### `go/internal/bridge/panestream/livenesscenter_parallelevaluate_test.go:3` — above `import (`

```text
// livenesscenter_parallelevaluate_test.go — RED/regression tests for cycle-433
// slice S5, Task 1 (s5-parallelevaluate-stress-race): a ParallelEvaluate-style
// mixed-op stress harness on ONE shared *LivenessCenter, proving the RWMutex
// model (ADR-0068 Option C) is race-clean under many concurrent, DISTINCT
// session producers plus concurrent Aggregate/Busy/Changed readers plus
// concurrent RegisterHandler calls — not merely the ≥8 single-op producers
// the existing livenesscenter_test.go / livenesscenter_busychange_test.go cover.
//
// TDD contract: written against the ALREADY-SHIPPED LivenessCenter (S2-S4, on
// main). No production change is required for this task — it is expected to
// run GREEN today (pre-existing GREEN; see test-report.md). Its job is to PIN
// the concurrency invariant so Task 2's evidence-driven sharding decision
// (implement per-session locks, or record single-mutex-sufficient) cannot
// silently regress correctness.
// DO NOT MODIFY THESE TESTS — any Task 2 refactor must keep them GREEN
// unmodified, under EITHER branch of the sharding decision.
```

### `go/internal/bridge/panestream/livenesscenter_parallelevaluate_test.go:26` — above `var validAggregateStates = map[LivenessState]bool{`

```text
// validAggregateStates is the complete, documented Aggregate() return set
// (ADR-0068 aggregation rule + ADR-0070): 0 (empty center, no observations) plus
// the five LivenessState values. Any other value is a spec violation.
```

### `go/internal/bridge/panestream/livenesscenter_parallelevaluate_test.go:79` — above `const numHandlers = 4`

```text
// Concurrent RegisterHandler calls, overlapping the producers above —
// exercises the registry-write path under contention (ADR-0068 registry
// map, guarded by the same mutex as the sessions map).
```

### `go/internal/bridge/panestream/livenesscenter_test.go:9` — above `func TestSignalCenter_ObserveAndAggregate(t *testing.T) {`

```text
// livenesscenter_test.go — Behavioral tests for LivenessCenter (S2, cycle 430).
// TDD contract: these tests are written BEFORE the production code (livenesscenter.go).
// They compile-fail until Builder implements the LivenessCenter type.
// DO NOT MODIFY THESE TESTS — Builder implements to make them GREEN.
```

### `go/internal/bridge/panestream/panebusy_test.go:49` — above `func TestPaneBusy_Codex0_139_Working(t *testing.T) {`

```text
// TestPaneBusy_Codex0_139_Working — codex was previously treated as a busy
// "weak-signal degradation" because its fixture was the IDLE boot frame. A real
// codex 0.139 generating frame renders "• Working (<dur> • esc to interrupt)" —
// captured live (cycle-336 mutation-gate). Reading it as idle is the same wound
// as the claude case: a quiet-thinking codex builder/auditor → stall verdict →
// pause → exit=81, AND on the drive side the escalate-while-busy guard can't
// protect codex so a transient banner benches the whole family (the rate_limit
// false-positive). This pins codex's positive busy signal.
```

### `go/internal/bridge/panestream/panebusy_test.go:64` — above `func TestPaneBusy_Claude2_1_173_SpinnerOnly(t *testing.T) {`

```text
// TestPaneBusy_Claude2_1_173_SpinnerOnly — the 2026-06-11 soak-killer: the
// claude 2.1.173 self-update REMOVED the "esc to interrupt" affordance from
// generating panes; the only busy chrome left is the spinner stats line
// ("✢ Kneading… (6s · ↓ 244 tokens · thinking with high effort)"). Reading
// that frame as idle re-opens the cycles-254/255 wound (8ce42d6d): quiet
// extended thinking → stall verdict → pause → exit=81 (cycles 286/288).
// Fixture captured live from claude 2.1.173.
```

### `go/internal/bridge/panestream/panedelta.go:1` — above `package panestream`

```text
// Package panestream extracts newly-stabilized content lines from successive
// tmux `capture-pane` snapshots of an interactive LLM REPL.
//
// Background (see knowledge-base/research/tmux-live-capture-2026-06-04/NOTES.md):
// the live content source for the tmux bridge is polling `capture-pane` (tmux
// renders the pane to clean text) rather than raw `pipe-pane` (which needs a
// full terminal emulator to linearize). Each rendered snapshot is mostly stable
// scrollback with a volatile bottom UI (the empty input box, the footer, and
// the thinking spinner) re-painted every tick. This package emits only the NEW
// stable content above that volatile region.
//
// # Per-CLI PaneProfile
//
// The four supported tmux LLM CLIs (claude, codex, agy, ollama) each paint a
// different bottom-UI, so the extractor is configured per CLI via a PaneProfile
// (see Profiles). A profile carries the ONE thing that differs structurally —
// the input-line BoundaryMarker — and otherwise shares a single volatile-tail
// matcher derived from the real frames under testdata/.
//
// # Delta-boundary rule (general, tuned against the real frames in testdata/)
//
// Every CLI echoes the submitted prompt high in the pane on a line that starts
// with the same marker as its empty input box, then re-paints the EMPTY input
// box near the bottom. The bottom input box, the `────` separators around it,
// the footer (claude `⏵⏵ bypass permissions …`, codex `gpt-5.5 medium · /dir`,
// agy `? for shortcuts … Gemini …`, ollama folds its footer INTO the input
// line), and the status/spinner line (`✽ Inferring…`, `⣯ Generating…`,
// `▸ Thought for Ns`) are all VOLATILE and must never be emitted as content.
//
// The boundary is computed in two trims:
//
//  1. Drop everything at/below the LAST line whose TRIMMED text matches the
//     boundary. Match mode depends on PaneProfile.BoundaryExact: when false
//     (default), HasPrefix on the left-trimmed line is used — the marker must
//     be at line start, so an interior `>` never fires, and codex's
//     placeholder-bearing input box (`› Summarize recent commits`) is correctly
//     caught as the boundary. When true (agy only), the line must equal the
//     marker exactly after trimming — so the empty input box `>` matches but a
//     markdown blockquote `> quoted text` does not.
//     That removes the empty input box, any separator/footer below it, and any
//     spinner text painted on those rows. For codex this also drops the
//     next-prompt echo (`› Summarize recent commits`) that the REPL paints as
//     the new bottom prompt after an answer.
//  2. From the BOTTOM of what remains, drop a trailing run of volatile rows —
//     `────` separators, status/spinner lines, footers, and the blank lines
//     interleaved with them (isVolatileTailRow). This is the load-bearing
//     tuning the real frames forced: between the submitted prompt and the empty
//     input box sits a volatile zone whose height is the SAME in the thinking
//     frame as in the answer frame, so a naive append-only index cursor primed
//     on the thinking frame would skip the answer (it lands where the spinner
//     used to be) and emit only the spinner/separator tail. Trimming the
//     volatile tail makes the stable region end at the last real content line in
//     BOTH frames, so the index cursor diffs correctly.
//
// Because the FIRST snapshot primes the baseline (records the current stable
// length and emits nothing), the boot banner, the setup-warning, AND the echoed
// submitted-prompt line are counted at prime time and never re-emitted.
//
// # CLI-specific notes (verified against testdata/)
//
//   - claude (BoundaryMarker "❯"): two `❯` lines — echoed prompt + empty input.
//     Answer is `⏺ - …` / `  - …`; status leader is `✻`/`✽`.
//   - codex (BoundaryMarker "›"): three `›`-prefixed lines after an answer —
//     echoed prompt, the empty/next input echo, and the next-prompt line; the
//     LAST is dropped at boundary so the footer `gpt-5.5 medium · /dir` and the
//     next-prompt echo `› Summarize recent commits` never leak. Answer `• - …`.
//   - agy (BoundaryMarker ">"): the driver's BOOT marker is the footer
//     `? for shortcuts`, which sits BELOW the input box — NOT the content
//     boundary. The `>`-prefixed lines are the echoed prompt `> In exactly 3…`
//     and the empty input `>`; the LAST is the empty box → correct boundary.
//     agy emits a visible `▸ Thought for Ns` + reasoning preamble before the
//     `•` bullets; that reasoning IS real model output and is emitted as content
//     (only the `▸ Thought for Ns` status leader itself is volatile-trimmed when
//     it is the trailing row).
//   - ollama (BoundaryMarker ">>>"): the bottom line is
//     `>>> Send a message (/? for help)`. DECISION on gemma's chain-of-thought:
//     while generating, the thinking frame has NO bottom `>>>` input line at all
//     (it is replaced by `Thinking…`/CoT), so the last `>>>` at prime time is
//     the echoed prompt and almost nothing is baselined. When the answer frame
//     arrives the whole region above `>>> Send a message` — the visible CoT, the
//     `…done thinking.` delimiter, AND the `*  …` bullets — becomes stable and is
//     emitted. We treat the visible CoT as legitimate content (it is genuinely
//     streamed model output; ollama chose to surface it), NOT as something this
//     low-level extractor silently deletes from the middle of the transcript.
//     The only volatile rows trimmed are the trailing input line / blanks. This
//     keeps the shared rule honest (no per-CLI middle-of-buffer surgery) and
//     leaves any further CoT suppression to a downstream normalizer.
```

### `go/internal/bridge/panestream/panedelta.go:172` — above `var busySpinnerStatsRE = regexp.MustCompile('\(\s*\d[\d hms]*·\s*[↑↓]\s*[\d.,]+k?\s*tokens')`

```text
// busySpinnerStatsRE matches the in-turn spinner stats line — the ONLY busy
// chrome claude ≥2.1.173 renders (its self-update removed the esc-to-
// interrupt affordance from generating panes; the 2026-06-11 soak-killer,
// cycles 286/288). The shape "(<dur> · <arrow> <n> tokens" is structural —
// duration, middot, stream-direction arrow (↑ prompt / ↓ response), live
// token counter — and is rendered only while a turn runs, so it cannot
// false-match an idle answer the way bare spinner words ("Kneading"/
// "Inferring") could. The duration span is a digit-leading [\d hms]+ run so
// every format a turn passes through matches — "4s", "44s", "12m 34s",
// "1h 5m" (a miss on the hour shapes would re-open the stall wound exactly
// for the longest turns).
```

### `go/internal/bridge/panestream/panedelta.go:201` — above `for _, line := range strings.Split(clean, "\n") {`

```text
// Rule 1 — a live-turn affordance line is present. Projected from the single
// channel separator (ClassifyLine, ADR-0047) so the busy vocabulary cannot
// drift from the progress vocabulary that cleanPane reads.
```

### `go/internal/bridge/panestream/panedelta.go:217` — above `func PaneHasSubstantiveChange(prev, cur string) bool {`

```text
// PaneHasSubstantiveChange reports whether prev and cur differ once volatile
// chrome is stripped from both (cycle-432 S4: relocated from
// bridge/stopreview.go into panestream, the single home for pane-chrome
// parsing — panestream.LivenessCenter's Changed projection folds this in).
```

### `go/internal/bridge/panestream/panedelta.go:225` — above `func cleanPane(pane string) string {`

```text
// cleanPane keeps only the agent CONTENT lines, dropping every chrome/
// affordance line per the single channel separator (ClassifyLine, ADR-0047).
// This is what makes a ticking spinner-stats line (claude `· Schlepping… (Ns ·
// ↑ Nk tokens)`) NOT count as progress — it is the live-turn affordance, the
// same line PaneBusy reads as busy. A genuinely-working agent still progresses
// via its real transcript (tool calls, output); a stalled one whose only delta
// is the clock no longer reads as progress (closes the ticking-clock hole).
```

### `go/internal/bridge/panestream/panedelta.go:409` — above `var statusRE = regexp.MustCompile(`

```text
// statusRE is the small union of status/spinner/footer fragments observed in the
// real frames across all four CLIs. A trailing row matching any of these is
// volatile and trimmed. Kept deliberately small and frame-derived rather than a
// per-CLI list (see knowledge-base/research/tmux-live-capture-2026-06-04/).
```

### `go/internal/bridge/panestream/panedelta.go:417` — above `func isVolatileTailRow(line string) bool {`

```text
// isVolatileTailRow reports whether a row is part of the volatile zone that hugs
// the input box. A row is volatile iff it is NOT agent CONTENT — i.e. chrome
// (blank, `────` separator, spinner/status/token line) or a live-turn
// affordance. Projected from the single channel separator (ClassifyLine,
// ADR-0047) so the trim vocabulary cannot drift from PaneBusy and cleanPane.
```

### `go/internal/bridge/panestream/panedelta_test.go:10` — above `func readFrame(t *testing.T, name string) string {`

```text
// readFrame loads a committed real capture-pane snapshot from testdata/. The
// frames are local copies of the source-of-truth captures under
// knowledge-base/research/tmux-live-capture-2026-06-04/ so the test path is
// stable and does not depend on a fragile relative walk to the repo root.
```

### `go/internal/bridge/panestream/panedelta_test.go:381` — above `func TestPaneHasSubstantiveChange(t *testing.T) {`

```text
// TestPaneHasSubstantiveChange covers the spinner-disambiguation contract
// (ADR-0026 Stage 1 #4; relocated from bridge/stopreview_test.go in cycle-432
// S4 alongside PaneHasSubstantiveChange itself): volatile chrome — braille
// spinner frame, elapsed-time "Deliberating…" line, "↓ N.Nk tokens" counter —
// changing while the real transcript is unchanged is NOT substantive progress
// (→ false); genuinely new or changed transcript content IS (→ true). The
// "spinner frame advance only" subtest is the anti-no-op guard: a bare
// `prev != cur` implementation returns true here and FAILS it.
```
