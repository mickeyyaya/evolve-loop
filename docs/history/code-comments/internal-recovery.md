# Comment history: `internal/recovery`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/recovery/apicover_named_test.go:3` — above `import "testing"`

```text
// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise the exported recovery/stall types that no existing test names by
// identifier (apicover counts field access like d.Action as "uses Decision",
// but not as "names Decision"). Each test asserts a REAL contract.
```

### `go/internal/recovery/apicover_named_test.go:10` — above `func TestDecision_IntegrityEscalateFullStruct(t *testing.T) {`

```text
// TestDecision_IntegrityEscalateFullStruct pins the whole Decision the chain
// returns for an integrity-adjacent state: the locked ADR-0044 decision is
// escalate, claimed by the integrity link, with a justification. Full-struct
// equality on the typed Decision (names the type, not just a field).
```

### `go/internal/recovery/apicover_named_test.go:27` — above `func TestPhaseOutcome_AbortPreservesVerdictAndSpend(t *testing.T) {`

```text
// TestPhaseOutcome_AbortPreservesVerdictAndSpend pins PhaseOutcome's load-bearing
// structural invariant. PhaseOutcome is a deliberately LEAF DTO: its only
// producer is core's (unexported) phaseOutcomeFrom, and recovery must not import
// core (leaf constraint, see outcome.go), so there is no in-package or
// exported-cross-package call to exercise — the type IS the contract. The
// cycle-262 contract that type exists to enable: an abort is a cycle-level
// disposition recorded ALONGSIDE the agent's verdict, never a rewrite of it, and
// the burned spend is accounted even on abort. This proves Verdict/CostUSD/
// DurationMS are independent of AbortReason — start from a happy outcome, layer
// an abort on, and assert nothing else moved (it would catch a refactor that made
// Verdict derive from AbortReason, or dropped a spend field).
```

### `go/internal/recovery/detector.go:5` — above `type TerminalCause string`

```text
// TerminalCause is the typed classification of a fatal terminal state
// (ADR-0044 C2). "exit 81" is not a cause; "claude booted into an
// inaccessible-model error" is — recovery decisions are made on typed causes,
// never on raw exit codes or untyped pane text (design principle #2,
// classify-before-you-handle).
```

### `go/internal/recovery/detector.go:13` — above `CauseModelInvalid TerminalCause = "model_invalid"`

```text
// CauseModelInvalid: the CLI booted into its invalid/inaccessible-model
// error (cycle-262 retro: `claude --model auto`). The REPL is unusable;
// no amount of waiting produces an artifact.
```

### `go/internal/recovery/detector.go:17` — above `CauseCLISelfUpdated TerminalCause = "cli_self_updated"`

```text
// CauseCLISelfUpdated: the CLI updated its own binary mid-launch and asked
// for a restart (cycle-262 build: codex self-upgrade). The REPL exited;
// the pane is (or is about to become) a bare shell.
```

### `go/internal/recovery/detector.go:39` — above `Note string`

```text
// Note documents provenance (which incident taught us this signature) —
// carried into the justification trail when the signature matches.
```

### `go/internal/recovery/detector.go:67` — above `func SeedDetector() *FatalPaneDetector {`

```text
// SeedDetector returns the registry seeded with the known-fatal signatures,
// all three taught by cycle-262 (2026-06-09; forensics in
// docs/architecture/phase-recovery.md §2). Slice 5 adds durable promotion of
// advisor-classified novel signatures on top of these seeds.
```

### `go/internal/recovery/detector_amplify_test.go:3` — above `import "testing"`

```text
// detector_amplify_test.go — adversarial boundary tests for the cycle-276
// shell-spill signatures ("\nquote>" and "\nbquote>"). The builder's
// shell_spill_test.go proves positive + basic false-positive cases; these
// tests probe the newline-anchor contract that silently governs the detection
// surface: a pane that STARTS with "quote>" (no preceding \n) or has "quote>"
// inline without a newline prefix must NOT classify fatal.
```

### `go/internal/recovery/detector_amplify_test.go:148` — above `func TestDetect_NilDetector_SafeNoMatch(t *testing.T) {`

```text
// TestDetect_NilDetector_SafeNoMatch confirms the nil-receiver guard still
// holds with the cycle-276 signatures in play (regression-guard, not new
// behavior — detector.go:111 documents it).
```

### `go/internal/recovery/detector_test.go:3` — above `import "testing"`

```text
// detector_test.go — ADR-0044 C2 (Slice 2) RED tests: the deterministic
// FatalPaneDetector registry.
//
// cycle-262 burned ~40 min of a ~52 min cycle waiting out the maxExtends
// backstop on two SELF-DESCRIBING fatal pane states (the pane text literally
// says what is wrong) because nothing in the bridge recognizes them. The
// fixtures below are the real pane lines from the incident forensics
// (.evolve/runs/cycle-262/tmux-final-scrollback.txt + the post-mortem):
//
//   claude: "⏺ There's an issue with the selected model (auto). It may not
//            exist or you may not have access to it. Run /model to pick a
//            different model."
//   codex:  "Update ran successfully! Please restart Codex." (self-upgrade
//            mid-phase; the REPL exits to a bare shell)
//   shell:  "zsh: command not found: codex" (the bridge nudging a dead pane)
//
// Contract: Detect scans the recent pane tail and returns a typed
// TerminalCause on a seeded-signature match (first match wins, ordered
// registry); unknown panes return ok=false — classification only, no action
// (acting is the caller's stage-gated decision).
```

### `go/internal/recovery/handler.go:3` — above `import "fmt"`

```text
// handler.go — ADR-0044 C3: the recovery Chain of Responsibility, the single
// owner that turns a classified terminal state into a typed, justified
// recovery action. Modeled on internal/router/recovery.go (the repo's proven
// CoR shape: ordered handlers, first match wins, terminal catch-all).
//
// Order is LOAD-BEARING:
//
//	integrity-escalate   — an integrity breach never auto-recovers, period
//	busy-extend          — a visibly-working agent is never killed, even on
//	                       a fatal-looking pane (the stop-review prime
//	                       directive, cycle-254/255)
//	known-fatal-kill     — a typed cause from the deterministic registry
//	                       kills+retries immediately: zero LLM tokens, the
//	                       runner's exit-81 fallback chain owns the fresh
//	                       dispatch (cycle-262's rescue, minus the 20-min wait)
//	stall-budget-extend  — an unclassified stall within the extension budget
//	                       waits (the agent may be deep in thought)
//	unknown-advise       — the terminal catch-all: only the unknown residue
//	                       reaches the LLM failure-advisor tail (Rule 5),
//	                       whose verdict is then promoted so next time the
//	                       known-fatal-kill link catches it deterministically
//
// Pure decision logic: Recover never acts. The caller (observer policy,
// orchestrator hook) executes the action under its own stage gate.
```

### `go/internal/recovery/handler.go:51` — above `Kind string`

```text
// Kind names the incident: "fatal_pane", "stuck_no_output",
// "stuck_no_progress".
```

### `go/internal/recovery/handler.go:67` — above `type Decision struct {`

```text
// Decision is the chain's verdict: the action, the handler that claimed the
// input (forensics), and a human-readable justification (every recovery
// decision is justified — ADR-0044).
```

### `go/internal/recovery/handler.go:96` — above `name: "process-dead-kill",`

```text
// ABOVE busy-extend deliberately (R3.4, cycles 274/277): a dead
// process can render busy-looking chrome forever — pane echo is not
// liveness. Only integrity outranks a confirmed-dead process.
```

### `go/internal/recovery/handler_test.go:3` — above `import (`

```text
// handler_test.go — ADR-0044 C3 (Slice 6) RED tests: the recovery Chain of
// Responsibility — the single owner that turns a classified terminal state
// into a typed, justified recovery action. Modeled on router/recovery.go's
// chain (the repo's proven CoR shape). Order is load-bearing:
//
//   integrity-escalate → busy-extend → known-fatal-kill →
//   stall-within-budget-extend → unknown-advise (terminal)
//
// Integrity never auto-recovers; a Busy (visibly working) agent is never
// killed; a KNOWN fatal cause kills immediately (zero LLM tokens — the
// deterministic registry already classified it); only the UNKNOWN residue
// reaches the LLM failure-advisor tail.
```

### `go/internal/recovery/outcome.go:1` — above `package recovery`

```text
// Package recovery owns the Phase Recovery Pipeline (ADR-0044): given a
// phase dispatch result, produce a reconciled outcome or a typed terminal
// failure — recovery as a single-owner concern instead of point-fixes
// smeared across bridge, runner, orchestrator, and observer.
//
// cycle-262 (2026-06-09) is the motivating incident: a build whose CLI
// fallback succeeded (codex exit 81 → claude exit 0, valid PASS report,
// goal achieved) was recorded as if it never ran, because the post-phase
// tree-diff guard aborted the cycle on a real worktree leak and — like
// every abort path between dispatch return and the orchestrator's
// happy-path recording site — returned without recording the outcome.
// Reality and record diverged; the salvage had to be reconstructed by hand.
//
// Slice 1 (C1) ships the single-source outcome envelope: PhaseOutcome is
// the record EVERY terminal disposition of a phase dispatch must produce
// exactly once — happy advance and abort alike. Later slices layer
// terminal-state classification (C2), the recovery chain of responsibility
// (C3), the observer stall policy (C4), and the LLM escalation tail on top
// of this type.
//
// Leaf constraints (mirrors internal/router): recovery must stay importable
// by both core and phases/runner, so it never imports either (or bridge).
// Phase identifiers cross as plain strings; verdict strings are produced by
// the caller — core owns the canonical verdict vocabulary and the
// never-invent-PASS reconciliation rule (core.phaseOutcomeFrom), so this
// package carries no verdict constants to drift.
```

### `go/internal/recovery/outcome.go:34` — above `type PhaseOutcome struct {`

```text
// PhaseOutcome is the single-source record of one phase dispatch's terminal
// disposition (ADR-0044 C1). The orchestrator funnels every terminal path —
// success, exhausted retries, review-gate reject, worktree-leak recovery
// failure, tree-diff guard abort, persistence failure — through exactly one
// recording of this envelope, so the cycle record (PhasesRun,
// phase-timing.json, <phase>-usage.json) always reflects what actually ran,
// even when the cycle aborts.
```

### `go/internal/recovery/outcome.go:49` — above `CostUSD    float64`

```text
// CostUSD, DurationMS, and BootMS are the dispatch's resource usage as
// reported by the phase response — recorded even on aborts, so burned
// tokens are always accounted (cycle-262 lost the build's entire spend).
```

### `go/internal/recovery/outcome.go:82` — above `ModelSource   string`

```text
// ModelSource + ResolvedModel (T3, cycle-463) carry the phase response's
// per-phase model provenance through to phase-timing.json, so the dossier
// can record WHICH resolution path won ("profile"|"pin"|"advisor") plus
// the concrete resolved model/tier.
```

### `go/internal/recovery/process_dead_test.go:1` — above `package recovery`

```text
// process_dead_test.go — R3.3/R3.4 (concurrency-factory plan; inbox
// codex-update-menu-swallows-injection, cycles 274/277): "alive" must mean
// the agent PROCESS, not the pane. A wedged shell echoing prompt-lookalikes
// read as busy/live for 25+ min while the CLI was long gone.
//
// Pinned here:
//  1. Chain: Kind "process_dead" → ActionKillRetry even when the pane LOOKS
//     busy (dead process outranks busy-extend — the false-liveness trap);
//     integrity still outranks everything (ADR-0044 locked decision).
//  2. chainStallPolicy maps process_dead → StallKillRetry (the observer's
//     vocabulary).
//  3. Detector seeds (R3.3): the remaining zsh continuation variants from
//     the cycle-274/277 transcripts (dquote>, heredoc>) classify as
//     CauseDeadShell; a healthy busy pane stays unclassified.
```

### `go/internal/recovery/promote.go:3` — above `import (`

```text
// promote.go — ADR-0044 Slice 5: the Reflexion-style promotion loop. A novel
// fatal pane state the LLM failure-advisor classifies ONCE becomes a
// deterministic registry entry forever: in-memory for the running batch and
// durably under .evolve/instincts/fatal-signatures/ for every later boot.
// The deterministic frontier grows; the LLM never re-pays for a known
// failure (judgment at the frontier, determinism in the core).
//
// File format: a minimal fixed-key YAML subset written AND parsed here with
// zero dependencies (the leaf stays dependency-free; faillearn's lessons use
// the same write-if-absent atomic posture). Promotion ids are deterministic
// (content hash of the substring), so re-promotion is idempotent and the
// absent-only write means an operator-edited file always wins.
```

### `go/internal/recovery/promote.go:102` — above `if err := atomicwrite.Bytes(path, []byte(b.String())); err != nil {`

```text
// ADR-0049 N14: route through the atomicwrite SSOT, whose per-call os.CreateTemp
// gives every writer a UNIQUE temp. Two concurrent fleet cycles classifying the
// SAME novel pane resolve to the same content-addressed path; the old hand-rolled
// `path + ".tmp"` was therefore SHARED, so their write/rename interleaved — the
// loser's rename hit ENOENT (a lost promotion) and a partial write could tear the
// entry that every later boot replays. The SSOT also mkdirs the parent, so the
// explicit MkdirAll is gone. This is what the package header's "same write-if-absent
// atomic posture as faillearn's lessons" claim always meant; now it is true.
```

### `go/internal/recovery/promote.go:197` — above `for _, artifact := range []string{"[REDACTED]", "[untrusted]", "'''"} {`

```text
// ADR-0045 I5 backstop: the advisor reads a NEUTRALIZED pane digest, but
// Detect matches RAW panes — a substring carrying a neutralization
// artifact can never fire. Reject loudly (the caller escalates) instead
// of promoting a permanently-dead signature.
```

### `go/internal/recovery/promote_concurrency_test.go:9` — above `func TestPromoteSignature_ConcurrentSameSubstr_NoTornWriteOrLostError(t *testing.T) {`

```text
// TestPromoteSignature_ConcurrentSameSubstr_NoTornWriteOrLostError is the N14
// (ADR-0049) regression. Two or more concurrent fleet cycles that classify the
// SAME novel fatal pane resolve to the SAME content-addressed sig-<hash>.yaml —
// and, before the fix, to the SAME non-unique temp path (path + ".tmp"). Their
// os.WriteFile/os.Rename calls then interleave on one shared temp file: whichever
// renames first moves the inode out from under the others, so the losers' rename
// fails with ENOENT (a lost promotion) and a partially-written temp can be
// renamed over the target (a torn registry entry that poisons the deterministic
// frontier for every later boot).
//
// Under fleet mode the whole-cycle project lock is skipped, so these promotions
// genuinely overlap. The fix routes the write through the atomicwrite SSOT, which
// gives every caller a UNIQUE temp (os.CreateTemp), so concurrent same-target
// writers never collide on the temp file — every call succeeds and the final file
// is always a complete, parseable entry. A start barrier maximizes overlap so the
// pre-fix bug trips across the iterations.
```

### `go/internal/recovery/promote_test.go:3` — above `import (`

```text
// promote_test.go — ADR-0044 Slice 5 RED tests: the Reflexion-style promotion
// loop. When the LLM failure-advisor classifies a NOVEL fatal pane state, its
// classification is promoted into the deterministic registry — in-memory (the
// same batch's later phases get the fast catch) and durably
// (.evolve/instincts/fatal-signatures/<id>.yaml, replayed at startup) — so
// the deterministic frontier grows and the LLM never re-pays for a known
// failure. Judgment at the frontier, determinism in the core.
```

### `go/internal/recovery/promote_test.go:150` — above `func TestPromoteAdvice_RejectsNeutralizationArtifacts(t *testing.T) {`

```text
// TestPromoteAdvice_RejectsNeutralizationArtifacts — ADR-0045 I5 backstop:
// the advisor reads a NEUTRALIZED pane digest, but Detect matches RAW panes.
// A pane_substr quoting a neutralization artifact ([REDACTED], [untrusted],
// fence-softened ”') would promote a signature that can never fire — reject
// it loudly so the caller escalates instead of silently planting dead rules.
```

### `go/internal/recovery/recovery_adversarial_test.go:3` — above `import (`

```text
// recovery_adversarial_test.go — cycle-281 test amplification.
// Targets uncovered branches:
//   - Recover with zero-valued input (kind="", CauseUnknown) → unknown-advise
//   - Recover with stuck_no_progress kind (alternative to stuck_no_output)
//   - Recover at exact budget boundary (Attempts == MaxAttempts) → advise
//   - NewChainStallPolicy(0) → defaults maxExtends to 6
//   - NewChainStallPolicy(-1) → also defaults to 6
//   - Promote(nil) / Promote(empty-substr) guard clauses
//   - PromoteSignature(empty-substr) → error
//   - PromoteAdvice: neutralization artifact rejections ([untrusted], ''')
```

### `go/internal/recovery/signatures_c1117_test.go:3` — above `import "testing"`

```text
// signatures_c1117_test.go — owner-package coverage for FatalPaneDetector.
// Signatures (cycle-1117). The behavioural contract is pinned across the seam
// that consumes it (bridge's TestC1117_SignaturesAccessorMatchesRegistry), but
// the ADR-0050 apicover floor is per-package: the accessor must also be named
// and exercised HERE, by the package that owns it.
```

### `go/internal/recovery/stallpolicy.go:3` — above `type StallAction string`

```text
// stallpolicy.go — ADR-0044 C4: the Strategy that maps an observer stall
// incident to a recovery action.
//
// cycle-262 D5: the observer detects stalls but its only action is an inline
// "Enforce → SIGTERM" branch welded into the detector. Extracting the
// decision into this Strategy keeps the observer pure detection (SRP) and
// gives recovery one owner for "what do we DO about a stall": the policy is
// injected at the observer's two INCIDENT emit sites; a nil policy preserves
// the legacy branch byte-for-byte, so this seam ships behavior-neutral and
// the C3 composition slice wires a real, stage-gated implementation.
```

### `go/internal/recovery/stallpolicy.go:14` — above `type StallAction string`

```text
// StallAction is the typed verdict a StallPolicy returns for one stall
// incident. String values land verbatim in the INCIDENT envelope's "action"
// key — the justification trail (every recovery decision is recorded).
```

### `go/internal/recovery/stallpolicy.go:27` — above `StallEscalate StallAction = "escalate"`

```text
// StallEscalate: surface for the operator/advisor, take no action —
// the posture for integrity-adjacent states, which are never
// auto-recovered (ADR-0044 locked decision).
```

### `go/internal/recovery/stallpolicy.go:33` — above `type StallEvent struct {`

```text
// StallEvent describes one observer stall incident — the evidence a policy
// decides on. String/int-only leaf envelope (mirrors PhaseOutcome).
```

### `go/internal/recovery/stallpolicy.go:36` — above `Kind  string`

```text
// Kind is the incident type: "stuck_no_output" (idle clock tripped) or
// "stuck_no_progress" (babbling-but-livelocked backstop tripped).
```

### `go/internal/recovery/stallpolicy.go:50` — above `type StallPolicy interface {`

```text
// StallPolicy maps a stall incident to a recovery action plus a
// human-readable justification. Implementations must be side-effect-free —
// ACTING on the verdict (the kill, the log) is the observer's job, so a
// policy stays trivially testable.
//
// Call contract: Decide fires on EVERY observer poll tick for as long as the
// stall condition holds (legacy semantics — there is no once-at-onset
// guard), from a single goroutine per observer instance. An implementation
// shared across observers must be concurrency-safe; one that accumulates
// state (back-off counters) must expect rapid repeated calls per incident.
```

### `go/internal/recovery/strip.go:3` — above `import "strings"`

```text
// strip.go — the SINGLE source of the fatal-pane pane treatment, owned by the
// package that owns the registry (ADR-0044).
//
// A captured pane carries two kinds of text: the CLI's own chrome, and content
// the AGENT rendered into it (edit/diff views, patch scrollback, echoes of the
// injected prompt). Only the former is evidence about the CLI's state. Matching
// the fatal registry against the latter means classifying the agent's own edit
// buffer as a fatal pane — and the registry's two consumers fail in OPPOSITE
// directions when that happens:
//
//	bridge.fatalPaneVerdict (C2, fatalpane.go)  — false KILL: an agent editing
//	    the registry, its diff view rendering `+ Substr: "There's an issue with
//	    the selected model"`, is fast-failed on its own work (cycle-1117).
//	core.adviseOnUnclassifiedFailure (C3, failure_hook.go) — false SKIP: the
//	    deterministic-first short-circuit ("pane already classified") reads a
//	    GENUINELY NOVEL wedge as known, so the advise→promote path never runs
//	    and every recurrence burns the ~20 min maxExtends backstop again. The
//	    learning loop switched off by the agent's own text (cycle-1123).
//
// Both call this. Keeping one copy of the rules is the point: cycle-1117 fixed
// C2 alone and the two seams drifted for six cycles.
//
// The rules, and why they are NOT the exhaustion scan's (the cycle-1115 auditor
// rejected reusing bridge.strippedForExhaustionScan on both counts):
//
//	D1 — every matched line is BLANKED IN PLACE (content -> "", the "\n" kept),
//	     never deleted. Four dead-shell seeds are newline-ANCHORED ("\nquote>",
//	     "\nbquote>", "\ndquote>", "\nheredoc>") and the anchor is the whole
//	     defence against a bare-word false positive (cycle-274/277). Deleting
//	     the line ABOVE a continuation prompt strips that survivor's leading
//	     "\n" and silently reverts the cycle-274 fast-fail.
//	D2 — a line carrying any protected signature is exempt from the ECHO half.
//	     Echo-stripping is substring-keyed and two seeds are literal English
//	     sentences, so any prompt QUOTING one (a detector-hardening cycle, a
//	     retro) would make the CLI's real banner indistinguishable from an echo
//	     and silence the cycle-262 fast-fail. Callers pass det.Signatures(), so
//	     the protect-list comes FROM the live registry and can never suppress a
//	     signature the detector is looking for.
//
// The asymmetry is deliberate: protected applies to the echo half ONLY. Diff
// prefixing is proof of agent authorship by construction (cycle-314), and
// suppressing agent-authored seed text is the entire point.
//
// An empty (or whitespace-only) injectedPrompt strips no echoes — fail-open:
// never suppress a genuine signal on missing context. The C3 hook passes ""
// deliberately: D2 neuters the echo half for a fatal-pane scan anyway, so
// plumbing the phase prompt in would add I/O and zero behaviour.
//
// Deliberately depends on "strings" alone: go/acs/cycle1123 mutates
// StripAgentContent's body by `go test -overlay`, and an import used solely by
// this function would make the mutant fail to COMPILE — turning a mutation
// verdict into a build error.
```

### `go/internal/recovery/strip_c1123_test.go:3` — above `import (`

```text
// strip_c1123_test.go — cycle-1123 RED tests for `fatalpane-strip-agent-content`,
// SECOND call site.
//
// WHAT CYCLE-1117 CLOSED, AND WHAT IT DID NOT. Cycle-1117 routed the bridge's
// stop-review fast-fail (bridge.fatalPaneVerdict, fatalpane.go:71) through an
// agent-content stripper, so an agent EDITING the fatal registry — its diff
// view literally rendering `+ Substr: "There's an issue with the selected
// model"` — is no longer fast-failed on its own edit buffer. It left the
// registry's OTHER consumer raw: core.adviseOnUnclassifiedFailure
// (failure_hook.go:87) still calls det.Detect(report.FinalPane) on the
// unstripped escalation pane. (scout-report.md named this second site
// "fatalpane_persistence.go:73"; no such file exists in the tree — the second
// raw Detect consumer is failure_hook.go:87. Same defect class, real file.)
//
// THE DEFECT IS INVERTED, NOT MILDER. In the bridge the raw read causes a false
// KILL; in the hook it causes a false SKIP. "Pane already classified" is the
// deterministic-first short-circuit that suppresses the C3 LLM consultation, so
// an agent-authored diff line quoting a seeded signature makes a GENUINELY
// NOVEL fatal pane look known: no advisor call, no promotion, and every future
// occurrence of that novel wedge burns the ~20 min maxExtends backstop again.
// ADR-0044's whole learning loop is silently disabled by the agent's own text.
//
// THE CONTRACT THESE TESTS PIN (production API Builder must create — RED today
// is a compile failure, which is the correct RED for a missing API):
//
//	recovery: func StripAgentContent(pane, injectedPrompt string, protected []string) string
//	    in go/internal/recovery/strip.go, whose ONLY import is "strings"
//	    (go/acs/cycle1123 mutates this function by overlay; an extra import
//	    would make the mutant fail to compile and the mutation predicates
//	    would report a build error instead of a verdict).
//
// It is the SINGLE source of the fatal-pane pane treatment, exported from the
// package that OWNS the registry, so both consumers strip identically:
// bridge.strippedForFatalPaneScan must DELEGATE to it (it may not keep a second
// copy of the rules — see the acs mutation predicates), and failure_hook.go
// must call it before Detect. Semantics are cycle-1117's, verbatim:
//
//	D1 — matched lines are BLANKED IN PLACE (content -> ""), never deleted, so
//	     no survivor loses its leading "\n" and the four newline-ANCHORED
//	     dead-shell seeds ("\nquote>" &c., cycle-274/277) keep matching.
//	D2 — a line carrying any entry of protected is left untouched by the
//	     prompt-echo half, so a prompt QUOTING a seed can never silence that
//	     seed's real banner (cycle-641/642). The diff half is deliberately NOT
//	     protect-listed: diff-prefixing is proof of agent authorship
//	     (cycle-314), and suppressing agent-authored seed text is the point.
//	Empty prompt strips no echoes (fail-open: never suppress a genuine signal
//	on missing context). Nil/blank protected entries protect nothing.
//
// DO NOT MODIFY THESE TESTS to make them pass — they are the acceptance
// criteria. The function name, parameter names and file path above are part of
// the contract (the acs overlay mutants are Go source compiled against them).
```

### `go/internal/recovery/strip_c1123_test.go:98` — above `func TestC1123_StripAgentContentPreservesNewlineAnchor(t *testing.T) {`

```text
// TestC1123_StripAgentContentPreservesNewlineAnchor is D1's discriminating case:
// a diff line directly ABOVE a continuation prompt. Blanked in place, the
// survivor keeps its leading "\n" and "\nquote>" still matches; deleted, it
// becomes line one and the dead-shell fast-fail silently reverts (cycle-274).
```

### `go/internal/recovery/strip_c1123_test.go:116` — above `func TestC1123_StripAgentContentProtectsSeededSignatureFromEchoStrip(t *testing.T) {`

```text
// TestC1123_StripAgentContentProtectsSeededSignatureFromEchoStrip is D2: the
// prompt for a detector-hardening cycle (this one included) quotes the seeds
// verbatim, and echo-stripping is substring-keyed. Without the protect-list the
// CLI's REAL banner becomes indistinguishable from an echo and the cycle-262
// fast-fail goes silent. The second half proves the protect-list is what saves
// it — not some incidental property of the input.
```
