# Comment history: `internal/bridge`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/bridge/adversarial_faults_test.go:62` — above `adversarialFaultCase{`

```text
// rate-limit-wall: the cycle-283 quota wall — the REPL boots, work
// is submitted, then the provider wall appears. The autoresponder
// must CLASSIFY (pattern rate_limit), persist the escalation
// report (the artifact the runner's bench-writer consumes), and
// escalate with ExitUnknownPrompt instead of stalling to the
// artifact timeout.
```

### `go/internal/bridge/adversarial_faults_test.go:89` — above `func rateLimitWallFor(family string) string {`

```text
// rateLimitWallFor returns each family's real wall phrasing (matching that
// family's manifest rate_limit regex). The codex text is verbatim from the
// cycle-283 escalation report — the incident this fault replays.
```

### `go/internal/bridge/agy_concurrency_test.go:1` — above `package bridge`

```text
// agy_concurrency_test.go — the agy-only fleet concurrency contract. In an
// agy-ONLY environment every lane, every phase, every retry launches an
// agy-tmux pane; the ONE deterministic invariant that keeps concurrent lanes
// from fighting over a single tmux session is resolveSession's atomic
// per-process nonce (ADR-0049 N15). The pre-existing nonce test only mints two
// sessions SEQUENTIALLY; these tests mint many CONCURRENTLY under a frozen
// clock (worst case: every lane in the same wall-clock second) and assert with
// `-race` that the atomic increment is safe and every name is distinct.
```

### `go/internal/bridge/agy_model_names_test.go:3` — above `import (`

```text
// agy_model_names_test.go — every agy tier model must be a name agy actually
// ACCEPTS, not merely a plausible one.
//
// Live incident 2026-08-28. Two independent spellings had drifted, and neither
// failed loudly:
//
//	manifest model_tier_map : "Gemini Flash 3.7 (Low)"   <- words transposed
//	live catalog tier_models: "Gemini 3.7 Flash"         <- suffix missing
//
// agy rejects both. It does not exit non-zero; it prints one warning and then
// serves a different model for the whole session:
//
//	⎿ model Gemini 3.1 Pro is not recognized as a known model or custom model
//	  in settings. Using "Gemini 3.5 Flash (Medium)" instead.
//
// Because catalog_overlay merges the live catalog OVER the manifest, the
// catalog's spelling won, so router and memo ran Gemini 3.5 Flash (Medium) at
// EVERY tier from 2026-08-14 until this fix. Nothing detected it: the flag was
// emitted correctly, the launch succeeded, and every flag-shape assertion in
// the suite stayed green. Only the running process knew.
//
// Verified live against agy 1.1.22 (`agy --model "<name>"`, reading the banner
// the process prints about ITSELF):
//
//	"Gemini 3.7 Flash (Low)"     -> Gemini 3.7 Flash (Low)      accepted
//	"Gemini 3.7 Flash (High)"    -> Gemini 3.7 Flash (High)     accepted
//	"Gemini 3.1 Pro (High)"      -> Gemini 3.1 Pro (High)       accepted
//	"Gemini 3.7 Flash"           -> Gemini 3.5 Flash (Medium)   REJECTED
//	"Gemini Flash 3.7 (Low)"     -> Gemini 3.5 Flash (Medium)   REJECTED
```

### `go/internal/bridge/agy_model_names_test.go:59` — above `if strings.Contains(model, "Flash") && !strings.Contains(model, "Flash (") {`

```text
// The 2026-08-14 transposition specifically: "Gemini Flash 3.7"
// instead of "Gemini 3.7 Flash". Both match the shape rule above, so
// the shape rule alone would NOT have caught it — pin the word order.
```

### `go/internal/bridge/agy_model_tier_test.go:3` — above `import (`

```text
// agy_model_tier_test.go — cycle-447 Task 1 (agy-model-channel-probe-and-wire):
// unit pins for the agy-tmux model_tier channel wired from noop → flag.
//
// Probe evidence (live, 2026-07-02, agy 1.0.15 — the fresh probe incident
// cycle-154 demands): `agy --help` lists `--model` ("Model for the current
// CLI session"); `agy -m X` still errors "flags provided but not defined: -m";
// `agy models` lists 8 display-name tokens matching the live catalog's
// `available` byte-for-byte; a tmux launch `agy --model "Claude Opus 4.6
// (Thinking)" --dangerously-skip-permissions` boots to the "? for shortcuts"
// footer in ~2s with the model shown in banner + footer. Transcript in
// cycle-447 build-report.md.
//
// The tokens are display names with spaces and parens, so launchCmdLine
// shell-quotes every realized flag token (safe tokens pass through verbatim —
// claude/codex/ollama launch lines stay byte-identical).
```

### `go/internal/bridge/agy_model_tier_test.go:91` — above `func TestAgyModelTierAutoSentinelOmitted(t *testing.T) {`

```text
// TestAgyModelTierAutoSentinelOmitted pins the cycle-262 guard through the
// NEWLY-wired channel: "auto" is the loop's resolve-me sentinel, never a
// concrete model — tier=auto must emit no --model pair, no REPL input, and
// must never leak the literal "auto" into the launch flags.
```

### `go/internal/bridge/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — public-API coverage closure (ADR-0050 Phase 5) for
// the test-support + DTO surface of internal/bridge that no prior test NAMED:
//
//   - FakeTmuxController.PaneCommand  — invoked + asserted (was UNCOVERED)
//   - FakeTmuxController.JiggleWindow — actually CALLED + effect asserted
//     (was FALSE-GREEN: only mentioned in a render_wedge_test.go comment)
//   - PaneCommander interface         — satisfaction proven + exercised
//   - Report / ArtifactRef / FileRef  — bound via the real BuildReport producer
//   - DoctorReport / DoctorResult / AuthInfo / BinaryInfo / DeepProbe — bound
//     via the real (*Engine).Doctor producer
//
// Each DTO is bound to a producer's OUTPUT (not a bare literal) and the
// producer-set fields are asserted, so the test verifies real wiring, not type
// shape alone. The methods are executed so they cannot regress to a false-green.
```

### `go/internal/bridge/artifact_timeout_diag_test.go:162` — above `{panestream.LivenessExhausted, "exhausted"},`

```text
// the one spelling, projected (ADR-0101 S3)
```

### `go/internal/bridge/artifact_timeout_diag_test.go:208` — above `func TestEngineLaunch_ArtifactTimeout_SummaryBeatsEarlierBridgeChatter(t *testing.T) {`

```text
// TestEngineLaunch_ArtifactTimeout_SummaryBeatsEarlierBridgeChatter is the
// discriminative proof that the engine's extractor is load-bearing rather than
// incidental: firstDiagnosticLine returns the FIRST `[bridge]`-prefixed line, so
// with a sandbox WARN (or a codex preflight note) ahead of the wait the recorded
// cause is that WARN — a launch-time note that says nothing about why the phase
// died. Driven through the real Engine.Launch wrapping site.
```

### `go/internal/bridge/artifact_timeout_transient_test.go:3` — above `import (`

```text
// artifact_timeout_transient_test.go — an artifact-timeout death whose pane
// literally states the cause was a temporary upstream server error must SAY SO.
//
// The defect (inbox item transient-api-error-invisible-inside-artifact-timeout,
// weight 0.90): a transient provider error inside the artifact wait is
// recognized by NOTHING. It is not a quota wall (controls.usage.exhausted_regex
// correctly ignores it), not a transient exit code (80/85/86), and it surfaces
// as exit 81 — contractually non-retryable (transient-bridge-retry AC-1, which
// must stay green). Live evidence: 3 of 4 observed router stalls across cycles
// 1523/1524/1526 burned 600s each on "API Error: 529 Overloaded".
//
// Contract: the ONE self-describing artifact-timeout marker line carries a
// driver-authored transient= field derived from matching the family's
// manifest-declared transient_regex against the CAPTURED PANE.
//
// Two constraints are load-bearing and both are asserted below:
//
//  1. The classifier reads the PANE, never the stderr buffer Engine.Launch
//     inspects (cycle-1528 premise-challenge, severity CRITICAL): every stderr
//     write on the exit-81 path is a bridge-authored note, so a stderr
//     classifier would pass synthetic fixtures 3/3 while never firing live.
//     Hence the fixture here is the VERBATIM final_pane of cycle-1523's
//     router-escalation-report.json, not a hand-written string.
//  2. The field EXTENDS the existing marker line rather than competing with it,
//     so it can never displace artifactTimeoutSummary's cause selection — the
//     engine.go:551-561 regression class stays structurally unreachable.
```

### `go/internal/bridge/artifact_timeout_transient_test.go:40` — above `func livePane529(t *testing.T) string {`

```text
// livePane529 is the unedited pane captured when cycle-1523's router died at
// exit 81. Provenance: .evolve/runs/cycle-1523/router-escalation-report.json,
// key final_pane. Read from testdata rather than inlined so the evidence stays
// auditable against the source report.
```

### `go/internal/bridge/artifact_timeout_transient_test.go:118` — above `type transientFamilyCase struct {`

```text
// transientFamilyCase is one CLI family's transient-recognition contract.
//
// Recognition MUST be family-agnostic: the driver resolves the pattern from
// whichever manifest the launch names (lp.name), so a family that declares no
// transient_regex silently loses the diagnosis — the router stalls the same 600s
// on codex or agy as it did on claude, with nothing in the record to say why.
// The exemplars are that provider's OWN transient surface; only claude's is
// live-captured (cycle-1523), the rest are the providers' documented error
// shapes and are marked as such.
```

### `go/internal/bridge/artifact_timeout_transient_test.go:137` — above `transient: []string{`

```text
// Live-captured: .evolve/runs/cycle-1523/router-escalation-report.json.
```

### `go/internal/bridge/artifact_timeout_transient_test.go:188` — above `var bridgeAuthoredChatter = map[string]string{`

```text
// bridgeAuthoredChatter is stderr the BRIDGE itself writes on the exit-81 path.
// None of it may read as transient for ANY family: matching it would prove the
// classifier is pointed at the stderr buffer rather than the pane (the
// cycle-1528 premise-challenge, severity CRITICAL) and would resurrect the class
// of regression where the drift alarm or a workspace listing displaces the
// recorded cause.
```

### `go/internal/bridge/artifact_timeout_transient_test.go:297` — above `func TestClassifyTransientPane_IgnoresEchoedPromptText(t *testing.T) {`

```text
// TestClassifyTransientPane_IgnoresEchoedPromptText: the classifier is scanned
// on the AGENT-STRIPPED pane, mirroring the exhaustion detector (cycle-654
// prompt-echo veto). This repo is the sharpest case for it — a cycle whose task
// IS "handle API Error: 529" injects that phrase into the prompt, and the agent
// echoes it back. A raw-pane scan would then label EVERY artifact timeout in
// that cycle a transient upstream failure, which is the classic
// classifier-reads-its-own-instructions defect.
```

### `go/internal/bridge/askbroker_rung_test.go:3` — above `import (`

```text
// askbroker_rung_test.go — ADR-0045 I3 (§8): the pre-85 AskBroker rung inside
// the auto-respond escalate branch. White-box: drives a real autoResponder
// with a manifest rule whose policy is `escalate` (so a matching pane yields
// rc 85), and a KernelAnswerer whose facts cover the question.
```

### `go/internal/bridge/attempt_telemetry.go:52` — above `func (c attemptLogContext) warn(code signalcenter.Code, detail string) {`

```text
// warn is the engine's telemetry-warning producer (ADR-0101 S3): one
// bridge.warning whose code names the rule and whose reason is the detail —
// launchWarn under its historical origin with no step (the payload stays
// exactly the call identity). The old "[engine] WARN" line is rendered by
// the root's stderr sink.
```

### `go/internal/bridge/attempt_telemetry.go:70` — above `func (c attemptLogContext) launchWarn(origin, step string, code signalcenter.Code, reason string, fields map[string]stri…`

```text
// launchWarn is the module's ONE bridge.warning producer (ADR-0103 unit 10):
// one event under the attempt's identity whose fields name the host step
// (fields.step, omitted when empty) beside the call identity, plus the
// step's own facts. The origin is the producer's Type.Method — vocabulary,
// never a hidden default. A nil Center is the Null Object (Emit on nil is a
// no-op).
```

### `go/internal/bridge/attempt_telemetry.go:241` — above `logContext.tripwire(end.Sub(start).Milliseconds())`

```text
// ADR-0101 S3: the escalation is a bridge.tripwire signal; the root's
// sink renders it (the hand-written "[engine] TRIPWIRE" line is gone).
```

### `go/internal/bridge/autorespond.go:38` — above `var agentDiffLineRE = regexp.MustCompile('^[ \t]*(?:\d+[ \t]+)?[+-]')`

```text
// agentDiffLineRE marks a captured line as agent-authored edit content: a
// numbered diff line ("   224 +\tpane := ...") as rendered in the codex/claude
// editor view, or a bare unified-diff content line ("+text" / "-text") from
// lingering patch scrollback. CLI chrome — prompts, dialogs, rate-limit banners
// — is never diff-prefixed, so these lines carry the agent's content (which can
// contain prompt-shaped text the agent is writing ABOUT, e.g. a clihealth
// rate-limit fixture) and must not drive interactive-prompt matching.
// soak #4 cycle 314: an agent editing the clihealth parser typed "You've hit
// your usage limit" into a test fixture and the escalate rule benched codex.
```

### `go/internal/bridge/autorespond.go:72` — above `func stripPromptEchoLines(pane, injectedPrompt string) string {`

```text
// stripPromptEchoLines removes captured-pane lines that are a verbatim echo of
// the injected prompt, so the exhaustion / escalation scan never fires on the
// agent's OWN instruction text. Mirrors stripAgentDiffLines, but keyed off the
// prompt rather than diff-prefixing: cycle-641/642 — an echoed Deliverable-
// Contract line ("...reached your usage limit...") or a Reviewer exploit
// checklist benched a PASSING phase because the matcher saw the prompt the
// agent was quoting, not a CLI wall. A line is dropped when its trimmed form is
// a substring of the prompt; the CLI's real banners — absent from the prompt —
// pass through unchanged. An empty prompt strips nothing (fail-open: never
// suppress a genuine signal on missing context).
```

### `go/internal/bridge/autorespond.go:111` — above `func strippedForFatalPaneScan(pane, injectedPrompt string, protected []string) string {`

```text
// strippedForFatalPaneScan is the bridge's view of the fatal-pane pane
// treatment. It DELEGATES to recovery.StripAgentContent — the rules live once,
// in the package that owns the registry, because this seam is not the registry's
// only consumer: core.adviseOnUnclassifiedFailure (ADR-0044 C3) strips the same
// way before the same Detect. Cycle-1117 fixed this seam alone and the two
// drifted for six cycles, so the C3 hook kept classifying agent-authored diff
// content as "already known" and silently skipping the advisor.
//
// It stays SEPARATE from strippedForExhaustionScan (which deletes matched lines
// and has no protect-list) — the cycle-1115 auditor rejected that reuse because
// the fatal registry's matchers are newline-anchored and partly literal English.
// See recovery/strip.go for D1 (blank in place) and D2 (protect-list) in full.
```

### `go/internal/bridge/autorespond.go:153` — above `if paneBusy && p.Policy == "escalate" {`

```text
// ADR-0047 state-gate: a policy=escalate prompt (rate_limit/quota/auth)
// means the CLI is BLOCKED needing intervention — mutually exclusive with
// the CLI actively generating. If the pane is BUSY, an escalate match is
// the agent QUOTING the banner in its own output, not the CLI's chrome —
// skip it (don't count toward the loop guard) and keep scanning. Cycle-314:
// a clihealth coverage cycle wrote "You've hit your usage limit" into a
// test fixture while codex showed "Working… esc to interrupt"; the bridge
// benched the codex family 30min on the agent's own content. Scoped to
// escalate only — auto_respond prompts (menus/approvals) legitimately
// co-occur with an "esc to cancel" affordance and must still fire.
```

### `go/internal/bridge/autorespond.go:244` — above `injectedPrompt string`

```text
// injectedPrompt is the resolved prompt text delivered to this session.
// tick() strips pane lines that are a verbatim echo of it before the
// prompt-match and exhaustion scans (stripPromptEchoLines, cycle-654
// helper wired cycle-672), so the agent quoting its OWN instructions —
// e.g. an echoed "...reached your usage limit..." Deliverable-Contract
// line — never escalates rc 85. Empty strips nothing (fail-open).
```

### `go/internal/bridge/autorespond.go:271` — above `rec     *interaction.Recorder`

```text
// Interaction telemetry (ADR-0045 I1): rec records every send with its
// deterministically-resolved outcome ("prompt-pattern cleared on next
// capture"). nil = no telemetry (the recipe adapter's capability runs are
// outside the phase-interaction surface). phase/cycle stamp the events;
// pending is the one in-flight send awaiting resolution.
```

### `go/internal/bridge/autorespond.go:280` — above `broker      *interaction.KernelAnswerer`

```text
// I3 AskBroker (ADR-0045): when an escalation would fire (rc 85) and the
// kernel KNOWS the answer to the blocking question, inject it ONCE and buy
// one more interval instead of failing the whole phase to a cross-family
// re-dispatch. nil broker / non-enforce stage / a miss all fall through to
// the unchanged 85 → fallback chain (the unconditional floor). brokerTried
// bounds it to once per launch.
```

### `go/internal/bridge/autorespond.go:388` — above `ar.resolvePending(pane)`

```text
// ADR-0045 I1: resolve the in-flight send against THIS capture before
// deciding — the pattern no longer matching is the deterministic
// "it worked" signal ("prompt-pattern cleared on next capture").
```

### `go/internal/bridge/autorespond.go:419` — above `paneBusy := ar.deps.LivenessCenter.BusyOf(pane, panestream.Profiles[strings.TrimSuffix(ar.cli, "-tmux")])`

```text
// pane here stays RAW: resolvePending, shadow matching, and writeEscalation
// (above/below) need the real terminal. stripAgentDiffLines runs only
// inside decideAutoRespond, scoped to the prompt-matching decision.
// Routed through LivenessCenter.BusyOf (cycle-434 S4 completion), not the
// standalone PaneBusy: BusyOf is nil-receiver-safe and stateless (no
// Observe), so ar.deps.LivenessCenter — nil outside the driver's
// Deps-injected test seam — needs no guard here, and this tick's read
// can never pollute the checkpoint's own Observe/Aggregate baseline.
```

### `go/internal/bridge/autorespond.go:428` — above `scanPane := stripPromptEchoLines(pane, ar.injectedPrompt)`

```text
// scanPane: prompt-echo lines removed (stripPromptEchoLines, cycle-654
// helper wired cycle-672) so neither the prompt-match nor the exhaustion
// scan below fires on the agent quoting its OWN instructions. pane stays
// raw for forensics (writeEscalation), the busy probe, and tryKernelAnswer.
```

### `go/internal/bridge/autorespond.go:465` — above `if rc != 85 && ar.exhaustedRegex != "" && !ar.wallScanSuppressed {`

```text
// Exhaustion override (reuses THIS tick's capture — no extra CapturePane, so no
// paneSeq churn): a quota/rate-limit wall escalates (rc 85), ungated by paneBusy
// (a wall blocks regardless of the spinner) and overriding a lesser verdict —
// the artifact will never come. Detected via the LivenessCenter (ExhaustedOf, the
// fast-poll twin of BusyOf); the rc==85 arm below then writes the escalation
// report exactly as for any escalate, so the fallback fires within one poll.
//
// Two guards against fast-failing a WORKING agent that merely RENDERS
// wall-shaped text (a cat/grep/diff quoting a provider's "reached your … limit"
// message — the cardinal false-FAIL sin, cycle-314/641): (1) the scan runs on
// the diff-stripped pane too (scanPane already has prompt-echo removed), and
// (2) it is persistence-gated (exhaustion_persistence.go) — a single transient frame
// never crosses; a real wall, present every frame, crosses in a couple ticks.
```

### `go/internal/bridge/autorespond.go:484` — above `if !ar.wallProbed {`

```text
// Corroborate before the verdict (2026-08-15 false-wall class):
// a lane whose WORK CONTENT is wall vocabulary persists it
// on-pane exactly like a real wall. EXACTLY ONE probe per
// responder, whatever it answers: healthy disables the scan
// (the fixture stays on screen), and a CONFIRMED wall latches
// too — the gate itself latches, and several tick callers
// discard rc (boot loop, recipe adapter), so an unlatched
// verdict would re-fire a bounded-60s quota-consuming probe on
// every subsequent tick of an already-confirmed wall.
```

### `go/internal/bridge/autorespond.go:523` — above `if ar.tryKernelAnswer(ctx, session, pane) {`

```text
// ADR-0045 I3: the pre-85 rung. If the kernel can answer the blocking
// question, inject it once and return "responded" (rc 1) to buy one
// more interval. Any miss / non-enforce / no-typed-question falls
// through to today's escalation — I3 never suppresses the 85 chain.
```

### `go/internal/bridge/autorespond.go:711` — above `const autoRespondInterKeyPause = 500 * time.Millisecond`

```text
// autoRespondInterKeyPause spaces out the keystrokes of a multi-step response
// so the inner CLI's TUI gets a render frame between them. claude's multi-
// select navigation (toggle → Right to Submit → Enter) is unreliable when the
// three keys arrive as one rapid burst — the cursor move lands before the
// toggle has re-rendered — but reliable once paced. Verified 2026-05-26: a
// zero-gap burst intermittently failed to submit; a 500 ms gap submitted on
// every run. The pause is delivered via Deps.Sleep, so the deterministic tests
// (no-op / scaled Sleep) stay fast and only a real launch waits.
```

### `go/internal/bridge/autorespond_busyof_migration_test.go:3` — above `import (`

```text
// autorespond_busyof_migration_test.go — RED tests for cycle-434 slice S4
// completion (s4-complete-residual-busy-callsites, Task 1): the
// autoResponder.tick busy-gate (autorespond.go:282) is one of the two
// surviving direct panestream.PaneBusy consumers the S4 charter targeted
// (scout finding F1). It must route through panestream.LivenessCenter.BusyOf
// instead of calling panestream.PaneBusy inline, so the LivenessCenter remains
// the sole liveness facade (ADR-0068) and no bridge consumer parses CLI
// chrome directly.
//
// TDD contract: written BEFORE the migration lands. AC1 pins the ALREADY-
// correct busy-gating VALUE through the tick() entry point specifically (the
// existing TestDecideAutoRespond_IdleGatesEscalateWhileBusy only exercises
// decideAutoRespond with a pre-computed bool, never tick()'s own PaneBusy
// call) — it may show pre-existing GREEN for "the value is right" since the
// migration is designed to be behavior-preserving (H1). AC3 is the
// discriminating RED test: it fails today because tick() still calls
// panestream.PaneBusy inline. DO NOT MODIFY THESE TESTS — Builder migrates
// the call site to make AC3 pass without breaking AC1.
```

### `go/internal/bridge/autorespond_busyof_migration_test.go:32` — above `func TestAutoResponderTick_BusyGateViaCenter_SuppressesEscalate(t *testing.T) {`

```text
// TestAutoResponderTick_BusyGateViaCenter_SuppressesEscalate (AC1, positive):
// a pane matching an escalate-policy prompt (the agent quoting a banner in
// its own output, cycle-314 class) while ALSO carrying a live-turn affordance
// must NOT escalate — tick() must return rc 0 (noop), the busy-gated
// suppression decideAutoRespond performs when paneBusy is true.
```

### `go/internal/bridge/autorespond_decision_test.go:9` — above `func TestAutoRespond_RealManifestDecisionMatrix(t *testing.T) {`

```text
// autorespond_decision_test.go — the full interactive-prompt decision matrix,
// driven by the REAL embedded manifests (LoadManifest) against REAL observed
// pane text. This is the scenario coverage of "what we learned from each LLM
// CLI in tmux": every auto_respond / escalate / extend / noop branch that the
// production rules must classify, encoded as one truth-table row per scenario.
//
// Pane fixtures are the actual strings captured 2026-05-26 (claude v2.1.150,
// codex v0.133.0, agy 1.0.2) — see knowledge-base/research/tmux-repl-cli-
// behavior-2026-05-26.md. decideAutoRespond is pure (no tmux), so this stays
// deterministic and fast; the real-tmux delivery + live-CLI proofs live in
// tmux_repl_interactive_test.go and tmux_repl_interactive_livecli_test.go.
```

### `go/internal/bridge/autorespond_decision_test.go:49` — above `{"claude rate_limit code-grep → noop (no false escalate)", "claude-tmux",`

```text
// Regression: an agent grepping rate-limit DETECTION CODE prints the
// token "rate_limit" all over its pane. That must NOT be mistaken for a
// real rate-limit banner (the old `rate.?limit` regex false-escalated
// here, killing cycle 113 mid-research). Underscore token ≠ banner.
```

### `go/internal/bridge/autorespond_decision_test.go:62` — above `{"codex model-unsupported 400 → escalate", "codex-tmux",`

```text
// Lane 1676 (2026-09-14): the account rejected the deep-tier model with a
// 400 and the pane sat idle for the whole 20-minute artifact window before
// the runner fell back to claude (exit 81). A dead model is a wall like a
// quota wall — escalate at once so the family fallback runs in seconds.
```

### `go/internal/bridge/autorespond_decision_test.go:69` — above `{"codex model-unsupported quoted far above the tail → noop", "codex-tmux",`

```text
// The same JSON quoted by an agent that is READING an incident record must
// not fire: the rule matches the pane tail only, and the busy gate holds.
```

### `go/internal/bridge/autorespond_decision_test.go:74` — above `{"codex usage-limit ChatGPT quota → escalate", "codex-tmux",`

```text
// Cycle-144: a ChatGPT-account codex auditor hit its quota mid-audit. The
// actual banner ("You've hit your usage limit. Upgrade to Plus to
// continue…") did NOT match the original (usage|rate)[ -]limit
// (reached|exceeded|hit) regex ("hit" precedes "usage limit"), so codex
// sat at the message until the artifact-wait deadline (generic exit 81)
// instead of escalating. Must now fail fast.
```

### `go/internal/bridge/autorespond_decision_test.go:83` — above `{"codex generic upgrade CTA (no limit) → noop", "codex-tmux",`

```text
// Negative guard (cycle-144 review HIGH): the real banner is caught by
// "hit your usage limit", so we deliberately do NOT match a bare
// "Upgrade to Plus to continue" — an agent echoing pricing/doc text with
// that phrase (but no limit reached) must stay noop, not falsely abort.
```

### `go/internal/bridge/autorespond_decision_test.go:90` — above `{"codex per-edit-approval → 1,Enter (cycle-124 G1b)", "codex-tmux",`

```text
// Cycle-124 G1b: per-edit-approval modal that hung cycle-123 tdd.
// '1' selects 'Yes, proceed'. Defense-in-depth behind G1a's --yolo
// boot flag — covers the case where --yolo is dropped/renamed/
// overridden. Pane fragment is the actual cycle-123 capture.
```

### `go/internal/bridge/autorespond_decision_test.go:120` — above `func TestAutoRespond_TrustPromptFiresOnce(t *testing.T) {`

```text
// TestAutoRespond_TrustPromptFiresOnce reproduces the codex-tmux loop-guard
// abandon (exit 86) and pins its fix. A boot-time trust dialog is dismissed by
// one `1,Enter`, but the artifact-wait loop re-captures bootScrollback=200 lines
// every poll — so the DISMISSED dialog text lingers in scrollback and re-matches
// trust_prompt on every subsequent tick. Without a fire-once guard the responder
// keeps re-sending `1,Enter` until counts>5 trips the loop guard and kills the
// run. With `"once": true` on the trust rule, it auto-responds exactly once and
// then noops on later ticks (sharing the live counts map, as ar.counts does
// across the boot→wait phases). The intentional cycle-121 disjuncts are
// untouched — this fixes the re-fire, not the detection.
```

### `go/internal/bridge/autorespond_decision_test.go:222` — above `func TestAutoRespond_CodexPerEditApprovalRegex(t *testing.T) {`

```text
// TestAutoRespond_CodexPerEditApprovalRegex covers each disjunct of the
// cycle-124 G1b regex in isolation. The regex is alternation —
// "Would you like to make the following edits|Press enter to confirm or
// esc to cancel|Yes, proceed" — so any of the three strings as a
// substring should fire `send:1,Enter`. Pinning each branch separately
// catches a regression where someone narrows the regex (e.g., to require
// all three substrings).
```

### `go/internal/bridge/autorespond_decision_test.go:254` — above `{`

```text
// Full cycle-123 modal text — all 3 branches present.
```

### `go/internal/bridge/autorespond_decision_test.go:370` — above `pane := 'Bash(grep -rn '"Yes, proceed"' internal/) ' + "\n" +`

```text
// Agent output that mentions "Yes, proceed" as a string literal in
// what looks like a code grep — exactly the cycle-113-style false
// match pattern. We expect this to STILL fire send:1,Enter (it's
// the documented contract; the safety net is loop_guard).
```

### `go/internal/bridge/autorespond_decision_test.go:388` — above `func TestAutoRespond_ClaudeTrustDialog_v2252(t *testing.T) {`

```text
// TestAutoRespond_ClaudeTrustDialog_v2252 pins the claude 2.1.252 folder-trust
// dialog (live-captured 2026-09-01, fresh dir, --dangerously-skip-permissions).
// The 2.1.193-era rule is a fossil against this pane twice over: its regex
// anchors on the NUMBERED option ("1. Yes, I trust this folder") that 2.1.252
// removed, and its response (Enter) now confirms the NEW default — "❯ No,
// exit" — killing the REPL. Live cost: wave-20260901b, all three lanes
// (cycles 1598/1599/1600) teardown-FAILed in triage with artifact-timeout;
// the phase prompt typed into the modal (submit_wedged resends=3) and the
// nudge's Enter chose "No, exit". Remedy verified live the same day: Down
// moves ❯ to "Yes, I trust this folder", Enter boots a trusted REPL.
```

### `go/internal/bridge/autorespond_decision_test.go:403` — above `pane := ' Accessing workspace:`

```text
// VERBATIM from the live tmux capture (2026-09-01, claude 2.1.252).
```

### `go/internal/bridge/autorespond_decision_test.go:437` — above `func TestAutoRespond_TrustRulesDoNotMatchThisRepositorysOwnFiles(t *testing.T) {`

```text
// TestAutoRespond_TrustRulesDoNotMatchThisRepositorysOwnFiles mirrors the
// plan-mode precedent (TestAutoRespond_PlanModeDoesNotMatchThisRepositorysOwnFiles)
// for the trust_prompt* family: this repo quotes both trust dialogs verbatim
// on tracked files (this test's fixtures, the manifest notes, the boot-path
// fixture, the incident doc), and an agent Reads/cats exactly these files
// while working on the rules. The 2026-09-02 architecture-review probe showed
// the UNANCHORED forms firing on them — a stray Enter into a working agent,
// plus a burned once-budget that would suppress a genuine later dialog. The
// bottom anchor (footer + \z + tail_lines) is what makes this pass: quoted
// dialogs sit mid-document, never at end-of-capture.
```

### `go/internal/bridge/autorespond_planmode_test.go:9` — above `const claudePlanApprovalBypassPane = '   2. hello.go`

```text
// autorespond_planmode_test.go — plan-mode dialogs, the one interactive class
// the responder was blind to.
//
// Both CLIs can reach a blocking plan-mode dialog, and no manifest rule matched
// either. Captured live 2026-08-27 (claude v2.x, codex-cli 0.147.0):
//
//   - claude: an agent called EnterPlanMode from a --dangerously-skip-permissions
//     session — the footer flipped "⏵⏵ bypass permissions on" → "⏸ plan mode on" —
//     and ExitPlanMode raised the approval dialog. Bypass does NOT prevent plan mode.
//   - codex: collaboration_modes is graduated-on; /plan engages "Plan mode" and
//     the agent asks clarifying questions through a blocking picker.
//
// Zero occurrences across 384 retained run dirs (cycles 1217-1574), so these
// rules are a net for a reachable hole, not a fix for an observed burn.
//
// TWO FIXTURE FACTS THAT DROVE THE RULE DESIGN, both found by probing rather
// than by reading:
//
//  1. claude's option 1 text VARIES by launch mode ("Yes, and use auto mode"
//     under --permission-mode plan; "Yes, and switch to BYPASS PERMISSIONS…"
//     under bypass). A rule anchored on option 1 passes a test written from one
//     capture and fails silently in production. Both variants are pinned.
//  2. The live dialog is always at the BOTTOM of the capture (measured: 155
//     chars of tail on claude, 47 on codex, one trailing newline). Both rules
//     are therefore bottom-anchored, which is what makes them safe — see
//     TestAutoRespond_PlanModeDialogs' noop rows.
```

### `go/internal/bridge/autorespond_planmode_test.go:91` — above `{"claude: answered dialog, ONE line of output below", "claude-tmux",`

```text
// --- the bottom anchor: scrollback that scrolled up must NOT fire ---
// This is the rule's load-bearing safety property. A dialog that has
// been answered, or one quoted in a document an agent is reading, sits
// ABOVE later output and therefore cannot match \z. Without this, an
// answered dialog lingering in the capture re-fires every 2s poll and
// trips the loop guard (limit 5) — killing the lane in ~10s. That is
// not hypothetical: codex-tmux.json records exactly that abandon on
// 2026-06-03 from a dismissed trust dialog at bootScrollback=200.
// ONE line of new output is the boundary that matters, and it is the
// boundary an earlier revision of these rules FAILED: with a
// `.{0,N}\s*\z` tail, `.` already matches newlines under (?s), so the
// bound absorbed real output and the rule re-fired on a dismissed
// dialog at 1-3 trailing lines. Those tests passed only because their
// 3-4 line trailers happened to also cross the TailLines window — the
// right answer for the wrong reason. The tails are now anchored to each
// dialog's OWN final line (claude's `ctrl+g … <plan>.md`; codex's
// footer via `[^\n]*`, which cannot cross a newline), so ANY new line
// below the dialog ends the match structurally.
```

### `go/internal/bridge/autorespond_planmode_test.go:118` — above `{"claude: prose quoting the approval sentence", "claude-tmux",`

```text
// --- prose/paraphrase must not fire (the cycle-314 class) ---
```

### `go/internal/bridge/autorespond_test.go:14` — above `func TestDecideAutoRespond_IdleGatesEscalateWhileBusy(t *testing.T) {`

```text
// TestDecideAutoRespond_IdleGatesEscalateWhileBusy pins the ADR-0047 state-gate
// (cycle-314): a policy=escalate match while the CLI is BUSY is the agent
// QUOTING the banner in its output, not the CLI's own chrome — it must NOT
// escalate/bench. A real banner on an IDLE pane still escalates. This catches
// the residual the diff-line strip missed: a BARE (unnumbered) "+\t..." edit
// line carrying the quoted banner.
```

### `go/internal/bridge/autorespond_test.go:88` — above `func TestDecideAutoRespond_AgentDiffContentNotChrome(t *testing.T) {`

```text
// TestDecideAutoRespond_AgentDiffContentNotChrome pins the soak-#4 cycle-314
// false positive: the codex agent editing the clihealth package (the
// rate-limit PARSER) types a test fixture containing "You've hit your usage
// limit" in a numbered-diff line; the escalate rule matched that agent
// content and benched codex 30min on a false rate-limit. CLI rate-limit
// chrome is never a numbered-diff line — agent diff content must be excluded
// from escalate-pattern matching, while a real banner still escalates.
```

### `go/internal/bridge/autorespond_test.go:99` — above `agentEditPane := "" +`

```text
// The exact cycle-314 shape: the agent's editor shows a numbered diff of
// clihealth_test.go, and the footer shows it actively Working.
```

### `go/internal/bridge/autorespond_test.go:125` — above `func TestDecideAutoRespond_BareDiffLineNotChrome(t *testing.T) {`

```text
// TestDecideAutoRespond_BareDiffLineNotChrome pins the cycle-314 RESIDUAL that
// the numbered-diff strip and the busy idle-gate both miss (inbox
// bridge-ratelimit-matches-agent-content, remaining sub-fix): a BARE unified-
// diff line ("+content" / "-content" WITHOUT a leading line number) carrying a
// quoted rate-limit banner is still agent edit content, not CLI chrome. After
// codex finishes an edit the diff lingers in the IDLE scrollback (paneBusy=
// false), so the ADR-0047 idle-gate does not fire and agentDiffLineRE (numbered
// only) does not strip it — the escalate rule then benches codex on the agent's
// own content. The fix must exclude bare diff lines from escalate-pattern
// matching while leaving genuine banner chrome (never diff-prefixed) intact.
```

### `go/internal/bridge/boot_handshake_test.go:1` — above `package bridge`

```text
// boot_handshake_test.go — RED contract for inbox
// codex-update-menu-swallows-injection (2026-06-10T11-10Z, 3× recurrence
// cycles 274/277): the boot loop declared "REPL ready" on a prompt-marker
// SUBSTRING alone. A marker lookalike rendered over a dead shell (codex's
// update menu exited to zsh; the pasted prompt spilled into quote>/bquote>
// continuation) read as ready, the injection landed in the shell, and the
// phase wedged 25+ min behind a false-positive liveness signal.
//
// Contract pinned here (R3.1/R3.2 of the concurrency-factory plan):
//
//  1. Marker visible but the pane's foreground process is a SHELL → NOT
//     ready. The shell set is closed (zsh/bash/…); CLI binary names vary
//     (claude runs under node), so the predicate rejects-known-shell rather
//     than requires-known-binary — degradation-safe for controllers that
//     don't implement PaneCommander.
//  2. Post-paste: the first wait-loop pane (the existing interval baseline,
//     no new capture) showing shell-spill signatures WITH shell-process
//     confirmation → fail fast with ExitREPLBootTimeout (transient → the
//     fallback chain), never a 25-min stall. Mid-run process death stays
//     the observer's job (plan R3.4).
//  3. A pane that merely CONTAINS spill-lookalike text (e.g. a prompt
//     discussing shell errors) while the process is the CLI → ignored.
```

### `go/internal/bridge/boot_handshake_test.go:30` — above `type shellWedgeTmux struct {`

```text
// shellWedgeTmux is a FakeTmuxController whose reported pane process flips
// to a shell after the prompt paste — the exact cycle-274 sequence (codex
// exited to zsh; the paste spilled into the shell).
```

### `go/internal/bridge/boot_handshake_test.go:46` — above `const markerOverDeadShell = '❯ previous output above`

```text
// markerOverDeadShell renders the cycle-274 trap: a stale prompt marker in
// scrollback above a wedged zsh continuation prompt.
```

### `go/internal/bridge/boot_handshake_test.go:116` — above `base := &FakeTmuxController{`

```text
// The pane QUOTES spill text (an agent discussing shell errors) but the
// foreground process is still the CLI → must not fail. The artifact
// appears on paste, so the run completes normally.
// The spill-lookalike frame is queued TWICE: under the cycle-1233 cross-poll
// stability window the artifact completes one tick later than it used to, so
// the pane is captured once more while the deliverable settles. Repeating the
// same frame is what a real settled pane does — and it keeps the lookalike
// text on screen for that extra tick, which is exactly what this test wants
// ignored. The frame budget is grown, not the panic-on-underrun contract
// weakened (see TestFakeTmuxController_UnderrunAfterExhaustion).
```

### `go/internal/bridge/bootsmoke.go:56` — above `rc, _ = d.Launch(ctx, cfg, deps)`

```text
// Dead-shell guard is armed by the real driver constructor (guardDeadShell),
// so smoke boots get the same cycle-274 rejection a phase launch gets.
```

### `go/internal/bridge/bridge_dialog_autorespond_test.go:1` — above `package bridge`

```text
// bridge_dialog_autorespond_test.go — cycle-245 task `bridge-dialog-auto-respond` (RED).
//
// Migration step 6 (carryover `bridge-weak-signal-profiles`, the "biggest
// latency lever ~10min/cycle"): two weak-signal gaps in the tmux bridge.
//
//  1. agy's end-of-session rating dialog ("Rate this response" / "How helpful
//     was this session") has NO rule in agy-tmux.json's interactive_prompts,
//     so the auto-responder noops and the run burns the full artifact-wait
//     window before failing. Contract: the dialog must auto-respond (some key
//     sequence is sent; the run is not stalled by a survey).
//  2. When the artifact-wait review checkpoint finds the agent IDLE with the
//     artifact missing, the driver gives up straight into the
//     ExitArtifactTimeout relaunch path. Contract: before giving up, send
//     EXACTLY ONE in-pane nudge naming the artifact path (the agent often
//     finished the work and just forgot the Write); only on a subsequent
//     idle checkpoint does the existing timeout path proceed.
//
// RED note: compiles against existing API, fails at RUNTIME today —
// the rating pane noops and zero nudges are delivered. Builder makes these
// GREEN via (a) a new interactive_prompts rule in
// go/internal/bridge/manifests/agy-tmux.json (config-only; policy
// auto_respond) and (b) a one-shot nudge guard in driver_tmux_repl.go's
// review-checkpoint path. DO NOT modify this file.
```

### `go/internal/bridge/budget_scale.go:5` — above `func scaledArtifactBudget(base int, scale float64) int {`

```text
// budget_scale.go — ADR-0076 slice A: difficulty-conditioned artifact budgets.
// A BridgeRequest.BudgetScale > 1 (set by the orchestrator for the build phase
// of a medium/large cycle) stretches the artifact-wait deadline the engine
// passes to the driver, so hard cycles stop starving their verification tail.
```

### `go/internal/bridge/budget_scale_test.go:11` — above `func TestScaledArtifactBudget(t *testing.T) {`

```text
// budget_scale_test.go — ADR-0076 slice A (A3): a build launch for a large
// cycle carries a scaled --artifact-timeout-s. The scaling seam is
// scaledArtifactBudget (pure) consumed by launchArgs (the pure extraction of
// Launch's inline arg construction), so the composed flag emission is testable
// without driving a real CLI.
```

### `go/internal/bridge/capture_models.go:30` — above `drv, _, derr := newRecipeDriver(cfg, deps, cli)`

```text
// I1 NOTE (resolved 2026-08-05): liveRefresh now passes a throwaway scratch
// dir as cfg.Workspace, so Workspace-relative writes (escalation reports,
// llm-calls.ndjson, launch errors) land outside the repo and are salvaged
// to .evolve/models-probe before teardown (salvageProbeDiagnostics,
// cmd_models_live.go). Two distinct knobs, easy to conflate: the tmux
// session's CWD is governed by cfg.Worktree (never set on this path — the
// recipe falls back to os.Getwd()); cfg.Workspace only anchors those
// diagnostic writes. Do not "fix" the cwd by pointing Worktree at scratch —
// a picker capture needs no cwd guarantee at all.
```

### `go/internal/bridge/catalog_overlay.go:24` — above `var modelCatalogDirFn = func() string {`

```text
// modelCatalogDirFn resolves the directory holding model-catalog.json.
// Reads BridgePolicy.CatalogDir from policy.json; falls back to the project's
// .evolve directory. Replaced EVOLVE_MODEL_CATALOG_DIR env read (cycle-17).
```

### `go/internal/bridge/channel_e2e_test.go:75` — above `if _, err := os.Stat(artifact); err != nil {`

```text
// Both breadcrumbs in → complete. Written ONCE: under the cycle-1233
// cross-poll stability window (completion.go) a file rewritten on
// every tick keeps bumping its mtime and never settles.
```

### `go/internal/bridge/channel_e2e_test.go:103` — above `paneLivePath := filepath.Join(ws, "build-pane.live")`

```text
// The driver seeks the inbox cursor to EOF at boot, so an ask appended
// BEFORE that seek is skipped forever — and with this test's self-paced
// tmux, a skipped ask means the answer never arrives. A bare sleep here
// lost that race on a loaded CI runner (driver goroutine scheduled late →
// 10m package-timeout panic, macOS 2026-08-03). Sync positively instead:
// the driver creates build-pane.live strictly AFTER the cursor seek in the
// same goroutine, so once the file exists the seek has happened and an
// appended ask is guaranteed to be delivered.
```

### `go/internal/bridge/channel_e2e_test.go:121` — above `var supTick int64`

```text
// The supervisor's clock must ADVANCE: its Ask deadline is computed and
// checked via the injected Now, so the producer's frozen clock here turned
// every lost-answer scenario into an infinite poll — the 10s Timeout could
// structurally never fire, and a missed ask became the package's 10-minute
// timeout panic (macOS CI 2026-08-03) instead of a 10s ErrResponseTimeout
// with diagnostics. One fake millisecond per reading keeps the test free of
// wall-clock timestamps while making the failsafe real.
```

### `go/internal/bridge/codex_model_clamp.go:3` — above `func codexAuthMode(deps Deps) string {`

```text
// codex_model_clamp.go — cycle-142 incident fix. The codex ModelTierMap
// translates a tier to a model, but a ChatGPT/subscription codex account
// 400-rejects models outside its plan tier (the 2026-06 case was gpt-5.4)
// and pops a "Switch to <fallback>?" modal that the auto-responder does not
// dismiss — stalling the phase for the full artifact-wait window and
// surfacing as a generic ExitArtifactTimeout. The clamp substitutes a
// manifest-declared ChatGPT-safe model on subscription auth; API-key auth is
// left untouched so it can still use the larger models.
//
// Auth mode is determined entirely from the launch env — the codex-tmux
// credential-isolation guard already requires BRIDGE_ALLOW_OPENAI_API_KEY=1
// for any OPENAI_API_KEY, so reaching the clamp with an allowed key means the
// operator explicitly opted into API-key mode; everything else is subscription
// ("Sign in with ChatGPT"), which is this driver's documented default.
```

### `go/internal/bridge/codex_model_clamp_test.go:11` — above `func TestCodexAuthMode(t *testing.T) {`

```text
// codex_model_clamp_test.go — cycle-142 incident: the auditor ran codex-tmux
// with model gpt-5.4 (resolved from tier "sonnet"), which a ChatGPT/
// subscription codex account rejects (400 invalid_request_error → model-switch
// modal → 10-min hang → ExitArtifactTimeout). The clamp substitutes a
// ChatGPT-safe model on subscription auth; API-key auth keeps the big model.
```

### `go/internal/bridge/codex_model_clamp_test.go:187` — above `func TestCodexTmuxManifest_HasChatGPTClampPolicy(t *testing.T) {`

```text
// TestCodexTmuxManifest_HasChatGPTClampPolicy pins the data contract: the real
// embedded manifest must declare a ChatGPT-safe set + default, or the clamp is
// inert and cycle-142 regresses silently.
```

### `go/internal/bridge/codex_plan_effort_test.go:8` — above `func TestCodexEffortRealizesPlanModeOverride(t *testing.T) {`

```text
// codex_plan_effort_test.go — plan mode must inherit the phase's reasoning tier.
//
// Codex's plan mode does NOT fall back to model_reasoning_effort. Its own config
// reference states plan_mode_reasoning_effort is a "plan-mode-specific reasoning
// override" and that "when unset, Plan mode uses its built-in preset default" —
// so the general effort flag the loop already passes is simply ignored the
// moment /plan engages.
//
// Observed live 2026-08-27 on codex-cli 0.147.0, launched exactly as the loop
// launches it (`-m gpt-5.6-sol -c model_reasoning_effort=xhigh`):
//
//	before /plan : "gpt-5.6-sol xhigh · … "
//	after  /plan : "gpt-5.6-sol medium · …  Plan mode"     <- silent downgrade
//
// and with the override added (`-c plan_mode_reasoning_effort=xhigh`):
//
//	after  /plan : "gpt-5.6-sol xhigh · …  Plan mode"      <- tier preserved
//
// The model is unaffected either way: there is no plan-mode model key, and
// `model` applies across all modes.
//
// Why this matters beyond tidiness: every existing parity assertion pins LAUNCH
// FLAGS (realizer_realmanifest_test.go, driver_credentials_test.go), so they all
// stay green while the session actually runs a tier lower. A gate that cannot
// see the downgrade is worse than no gate, because it gets cited as evidence the
// downgrade did not happen. Pinning the flag here is the cheap half; the live
// status-bar reading above is the evidence that the flag does the work.
```

### `go/internal/bridge/codex_pretrust.go:14` — above `var tomlKeyEscaper = strings.NewReplacer(`

```text
// tomlKeyEscaper escapes the characters TOML basic strings prohibit
// inside table-key quotes. Per TOML spec §2.4, ALL control characters
// (U+0000–U+001F except U+0009 tab, which is technically allowed) must
// be escaped or the file is unparseable. A path that smuggled a literal
// \n through codexProjectHeader would corrupt ~/.codex/config.toml and
// codex would refuse to start — far worse than the modal-stall this
// helper exists to prevent. Per cycle-122 review HIGH-2 finding.
```

### `go/internal/bridge/codex_pretrust.go:31` — above `func pretrustCodexProjects(cfg *Config) error {`

```text
// pretrustCodexProjects writes per-cycle trust entries into
// ~/.codex/config.toml for cfg.Worktree and cfg.Workspace so codex's own
// permission layer (separate from the bridge's sandbox-exec/bwrap host
// sandbox) treats writes to those paths as allowed and does NOT render
// the "Press enter to confirm" runtime modal that hung cycle-122 tdd.
//
// Cycle-121's research dossier flagged this as "Fix A" deferred at the
// time; cycle-122 made the deferral cost concrete. See
// docs/incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md
// and codex Fix A in
// knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md.
//
// The merge is APPEND-ONLY and idempotent: if a `[projects."<path>"]`
// section is already present, the path is skipped (no duplicate).
//
// Concurrency (ADR-0049 N10): under `evolve fleet` the whole-cycle project
// lock is skipped, so several cycles' codex Preflights pre-trust DISTINCT
// paths into this one host-global file at once. A unique CreateTemp keeps each
// writer's temp private, but the read-merge-write-RENAME was last-writer-wins:
// two cycles that each read a config WITHOUT the other's entry each rename
// their own snapshot, so the final file keeps only the last writer's trust
// entries. The cycle whose entry was dropped then hits the "Press enter to
// confirm" modal that hung cycle-122. The whole RMW now runs under
// flock.WithPathLock(configPath) so the append-only merges serialize and
// compose losslessly — every path stays trusted. (The lock pairs with, it does
// not replace, the atomic CreateTemp+Rename: the lock prevents lost updates,
// the rename keeps lock-free readers tear-free.)
//
// No-ops when cfg.Worktree AND cfg.Workspace are both empty. Returns nil
// (best-effort: a pretrust failure must NOT block phase launch — the
// modal-stall path still defends via Fix 2's extended fallback trigger
// list, and the operator sees the warning on stderr).
//
// Test seam: EVOLVE_CODEX_CONFIG_PATH overrides the resolved path.
```

### `go/internal/bridge/codex_pretrust.go:85` — above `merged = appendCodexNotice(merged)`

```text
// cycle-142: also suppress codex's "Approaching rate limits / Switch to
// <mini>?" model-switch modal, which is undismissable by the auto-responder
// and stalls the phase until the artifact-wait deadline.
```

### `go/internal/bridge/codex_pretrust.go:225` — above `const codexRateLimitNudgeKey = "hide_rate_limit_model_nudge"`

```text
// codexRateLimitNudgeKey is the config.toml key that suppresses codex's
// "Approaching rate limits / Switch to <mini>?" model-switch modal. Without it,
// codex can render an undismissable modal mid-run that stalls the phase until
// the artifact-wait deadline (cycle-142). Per the codex config reference
// ([notice] hide_rate_limit_model_nudge).
```

### `go/internal/bridge/codex_pretrust_amplify_test.go:1` — above `package bridge`

```text
// codex_pretrust_amplify_test.go — Cycle-1 test-amplification adversarial tests.
//
// These probe invariants orthogonal to the TDD 2-goroutine regression guard:
//   - 3-goroutine concurrent pretrust (all entries survive, not just 2)
//   - Pre-seeded file preservation under concurrent writes
//   - Same-path idempotency under concurrent writes (no duplicate sections)
//   - High-stress 10-goroutine scenario under -race
//
// Anti-bias: written from the specification only; implementation not read.
// Run with -race to exercise the flock.WithPathLock serialization path.
```

### `go/internal/bridge/codex_pretrust_concurrent_test.go:11` — above `func TestPretrustCodexProjects_ConcurrentTwoGoroutines(t *testing.T) {`

```text
// TestPretrustCodexProjects_ConcurrentTwoGoroutines is the Slice-2 regression
// test for ADR-0049 N10 (concurrency-arch-slices campaign). Two goroutines each
// pretrust a DISTINCT worktree path against ONE shared EVOLVE_CODEX_CONFIG_PATH
// temp file concurrently, then assert the final TOML contains BOTH
// [projects."..."] entries.
//
// Pre-fix behaviour: read-merge-write-RENAME was last-writer-wins — two
// goroutines that each read the empty initial state before either writes
// overwrite each other's output, leaving only the last writer's entry.
// Post-fix: flock.WithPathLock(configPath) at codex_pretrust.go:79 serializes
// the whole RMW so every append composes losslessly and no entry is dropped.
//
// Run with -race to surface unprotected shared state; the start barrier forces
// both goroutines to read the same (empty) initial state simultaneously,
// maximising contention. -count=5 (eval grader) repeats the race 5 times to
// prevent a lucky interleaving from hiding a regression.
```

### `go/internal/bridge/codex_pretrust_launch_test.go:1` — above `package bridge`

```text
// codex_pretrust_launch_test.go — CB.3 contract (concurrency campaign W4):
// the first codex launch into a FRESH worktree never renders the trust
// prompt, because the trust entry is written BEFORE the REPL boots.
//
// DELIBERATE PLAN DEVIATION (recorded here per the no-silent-changes rule):
// the campaign WBS sketched CB.3 as "a hook beside linkGuardDeps calls
// pretrustCodexProjects at worktree PROVISIONING time". Verified ground truth:
// the launch chokepoint (Engine.LaunchArgs → CLIPreflight dispatch, cycle-124
// G3) already runs codexTmuxDriver.Preflight → pretrustCodexProjects(cfg) for
// the EXACT worktree of every launch — fresh, reused, or resumed — strictly
// before driver.Launch boots the session. A provisioning-time hook would
// duplicate the same TOML write through a new core→bridge seam (core cannot
// import bridge) for zero behavioral gain — the no-duplication command
// outranks plan literalism. What CB.3 therefore ships is the PIN: these tests
// fail if either half of the guarantee (preflight-writes-trust, or
// preflight-before-launch) ever regresses.
```

### `go/internal/bridge/codex_pretrust_launch_test.go:29` — above `func TestCodexTmuxPreflightTrustsFreshWorktree(t *testing.T) {`

```text
// TestCodexTmuxPreflightTrustsFreshWorktree: the acceptance fixture — a fresh
// (never-seen) worktree path handed to the codex-tmux driver's Preflight ends
// up trusted in the codex config, so the boot that follows renders no
// "Press enter to confirm" modal (the cycle-122 tdd hang).
```

### `go/internal/bridge/codex_pretrust_test.go:11` — above `func TestPretrustCodexProjects(t *testing.T) {`

```text
// TestPretrustCodexProjects covers Fix 1 of the cycle-122 remediation
// (see docs/incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md):
// codex-tmux must pre-trust the worktree + workspace paths in
// ~/.codex/config.toml so codex's own permission layer does not prompt
// "Press enter to confirm" at runtime when the agent shells out a
// command that writes outside the worktree boundary.
//
// Test seam: EVOLVE_CODEX_CONFIG_PATH redirects the merge target to a
// per-test tempdir so the real ~/.codex is never touched.
```

### `go/internal/bridge/codex_pretrust_test.go:42` — above `name: "IdempotentWhenAlreadyTrusted",`

```text
// Fully idempotent only when BOTH the trust entries AND the cycle-142
// [notice] rate-limit-nudge suppression are already present; pretrust
// adds the notice alongside the trust entries, so a fixture missing it
// would (correctly) be rewritten.
```

### `go/internal/bridge/codex_pretrust_test.go:138` — above `name:      "PathWithControlChars_AllEscaped",`

```text
// HIGH-2 from cycle-122 review: control characters in a path
// would corrupt config.toml and prevent codex from starting.
// All TOML §2.4 prohibited chars must be escaped.
```

### `go/internal/bridge/codex_pretrust_test.go:215` — above `if tt.worktree != "" && tt.worktree == tt.workspace {`

```text
// MEDIUM-2 from cycle-122 review: when worktree==workspace,
// the section must appear EXACTLY once, not just at-least-once.
```

### `go/internal/bridge/codex_pretrust_test.go:227` — above `func TestPretrustCodexProjects_ConcurrentCalls_AllEntriesSurvive(t *testing.T) {`

```text
// TestPretrustCodexProjects_ConcurrentCalls_AllEntriesSurvive is the N10
// (ADR-0049) regression. Under `evolve fleet` the whole-cycle project lock is
// skipped, so multiple cycles' codex Preflights pre-trust DISTINCT paths into
// the SAME host-global ~/.codex/config.toml concurrently. The unique CreateTemp
// already prevents temp-file clobbering, but the read-merge-write-RENAME was
// last-writer-wins: cycles that each read a config WITHOUT the others' entries
// each rename their own snapshot, so the final file keeps only the last writer's
// trust entries. A dropped trust entry re-arms codex's "Press enter to confirm"
// runtime modal that hung cycle-122 for the cycle whose entry lost the race.
//
// The fix serializes the whole RMW under flock.WithPathLock(configPath), so the
// append-only merges compose losslessly: EVERY path stays trusted. A start
// barrier forces all readers to observe the same (empty) initial state, so the
// pre-fix lost-update trips reliably across the iterations. (This REPLACES the
// old test, which asserted only "at least the LAST writer's entries" — it
// enshrined the very lost-update this fix removes.)
```

### `go/internal/bridge/codex_pretrust_test.go:329` — above `func TestPretrustCodexProjects_WritesHideRateLimitNudge(t *testing.T) {`

```text
// TestPretrustCodexProjects_WritesHideRateLimitNudge — cycle-142: the pretrust
// pass must also suppress codex's "Approaching rate limits / Switch to mini?"
// model-switch modal, which otherwise hangs the phase until the artifact-wait
// deadline. The [notice] key is written alongside the trust entries and is
// idempotent.
```

### `go/internal/bridge/codex_tier_map_test.go:10` — above `const codexDeepModel = "gpt-5.6-sol"`

```text
// codexDeepModel is the 2026-09-10 operator directive: codex's high (deep/top)
// tiers run gpt-5.6-sol at the high reasoning rung — the 2026-09-09 gpt-6-astra
// cutover was withdrawn the next day for token cost (astra burned the
// subscription far faster; the operator keeps sol for codex and opus for
// claude). The reasoning rung is unchanged (the rung is
// pinned by profiles/effort_defaults_test.go — codexDeepTopRung — not here).
// HISTORICAL (superseded): astra was verified live before the 2026-09-09
// cutover — `codex exec -m gpt-6-astra -c
// model_reasoning_effort=high` on the ChatGPT subscription answered. This is
// the ONE value pin for the directive; every other codex tier test asserts
// relationally against the family manifest.
```

### `go/internal/bridge/codex_tier_map_test.go:37` — above `func TestCodexManifest_DeepTopTiers_ValuePin(t *testing.T) {`

```text
// TestCodexManifest_DeepTopTiers_ValuePin pins the directive on the family
// manifest: deep and top resolve to gpt-5.6-sol, the ChatGPT clamp's default
// is gpt-5.6-sol, and the clamp's safe set admits it (otherwise the clamp would
// silently rewrite every deep launch back to the default — the cycle-142
// mechanism working against the directive). The fast/balanced rows are
// untouched by the directive.
```

### `go/internal/bridge/codex_tier_map_test.go:121` — above `func TestResolveTierModel_FollowsManifest(t *testing.T) {`

```text
// TestResolveTierModel_FollowsManifest is the single-source proof for inbox
// `codex-tier-map-single-source`: there is ONE tier→model ladder
// (resolveTierModel) and both transports go through it — the realizer's
// flag emit (Realize) and the headless driver's -m composition. Wiring proof:
// a fixture manifest with a different map is followed on both paths with no
// code edit; legacy aliases (haiku/sonnet/opus) resolve through their canonical
// tier; native ids and genuinely unknown values pass through unchanged (the
// cycle-378 contract).
```

### `go/internal/bridge/codex_tier_map_test.go:177` — above `func TestLaunch_Codex_ManifestUnavailable_OmitsModelFlag(t *testing.T) {`

```text
// TestLaunch_Codex_ManifestUnavailable_OmitsModelFlag covers the driver's
// degradation when the family manifest cannot load — the realistic trigger is
// a corrupt operator override in bridgeManifestDir() (`bridge add-rule` writes
// there and loadManifestRaw does NOT fall back to the embedded copy on a parse
// error). The tier then stays untranslated, the vocabulary guard omits -m (the
// CLI default beats a fatal `-m deep` boot — the cycle-378 class), and the
// WARN names the cause.
```

### `go/internal/bridge/codex_tier_map_test.go:209` — above `func TestCodexFamilyManifest_AstraIsNotSelectable(t *testing.T) {`

```text
// TestCodexFamilyManifest_AstraIsNotSelectable — 2026-09-10 cost directive:
// gpt-6-astra must not be reachable by accident. It is out of the tier table
// AND out of the subscription clamp's safe set, so even a stray pin to it is
// rewritten to the family default instead of burning the subscription.
```

### `go/internal/bridge/completion.go:15` — above `const (`

```text
// completion.go — the phase-completion Strategy (ADR-0027). runTmuxREPL used
// to hardcode one completion contract: poll for a non-empty artifact file
// (artifactReady). But there are several contracts in play —
//   - artifact: the agent's deliverable is a file it writes (scout/build/…);
//   - stdout:   the agent prints its answer to the REPL and writes no file
//     (the router/advisor — a meta phase whose JSON the orchestrator parses);
//   - git-evidence (ADR-0027, a later PR): the agent commits its deliverable
//     and completion is "HEAD advanced + Evolve-Phase trailer verified".
//
// A completionDetector decouples "is the phase done?" from the wait loop so
// the loop body (ADR-0026 stop-review/extend, auto-respond, inbox drain) stays
// identical regardless of contract. The detector ONLY decides readiness;
// liveness (extend vs pause) remains the reviewer's job.
//
// Default ("" / "artifact") preserves the legacy path-poll byte-for-byte, so
// the abstraction is dormant until a phase opts into a different contract.
```

### `go/internal/bridge/completion.go:32` — above `const (`

```text
// The contract vocabulary — what core.BridgeRequest.Completion / Config.
// Completion carry (ports.go:275 documents the request side). Spelled ONCE:
// the factory switches on these names, and completionContractName is the one
// spelling of the "" ⇒ artifact default that the engine's result read and
// the tmux DONE line project (ADR-0103 unit 10, review fold).
```

### `go/internal/bridge/completion.go:125` — above `type artifactBaseline struct {`

```text
// artifactBaseline is the PRE-DISPATCH snapshot of the artifact path: what was
// already on disk before this dispatch's prompt was delivered. Cycle-1550 (and
// the 1554 lane that pinned it): a correction re-dispatch whose prior failed
// attempt left its report at the canonical path had those UNCHANGED bytes
// certified as completion after two stability ticks — no post-dispatch write
// required — so a stale FAIL report re-graded itself forever. An observation
// identical to the baseline is the PRIOR attempt's work: it never begins a
// stability window and never completes, the finality concession included —
// timeout is the honest outcome for an agent that wrote nothing.
```

### `go/internal/bridge/completion.go:175` — above `type gitEvidenceDetector struct {`

```text
// gitEvidenceDetector implements the ADR-0027 git-evidence contract: completion
// = the worktree HEAD advanced to a NEW commit carrying an Evolve-Phase trailer
// for this phase AND the cycle's challenge token. HEAD-advance alone is
// insufficient (a stray/unrelated commit must not false-complete), so the
// trailer is verified; an advance without a matching trailer just re-baselines
// and keeps watching. gitCmd is a seam (default shells `git -C <worktree>` via
// deps.Runner) so the detector is unit-testable without a real repo.
```

### `go/internal/bridge/completion.go:260` — above `type artifactDetector struct {`

```text
// artifactDetector implements the artifact contract: completion = a non-empty
// file at cfg.Artifact (with the cycle-108 non-canonical relocate tolerance)
// that has STOPPED CHANGING. artifactLocate answers "is it there?" without
// touching the file; a cross-poll stability window answers "is it finished?";
// artifactReady then canonicalizes it.
//
// Why the window (cycle-1198): an agent's deliverable is typically a Write
// followed seconds later by an Edit. First-sight completion accepted the
// half-written intermediate — the gate rejected a scout-report.md that parsed
// perfectly moments afterwards. The deliverable-side grace retry
// (deliverable.go) covers absence/emptiness only; a "parses fine, wrong
// content" read is not retried, by design. So the fix belongs here, at the
// source.
//
// The state is carried ACROSS poll calls, not within one: an in-poll settle
// sleep (the rejected cycle-1212 design) is tens of milliseconds and cannot
// span a multi-second Write→Edit gap. The wait loop already calls poll every
// ~2s; that cadence IS the window. mtime is in the stability key because size
// alone is content-blind to an equal-length fix-up Edit.
// The window gates the DESTRUCTIVE relocation, it does not merely follow it
// (cycle-1249). poll's read-only half is artifactLocate; relocateFile — whose
// cross-device branch copies a partial file into the canonical path and then
// REMOVES the source the agent is still appending to — runs only on the tick the
// window closes. Relocating that way on first sight would defeat the debounce on
// the very path scout flagged as highest-risk, leaving a permanently stable,
// permanently truncated deliverable that the window then certifies as finished.
//
// Precisely stated, because the earlier wording of this paragraph claimed more
// than the code did and cycle-1256 audited it as a refuted safety claim (D2):
// the ONE path that completes without a closed window — the finality
// short-circuit below — still canonicalizes a non-canonical fallback, but it is
// restricted to renameOnlyRelocate. Rename relinks an inode and so is safe for a
// file that may still be growing; copy+remove is not, and under finality it
// never runs.
```

### `go/internal/bridge/completion.go:311` — above `if isFinalPoll(ctx) || ctx.Err() != nil {`

```text
// Final look: this is the wait loop's ONE last poll before it gives up
// (driver_tmux_repl.go). Demanding a fresh window it can never get would
// launder every finished-at-the-buzzer session into ExitArtifactTimeout —
// turning a truncated-read fix into a worse false-FAIL generator. The
// artifact is on disk, which is the evidence; short-circuit. Checked AFTER
// artifactLocate, whose found result already proves a non-empty artifact
// exists, so finality can never manufacture completion from nothing.
//
// stable is 0 here — no window was ever closed on this artifact — so the
// mover is renameOnlyRelocate, NOT relocateFile (cycle-1256 D1). Completing
// on an unwitnessed artifact is a deliberate, bounded concession; deleting
// the agent's source file after snapshotting it half-written is not, and
// that is exactly what relocateFile's copy+remove branch does. Rename-only
// keeps the concession reversible: worst case the canonical path holds a
// file the agent's fd is still appending into, best case a finished one, and
// never a truncated snapshot with the original destroyed. If the rename
// cannot be done, the poll reports the error and the phase takes its
// artifact timeout — the honest outcome for "we could not safely finish".
//
// Two keys, deliberately: isFinalPoll is the explicit signal the wait loop
// now sends (its final context is LIVE, so ctx.Err() would never fire there
// again), and ctx.Err() still covers a detector polled on a context that
// died under it mid-wait — the pre-existing contract, unchanged.
```

### `go/internal/bridge/completion.go:335` — above `if fi, serr := os.Stat(path); serr == nil && d.baseline.matches(path, fi) {`

```text
// The finality concession stops at the pre-dispatch baseline: an
// artifact byte-identical to what was on disk BEFORE the prompt went
// out is the prior attempt's report, and completing on it launders a
// stale verdict into a fresh one (cycle-1550). Only an affirmative
// match refuses — a stat error here keeps the concession (fail-open).
```

### `go/internal/bridge/completion.go:352` — above `if d.baseline.matches(path, fi) {`

```text
// The unchanged PRE-DISPATCH artifact never begins a window: those bytes
// predate this dispatch's prompt and are the prior attempt's work, not
// evidence this agent finished (cycle-1550 — the stale-FAIL re-grade loop).
```

### `go/internal/bridge/completion_baseline_test.go:3` — above `import (`

```text
// completion_baseline_test.go — the pre-dispatch artifact baseline
// (cycle-1550; red test salvaged from the 1554 lane's ADR-0076 continuation
// snapshot). A correction re-dispatch whose prior failed attempt left its
// report at the canonical path had those UNCHANGED bytes certified as
// completion after two stability ticks — no post-dispatch write required —
// so a stale FAIL verdict re-graded itself on every retry. The contract: an
// observation identical to the pre-dispatch baseline never begins a stability
// window and never completes, the finality concession included; the moment
// the agent actually writes (mtime/size change), everything behaves as before.
//
// newArtifactDetectorAt (the baseline-FREE helper) deliberately stays for the
// existing suite: those tests write files as harness setup for
// "agent-wrote-mid-session" scenarios, which is exactly what an absent
// baseline models. Only these tests construct with a captured baseline, the
// way production does before prompt delivery.
```

### `go/internal/bridge/completion_baseline_test.go:111` — above `func TestRunTmuxREPL_StalePreDispatchArtifactTimesOutInsteadOfCompleting(t *testing.T) {`

```text
// PRODUCTION-PATH wiring pin (the layer the unit tests cannot see): the
// baseline capture happens inside runTmuxREPL itself, BEFORE prompt delivery,
// and is threaded into the detector. Driven through the full engine
// (LaunchArgs → runTmuxREPL): a stale artifact left at the canonical path by
// a prior failed attempt, with a live session that never writes, must drive
// the run to ExitArtifactTimeout — pre-fix, the detector certified the stale
// bytes after two stability ticks and the run "completed" with the prior
// attempt's verdict (cycle-1550's re-grade loop).
```

### `go/internal/bridge/completion_cancel_parity_test.go:3` — above `import (`

```text
// completion_cancel_parity_test.go — the RED contract for cycle-1236's
// completion-contract-cancel-parity.
//
// The defect. When the wait loop's context is cancelled (orchestrator timeout,
// SIGTERM, the next phase tearing the session down) it takes ONE final
// completion poll before giving up, so a session that finished at the buzzer is
// not laundered into ExitArtifactTimeout (driver_tmux_repl.go:565-582, the
// c2258b72 fix). That final poll is handed the ALREADY-CANCELLED ctx, and the
// comment says why that was thought safe: "the artifact detector is a pure file
// stat, so the dead ctx cannot fail this last look".
//
// True for exactly ONE of the three completionDetector implementations the same
// line dispatches to (driver_tmux_repl.go:481 builds the detector from
// cfg.Completion):
//
//	artifactDetector      — os.Stat, ctx-free, plus an explicit short-circuit
//	stdoutDetector        — deps.Tmux.CapturePane(ctx, …)     (completion.go:266)
//	gitEvidenceDetector   — deps.Runner(ctx, "git", …)        (completion.go:106)
//
// exec.CommandContext REFUSES to start a process on an already-cancelled
// context, so for the latter two the final look cannot run at all: the transport
// errors, both detectors correctly swallow that as "not ready" (right policy for
// a transient mid-wait capture failure), and the finished session exits
// ExitArtifactTimeout. The benign-teardown grace exists for one contract out of
// three.
//
// The trap this contract also pins. The artifact short-circuit
// (completion.go:213) is keyed on `ctx.Err() != nil`. Handing the final poll a
// LIVE context — the obvious fix — silently switches that short-circuit off, and
// artifactDetector then demands a fresh 2-tick stability window it can never
// accrue inside one call. So the naive fix regresses the one contract that
// already works. Finality must be signalled to the detector EXPLICITLY rather
// than inferred from cancellation; AC-4 below is the guard.
//
// Test map (every case drives the REAL production wait loop via
// Engine.LaunchArgs — a detector polled in isolation proves nothing about the
// caller that starves it, and the caller IS the fault site):
//
//	AC-1 stdout parity — CancelAfterIdle_CompletesNotTimeout (+ negative)
//	AC-2 git parity    — CancelAfterEvidenceCommit_CompletesNotTimeout (+ negative)
//	AC-3 transport     — the fakes REFUSE to run on a dead ctx, so a fix that
//	                     merely re-orders code without supplying a usable context
//	                     cannot pass. This is the anti-no-op axis.
//	AC-4 artifact non-regression — the pre-existing guards
//	                     TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout and
//	                     TestArtifactDetector_CtxCancelledShortCircuitsDebounce must
//	                     still hold after the finality signal is re-keyed.
```

### `go/internal/bridge/completion_contract_test.go:3` — above `import (`

```text
// completion_contract_test.go — ADR-0103 unit 10, review fold (architecture
// MEDIUM 1): the phase-completion contract vocabulary is spelled ONCE. The
// "" ⇒ artifact default was three beliefs — completion.go's `default:` arm,
// driver_tmux_repl.go's DONE line and engine.go's readResult (the
// BRIDGE_RESULT_READ_FAILED `completion` field) — so a renamed or re-pointed
// default would have left the triage field lying silently.
```

### `go/internal/bridge/completion_debounce_test.go:14` — above `var fixedMTime = time.Unix(1_700_000_000, 0)`

```text
// completion_debounce_test.go — the RED contract for cycle-1233's
// artifact-ready-crosspoll-debounce.
//
// The defect (cycle-1198, observed): artifactDetector.poll completes on the
// FIRST non-empty read of the deliverable. An agent that writes a file and then
// fixes it up with a follow-up Edit seconds later gets its half-written
// intermediate accepted — the gate rejected a scout-report.md that parsed
// perfectly moments afterwards. The deliverable-side grace window that already
// shipped (deliverable.go:180) covers absence/emptiness only; a
// "parses fine, wrong content" read is not retried, by design.
//
// The fix is the artifact twin of stdoutDetector's stdoutIdlePolls debounce:
// artifactDetector must observe the SAME (size, mtime) across
// artifactStableTicks consecutive poll ticks (~2s apart, driven by the wait
// loop at driver_tmux_repl.go:562) before declaring ready. mtime participates
// because size alone is content-blind to a same-length rewrite.
//
// Explicitly NOT the fix (rejected, HIGH review of the cycle-1212 attempt): an
// in-poll settle sleep. A tens-of-ms sleep inside one poll() call cannot span
// the multi-second gap between an agent's Write and its Edit. The state must be
// carried ACROSS calls, on the detector.
//
// Test map:
//
//	AC-1 stability  → ReadyOnlyAfterCrossPollStability (positive) +
//	                  NotReadyWhileArtifactStillGrowing (negative, size axis) +
//	                  NotReadyOnSameSizeRewrite (negative, mtime axis)
//	AC-2 ctx-cancel → CtxCancelledShortCircuitsDebounce (+ its absent-file negative)
//	AC-3 relocation → RelocationNoteSurvivesUntilStable
//	AC-4 wiring     → TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop (drives the
//	                  REAL production wait loop; a detector-only test proves nothing
//	                  about the caller) + the untouched fixture-budget regression
//	                  guard TestRunTmuxREPL_ExtendNoEscalationReport.
```

### `go/internal/bridge/completion_debounce_test.go:96` — above `func TestArtifactDetector_ReadyOnlyAfterCrossPollStability(t *testing.T) {`

```text
// TestArtifactDetector_ReadyOnlyAfterCrossPollStability pins the positive half
// of AC-1: a settled file completes, but never on the tick it is first seen.
// The first-sighting assertion is the whole point — cycle-1198's truncated
// deliverable was a perfectly non-empty file on exactly that tick.
```

### `go/internal/bridge/completion_debounce_test.go:154` — above `func TestArtifactDetector_NotReadyOnSameSizeRewrite(t *testing.T) {`

```text
// TestArtifactDetector_NotReadyOnSameSizeRewrite is the negative half of AC-1 on
// the MTIME axis, and the reason mtime is in the key at all: an agent's fix-up
// Edit that swaps equal-length text (a typo, a flipped verdict word) leaves the
// size identical. A size-only debounce silently degrades back to the cycle-1198
// bug for exactly that shape.
```

### `go/internal/bridge/completion_debounce_test.go:218` — above `func TestArtifactDetector_RelocationNoteSurvivesUntilStable(t *testing.T) {`

```text
// TestArtifactDetector_RelocationNoteSurvivesUntilStable pins AC-3. artifactReady
// returns relocatedFrom exactly ONCE — on the tick it moves a non-canonical
// write into place (cycle-108/141 tolerance). Every later tick sees the file at
// the canonical path and returns from == "". A debounce that discards the note
// of a not-yet-stable tick therefore permanently swallows the "the agent wrote
// to the wrong place" diagnostic. The detector must stash it.
```

### `go/internal/bridge/completion_debounce_test.go:287` — above `if code != ExitArtifactTimeout {`

```text
// Assert the POSITIVE outcome, not merely "not ExitOK". A churning
// deliverable must exhaust the stop-review budget and die as
// ExitArtifactTimeout; every other non-OK exit means the driver never
// reached the artifact wait loop at all (REPL boot timeout, dead-shell
// guard, auto-respond escalation/loop-guard — each of which narrates itself
// on stderr). `!= ExitOK` accepted all of those as proof of a debounce they
// never exercised, and the reviewer-count guard below then failed with a
// count and nothing else: cycle-1252's audit red was exactly this shape
// ("reviewer ran 0 time(s)", red-on-retry) and was undiagnosable afterwards
// because the driver's own explanation was discarded. Both assertions now
// carry stderr, so a recurrence names its cause instead of its symptom.
```

### `go/internal/bridge/completion_debounce_test.go:349` — above `func TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv(t *testing.T) {`

```text
// TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv is the
// ENVIRONMENT-invariance regression, and the reason it is pinned to the caller
// proof above rather than living in a general hygiene test: the debounce's
// gating predicate is exactly the one that kept losing runs to this.
//
// Cycles 1252 and 1254 were both FAILed at audit by the caller proof going red
// in the ACS/EGPS gate while the Builder, the Auditor and `evolve selfcheck
// build` all ran it green. Not a flake — a one-variable difference. The gate
// (internal/acsrunner/runner.go) shells `go test` with a bare exec and no
// cmd.Env, so it inherits the orchestrator's EVOLVE_FLEET=1 (internal/fleet).
// runTmuxREPL reads that key through lookupEnv, and with a nil Deps.LookupEnv
// the read reached the ambient process env: under a fleet supervisor with no
// --worktree, the CB.2 guard correctly refuses with errWorktreeRequired ->
// ExitBadFlags(10) BEFORE the artifact wait loop ever runs. 20 tests in this
// package failed that way with the variable set and 0 with it unset.
//
// The guard is right and stays untouched. What was wrong was fixtures reading
// the ambient environment at all, fixed at the SOURCE in newTestEngine
// (launch_test.go) rather than by sanitizing EVOLVE_* at yet another consumer —
// internal/core's sanitizeEnv is precisely that consumer-side workaround, and
// it names this failure in its own comment, which is why the defect survived
// to bite two more cycles.
//
// This test asserts the invariant directly: with EVOLVE_FLEET=1 exported into
// the process, the caller proof must still reach the wait loop and time out.
// It fails with exit 10 if anyone reintroduces an ambient env read here.
```

### `go/internal/bridge/completion_finalpoll_relocate_test.go:11` — above `func freezeSourceDir(t *testing.T, dir string) {`

```text
// completion_finalpoll_relocate_test.go — cycle-1258's closure of the cycle-1256
// audit findings against the artifact cross-poll debounce.
//
// The window that completion_debounce_test.go and
// completion_relocate_stability_test.go pin is correct on honest polls. The
// audit found it bypassed on the ONE path every cancelled or timed-out phase
// takes, and found that path carrying 0 test hits after 716 new test lines
// (D5). Everything here exercises a branch that was previously unentered:
//
//	D1 — artifactDetector.poll's isFinalPoll/ctx.Err short-circuit reached the
//	     mover with stable==0, so relocateFile's copy+remove could snapshot a
//	     still-growing fallback into the canonical path and DELETE the source.
//	     Fix: finality uses renameOnlyRelocate.
//	D3 — artifactLocate qualified candidates with os.Stat, following symlinks, so
//	     a planted link promoted arbitrary readable bytes into the deliverable.
//	     Fix: regularFileNonEmpty (os.Lstat + IsRegular).
//	D4 — relocateFile wrote a predictable "<dst>.tmp.<pid>". Fix: os.CreateTemp.
//
// Test design note on the D1 tests. The harm needs a rename that FAILS while a
// copy would succeed — otherwise the two movers are indistinguishable and the
// test proves nothing. `os.Rename` needs write permission on the SOURCE's
// directory (it unlinks the old entry); reading the file does not. So chmodding
// the fallback's directory to r-x forces exactly relocateFile's copy+remove
// branch without needing a second filesystem, and does it deterministically on
// every platform the loop runs on.
```

### `go/internal/bridge/completion_finalpoll_relocate_test.go:98` — above `func TestArtifactDetector_FinalPollStillCanonicalizesByRename(t *testing.T) {`

```text
// TestArtifactDetector_FinalPollStillCanonicalizesByRename is the honest
// counterweight: "never relocate under finality" would pass the test above and
// silently break the cycle-108/141 fallback tolerance for every phase that
// finishes at the buzzer. Rename is safe and must still happen.
```

### `go/internal/bridge/completion_finalpoll_relocate_test.go:136` — above `func TestArtifactDetector_SettledFallbackStillUsesTheFullMover(t *testing.T) {`

```text
// TestArtifactDetector_SettledFallbackStillUsesTheFullMover proves the guard is
// scoped to FINALITY and is not a blanket disable of copy+remove. Same frozen
// source directory, but the window is allowed to close honestly: the artifact
// has been observed unchanged across artifactStableTicks ticks, so copying it is
// safe and the cycle-108/141 tolerance must still work on a cross-device source.
```

### `go/internal/bridge/completion_relocate_stability_test.go:11` — above `func growingFallback(t *testing.T, path string, i int) {`

```text
// completion_relocate_stability_test.go — the RED contract for cycle-1249's
// residual half of artifact-ready-crosspoll-debounce.
//
// What already landed (cycle-1233, folded in by 841e676f; verified-and-closed by
// cycle-1236 predicate 004): artifactDetector carries a (size, mtime) stability
// window ACROSS poll ticks, so a deliverable still being written at the CANONICAL
// path never completes the phase. completion_debounce_test.go pins that half.
//
// The hole that half does NOT close. artifactDetector.poll's very first
// statement is `artifactReady(d.cfg)` (completion.go:223), and artifactReady
// RELOCATES a non-canonical fallback the instant it observes it non-empty
// (driver_common.go, the cycle-108/141 tolerance) — before any stability
// observation has been made about that fallback. The debounce is therefore
// applied strictly downstream of an irreversible move:
//
//	tick 1: agent is mid-write at <worktree>/report.md (size>0, incomplete)
//	        → artifactReady relocates it to the canonical path, source removed
//	tick 2+: the window runs at the canonical path
//
// On the rename branch this is survivable — rename preserves the inode, so the
// agent's still-open fd keeps appending at the canonical path and the window
// then sees the growth. On the COPY+REMOVE branch (relocateFile's cross-device
// fallback) it is not: the copy snapshots a partial file, `.tmp.<pid>`-renames
// that truncated snapshot into the canonical path, and then REMOVES the source
// the agent is still writing to. The result is a permanently stable, permanently
// truncated deliverable — the debounce declares it finished, and the bytes the
// agent wrote after the copy are gone. That is the exact mid-write-truncation
// class deliverable.go:180 names this mechanism as the source-side closure of,
// reached through the path scout flagged (hypothesis 2) as the highest-risk one
// precisely because it carries an extra copy step.
//
// The fix contract: the stability window must gate the RELOCATION, not merely
// follow it. The detector must observe the artifact — wherever it currently
// lives, canonical or fallback — settle across artifactStableTicks consecutive
// ticks BEFORE the file is moved. A fallback that is still growing must be left
// exactly where it is.
//
// Test map (each AC gets a positive and a negative so no-op cannot pass):
//
//	AC-1249-1 defer-move    → RelocationDeferredWhileFallbackStillGrowing (negative:
//	                          the move must NOT happen) +
//	                          RelocationHappensOnceFallbackSettles (positive: it must
//	                          still happen, so "never relocate" is not a passing fix)
//	AC-1249-2 no-regression → RelocatedCompleteFallbackStillCompletes (the cycle-108/141
//	                          tolerance and its single-shot diagnostic survive)
```

### `go/internal/bridge/completion_relocate_stability_test.go:101` — above `func TestArtifactDetector_RelocationHappensOnceFallbackSettles(t *testing.T) {`

```text
// TestArtifactDetector_RelocationHappensOnceFallbackSettles is the honest
// counterweight: "never relocate anything" would pass the negative above and
// break the cycle-108/141 tolerance outright. Once the fallback stops changing,
// the detector must move it to the canonical path, complete, and still carry the
// wrote-to-the-wrong-place diagnostic.
```

### `go/internal/bridge/completion_secondary_test.go:3` — above `import (`

```text
// completion_secondary_test.go — Phase B pins (plan parallel-weaving-wolf,
// ADR-0084 lineage): the artifact detector must HOLD phase-complete while a
// contract secondary is absent, so the session survives long enough for the
// agent to write it — the single-artifact cutoff killed retro's
// disposition.json on 86/88 recent cycles and audit's
// defect-dispositions.json across 1397-1429. The settle window itself stays
// primary-only (cycle-1210/1212 race design), and phases with no secondaries
// are byte-identical.
```

### `go/internal/bridge/completion_test.go:12` — above `func TestNewCompletionDetector_DefaultsToArtifact(t *testing.T) {`

```text
// completion_test.go — unit coverage for the ADR-0027 completion Strategy:
// the factory's default-to-artifact safety, the artifact detector's parity
// with artifactReady, and the stdout detector's idle-debounce + activity
// gating (the router/advisor's contract).
```

### `go/internal/bridge/completion_test.go:40` — above `if err := os.WriteFile(canonical, []byte("DONE\n"), 0o644); err != nil {`

```text
// Present (non-empty) and UNCHANGING → ready with an "appeared" note.
// Readiness is no longer first-sight: cycle-1233 added the cross-poll
// stability window (see completion_debounce_test.go for that contract), so
// parity with artifactReady is asserted at the point the window closes.
```

### `go/internal/bridge/completion_test.go:127` — above `fx := newFixture(t, "claude-tmux", "")`

```text
// The stdout contract completes WITHOUT any artifact file: the REPL boots
// (marker in pane[0]), shows activity, then settles on the marker — and the
// driver returns ExitOK even though fx.artifact was never written. This is
// the cycle-117 advisor deadlock, fixed.
```

### `go/internal/bridge/contextfill_contributors_test.go:3` — above `import (`

```text
// contextfill_contributors_test.go — WIRING half of the cycle-1482 task
// `context-fill-warning-attribution` RED contract. This is a REACHABILITY
// test, not a unit test: it drives the real production caller
// (Engine.recordTokenUsage) and asserts the contributor breakdown attached to
// the CONTEXT-FILL WARN comes from the SAME basis result.FillPct was derived
// from — never a second, disagreeing total. A test that called
// tokenusage.FillWarnWithContributors directly would pass on dead code.
//
// RED: tokenusage.Result carries no PeakUsage field yet, so this file fails to
// COMPILE until Builder adds it and wires recordTokenUsage to prefer
// result.PeakUsage over result.Usage whenever result.PeakPromptTokens != 0 —
// the same distinction fillpct.go's windowOccupancy already documents for the
// percentage itself (compile-fail = RED evidence).
//
// Reuses runContextFillCase's sibling helpers (contextFillLine) from
// contextfill_warn_test.go, same package.
```

### `go/internal/bridge/contextfill_contributors_test.go:31` — above `func TestContextFillWarn_ContributorsMatchPeakPromptReading(t *testing.T) {`

```text
// TestContextFillWarn_ContributorsMatchPeakPromptReading is the cycle-1458 M1
// continuation predicate: the contributor breakdown attached to a fill WARN
// must be measured on the SAME basis as the percentage it annotates — the
// fullest single observed turn — never the whole-launch summed total
// (adversarial-review F1: a 70% reading annotated with contributor figures
// that total far more than the window).
//
// Fixture: an early, large-cache_read turn dominates the whole-launch SUM, but
// a LATER, smaller turn is the actual PEAK (the one the resolver already
// selected for FillPct via windowOccupancy). Only a fix that carries the peak
// turn's own components through to the contributor breakdown can pass.
```

### `go/internal/bridge/contextfill_warn_test.go:3` — above `import (`

```text
// contextfill_warn_test.go — WIRING proof for cycle-1444 task
// `context-fill-warn-threshold`. This is a REACHABILITY test, not a unit test:
// it drives the real production caller (Engine.recordTokenUsage, engine.go:640 —
// the one site every Launch funnels its token telemetry through) and asserts the
// fill WARN and the persisted fill_pct both come out of THAT path. A test that
// called tokenusage.FillWarn directly would pass on dead code.
//
// RED: Deps.ContextFillWarnPct, tokenusage.Result.FillPct, the llm-calls
// fill_pct field and the WARN emission do not exist yet — this file fails to
// COMPILE until Builder adds them (compile-fail = RED evidence).
```

### `go/internal/bridge/contextfill_warn_test.go:30` — above `var contextFillMarker = string(CodeContextFillHigh)`

```text
// the code the rendered bridge.warning line carries (ADR-0101 S3)
```

### `go/internal/bridge/coverage_batch7_test.go:51` — above `func TestPreparePrompt_ReadsExistingChallengeToken(t *testing.T) {`

```text
// TestPreparePrompt_ReadsExistingChallengeToken (cycle-136 lesson, PR 7):
// when workspace/challenge-token.txt already exists (orchestrator minted +
// wrote it at cycle start per PR 6), preparePrompt MUST reuse the existing
// value and NOT mint+overwrite. One token per cycle is the invariant; the
// bridge's per-phase mint was overwriting the orchestrator's token, causing
// scout-report.md to use the orchestrator's token (plumbed via Context per
// PR 6 / scout.go:64) while later phases saw the bridge's new token in the
// workspace file. Cycle 136 audit C1 surfaced the divergence.
```

### `go/internal/bridge/delivery_failure_errhelper_test.go:9` — above `func errorsIsArtifactTimeout(err error) bool { return errors.Is(err, core.ErrArtifactTimeout) }`

```text
// errorsIsArtifactTimeout keeps the sentinel check in one place for the
// cycle-1562 delivery-failure contract: a delivery failure must remain an
// ErrArtifactTimeout (exit 81), because that is the sentinel
// core.IsInfraTeardownError matches to perform its ONE bounded relaunch.
// Reclassifying it to any other sentinel would silently remove the recovery
// this task exists to reach.
```

### `go/internal/bridge/driver.go:33` — above `type CLIPreflight interface {`

```text
// CLIPreflight is an OPTIONAL Driver capability for per-CLI prep work that
// must complete BEFORE the inner CLI process is launched. The Engine
// dispatches it via type assertion (`driver.(CLIPreflight)`) so a driver
// that needs no prep work simply omits the method — no no-op stubs in every
// concrete driver. Establishing this seam (cycle-124 G3, redesign of the
// inline pretrust call at the top of codexTmuxDriver.Launch) gives every
// CLI a uniform place to mutate config files / refresh credentials / probe
// the binary BEFORE the user-visible launch path runs. Today only
// codex-tmux implements it (pre-trust worktree + workspace paths in
// ~/.codex/config.toml per cycle-122 Fix 1); claude-tmux / agy-tmux /
// ollama-tmux opt out by not declaring the method.
//
// Semantics: best-effort. The Engine LOGS a non-nil error to stderr but
// continues to Launch — this matches the existing inline call's posture
// (Fix 2's extended fallback trigger list is the downstream defense
// against any preflight failure). A driver that needs Preflight to be
// load-bearing (abort launch on failure) MUST encode that in its own
// Launch body, not here.
```

### `go/internal/bridge/driver_agytmux.go:5` — above `type agyTmuxDriver struct{}`

```text
// agyTmuxDriver drives an interactive `agy` (Gemini-backed) TUI through
// tmux — the Go port of drivers/agy-tmux.sh. agy 1.0.15 selects its model
// via the --model launch flag (display-name tokens, cycle-447 probe); it has
// no permission-mode, renders alt-screen (boot wait reads scrollback), its
// ready marker is the "? for shortcuts" footer, and it exits via Ctrl+C ×2.
```

### `go/internal/bridge/driver_agytmux.go:23` — above `return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{`

```text
// Launch flags come from the per-CLI Realization (ADR-0022): agy realizes
// permission=bypass → --dangerously-skip-permissions and model_tier →
// --model "<display name>" (agy 1.0.15, cycle-447; launchCmdLine quotes
// the space/paren tokens). claude-keyed raw flags realize to nothing.
```

### `go/internal/bridge/driver_artifact_relocate_test.go:11` — above `func TestArtifactReady_CanonicalPresent(t *testing.T) {`

```text
// driver_artifact_relocate_test.go — artifactReady tolerance for the
// cycle-108 ExitArtifactTimeout root cause: agents intermittently wrote the
// report to <workspace>/workspace/<file> (reading the doc's "workspace/"
// prefix as a literal subdir) while the driver polls only the canonical
// <workspace>/<file>. artifactReady accepts either location and relocates the
// non-canonical write so downstream phases — which read the canonical path —
// still resolve it. See docs/architecture/adr/0024-*.md (Step 0).
```

### `go/internal/bridge/driver_artifact_relocate_test.go:167` — above `func TestArtifactReady_RelocatesFromWorktreeRoot(t *testing.T) {`

```text
// TestArtifactReady_RelocatesFromWorktreeRoot — cycle-141 ExitArtifactTimeout
// root cause: the builder runs with cwd=worktree (driver_tmux_repl.go:77) and
// the prompt names the artifact by bare relative path ("Write build-report.md"),
// so the agent writes it into the worktree root — a path the driver did not
// poll. artifactReady must search the worktree cwd and relocate to canonical.
```

### `go/internal/bridge/driver_artifact_relocate_test.go:227` — above `func TestArtifactReady_WorkspaceSubdirWinsOverWorktree(t *testing.T) {`

```text
// TestArtifactReady_WorkspaceSubdirWinsOverWorktree — when the artifact exists
// in BOTH the workspace/ subdir (cycle-108 path) and the worktree (cycle-141
// path), the workspace/ subdir is preferred: it is closest to the canonical
// location and is the path most agents already use. Pins search-order priority.
```

### `go/internal/bridge/driver_artifact_relocate_test.go:261` — above `func TestArtifactReady_NoWorktreeConfigured_UnchangedBehavior(t *testing.T) {`

```text
// TestArtifactReady_NoWorktreeConfigured_UnchangedBehavior — when cfg.Worktree
// is empty (headless drivers, probes), behavior is byte-identical to the
// pre-cycle-141 code: only the canonical path + the workspace/ subdir count.
```

### `go/internal/bridge/driver_claudep.go:10` — above `func claudePArgs(cfg *Config, prompt string) (args []string, omittedModel string) {`

```text
// claudePArgs builds the `claude -p` argv from cfg and the prepared prompt.
// Pure — extracted from Launch so flag emission is testable without driving a
// real CLI (the same seam as engine.launchArgs).
//
// omittedModel is the model value that was SUPPRESSED, empty when none was.
// An unresolved vocabulary token (isUnresolvedModelToken) must not be sent:
// `claude -p --model top` fails exactly like the cycle-262 `--model auto`
// incident, and this driver builds its own argv, so the realizer's guard never
// saw it. An empty cfg.Model is "nothing requested" rather than "a request we
// refused", so it emits no flag and reports no suppression.
```

### `go/internal/bridge/driver_claudetmux.go:65` — above `tickDuringBoot:  true,`

```text
// claude shows boot-time folder-trust dialogs whose ❯ cursor collides with the REPL marker (v2.1.193 numbered/Yes-default; v2.1.252 unnumbered/No-default — see the manifest trust_prompt* rules)
```

### `go/internal/bridge/driver_claudetmux_test.go:215` — above `dialogs := []struct {`

```text
// Regression, table-ized over BOTH dialog generations (2026-09-02 review):
// each renders its selection cursor as ❯ — the same char claude-tmux uses
// as its REPL prompt marker — so the boot loop must auto-dismiss the
// dialog (tick) BEFORE delivering the prompt, or the paste lands in the
// dialog and is lost (rc=81 artifact-timeout). v2.1.193: numbered options,
// Yes pre-highlighted, one Enter. v2.1.252: unnumbered, default flipped to
// "No, exit", Down+Enter — the dialog that burned all three
// wave-20260901b lanes (see docs/incidents/
// 2026-09-01-claude-2252-trust-default-flip.md). Same class as the
// Cycle-121 codex trust-modal bug.
```

### `go/internal/bridge/driver_codex.go:10` — above `type codexDriver struct{}`

```text
// codexDriver is the OpenAI Codex CLI driver — the Go port of
// drivers/codex.sh (`codex exec --output-last-message`). Codex exposes no
// --permission-mode FLAG (it uses approval_policy/sandbox_mode instead), so it
// rejects permission_mode loudly rather than silently ignoring an operator's
// safety declaration. NOTE: this is about the flag, not the feature — codex DOES
// have plan mode (0.147.0 ships collaboration_modes graduated-on; entry is
// `/plan` or Shift+Tab, in-session). An earlier form of this comment said "codex
// has no claude-style plan mode" and was read as settling the capability
// question; see docs/incidents/2026-08-27-plan-mode-dialog-blind-spot.md.
```

### `go/internal/bridge/driver_codex.go:74` — above `if !argsContainEffort(cfg.Realization.LaunchFlags) && !argsContainEffort(cfg.ExtraFlags) {`

```text
// Codex's SECOND model layer (2026-08-15 operator directive): pin the
// reasoning effort so the headless path never runs the CLI's own default —
// mirrors the tmux manifest's params.effort default=high. Skipped when the
// caller already set one (raw flags / extra flags own the override).
```

### `go/internal/bridge/driver_codex_test.go:5` — above `func TestResolveTierModel_CanonicalTiers_Regression(t *testing.T) {`

```text
// TestResolveTierModel_CanonicalTiers_Regression guards the cycle-378
// incident: a policy pin storing the canonical tier "deep" (the vocabulary
// `evolve setup apply` mandates — pins store fast|balanced|deep, never a
// native model id) reached the codex driver, whose tier table only understood
// the legacy aliases haiku/sonnet/opus. "deep" passed through unrecognized →
// the driver logged "unrecognized model 'deep'", omitted -m, and codex exited
// rc=1, spinning the loop. The shared ladder MUST translate every canonical
// tier identically to its legacy alias (relational against the family
// manifest — the values themselves are pinned once, in codex_tier_map_test.go),
// while still passing native ids and genuinely unknown values through.
```

### `go/internal/bridge/driver_codextmux.go:16` — above `func (codexTmuxDriver) Preflight(ctx context.Context, cfg *Config, deps Deps) error {`

```text
// Preflight pre-trusts cfg.Worktree + cfg.Workspace in ~/.codex/config.toml so
// codex's own permission layer doesn't render the runtime workspace-write
// modal that hung cycle-122 tdd (incident report + research dossier codex
// Fix A). This was an inline call at the top of Launch until cycle-124 G3
// promoted it through the optional CLIPreflight interface (driver.go). Same
// best-effort semantics — a returned error is logged by Engine.Launch and
// does NOT abort the phase (Fix 2's extended fallback trigger list defends
// downstream). ctx + deps are retained for future codex-specific prep work
// (binary-version probe, OAuth refresh) that may need them; the current
// implementation only reads cfg.
```

### `go/internal/bridge/driver_codextmux.go:49` — above `flags := cfg.Realization.LaunchFlags`

```text
// Launch flags come from the per-CLI Realization (ADR-0022): codex resolves
// the model tier via its manifest model_tier_map (tier → the family table's model) and
// emits it as -m; permission is a controller no-op (trust handled by the
// auto-responder). No claude argv reaches codex.
```

### `go/internal/bridge/driver_codextmux.go:55` — above `if m, err := LoadManifest("codex-tmux"); err == nil {`

```text
// cycle-142: clamp the model to a ChatGPT-safe one on subscription auth.
// A model outside the manifest's chatgpt_safe_models is 400-rejected on
// ChatGPT accounts (API-key-only by plan tier), which otherwise hangs the
// phase on an undismissable model-switch modal. Best-effort: a manifest load failure leaves flags
// untouched (the legacy behavior).
```

### `go/internal/bridge/driver_common.go:167` — above `func IsDir(path string) bool {`

```text
// IsDir reports whether path is an existing directory. Exported because the
// fleet worktree guard below (driver_tmux_repl.go: `if !IsDir(workingDir)` →
// ExitBadFlags) is a launch-refusal predicate that callers OUTSIDE this package
// must be able to test against before dispatching — a phase that re-derives it
// locally can drift from the guard and strand a lane (cycle-1278: retro handed
// the bridge a torn-down lane's stale worktree and lost the retrospective).
// One predicate, one definition.
```

### `go/internal/bridge/driver_common.go:179` — above `func artifactReady(cfg *Config) (ready bool, relocatedFrom string, err error) {`

```text
// artifactReady reports whether the phase artifact is present and non-empty.
// It accepts the canonical cfg.Artifact path and, as a tolerance for agent
// doc-compliance variance, an ordered set of fallback locations that get
// relocated to the canonical path (the single source of truth downstream
// phases read). relocatedFrom returns the fallback the artifact was found at
// so the caller can log the normalization. Empty files never count (matching
// fileNonEmpty / the bash `[[ -s ]]` test).
//
// Fallback search order (first non-empty wins):
//  1. <workspace>/workspace/<base> — cycle-108: agents read the doc's
//     "workspace/" prefix as a literal subdir under their cwd.
//  2. <worktree>/<base> — cycle-141 ExitArtifactTimeout: the builder runs with
//     cwd=worktree (driver_tmux_repl.go), and the prompt names the artifact by
//     bare relative path ("Write build-report.md"), so the agent writes it into
//     the worktree root — which the driver did not poll.
//  3. <worktree>/workspace/<base> — the same "workspace/" literal-subdir
//     misread, but relative to the worktree cwd.
//
// Worktree candidates are only searched when cfg.Worktree is set (headless
// drivers / probes leave it empty), so that path is byte-identical to the
// pre-cycle-141 behavior.
//
// When a fallback artifact exists but the relocation fails (e.g. a read-only
// workspace), the error is RETURNED rather than swallowed: a silent (false, "")
// would make the driver spin the full artifact-wait window with no signal,
// hiding a "wrote to the wrong place AND could not be moved" condition from the
// operator. The caller logs it.
// See docs/architecture/adr/0024-conditional-ship-gate-floor-and-phase-advisor.md.
```

### `go/internal/bridge/driver_common.go:211` — above `func artifactCanonicalize(cfg *Config, move func(src, dst string) error) (ready bool, relocatedFrom string, err error) {`

```text
// artifactCanonicalize is artifactReady with the mover injected, so a caller
// that has NOT confirmed the artifact settled can restrict which relocation
// semantics are allowed to run (cycle-1256 D1). move is only ever consulted for
// a NON-canonical artifact; the canonical case moves nothing under either mover.
//
// Two movers exist, and the difference is not stylistic:
//   - relocateFile — rename, falling back to copy+remove. Correct only for a
//     file already observed to have stopped changing, because copy+remove
//     snapshots the source and then deletes it.
//   - renameOnlyRelocate — rename, or fail. Safe for a file that may still be
//     growing, because rename preserves the inode: an agent's open fd keeps
//     appending into the file at its new canonical path.
```

### `go/internal/bridge/driver_common.go:237` — above `func artifactLocate(cfg *Config) (path string, found bool) {`

```text
// artifactLocate reports where the phase artifact currently IS — the canonical
// path when it holds a non-empty file, otherwise the first non-empty fallback
// in artifactReady's search order — WITHOUT moving anything. It is the
// read-only half of artifactReady, which layers the relocation on top.
//
// The split exists because relocation is irreversible and, on relocateFile's
// cross-device copy+remove branch, destructive: it snapshots the source and
// then removes it. Observing a fallback that is still being written and moving
// it on that first sighting truncates the deliverable permanently. So
// artifactDetector runs its cross-poll stability window against this read-only
// answer and only calls artifactReady — the mover — once the file has settled.
// Candidates are qualified with regularFileNonEmpty, not fileNonEmpty: this is
// the chokepoint that decides which bytes become the committed deliverable, so
// a symlink is never followed here (cycle-1256 D3).
```

### `go/internal/bridge/driver_common.go:284` — above `func relocateFile(src, dst string) error {`

```text
// relocateFile moves src to dst, creating dst's parent directory. It tries an
// atomic rename first and falls back to copy+remove when rename fails (e.g. a
// cross-device move). The copy goes through a "<dst>.tmp.<pid>" temp file in
// dst's directory that is renamed into place, so a write that fails partway
// (ENOSPC, network I/O) never leaves a truncated non-empty file at the
// canonical path — which the poll loop would otherwise read as "ready". This
// mirrors the ${file}.tmp.$$ + mv discipline used across the codebase's
// atomic writers. The temp file is created with os.CreateTemp — O_EXCL with an
// unpredictable suffix — rather than the old "<dst>.tmp.<pid>" name, whose PID
// component is small, guessable and disclosed in logs: an agent that pre-planted
// that name as a symlink turned this copy into an arbitrary-write primitive
// (cycle-1256 D4). Used by artifactReady to canonicalize a non-canonical write
// that has already been observed to stop changing; callers that have NOT
// established that must use renameOnlyRelocate instead.
```

### `go/internal/bridge/driver_common.go:335` — above `func renameOnlyRelocate(src, dst string) error {`

```text
// renameOnlyRelocate canonicalizes src → dst with rename semantics ONLY: if the
// rename cannot be done (a cross-device src, an unwritable source directory) it
// reports the error instead of degrading to relocateFile's copy+remove.
//
// This is the mover for a caller that has NOT confirmed the artifact stopped
// changing — today, artifactDetector's finality short-circuit. Rename is the one
// canonicalization that is safe for a file still being written: it relinks the
// same inode, so the agent's open fd keeps appending into the file at its new
// path and the deliverable reader still sees every byte. copy+remove on the same
// file snapshots it half-written and then deletes the original, which is
// permanent data loss dressed up as a settled artifact (cycle-1256 D1).
```

### `go/internal/bridge/driver_credentials_test.go:143` — above `if !fr.argvContainsPair("-c", "model_reasoning_effort=high") {`

```text
// Codex's SECOND model layer (2026-08-15 operator directive): the
// headless path must pin the reasoning effort too, or the model
// runs at the CLI's own default.
```

### `go/internal/bridge/driver_inject_test.go:74` — above `func TestInjectEnvelope_Keystroke_RawSendNoEscNoGate(t *testing.T) {`

```text
// TestInjectEnvelope_Keystroke_RawSendNoEscNoGate covers the cycle-124 F4
// hatch: a keystroke envelope is sent via SendKeys verbatim — NO ESC prefix
// (unlike interrupt), NO idle-gate (unlike command/nudge/system_rule), NO
// paste-buffer scratch file (unlike everything else), NO auto-Enter. This
// is the "full tmux control" channel the operator needs to dismiss the
// codex per-edit-approval modal that hung cycle-123 (`--body=Enter`),
// confirm y/N prompts (`--body=y`), navigate menus (`--body=Up`), or send
// control chars (`--body=C-c`). The gate-bypass is intentional: the
// operator may need to send keys precisely BECAUSE the agent isn't idle.
```

### `go/internal/bridge/driver_inject_test.go:331` — above `func TestInjectEnvelope_Keystroke_SendKeysErrorSurfaced(t *testing.T) {`

```text
// TestInjectEnvelope_Keystroke_SendKeysErrorSurfaced is the cycle-124
// review MEDIUM regression guard: a failing SendKeys MUST produce a
// "keystroke send failed" stderr line, NOT a "injected keystroke" success
// line. Prevents the silent-failure mode where an operator sees `injected
// keystroke "Enter"` in logs but nothing actually reached the (vanished)
// pane.
```

### `go/internal/bridge/driver_inject_test.go:358` — above `type errInjectingTmux struct {`

```text
// errInjectingTmux is a minimal fakeTmux that returns a configured error
// from SendKeys — used only by TestInjectEnvelope_Keystroke_SendKeysErrorSurfaced
// to drive the error branch added in cycle-124 review MEDIUM fix.
```

### `go/internal/bridge/driver_inject_test.go:450` — above `func (c *captureHookTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {`

```text
// CapturePane drops the artifact on the second capture and then leaves it
// alone. The write-once guard is load-bearing under the cycle-1233 cross-poll
// stability window (completion.go): rewriting the file on every capture would
// bump its mtime on every tick, so the window could never close and the driver
// would run out its whole wait budget. A real agent writes its deliverable once.
```

### `go/internal/bridge/driver_liveness_routing_test.go:88` — above `func TestDriverLivenessRouting_StopReviewHasNoCLILiterals(t *testing.T) {`

```text
// TestDriverLivenessRouting_StopReviewHasNoCLILiterals is the grep-assert that
// stopreview.go contains zero CLI-name literals ("claude", "codex", "agy", "ollama").
// All per-CLI branching must live in panestream.DetectorFor (ADR-0047 §3), not the
// reviewer — this test pins that invariant at the file level.
```

### `go/internal/bridge/driver_model_token_test.go:3` — above `import (`

```text
// driver_model_token_test.go — the unresolved-model-token invariant across EVERY
// driver that builds a model argument, not just the realizer.
//
// realizer_modelpolicy_test.go pins omit-on-"auto" for the tmux/flag matrix
// (ADR-0044 C2/D3, cycle-262). That guard is only matrix-wide for CLIs whose
// model flag the REALIZER emits. The headless drivers build their own argv:
// driver_codex.go has always had its own omit-on-auto, and claude-p had none —
// `claude -p --model <tier>` (and `--model auto`) went straight to the CLI.
//
// These tests drive the pure argv builders, so a driver that reintroduces an
// unguarded model flag fails here without launching anything.
```

### `go/internal/bridge/driver_ollamatmux.go:18` — above `func ollamaComposeLaunchCmd(binary, model string, extras []string) string {`

```text
// ollamaComposeLaunchCmd composes the single REPL launch line `<binary> run
// <model> [extras...]`. Lives on the driver (not in a test mirror) so test
// pins exercise the SAME function the driver runs — preventing the silent
// drift the reviewer caught when the helper had been duplicated in a test
// file. The binary is parameterized to keep the function pure + trivially
// testable.
//
// extras lands AFTER the positional model — ollama's CLI parses
// `ollama run <model>` as a subcommand-with-positional, and any flag the
// operator wants to pass (e.g. `--experimental-yolo` to short-circuit the
// per-edit-approval prompt cycle-123 surfaced for codex) must follow the
// model token. Wired in cycle-124 G1a: the realizer's default_args
// activation makes manifest.default_args reach LaunchFlags, and this
// helper threads them into the right launch-line position. extras=nil or
// empty is a no-op preserving the pre-fix launch shape exactly.
```

### `go/internal/bridge/driver_ollamatmux.go:103` — above `launchCmd := ollamaComposeLaunchCmd(resolveBinary(deps, "ollama"), model, cfg.Realization.LaunchFlags)`

```text
// Model is the POSITIONAL first arg for `ollama run <model>` — NOT a
// flag. The manifest declares model_tier channel:noop so the realizer
// emits no model flag; we compose the launch line directly via the
// shared ollamaComposeLaunchCmd (also exercised by the launch-cmd test
// pins, so the driver and tests can't silently drift). cfg.Realization.
// LaunchFlags carries manifest.default_args (cycle-124 G1a:
// --experimental-yolo) plus any operator raw-by-CLI extras, threaded
// AFTER the positional model per ollama's CLI grammar.
```

### `go/internal/bridge/driver_ollamatmux_test.go:32` — above `func TestOllamaTmux_ManifestRealizesYoloOnly(t *testing.T) {`

```text
// TestOllamaTmux_ManifestRealizesYoloOnly is the realizer contract after the
// cycle-124 G1a wire-up: ollama-tmux declares all params channel:noop EXCEPT
// session_mode (controller), so the params table contributes NOTHING to
// LaunchFlags for a typical LaunchIntent — but manifest.default_args =
// ["--experimental-yolo"] now lands in LaunchFlags (cycle-124 activated the
// previously-dead default_args channel in Realize()). The model still
// composes positionally in the driver, not via the realizer; the realized
// --experimental-yolo flag is threaded through cfg.Realization.LaunchFlags
// into ollamaComposeLaunchCmd's extras tail (after the positional model).
```

### `go/internal/bridge/driver_ollamatmux_test.go:161` — above `func TestOllamaTmux_LaunchCmd_AppendsExtrasAfterModel(t *testing.T) {`

```text
// TestOllamaTmux_LaunchCmd_AppendsExtrasAfterModel pins the cycle-124 G1a
// contract: `ollama run <model>` is followed by manifest default_args (and
// any operator raw extras) in the launch line, AFTER the positional model.
// Order is load-bearing — ollama's CLI parses `ollama run <model>` as a
// subcommand-with-positional; flags before the model would be misparsed.
```

### `go/internal/bridge/driver_ollamatmux_test.go:207` — above `func TestOllamaTmux_RejectsShellInjectionInModelTag(t *testing.T) {`

```text
// TestOllamaTmux_RejectsShellInjectionInModelTag is the cycle-119-class
// security pin: the launchCmd reaches the shell via tmux send-keys (NOT
// exec), so an unvalidated model tag like `llama3.1:8b; rm -rf /` would
// execute the trailing command. The driver MUST reject any tag containing
// shell-special chars before composing the launch line.
```

### `go/internal/bridge/driver_seed_test.go:12` — above `func indexOf(seq []string, want string) int {`

```text
// driver_seed_test.go — Realization.REPLInput seed injection: lines fed into
// the REPL after the boot marker, before the task prompt. Closes the
// previously-dead REPLInput field (ADR-0022).
```

### `go/internal/bridge/driver_tmux_boot.go:57` — above `if cfg.ProjectRoot != "" {`

```text
// The pane is a shell the bridge did not start: it inherits the tmux
// server's environment, not this process's, so Deps.Env never reaches it
// the way driverEnv hands it to a headless CLI. Export the one variable
// the subprocess contract promises (core/phase.go: ProjectRoot is "what a
// subprocess sees as EVOLVE_PROJECT_ROOT"). Without it every `evolve`
// subcommand the agent runs resolves its root from the worktree cwd —
// batch cycle 1631 (2026-09-12) claimed an inbox item in the worktree's
// git-tracked inbox snapshot while the plane's queue never moved.
```

### `go/internal/bridge/driver_tmux_delivery_failure_test.go:3` — above `import (`

```text
// driver_tmux_delivery_failure_test.go — RED contract for cycle-1562 tasks
// `retrospective-delivery-relaunch` and the bridge half of
// `retrospective-delivery-evidence-contract`.
//
// Evidence (.evolve/runs/cycle-1510/retrospective-launch-error.txt and
// retrospective-interactions.ndjson): the retro launch logged "prompt
// delivered", produced ZERO tokens and zero cost, and then burned two full
// 900s stop-review intervals before dying with ExitArtifactTimeout. The
// submit-verify guard (driver_tmux_submitverify.go) had ALREADY classified
// that pane as `submit_wedged` within milliseconds — but both call sites in
// driver_tmux_repl.go pipe verifySubmitted's outcome straight into
// recordSubmitVerify, which only appends to the ndjson ledger and returns
// nothing. The classification is produced and never consumed: at the
// control-flow level a detected delivery failure is indistinguishable from a
// healthy launch that simply never speaks.
//
// Contract, in two halves:
//
//   1. RELAUNCH — a `submit_wedged` outcome must short-circuit the artifact
//      wait immediately (ExitArtifactTimeout, which cyclerun_dispatch.go
//      already treats as retryable via IsInfraTeardownError and relaunches
//      exactly once), instead of consuming the full silence budget first.
//   2. EVIDENCE — that early exit must reuse the existing artifactTimeoutMarker
//      shape with a CLASSIFIED reason naming the site and the resend count, so
//      artifactTimeoutSummary lifts it into phaseErr unchanged and the cause
//      survives into failure-learning as data rather than discarded stderr.
//
// The false-negative guards are load-bearing and tested here as negatives: a
// clean submission and a generically silent pane must NOT be classified as
// delivery failures. Over-firing this classifier would convert every ordinary
// slow phase into a bridge relaunch.
//
// Every test drives the REAL production entry point (runTmuxREPL /
// Engine.Launch) over a fake tmux. A helper called directly would prove
// nothing about reachability.
```

### `go/internal/bridge/driver_tmux_delivery_failure_test.go:59` — above `type parkedPromptTmux struct {`

```text
// parkedPromptTmux keeps the pasted prompt visible at the `❯` input line
// forever: no number of bare Enters clears it. This is the cycle-1510 pane
// shape — the paste landed, the submit never took.
```

### `go/internal/bridge/driver_tmux_delivery_failure_test.go:160` — above `func TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker(t *testing.T) {`

```text
// TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker covers the
// SECOND consumer site. The nudge fires from inside the stop-review pause
// branch, so it cannot skip a silence budget it has already spent — but its
// wedged outcome is the same evidence, and the terminal artifact-timeout
// marker must name it instead of reporting the generic stall reason. Without
// this, cycle-1510's ndjson (`"result":"no_effect"` on every nudge) stays the
// only place the cause exists.
```

### `go/internal/bridge/driver_tmux_delivery_failure_test.go:240` — above `func TestTmuxREPL_ParkedPaneWithDeliveredArtifact_CompletesOK(t *testing.T) {`

```text
// GROUND TRUTH beats the pane heuristic (v22.20.0 release red): a pane that
// LOOKS parked while a post-dispatch deliverable is already on disk means the
// submission landed — a REPL that answers by side effect alone never redraws
// its input line. The wedged short-circuit must yield to the artifact and let
// the normal wait complete; only a parked pane with NO deliverable keeps the
// fast-fail (pinned by the AC-001 positive above).
```

### `go/internal/bridge/driver_tmux_repl.go:63` — above `inputLineMarker string`

```text
// inputLineMarker locates the LIVE INPUT LINE: text after its LAST
// occurrence is what has been typed but not yet submitted. Deliberately
// DISTINCT from promptMarker, which only answers "has the REPL booted" —
// for agy that is the footer hint "? for shortcuts", which says nothing
// about where input begins (cycle-1526 audit). Empty means this family
// declares NO input-line marker: submit-verify then refuses to guess and
// says so on stderr, rather than anchoring a re-send on a footer and
// submitting whatever the agent typed.
```

### `go/internal/bridge/driver_tmux_repl.go:78` — above `guardDeadShell bool`

```text
// guardDeadShell arms the cycle-274 dead-shell checks (boot rejection +
// post-paste spill fast-fail). Set by the REAL CLI drivers — their
// foreground process is never a shell, so a shell pane means the CLI is
// gone. MUST stay false for harnesses whose "REPL" legitimately IS a
// shell script (the RealTmux integration fixtures — the PR-71 Ubuntu CI
// failure this field exists for).
```

### `go/internal/bridge/driver_tmux_repl.go:87` — above `func launchCmdLine(binary string, flags []string) string {`

```text
// launchCmdLine joins an inner-CLI binary with its realized launch flags
// (ADR-0022) into the single REPL launch command line. The flags are the
// per-CLI Realization, so the line carries only argv this CLI understands.
// Each token is shell-quoted because SendKeys delivers ONE shell line, not an
// argv slice: agy 1.0.15's --model values are display names with spaces and
// parens ("Gemini 3.1 Pro (High)", cycle-447). shellQuotePOSIX passes
// safe-charset tokens through verbatim, so claude/codex/ollama launch lines
// are byte-identical to the pre-quoting join.
```

### `go/internal/bridge/driver_tmux_repl.go:126` — above `ar.injectedPrompt = resolvedPrompt`

```text
// Echo-veto (cycle-672): tick() strips pane lines that verbatim-echo this
// session's own delivered prompt before its exhaustion/escalation scans.
```

### `go/internal/bridge/driver_tmux_repl.go:130` — above `phaseName := orDefault(cfg.Agent, lp.name)`

```text
// ADR-0045 I1: interaction telemetry — every injection this launch fires
// (auto-respond sends, the one-shot nudge) records a typed outcome in
// <workspace>/<phase>-interactions.ndjson. Recording runs at EVERY
// EVOLVE_PHASE_RECOVERY stage including `off`: observation is never the
// kill-switch's business; only corrective ACTIONS gate on the stage.
```

### `go/internal/bridge/driver_tmux_repl.go:138` — above `ar.broker = interaction.NewKernelAnswerer(interaction.KernelFacts{`

```text
// ADR-0045 I3: the AskBroker's KernelAnswerer over THIS dispatch's closed
// fact set. It answers only facts the agent's own prompt already carried
// (artifact path, workspace, worktree, cycle) — structurally unable to
// disclose anything off-list (threat S7). Gated by the same
// EVOLVE_PHASE_RECOVERY stage as every other corrective ACTION.
```

### `go/internal/bridge/driver_tmux_repl.go:150` — above `ar.prompts = append(ar.prompts, loadPromotedPrompts(cfg.ProjectRoot)...)`

```text
// ADR-0045 I4: merge ENFORCE-stage promoted auto-respond rules (durable
// registry under .evolve/instincts/interaction-rules), re-validated against
// the immutable healthy-pane corpus at load — a rule a new CLI banner now
// matches is demoted, never fired. Appended AFTER the manifest rules so a
// promoted rule can never shadow a vetted built-in (first match wins).
```

### `go/internal/bridge/driver_tmux_repl_bootms_test.go:3` — above `import (`

```text
// driver_tmux_repl_bootms_test.go — A0 cold-boot instrumentation (ADR-0043).
// Proves the full chain driver→Deps.OnBoot→BridgeResponse.BootMS: the tmux-REPL
// driver reports the cold-boot wait (the 2 fixed readiness sleeps + the marker
// poll), the Engine captures it onto the response, and a launch that never
// completes a cold boot reports BootMS=0.
```

### `go/internal/bridge/driver_tmux_repl_bootms_test.go:100` — above `const sessName = "warm1"`

```text
// A pre-existing named session → namedExists=true → the driver RESUMEs and
// skips the entire cold-boot block, so OnBoot is never reached and BootMS
// stays 0. This is the ADR-0043 contract ("warm named session → 0") that the
// boot-timeout test only covers by proxy; here it is exercised directly, so a
// refactor that moved the OnBoot call out of the `if !namedExists` block fails.
```

### `go/internal/bridge/driver_tmux_repl_cancel_test.go:3` — above `import (`

```text
// driver_tmux_repl_cancel_test.go — session-lifecycle-verdict-clobbers residual
// (primary settle-retry fix c2258b72): the wait loop broke on ctx.Err() WITHOUT
// a final completion check, so a context-cancel landing AFTER the deliverable
// was already on disk (the next phase tearing down a finished session) was
// laundered into ExitArtifactTimeout — telemetry recorded a phase "timeout" for
// a benignly-torn-down COMPLETED session, and only the runner's settle-retry
// stood between that mislabel and a false FAIL. The fix: one final detector
// poll on cancellation (the artifact detector is a pure file stat, so it works
// under a dead ctx); a ready deliverable = a completed session = the normal
// success path, not a timeout.
```

### `go/internal/bridge/driver_tmux_repl_corr_test.go:33`

```text
// NOTE: the busy→idle bracket integration (formerly
// TestRunTmuxREPL_EmitsBothBreadcrumbsOnBusyToIdle, which asserted breadcrumbs on
// stderr) moved to TestRunTmuxREPL_ChannelOn_BreadcrumbsToFile in
// driver_tmux_repl_panelive_test.go when RT2 (ADR-0037) redirected breadcrumbs to
// the <agent>-breadcrumbs.live file the Producer tails.
```

### `go/internal/bridge/driver_tmux_repl_escalation_test.go:81` — above `type writingReviewer struct {`

```text
// writingReviewer models the agent finishing its deliverable mid-wait: the
// artifact is written ONCE, at the first review checkpoint, and then left alone.
//
// The write-once guard is load-bearing under the cycle-1233 cross-poll
// stability window (completion.go): a reviewer that rewrote the file at every
// checkpoint would bump its mtime on every tick, so the window could never
// close and this fixture would spin forever. A real agent writes its deliverable
// and stops — the fixture now models that instead of a permanent rewriter.
```

### `go/internal/bridge/driver_tmux_repl_exhaustion_test.go:21` — above `walled := "❯\nYou've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model.\n❯"`

```text
// Boots (prompt marker ❯ present) AND shows the quota wall. This is the
// EXACT wall captured in cycles 904–911's audit-escalation-report.json —
// the per-model wording ("your Fable 5 limit") that the pre-fix
// exhausted_regex ("reached your (usage|weekly) limit") did NOT match, so
// the fast-fail was bypassed and 8 audit cycles burned the full artifact
// timeout. The artifact is never written, so only the exhaustion override
// can end the run.
```

### `go/internal/bridge/driver_tmux_repl_exhaustion_test.go:54` — above `walled := "❯\nYou've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model.\n❯"`

```text
// Real per-model wall wording (cycle-910/911 capture), as above.
```

### `go/internal/bridge/driver_tmux_repl_exhaustion_test.go:83` — above `func TestTmuxREPL_ContentWall_SuppressedByCorroborator(t *testing.T) {`

```text
// End-to-end wiring proof for the corroboration seam (2026-08-15 false-wall
// incident): the SAME walled pane that fast-fails above must NOT fail over
// when a live probe answers — the driver plumbs Deps.CorroborateWall through
// to the scan sites, suppresses loudly, and concludes with the ordinary
// artifact timeout instead of forging a quota wall.
```

### `go/internal/bridge/driver_tmux_repl_fleet_accept_test.go:3` — above `import (`

```text
// driver_tmux_repl_fleet_accept_test.go — cycle-1270 Task 2
// (`retro-fleet-worktree-dispatch`), the missing POSITIVE half.
//
// Both halves of the fleet-worktree contract are tested today and neither
// proves the contract holds: retro proves it MINTS a scratch cwd
// (phases/retro/retro_worktree_fallback_test.go), and this package proves the
// guard REFUSES an empty one (TestFleetModeRefusesEmptyWorktree). Nothing
// proves a minted directory actually CLEARS the guard.
//
// A future tightening of that guard (e.g. requiring a .git entry) would break
// every fleet-lane retro with both existing suites still green — the exact
// silent-regression shape the item exists to close. The pair IS the contract:
// accepts a real owned cwd, refuses an empty one.
```

### `go/internal/bridge/driver_tmux_repl_idlereached_busyof_test.go:3` — above `import (`

```text
// driver_tmux_repl_idlereached_busyof_test.go — RED tests for cycle-434
// slice S4 completion (s4-complete-residual-busy-callsites, Task 1): the
// idle_reached correlation-span bracket (driver_tmux_repl.go:587) is the
// second surviving direct panestream.PaneBusy consumer the S4 charter
// targeted (scout finding F2). It must route through
// panestream.LivenessCenter.BusyOf instead of calling panestream.PaneBusy
// inline, so the LivenessCenter remains the sole liveness facade (ADR-0068).
//
// AC2 (the bracket fires idle_reached exactly once on a real busy→idle
// transition) is already pinned end-to-end against real captured claude
// frames by TestChannelE2E_RealFixtures_ClaudeSpan (channel_e2e_test.go) —
// pre-existing GREEN, unaffected by this migration because BusyOf delegates
// to the SAME PaneBusy definition (H1: verdict-identical). This file adds
// only the AC3 discriminating negative test.
//
// TDD contract: written BEFORE the migration lands. Fails today (RED)
// because the bracket still calls panestream.PaneBusy inline. DO NOT MODIFY
// THIS TEST — Builder migrates the call site to make it pass.
```

### `go/internal/bridge/driver_tmux_repl_livenesscenter_test.go:3` — above `import (`

```text
// driver_tmux_repl_livenesscenter_test.go — RED tests for cycle-431 slice S3
// (LivenessCenter consolidation): the driver's stop-review checkpoint must
// route liveness through panestream.LivenessCenter.Observe/Aggregate
// (ADR-0068), not the bare per-run detectorFor(lp) probe, and the
// reviewer's pre-S3 Progressed/Busy boolean fallback must be retired —
// verdict becomes a pure function of StopEvent.State.
//
// Seam: Deps.LivenessCenter (optional override; nil ⇒ the driver builds its
// own panestream.NewLivenessCenter()) — a minimal DI seam (scout BA2) so a
// test can register a distinctive probe and prove the checkpoint actually
// consults it, without adding a new apicover-tracked symbol (Deps is
// already covered elsewhere; a struct field is not a SymbolKind apicover
// enumerates — see cmd/apicover/enumerate.go).
```

### `go/internal/bridge/driver_tmux_repl_livenesscenter_test.go:39` — above `func TestRunTmuxREPL_SignalCenterStateWins(t *testing.T) {`

```text
// TestRunTmuxREPL_SignalCenterStateWins (AC1, positive): a LivenessProbe
// registered on an injected LivenessCenter for the "claude" profile name must
// be the state the checkpoint assigns to StopEvent.State — proof the driver
// routes through center.Observe + center.Aggregate() (ADR-0068), not a
// private per-run detectorFor(lp) probe that never learns about the
// registration. Nothing in a normal claude-tmux boot-only pane sequence
// classifies as Hung on its own, so seeing Hung here can only come from the
// registered handler winning.
```

### `go/internal/bridge/driver_tmux_repl_livenesscenter_test.go:105` — above `func TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant(t *testing.T) {`

```text
// TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant (AC6, edge): the
// cycle-291 render-wedge override — a blank pane from a LIVE session reads
// as BusyButStagnant, never Idle — must survive the migration to the
// center-authoritative State source. The scout keeps this as a post-
// Aggregate override in the driver (behavior-identical), not a center
// handler (deferred to S4).
```

### `go/internal/bridge/driver_tmux_repl_nonce_test.go:1` — above `package bridge`

```text
// driver_tmux_repl_nonce_test.go — ADR-0049 N15: ephemeral tmux session names
// must be unique even when two are minted at the SAME wall-clock instant.
// Concurrent fleet cycles (and same-phase retries within a cycle) can dispatch
// in the same second; a second-granularity timestamp alone collides, and tmux
// would then have two cycles fighting over one session.
```

### `go/internal/bridge/driver_tmux_repl_panelive_test.go:177` — above `if _, err := os.Stat(cfg.Artifact); err != nil {`

```text
// Drop the artifact only after both breadcrumbs are in the file so
// the loop cannot exit before idle_reached fires — and write it ONCE.
// Under the cycle-1233 cross-poll stability window (completion.go) a
// file rewritten every tick bumps its mtime forever and never settles.
```

### `go/internal/bridge/driver_tmux_repl_s4_migration_test.go:3` — above `import (`

```text
// driver_tmux_repl_s4_migration_test.go — cycle-432 slice S4 regression tests:
// the stop-review checkpoint must not parse CLI chrome directly through
// panestream.PaneBusy / PaneHasSubstantiveChange. It reads the
// panestream.LivenessCenter projections added in Task 1
// (livenessCenter.Busy(session) / livenessCenter.Changed(session)).
//
// AC1/AC2 pin the projected values and AC3 guards the implementation boundary,
// which now spans the wait coordinator and its checkpoint module.
```

### `go/internal/bridge/driver_tmux_repl_submitverify_test.go:3` — above `import (`

```text
// driver_tmux_repl_submitverify_test.go — RED contract for cycle-1526 task
// `submit-verify-retro-paste`.
//
// Evidence (premise-challenge-report.md, cycle-1526): in cycles 1505, 1510 and
// 1517 the one-shot nudge sent at driver_tmux_repl.go:806-818 was still sitting
// UNSUBMITTED at the pane's `❯` input line in the final capture, and every
// nudge record in <phase>-interactions.ndjson read "result":"no_effect". The
// driver sends the keys once, sets nudgeSent=true, and never verifies that the
// input line cleared — a fire-and-forget Enter.
//
// Contract: every driver-initiated submission (the nudge AND the prompt-paste
// delivery at :368-376) must verify the input line cleared on the next capture
// and, when it did not, re-send Enter — bounded, and loud on stderr.
//
// These tests drive the REAL production entry point (Engine.LaunchArgs ->
// runTmuxREPL) over a fake tmux; a helper called directly would prove nothing
// about reachability.
```

### `go/internal/bridge/driver_tmux_repl_submitverify_test.go:40` — above `func unsubmittedPane(text string) string {`

```text
// unsubmittedPane renders the recorded cycle-1505/1510/1517 shape: text parked
// at the `❯` input line, never submitted.
```

### `go/internal/bridge/driver_tmux_repl_submitverify_test.go:202` — above `func TestTmuxREPL_NudgeSubmitted_NoResend(t *testing.T) {`

```text
// TestTmuxREPL_NudgeSubmitted_NoResend — the anti-double-submit control. When
// the input line DID clear, the driver must not re-send: a spurious extra
// Enter re-submits whatever the agent typed next and desyncs the pane (the
// highest-risk edge flagged by the cycle-1526 premise challenge).
```

### `go/internal/bridge/driver_tmux_repl_submitverify_test.go:252` — above `func TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter(t *testing.T) {`

```text
// TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter — the same verification must
// cover the prompt-delivery site the cycle committed to
// (driver_tmux_repl.go:368-376): paste, Enter, and if the prompt text is still
// at the input line on the next capture, re-send. NOTE (cycle-1526 premise
// challenge, finding #1/#6): no recorded cycle exhibits an unsubmitted PROMPT —
// this is the generalization of the nudge fix to the shared submit path, and
// its pane state is stipulated, not replayed.
```

### `go/internal/bridge/driver_tmux_repl_submitwedge_shortcircuit_test.go:34` — above `func TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget(t *testing.T) {`

```text
// TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget reproduces the
// cycle-1510 failure: submit verification detects a wedged prompt, but the
// result is discarded and the driver starts the normal artifact-wait loop.
```

### `go/internal/bridge/driver_tmux_submit_settle_test.go:3` — above `import (`

```text
// driver_tmux_submit_settle_test.go — the prompt-submission timing contract
// (verification wave 2026-09-14, cycles 1673–1675): a 17–35 KB prompt pasted
// into the codex TUI is still being ingested when the driver's fixed 1 s +
// three 500 ms re-sends have all fired, so the submission is declared
// "wedged" ~2.5 s after the paste and the whole dispatch is thrown away —
// while build/tdd/fault-localization in the same wave needed 2–3 re-sends
// just to land. The driver now (1) settles the first Enter by prompt size,
// (2) waits for the pane to stop changing before it presses Enter, and
// (3) backs off between re-sends instead of hammering. Vocabulary, stderr
// lines and the re-send cap are unchanged.
```

### `go/internal/bridge/driver_tmux_submit_settle_test.go:168` — above `func TestPasteTiming_NeverEqualsTheArtifactWaitInterval(t *testing.T) {`

```text
// H1 — the timing that precedes the artifact wait must never sleep exactly
// artifactWaitInterval: the wedge short-circuit pins count Sleeps of that value
// as artifact-wait polls, so a colliding settle would be counted as a poll and
// red a pin about an unrelated subsystem (with the 1 s/10 KB formula the
// 10–20 KB band — the incident's own scout prompt — landed on exactly 2 s).
```

### `go/internal/bridge/driver_tmux_submitverify.go:12` — above `const (`

```text
// driver_tmux_submitverify.go — every driver-initiated submission verifies
// that it was actually SUBMITTED.
//
// The hole this closes: the tmux REPL driver fired keys with enter=true and
// walked away — the prompt paste (driver_tmux_repl.go, "prompt delivered")
// and the one-shot idle nudge both. In cycles 1505, 1510 and 1517 the nudge
// was still sitting, unsubmitted, at the pane's `❯` input line in the final
// capture, and every nudge record in <phase>-interactions.ndjson read
// "result":"no_effect": the driver had no way to know its own key send did
// nothing. Fire-and-forget is the defect; one capture-and-confirm is the fix.
//
// The pairing matters as much as the re-send. An unconditional second Enter
// would "fix" the stall while re-submitting whatever the agent typed next —
// a worse pane desync than the stall. So a re-send fires ONLY when the input
// line still holds an echo of what THIS driver sent.
```

### `go/internal/bridge/driver_tmux_submitverify.go:38` — above `submitVerifyMinEchoRunes = 8`

```text
// submitVerifyMinEchoRunes is the floor BOTH match directions honour: below
// it a fragment is too generic to identify what this driver sent, and a
// match would arm re-sends into whatever the agent typed — the double-submit
// this guard exists to prevent (cycle-1526 audit M1: the forward direction
// had no floor, so a short first line like `---` matched almost any input
// line). Single-sourced: the reverse direction used to hardcode this.
```

### `go/internal/bridge/driver_tmux_submitverify.go:109` — above `type submitVerifyOutcome struct {`

```text
// verifySubmitted confirms a submission cleared the input line and, when it
// did not, re-sends a bare Enter — bounded by submitVerifyMaxResends and loud
// on stderr, so an operator reading a stalled cycle's log sees that the driver
// noticed and acted. Returns the number of re-sends issued.
//
// `pane` is the FIRST observation, supplied by the caller: the prompt site
// hands in the post-paste interval baseline the driver already captured, so
// the clean path adds no capture and no fixture-frame drift (the same rule the
// cycle-274 spill check follows). Only a genuinely pending input line — the
// exceptional path — costs a re-capture per re-send.
//
// site names the submission ("prompt", "nudge") in the log line; echoes are
// candidate renderings of what was sent.
// submitVerifyOutcome is what verifySubmitted learned. Returned rather than
// logged-and-forgotten: stderr for a phase dispatch is discarded on the success
// path (engine.go:531-534 returns before the :544 persistence), so the caller
// must be able to put this somewhere durable.
```

### `go/internal/bridge/driver_tmux_submitverify.go:134` — above `if lp.inputLineMarker == "" {`

```text
// No input-line marker ⇒ nothing to anchor a match to. Refuse LOUDLY: the
// alternative is matching against whatever follows a boot/footer marker,
// which re-sends the agent's own text (cycle-1526 audit — agy's marker is
// the footer "? for shortcuts").
```

### `go/internal/bridge/driver_tmux_submitverify.go:190` — above `func promptSubmitEcho(prompt string) string { return echoChunk(prompt) }`

```text
// promptSubmitEcho is the FORWARD-direction echo for a pasted prompt: the head
// of the whole prompt, whitespace-normalized so a wrapped render compares equal.
//
// Why the whole prompt and not just its first line: the first line is the only
// prompt-derived echo the prompt site had, and a prompt whose first non-empty
// line is short (YAML frontmatter `---`, a stub header) falls under
// submitVerifyMinEchoRunes — that echo would never match and the guard would
// silently miss the cycles 1505/1510/1517 stall it exists to catch.
//
// It is BOUNDED (echoChunk caps at submitVerifyEchoRunes) and is therefore NOT
// a drop-in replacement for the first line in BOTH match directions: the
// reverse direction reads the passed value directly, so a head-only echo would
// narrow it from "any fragment of the first line" to "any fragment of its first
// 40 runes" — a silent detection loss on a REPL that horizontally scrolls its
// input line (ollama readline) rather than wrapping. The prompt site therefore
// passes this AND firstNonEmptyLine: this one keeps the forward direction above
// the floor, the first line preserves the reverse direction's original reach.
// Passing the WHOLE prompt as an echo would be wrong in the other direction —
// reverse would then match any ≥floor phrase the agent typed that appears
// anywhere in the prompt body.
```

### `go/internal/bridge/driver_tmux_submitverify.go:224` — above `func recordSubmitVerify(rec *interaction.Recorder, phase string, cycle int, site string, o submitVerifyOutcome, paste pa…`

```text
// recordSubmitVerify puts a submit-verify outcome somewhere durable. The
// Recorder writes through to <workspace>/<phase>-interactions.ndjson on every
// Record (appendLedgerLine), independent of how the phase ends — which is the
// whole point: stderr from a phase dispatch survives only the FAILURE path
// (engine.go:531-534 returns before the :544 persistence), so a guard that
// recovers a stall and lets the phase succeed erased its own evidence.
//
// Recorded on the clean path too. Without the denominator a recovered stall is
// an anecdote, not a rate, and the cycles 1505/1510/1517 class cannot be tracked.
// Nil-recorder-safe by Recorder's own contract.
```

### `go/internal/bridge/driver_tmux_submitverify_guard_test.go:3` — above `import (`

```text
// driver_tmux_submitverify_guard_test.go — RED contracts for the cycle-1526
// audit prescriptions that shipped unaddressed with 75a8aed9 (WARN verdict,
// red_count==0).
//
// Two holes, both about the guard being SILENT when it cannot do its job:
//
//  1. agy's promptMarker is the footer "? for shortcuts", not an input-line
//     prompt. pendingAtInputLine reads the text AFTER the last marker, so for
//     agy it reads whatever follows a footer — inert at best, and a SPURIOUS
//     re-send if that footer ever renders mid-pane. The guard has to know the
//     difference between "where the REPL said it booted" and "where the live
//     input line starts", and say so out loud when it has no input-line marker.
//
//  2. A CapturePane error inside the re-send loop returned with no log line at
//     all — a silent exit from a loop whose entire purpose is to be loud.
```

### `go/internal/bridge/driver_tmux_submitverify_guard_test.go:226` — above `root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()`

```text
// Bind git-TRACKED state, not whatever sits on disk (ADR-0084 lens 5a): an
// untracked scratch driver_*tmux.go would otherwise become a subject of this
// guard and produce a false RED — the cd49274beab2 class.
```

### `go/internal/bridge/driver_tmux_submitverify_record_test.go:16` — above `func readInteractions(t *testing.T, ws, phase string) []interaction.Outcome {`

```text
// driver_tmux_submitverify_record_test.go — the guard added in #474 announces
// itself on stderr only, and Engine.Launch returns on the SUCCESS path
// (engine.go:531-534) BEFORE it persists stderr to <agent>-launch-error.txt
// (:544). So a submit-verify that DETECTS a parked prompt and RECOVERS it makes
// the phase succeed, which discards every line proving it fired. Four waves and
// eight cycles produced zero observations of the guard working.
//
// interactions.ndjson is the durable surface: interaction.Recorder.Record writes
// through to disk immediately (appendLedgerLine) regardless of phase outcome —
// cycle-1530's router-interactions.ndjson carries an auto_respond record from a
// phase that SUCCEEDED.
```

### `go/internal/bridge/driver_tmux_submitverify_record_test.go:113` — above `func TestTmuxREPL_SubmitVerify_RecordReachesLedger(t *testing.T) {`

```text
// TestTmuxREPL_SubmitVerify_RecordReachesLedger is the WIRING proof: it drives
// the real production entry point and asserts the record lands in the file an
// operator would read. PR #474's stderr lines were "loud" and still unreachable;
// a unit test of the outcome alone would repeat that mistake.
//
// BOTH rows matter. The recovered row proves the guard's win is durable; the
// CLEAN row is the denominator — without it a "don't spam the ledger" early
// return in recordSubmitVerify passes every other assertion here, and a
// recovered stall stays an anecdote instead of a rate.
```

### `go/internal/bridge/driver_tmux_submitverify_unit_test.go:8` — above `func TestPendingAtInputLine(t *testing.T) {`

```text
// TestPendingAtInputLine pins the discrimination the whole fix rests on: a
// re-send fires ONLY when the input line still holds what THIS driver sent.
// The "agent typed something else" row is the double-submit hazard the
// cycle-1526 premise challenge flagged as the highest risk of the fix — a
// driver that re-sends there submits the agent's half-typed text.
```

### `go/internal/bridge/driver_tmux_submitverify_unit_test.go:30` — above `{"short echo must not match agent typing", "❯ let me check --- the diff", []string{"---"}, false},`

```text
// cycle-1526 audit M1: the forward branch had no length floor while the
// reverse branch enforced one, so a SHORT echo matched almost any input
// line — arming up to submitVerifyMaxResends Enters into text the AGENT
// typed. That is the double-submit desync this guard exists to prevent.
// A prompt whose first non-empty line is short (frontmatter `---`, a
// stub header) is all it takes; today's 53-char first lines are a
// measurement, not an invariant.
```

### `go/internal/bridge/driver_tmux_submitverify_unit_test.go:63` — above `func TestPromptSubmitEcho(t *testing.T) {`

```text
// TestPromptSubmitEcho pins the prompt-site echo pair. Both cycle-1526 reviewers
// observed that reverting promptSubmitEcho to firstNonEmptyLine left the whole
// suite green — the helper's entire reason for existing was asserted in a
// comment and never demonstrated. These rows fail for that revert, and for the
// opposite degenerate form (returning the prompt unbounded).
```

### `go/internal/bridge/driver_tmux_variants_test.go:74` — above `fx := newFixture(t, "codex-tmux", "")`

```text
// haiku → "codex --yolo -m gpt-5.4-mini" reaches the REPL launch line.
// --yolo is the manifest default_args entry (cycle-124 G1a: codex's
// undocumented but parsed flag that sets approval=never AND
// sandbox=danger-full-access at boot, defusing the per-edit-approval
// modal that hung cycle-123 tdd). It lands FIRST per realizer order
// (default_args before per-param scalars), then -m gpt-5.6-luna from
// params.model_tier (tier_alias haiku → fast → gpt-5.6-luna).
```

### `go/internal/bridge/driver_tmux_variants_test.go:133` — above `if !tmux.sentContains("--model") {`

```text
// agy 1.0.15 selects its model via the --model launch flag (cycle-447
// probe); the tokens are display names, shell-quoted by launchCmdLine.
// 1.0.3 had no model flag at all and `-m` remains undefined — see
// docs/incidents/cycle-154-agy-tmux-m-flag-repl-boot-timeout.md.
```

### `go/internal/bridge/driver_tmux_wait.go:64` — above `finalCtx, finalCancel := withFinalPoll(ctx)`

```text
// Context cancelled (orchestrator timeout / SIGTERM / the next phase
// tearing down this session): before abandoning, ONE final completion
// poll — a deliverable already on disk means the session COMPLETED and
// the cancel is benign teardown, not a phase timeout. Pre-fix this
// break skipped straight to the !completed → ExitArtifactTimeout exit,
// laundering a finished session into a timeout (session-lifecycle
// residual; the runner's settle-retry was the only thing standing
// between that mislabel and a false FAIL). A genuinely unfinished
// session still exits ExitArtifactTimeout.
//
// The poll gets a context DETACHED from the cancellation (cycle-1236):
// this line dispatches to all three completionDetector strategies, and
// only artifactDetector is a pure file stat. stdoutDetector shells
// CapturePane and gitEvidenceDetector shells git — exec.CommandContext
// refuses to fork on a dead ctx, both swallow the transport error as
// "not ready", and a DELIVERED stdout/git phase exited 81. withFinalPoll
// hands them a live, finalPollGrace-bounded context carrying the
// explicit finality marker artifactDetector's short-circuit now keys on
// (a live ctx alone would have disarmed it — completion.go).
```

### `go/internal/bridge/driver_tmux_wait_checkpoint_test.go:156` — above `func TestRunTmuxREPL_FatalPaneDialIsIndependentOfPhaseRecovery(t *testing.T) {`

```text
// TestRunTmuxREPL_FatalPaneDialIsIndependentOfPhaseRecovery (F27): the C2
// fatal-pane fast-fail rides its OWN dial. Crossing the two dials proves the
// checkpoint reads Deps.FatalPaneStage and never the program dial: arming the
// fast-fail cannot arm the channel / ask-broker / advisor that PhaseRecovery
// gates, and the program's shadow cannot silence a soak-proven fast-fail
// (cycles 1595 and 1687 idled 1200s / 900s on dead panes it had classified).
```

### `go/internal/bridge/driver_tmux_wait_diagnostic.go:13` — above `type artifactTimeoutCause = launchoutcome.TimeoutCause`

```text
// artifactTimeoutCause is the artifact-timeout sub-cause vocabulary — the
// classifier's closed set (launchoutcome.TimeoutCause, ADR-0103 unit 10),
// projected here so the emitter and the parser read ONE list: a token added
// on one side without the other is caught by the driver → classifier
// round-trip pin (TestTimeoutCauseVocabulary_DriverAndClassifierAgree).
```

### `go/internal/bridge/driver_tmux_wait_state.go:94` — above `livenessCenter.RegisterLivenessHandler(paneLivenessHandler(w.deps.Signals, configIdentity(w.cfg)))`

```text
// ADR-0101 S3: every liveness edge this dispatch observes is a
// pane.liveness signal stamped with its cycle, run and phase.
```

### `go/internal/bridge/effort_routing_test.go:3` — above `import (`

```text
// effort_routing_test.go — cycle-566 RED tests for per-phase reasoning-EFFORT
// routing (inbox `per-phase-effort-routing`, weight 0.88). Triage committed ONLY
// the plumbing slice: an abstract `effort` (low|medium|high) dimension added to
// LaunchIntent, realized per-manifest to each CLI's native mechanism (claude
// effort flag, codex reasoning_effort; agy/ollama noop) — mirroring the existing
// model_tier params.channel pattern. Retry-escalation + telemetry/soak are
// explicitly OUT of scope this cycle (see triage-report.md Rationale).
//
// RED now: LaunchIntent carries no Effort field, so this file does not compile
// until Builder adds the field + the realizeScalar("effort", intent.Effort) call
// + the manifest `params.effort` entries. GREEN once effort is realized through an
// effective channel for the supporting CLIs and cleanly no-ops for the rest.
```

### `go/internal/bridge/effort_routing_test.go:52` — above `if reflect.DeepEqual(hi, lo) {`

```text
// The dial must be effective in BOTH directions: explicit high and
// low realize differently (an all-noop wiring fails here), and a
// manifest that declares a DEFAULT (codex: high, 2026-08-15
// operator directive — the CLI's second model layer must never run
// at the CLI's own default) already carries it in the base.
```

### `go/internal/bridge/effort_routing_test.go:116` — above `if def := m.Params["effort"].Default; def != "" {`

```text
// Contract split (2026-08-15): a manifest WITHOUT a declared effort
// default keeps the purely-additive guarantee (unset ⇒ identical
// to a pre-effort manifest). A manifest WITH one (codex: high)
// deliberately breaks it — unset now realizes the default, so the
// equivalence is asserted against the explicit-default form.
```

### `go/internal/bridge/effort_toprungs_test.go:3` — above `import (`

```text
// effort_toprungs_test.go — the top of the reasoning ladder, pinned per family.
//
// Directive history: 2026-08-24 put deep/top phases at xhigh; 2026-08-28
// moved the CODEX-routed ones to max; 2026-09-01 moved them to high (quota
// headroom). This file pins REALIZABILITY of the upper rungs regardless of
// which one the current directive selects — the max/xhigh rows below stay
// because the manifest must keep every mapped rung realizable (a directive
// can flip back with one profile edit); WHICH rung profiles actually pin
// lives in profiles/effort_defaults_test.go, not here.
//
// Two contracts: (1) each rung REALIZES on the families with an effort dial —
// realizeScalar silently drops unmapped enum values, so a missing codex
// mapping loses the dial with no error (observed exactly that way: the max row
// failed with flags [--yolo], the dial simply absent); (2) EVERY tracked
// profile's effort_level is realizable by its own family's manifest — the
// class guard, so a future profile value can never silently no-op.
//
// Ladder verified live against codex 0.147.0 (/model -> "More reasoning..."):
// low, medium, high, xhigh, max, ultra. claude exposes low..max via --effort
// (its picker's "Ultracode" is an orchestration mode, NOT an --effort token —
// `--effort ultracode` silently falls back to xhigh).
```

### `go/internal/bridge/effort_toprungs_test.go:115` — above `rungs := map[string]string{}`

```text
// Every rung the profile can dispatch at: effort_level plus each
// per-tier effort_overrides value (ADR-0096) — the override is realized
// by the same realizeScalar and silently dropped the same way.
```

### `go/internal/bridge/engine.go:65` — above `CaptureBaseline func(*Config) artifactBaseline`

```text
// CaptureBaseline snapshots the PRE-DISPATCH artifact state so the
// completion detector can refuse a prior attempt's leftover report
// (cycle-1550's stale re-grade loop). Nil defaults to the real
// captureArtifactBaseline. Test harnesses whose fake sessions cannot
// write files pre-seed the artifact as a stand-in for a MID-SESSION
// write; they inject a zero capture to declare exactly that intent —
// production never sets this field.
```

### `go/internal/bridge/engine.go:76` — above `RecoveryStage string`

```text
// RecoveryStage is the ADR-0044 Unified Phase Recovery rollout stage,
// injected by the orchestrator from the policy-resolved cfg.PhaseRecovery.
// Empty ⇒ channel.ResolveStage returns "shadow" (behavior-neutral default).
```

### `go/internal/bridge/engine.go:80` — above `FatalPaneStage string`

```text
// FatalPaneStage is the ADR-0044 C2 fatal-pane fast-fail's OWN rollout
// stage (F27), injected from the policy-resolved cfg.FatalPane. It gates
// ONLY the stop-review checkpoint's fatal-pane preemption; RecoveryStage
// keeps the channel, ask-broker and transient-dwell. Empty ⇒ "shadow"
// (an unwired Deps observes; only the composition root arms the kill path).
```

### `go/internal/bridge/engine.go:160` — above `OnBoot func(bootMS int64)`

```text
// OnBoot is called once by a tmux-REPL driver when the REPL prompt marker
// first appears, reporting the cold-boot latency in milliseconds (ADR-0043
// A0 instrumentation). Not called on a warm/resumed named session (no boot)
// or by headless drivers. Nil-safe: drivers check before invoking. The
// Engine wires this per-Launch to populate BridgeResponse.BootMS.
```

### `go/internal/bridge/engine.go:180` — above `MkScratchDir func(dir, pattern string) (string, error)`

```text
// MkScratchDir creates a fresh private scratch directory under dir
// (signature mirrors os.MkdirTemp(dir, pattern); default os.MkdirTemp,
// which creates the dir 0o700). It gives each dispatch a per-invocation
// directory for transient files that must NOT collide when two same-phase
// dispatches share one workspace — currently the macOS SBPL sandbox
// profile (ADR-0049 S0 / gap G6). Tests inject a stub to drive the
// mkdir-error fallback branch deterministically.
```

### `go/internal/bridge/engine.go:188` — above `LivenessCenter *panestream.LivenessCenter`

```text
// LivenessCenter (ADR-0068, S3) is the LivenessCenter the tmux-REPL stop-review
// checkpoint observes/aggregates for StopEvent.State — the authoritative
// liveness source, replacing the bare per-run detectorFor(lp) probe. nil (the
// production default) has the driver build a private panestream.NewLivenessCenter()
// per run; tests inject a shared instance so a registered LivenessProbe can be
// proven both to win (its state reaches StopEvent.State) and to be invoked
// (its call count is observable) — a bypassed center could satisfy the former
// by coincidence but never the latter. An injected center must be
// per-dispatch: newReplWaitState registers the pane.liveness handler on it
// and there is no unregister, so a center shared across dispatches would
// accumulate handlers stamped with stale identities.
```

### `go/internal/bridge/engine.go:200` — above `Signals *signalcenter.Center`

```text
// Signals is the ADR-0101 Signal Center the engine produces into:
// bridge.warning / bridge.tripwire from the attempt telemetry, and — through
// the LivenessHandler the tmux driver registers on the LivenessCenter —
// pane.liveness. nil is the Null Object for tests; the production Adapter
// injects it at construction (adapters/bridge.NewDefault) and SignalsWired
// proves it reached the engine.
```

### `go/internal/bridge/engine.go:329` — above `Completion string`

```text
// Completion selects the phase-completion contract (ADR-0027): "" /
// "artifact" = poll for the artifact file (default, legacy); "stdout" =
// complete on REPL-idle for agents that print their answer (router/advisor).
```

### `go/internal/bridge/engine.go:354` — above `Realization Realization`

```text
// Realization is the per-CLI launch realization (ADR-0022): the model,
// permission, and raw flags this CLI actually understands, resolved from a
// LaunchIntent against the CLI's manifest. The *-tmux drivers build their
// launch command from Realization.LaunchFlags rather than constructing
// model/permission flags inline, so one CLI's argv never leaks into another.
```

### `go/internal/bridge/engine.go:399` — above `d.Signals.Emit(signalcenter.Event{`

```text
// Fail-open must be loud while accurately describing what remains:
// lifecycle/outcome records continue, but token counts are unavailable.
// ADR-0101 S3: a bridge.warning the root's sink renders (no hand-written line).
```

### `go/internal/bridge/engine.go:411` — above `func (e *Engine) SignalsWired() bool { return e.deps.Signals != nil }`

```text
// SignalsWired reports whether a Signal Center reached this Engine — the
// wiring proof the production Adapter's tests assert (ADR-0101 S3).
```

### `go/internal/bridge/engine.go:431` — above `func launchArgs(req core.BridgeRequest, promptFile, stdoutLog, stderrLog string, deps Deps) []string {`

```text
// launchArgs is the pure construction of a Launch's bridge-CLI argument list
// (extracted from Launch so flag emission is testable without driving a real
// CLI). deps supplies the per-agent artifact budget; req.BudgetScale scales it
// (ADR-0076 slice A).
```

### `go/internal/bridge/engine.go:506` — above `func (e *Engine) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {`

```text
// Launch satisfies core.Bridge: the in-process entry the M7 adapter
// cutover routes to. It maps a BridgeRequest onto the LaunchArgs pipeline
// (materializing req.Prompt to a file, mirroring the bash bridge's
// --prompt-file contract), then reads the artifact into the response on
// success — matching the existing subprocess adapter's behavior so the
// cutover is a drop-in.
//
// The Template Method host, split in place into named steps (ADR-0103 unit
// 10): the gauntlet → materializeInputs → the argv → runScoped → the attempt
// record → clearBootStrike → readResult on ExitOK, else persistLaunchError →
// the unit-10 classifier → recordBootStrike → ONE BRIDGE_EXIT_* event. Each
// step's name is the fields.step a triage reads; the step order is the
// on-disk order (the launch-error persist before the strike record).
//
// Concurrent-safe on Engine state: Launch captures BootMS via a call-local
// OnBoot hook installed on a per-call Deps COPY (runScoped), so it never
// mutates the shared e.deps. Production still builds a fresh Engine per
// Launch (adapters/bridge); this makes that contract structural rather than
// convention. (A caller that injects genuinely shared, non-thread-safe Deps —
// e.g. a common BootTimeoutStore — still owns that dependency's own
// concurrency.)
```

### `go/internal/bridge/engine.go:598` — above `func (e *Engine) runScoped(ctx context.Context, args []string, env map[string]string) launchRun {`

```text
// runScoped runs the LaunchArgs pipeline against a scoped Engine holding a
// per-call Deps COPY: the call-local OnBoot (cold-boot latency, ADR-0043 A0)
// and onModelDispatch hooks chain any pre-wired callback, so EVERY method in
// the pipeline (LaunchArgs, runDryRun, requireFullCheck, driver dispatch)
// reads the call-local hooks by construction and the shared e.deps is never
// mutated — concurrent Launch on one Engine is race-free, no defer-restore
// needed. e.deps is already defaulted (NewEngine), so the scoped Engine needs
// no re-defaulting.
```

### `go/internal/bridge/engine.go:666` — above `func (e *Engine) persistLaunchError(c attemptLogContext, workspace, agent, stderr string) string {`

```text
// persistLaunchError keeps the captured stderr as <workspace>/<agent>-launch-
// error.txt and returns the path, or "" when there was nothing to persist or
// the write failed (BRIDGE_LAUNCH_ERROR_PERSIST_FAILED). R3.6 (inbox
// bridge-launch-validation-stderr-lost): a launch dying in the validate
// gauntlet fails BEFORE the per-agent stderr-log exists, so without this file
// the diagnostic evaporates (cycle-270: a bare "launch exit=10" cost a
// forensic session; the cause was one missing profile file). The classifier
// threads the first line into the error chain so <phase>-failure-diag.json
// carries the "[bridge] …" cause; bridgeExitCode's digit scan stops at the
// ':', so appending the cause never breaks exit-code parsing.
```

### `go/internal/bridge/engine.go:725` — above `const tripwireSuccessThreshold = 60 * time.Second`

```text
// tripwireSuccessThreshold is the wall-clock floor separating a genuine
// unmeasured success from a quiet quota-abort: only launches that ran longer
// than this can be real work worth building a collector for (cycle-1005).
```

### `go/internal/bridge/engine_ctxcancel_deliverable_test.go:3` — above `import (`

```text
// engine_ctxcancel_deliverable_test.go — deliverable-authority-centralization
// (cycle-859 false-FAIL, the 5th rung of the class). A phase whose completion
// signal never fires rides its wait window until the orchestrator cancels the
// context; exec.CommandContext then SIGKILLs the driver subprocess, and Go's
// (*exec.ExitError).ExitCode() reports -1 for a signal death. The engine mapped
// -1 to a PLAIN error (neither 81 nor the transient set), so the generic phase
// runner took its substantive-error door and hard-FAILed WITHOUT consulting the
// on-disk deliverable — discarding a green-ACS PASS audit whose report, sentinel,
// and challenge token were all well-formed on disk.
//
// A context-cancellation kill is infra teardown, the sibling of the 124 cmd-
// timeout and the 81 artifact-timeout: classify it as transient so the runner's
// reconcile door consults the deliverable. Gated on ctx.Err() so a genuine
// start/launch failure (also -1, but with a live context) still fails loud.
```

### `go/internal/bridge/engine_ctxcancel_deliverable_test.go:28` — above `func TestEngineLaunch_CtxCancelSignalKill_WrapsTransient(t *testing.T) {`

```text
// TestEngineLaunch_CtxCancelSignalKill_WrapsTransient: a driver SIGKILL'd by
// context cancellation (exit -1, ctx.Err() != nil) must wrap
// core.ErrTransientBridgeFailure so IsInfraTeardownError routes it to the
// runner's reconcile-against-the-deliverable door (cycle-859).
```

### `go/internal/bridge/engine_launch_pins_test.go:3` — above `import (`

```text
// engine_launch_pins_test.go — ADR-0103 unit 10, the host pins written on
// the pre-extraction code (8e8f080f) and kept: the four required-field
// strings, the stdout-completion read path, the boot-strike clear on a
// non-80 exit, the exit → sentinel table, and the ACS source tokens engine.go
// must keep spelling. Each was proven red against its named mutant before
// the launch-outcome classifier moved out (design §6 tests 1-4, 8).
```

### `go/internal/bridge/engine_launch_steps_test.go:3` — above `import (`

```text
// engine_launch_steps_test.go — ADR-0103 unit 10, the split Launch spine and
// its emissions (design §6 tests 41-48): the attempt context rides every
// unit-10 event (call_id is the llm-calls.ndjson join key), the four step
// failures are bridge.warning codes with per-step origins where two were
// "[engine]" stderr lines and two were silent, exactly one BRIDGE_EXIT_* per
// non-zero exit after the persist and the strike record, nothing on ExitOK.
// The four host codes are named by IDENTIFIER (apicover counts exported
// consts named by a bridge test).
```

### `go/internal/bridge/engine_launch_tokens_amplify_test.go:17` — above `func amplifyEngine(t *testing.T, ws string, usage cyclestate.TokenUsage, source tokenusage.Source) (*Engine, string) {`

```text
// engine_launch_tokens_amplify_test.go — adversarial amplification for
// token-telemetry S3 (cycle 598). These tests are designed black-box from the
// TDD "Builder Contract" (test-report.md) alone; the engine.go/ports.go
// implementations were NOT read. They target contract clauses the three RED
// tests in engine_launch_tokens_test.go leave unguarded:
//
//   - nil TokenResolver leaves usage unavailable while lifecycle telemetry remains.
//   - Attempt==0 (existing callers) must default to attempt 1, not 0.
//   - the append must not truncate a pre-existing llm-calls.ndjson.
//   - the full on-disk JSON schema S6/S7 rollups decode (phase==agent,
//     nested input/output/cache_read/cache_write, RFC3339 ts, exit_code).
//   - the record write is gated on resolver *presence*, not usage magnitude:
//     a zero-usage-but-no-error resolve still emits a record.
//
// The record schema type llmCallRecord and the harness helpers (fakeRunner,
// writeProfile, mapLookup, NewEngine, Deps, ExitOK) are reused from the
// existing package tests — this file only adds new test funcs.
```

### `go/internal/bridge/engine_launch_tokens_test.go:18` — above `type llmCallRecord struct {`

```text
// engine_launch_tokens_test.go — RED contract for token-telemetry S3
// (cycle 598): Engine.Launch (engine.go:323) must populate
// core.BridgeResponse.Tokens from an injected Deps.TokenResolver and append
// one record per Launch attempt to <Workspace>/llm-calls.ndjson. See
// test-report.md "Builder Contract" for the exact seam Builder must add
// (Deps.TokenResolver, core.BridgeRequest.Attempt) — this file does not
// implement production code, only the RED tests encoding the acceptance
// criteria named in the cycle-598 inbox item
// (token-telemetry-s3-engine-launch-instrumentation).
```

### `go/internal/bridge/engine_tripwire_amplify_test.go:3` — above `import (`

```text
// engine_tripwire_amplify_test.go — Test Amplification pass for cycle-1005
// (telemetry-coverage-tripwire-nonclaude-success). Black-box adversarial cases
// designed from the TDD contract in test-report.md / build-report.md ONLY —
// no implementation file was read while designing these. Reuses the
// tripwireCase / runTripwireCase / tripwireStderrLine harness defined in
// engine_tripwire_test.go (same package); that file is NOT modified.
//
// Contract under test (engine.go recordTokenUsage tripwire escalation):
//
//	trip_when := result.Source==SourceNone && code==0 &&
//	             end.Sub(start) > 60*time.Second && !isClaudeDriver(req.CLI)
//
// These cases probe boundaries and semantics the RED contract's 7 cases did
// not isolate: the exact 60s threshold edge, exit codes other than the
// documented 85 quota-abort, case sensitivity and substring-vs-prefix on the
// "claude" driver check, an empty CLI identity, coexistence of the tripwire
// line with the pre-existing generic WARN, ndjson field presence across every
// silent path (not just one), and a pathologically large duration.
```

### `go/internal/bridge/engine_tripwire_test.go:3` — above `import (`

```text
// engine_tripwire_test.go — RED contract for cycle-1005, fleet item
// telemetry-coverage-tripwire-nonclaude-success (scout Task 1
// telemetry-tripwire-nonclaude-exit0-warn + Task 2
// telemetry-tripwire-llm-calls-record).
//
// Today recordTokenUsage (engine.go:597-600) fires ONE generic per-driver
// coverage WARN on every SourceNone resolution, regardless of exit code or
// duration. So a quiet quota-abort (exit 85, a few seconds) and a genuine
// unmeasured success (exit 0, minutes long, non-claude) read IDENTICALLY — the
// operator cannot tell "nothing to see" from "go build a collector now".
//
// This contract requires an ESCALATION on top of the existing generic WARN: when
// a non-claude launch exits 0, ran longer than the success threshold (>60s), and
// still resolved to source=none, recordTokenUsage must emit a distinct
// TRIPWIRE-marked stderr line naming the CLI, the agent, and (best-effort, from
// the workspace path) the cycle — and fold a queryable "tripwire":true field into
// the llm-calls.ndjson record. Quota-abort, claude-baseline, covered, and
// short-duration launches must NOT trip.
//
// DO NOT modify these tests. Make them pass by adding the escalation at the
// call site (resolver stays fail-open, per engine.go:558-566).
```

### `go/internal/bridge/engine_tripwire_test.go:74` — above `var errBuf bytes.Buffer`

```text
// ADR-0101 S3: the engine no longer hand-writes telemetry lines; errBuf
// holds what the root's WARN-filtered stderr sink renders for the
// engine's signals — the same one-line format the operator reads.
```

### `go/internal/bridge/engine_tripwire_test.go:93` — above `req.Cycle = 1005`

```text
// The dispatch identity comes from the request (ADR-0101 S3): the
// cycle the workspace path names is the one the dispatcher stamped.
```

### `go/internal/bridge/escalation_evidence_test.go:1` — above `package bridge`

```text
// escalation_evidence_test.go — CB.6 contract (concurrency campaign W4):
// pane evidence SURVIVES the session's death. Cycle-286's tmux server was
// killed mid-phase; every later interval capture returned nothing, so the
// escalation report's final_pane carried no evidence and the retro
// misattributed the failure to plan limits. The wait loop must retain the
// last NON-EMPTY pane and fall back to it when the live capture is gone —
// scrollback captured before teardown, kept until the report is written.
```

### `go/internal/bridge/escalation_evidence_test.go:19` — above `type dyingServerTmux struct {`

```text
// dyingServerTmux serves scripted frames, then reports the session (and any
// capture) gone — the cycle-286 shape: server killed under a live launch.
```

### `go/internal/bridge/exec_integration_test.go:38` — above `ctxCancel, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)`

```text
// ctx-cancel mid-Wait → exec.CommandContext SIGKILLs the child → (-1, nil).
// This locks in the causal premise the deliverable-authority-ctxcancel fix
// (cycle-859) rests on: a driver killed by our own cancellation exits -1
// (ExitCode()'s signal-death value) with cmd.Wait() returning an *exec.ExitError
// (classified away → nil second return), distinct from the (-1, err) start
// failure above. Engine.Launch then maps this -1 (with ctx.Err() != nil) to a
// transient teardown so the runner's reconcile door consults the deliverable.
```

### `go/internal/bridge/exhaustion_drift.go:9` — above `func warnExhaustionRegexDrift(w io.Writer, pfx, cli, pane, exhaustedRegex string) {`

```text
// exhaustion_drift.go — a fail-loud DRIFT alarm for the exhausted_regex.
//
// Exhaustion detection keys off a regex that must track a provider's UI wording,
// which changes without notice. The per-model "You've reached your <Model> limit"
// wording (Claude Code v2.1.212) silently stopped matching the legacy pattern and
// 8 audit cycles burned as generic exit-81 artifact timeouts before an operator
// hand-read a pane and spotted it. A fail-OPEN detector (a regex miss degrades to
// "not exhausted") is indistinguishable from "all healthy" unless something
// watches for the miss. This is that watcher: it converts the NEXT such drift
// from an 8-cycle silent burn into a single loud line.
```

### `go/internal/bridge/exhaustion_drift_test.go:17` — above `const narrowExhausted = '(?i)reached your (usage|weekly) limit'`

```text
// A narrow pattern matching only the LEGACY wording — it misses the per-model
// wall, reproducing the exact gap the per-model incident hit.
```

### `go/internal/bridge/exhaustion_drift_test.go:133` — above `const driftAlarmMarker = "POSSIBLE EXHAUSTION-REGEX DRIFT"`

```text
// --- Call-site integration: the drift alarm must scan the AGENT-STRIPPED pane.
//
// The primary exhaustion detector runs on strippedForExhaustionScan (the
// fast-poll and the 300s checkpoint both do, so a working agent that merely
// QUOTES wall text is never benched — cycles 254/255/314/641). The drift alarm
// one hop downstream (driver_tmux_repl.go, post exit-81 teardown) is fed the RAW
// lastGoodPane, so the same agent-authored content that the real detector
// correctly ignored can still trip "POSSIBLE EXHAUSTION-REGEX DRIFT" — a false
// alarm that sends an operator chasing a regex that is working exactly as
// intended. These tests drive the REAL driver loop to its exit-81 teardown and
// assert on what the alarm actually printed, so they pin the call site's pane
// treatment, not a helper signature.
```

### `go/internal/bridge/exhaustion_drift_test.go:167` — above `func TestTmuxREPL_DriftAlarm_PromptEchoContent_NoFalseAlarm(t *testing.T) {`

```text
// Prompt-echo content (the agent's own injected instructions rendered back into
// the pane) must NOT raise the drift alarm either — the second half of what
// strippedForExhaustionScan removes (cycle-641/642). RED today.
```

### `go/internal/bridge/exhaustion_persistence.go:3` — above `const exhaustionPersistObservations = 2`

```text
// exhaustion_persistence.go — persistence guard for the quota/rate-limit fast-fail.
//
// The exhaustion detectors (the ~2s fast-poll's ExhaustedOf and the 300s
// stop-review checkpoint's Observe) match a regex against the RAW captured
// pane. That pane is not just CLI chrome: an agent doing ordinary work can
// render wall-shaped TEXT into it — a `cat`/`grep`/diff of a file, test
// fixture, or incident report that quotes a provider's "you've reached your …
// limit" message. Fast-failing on that single frame kills a WORKING agent
// (exit 85 → cross-family failover), the cardinal false-FAIL sin
// (cycle-254/255/314/641) the go-review of the per-model regex fix surfaced.
//
// The robust, regex-independent discriminator is PERSISTENCE. A genuine wall is
// the CLI's TERMINAL state — it parks there and the same text is present on the
// next observation too (a re-printing error still shows the wall every frame,
// preserving the agy-hang fix that motivated the override). Wall text merely
// PASSING THROUGH a working agent's pane is gone by the next observation, as the
// agent's fresh output scrolls it off. So: fast-fail only after the wall has
// persisted for `threshold` consecutive observations; a single transient match
// never crosses. This costs one extra observation of latency on a real wall
// (~one fast-poll tick), a trade the fail-over-vs-kill asymmetry makes trivially
// worth it: a missed wall merely fails over (safe); a killed working agent is not.
//
// One gate instance per detection loop (the fast-poll owns one, the checkpoint
// owns one) — they observe at different cadences and must each require their OWN
// consecutive frames, so they never share a streak.
```

### `go/internal/bridge/exhaustion_persistence_test.go:9` — above `func TestExhaustion_TransientWallTextDoesNotFastFail(t *testing.T) {`

```text
// A working agent that momentarily RENDERS wall-shaped text — a cat/grep/diff of
// a file, test fixture, or incident report that quotes a provider's "reached your
// … limit" message — must NOT be fast-failed. The text is gone by the next
// observation as the agent's fresh output replaces it, so the persistence gate
// never crosses. A genuine wall (the CLI parked, present every frame) DOES cross.
//
// This is the raw-pane false-FAIL class the two-round go-review of the per-model
// exhausted_regex surfaced: killing a working agent (exit 85 → cross-family
// failover) is the cardinal sin (cycle-254/255/314/641), strictly worse than
// missing a wall (which merely fails over). The regex tightening reduced the
// match surface; THIS persistence guard is the durable class fix.
```

### `go/internal/bridge/faketmux_amplify_test.go:3` — above `import (`

```text
// faketmux_amplify_test.go — adversarial tests for FakeTmuxController (cycle-276
// T1). The builder's tmux_repl_fixture_test.go proves the three named fixture
// scenarios (boot-success, boot-timeout, artifact-delivery). These tests probe
// the controller's own behavioral contracts: panic-on-underrun, event-recording
// fidelity, and multi-operation ordering — all properties the ACS predicates
// rely on but don't directly test.
```

### `go/internal/bridge/fatalpane.go:3` — above `import (`

```text
// fatalpane.go — ADR-0044 C2: the fatal-pane fast-fail seam at the
// stop-review checkpoint (the bridge half of the Phase Recovery Pipeline's
// Analyze stage; the registry itself lives in internal/recovery, the single
// recovery owner).
//
// cycle-262 burned ~40 min waiting out the maxExtends backstop on two
// self-describing fatal pane states — the pane literally said "There's an
// issue with the selected model" / "Please restart Codex", but nothing read
// it. Worse, the bridge's own one-shot nudge echoed into the dead pane and
// counted as "progress" the next interval, buying extensions for a REPL that
// no longer existed. This seam consults the deterministic
// recovery.FatalPaneDetector BEFORE the StopReviewer each checkpoint, so a
// known-fatal state exits the wait in ONE interval and hands the phase to
// the runner's exit-81 fallback chain (which is exactly what rescued the
// cycle-262 build — 20 minutes too late).
//
// Stage discipline (recovery.fatal_pane — this seam's OWN dial since F27,
// split out of the whole-program PhaseRecovery dial the way SpineFloor was):
//
//	off     → detector not consulted; byte-identical legacy flow
//	shadow  → detect + log the would-be fast-fail; legacy verdict decides
//	          (the soak stage; also what an unwired Deps resolves to)
//	enforce → a fatal match on a non-Busy pane preempts the reviewer (the
//	          policy DEFAULT since F27 — every shadow match on record was a
//	          dead pane that then idled 900-1200s)
//
// A Busy pane is never preempted regardless of stage: the stop-review
// layer's prime directive (never kill a working agent — see the cycle-254/255
// false-FAIL post-mortem in stopreview.go) outranks fast-fail.
```

### `go/internal/bridge/fatalpane.go:42` — above `func recoveryStageFromEnv(deps Deps) string {`

```text
// recoveryStageFromEnv resolves the bridge-side ADR-0044 phase recovery stage.
// The stage is injected by the orchestrator via Deps.RecoveryStage (policy-resolved);
// empty → channel.ResolveStage returns "shadow" (the behavior-neutral default).
// channel.ResolveStage is the single home of the resolution rule so the in-process
// and subprocess readers can never drift (ADR-0045 I6).
```

### `go/internal/bridge/fatalpane.go:51` — above `func fatalPaneStageOf(deps Deps) string {`

```text
// fatalPaneStageOf resolves the fatal-pane fast-fail's OWN stage (F27) through
// the same normalizer as the program dial — unset → shadow, typo → off — and
// reads ONLY Deps.FatalPaneStage, so the two dials can never borrow each other.
```

### `go/internal/bridge/fatalpane.go:85` — above `cause, sig, ok := det.Detect(strippedForFatalPaneScan(ev.StdoutTail, ev.InjectedPrompt, det.Signatures()))`

```text
// Scan the agent-STRIPPED pane, not the raw tail. Until cycle-1117 this
// seam read ev.StdoutTail directly while its twin one field away (the
// exhaustion scan) read a stripped pane — so an agent EDITING the fatal
// registry, its diff view rendering `Substr: "There's an issue with the
// selected model"`, was fast-failed on its own edit buffer. The protect-list
// comes FROM the live registry (det.Signatures()), so the echo half can
// never suppress a signature the detector is looking for.
```

### `go/internal/bridge/fatalpane_persistence.go:3` — above `import (`

```text
// fatalpane_persistence.go — persistence guard for the ADR-0044 C2 fatal-pane
// fast-fail, the exact sibling of exhaustion_persistence.go's quota-wall guard.
//
// The fatal-pane detector matches SUBSTRINGS against the RAW captured pane
// ("There's an issue with the selected model", "Please restart Codex" — see
// fatalpane.go's cycle-262 postmortem). That pane is not just CLI chrome: a
// WORKING agent can render fatal-shaped TEXT into it by cat/grep/diffing an
// incident report, a test fixture, or a log excerpt — and then be killed for
// quoting the thing it was asked to read. That is the cardinal false-FAIL
// (cycle-254/255/314/641) the quota-wall guard was built to close; the
// fatal-pane seam matched the same way but fired on ONE observation.
//
// Same regex-independent discriminator, same asymmetry argument: a genuinely
// fatal pane is the CLI's TERMINAL state — it parks there and the text is
// present on the next checkpoint too — while fatal-shaped text merely passing
// through a working agent's pane is gone by then. Cost of the guard is one
// extra checkpoint of latency on a real fatal pane; the trade is trivially
// worth it, because a missed fatal pane merely waits out the legacy reviewer
// (the pre-C2 behavior, safe), while a killed working agent is not.
//
// The gate is ADDITIVE state around fatalPaneVerdict, not a change to it: an
// observation that crosses the threshold delegates to the unchanged seam, so
// the stage discipline, the verdict shape, and the R8.3 durable evidence are
// all byte-identical for a gate-crossed call. Un-crossed observations record
// NOTHING at any stage — shadow must predict the GATED enforce action or the
// R8.5 would/did parity check compares two different semantics.
//
// One gate instance per checkpoint loop, exactly like checkpointExhaustGate:
// a gate reconstructed per checkpoint can never accumulate a streak, which
// would leave the unit tests green while production stayed un-gated.
```

### `go/internal/bridge/fatalpane_persistence_test.go:3` — above `import (`

```text
// fatalpane_persistence_test.go — RED tests for the fatal-pane persistence gate
// (cycle-1118). The exhaustion fast-fail is persistence-gated
// (exhaustion_persistence.go: a wall must be present on `threshold` CONSECUTIVE
// observations before it can kill the phase) precisely because the detectors match
// against the RAW captured pane, and a WORKING agent can render fatal-shaped TEXT
// into that pane — a cat/grep/diff of an incident report, a test fixture, a log
// excerpt. The fatal-pane seam (fatalpane.go) matches the same way — on pane
// substrings like "There's an issue with the selected model" / "Please restart
// Codex" — but fires on a SINGLE observation. Same false-FAIL class
// (cycle-254/255/314/641), unguarded.
//
// Contract under test — a loop-scoped gate wrapping the per-checkpoint decision:
//
//	gate := newFatalPaneGate()                      // ONE instance per checkpoint loop
//	v, preempted := gate.verdict(det, ev, stage, rec, stderr, pfx)
//
//  1. a fatal match must have persisted for fatalPanePersistObservations
//     CONSECUTIVE observations before it can preempt (enforce) or leave C2
//     evidence (shadow) — a transient frame never crosses;
//  2. any non-matching observation — healthy pane, or a Busy pane (busy outranks
//     the detector at every stage) — RESETS the streak;
//  3. a genuinely parked pane still fast-fails, at the threshold observation, with
//     the unchanged ADR-0044 C2 verdict — bounded extra latency, no regression of
//     the cycle-262 rescue path;
//  4. off / "" / nil detector never observe at all: a disabled path must not
//     silently accumulate a streak that a later stage flip could cash in.
//
// fatalPaneVerdict's OWN signature and single-observation semantics are unchanged
// (fatalpane_test.go / fatalpane_durable_test.go stay green unmodified) — the gate
// is additive state around the call, owned by the checkpoint loop the way
// checkpointExhaustGate is.
```

### `go/internal/bridge/fatalpane_persistence_test.go:116` — above `func TestFatalPaneGate_PersistentFatalPaneStillFastFails(t *testing.T) {`

```text
// TestFatalPaneGate_PersistentFatalPaneStillFastFails — the regression bound on
// the cycle-262 rescue path: a pane parked in a fatal state (present every
// checkpoint) must still fast-fail, at the threshold observation, with the
// unchanged ADR-0044 C2 verdict — and record fast_failed exactly ONCE (an inflated
// C2 count breaks the R8.5 would/did parity check).
```

### `go/internal/bridge/fatalpane_strip_c1117_test.go:3` — above `import (`

```text
// fatalpane_strip_c1117_test.go — cycle-1117 RED tests for
// `fatalpane-strip-agent-content` (re-attempt of the cycle-1115 diff the
// auditor FAILed on defects D1 + D2).
//
// THE ASYMMETRY BEING CLOSED. driver_tmux_repl.go's exhaustion scan reads a
// pane with agent-rendered content removed (strippedForExhaustionScan), but
// fatalpane.go hands the fatal-pane detector the RAW ev.StdoutTail. So an
// agent EDITING recovery/detector.go — its diff view literally rendering
// `+ Substr: "There's an issue with the selected model"` — can be fast-failed
// on its own edit buffer, while the exhaustion detector, one field away, is
// immune. Two detectors, two meanings of "the pane".
//
// WHY THE NAIVE FIX WAS REJECTED (cycle-1115 audit, confidence 0.90):
//
//	D1 — stripPromptEchoLines DELETES matching lines and rejoins with "\n",
//	     shifting every survivor's position. Four seeded signatures are
//	     newline-ANCHORED ("\nquote>", "\nbquote>", "\ndquote>",
//	     "\nheredoc>") precisely so a bare word cannot false-positive.
//	     Deleting the line above a continuation prompt makes the survivor
//	     line one, drops its leading "\n", and Detect silently misses —
//	     reverting the cycle-274 dead-shell fast-fail in exactly the
//	     prompt-spill scenario it was seeded for.
//	D2 — echo-stripping is substring-keyed, and two seeds are literal English
//	     sentences. Any phase prompt that QUOTES one (a detector-hardening
//	     cycle, a retro, this very todo's prose) makes the CLI's real banner
//	     indistinguishable from an echo and silences the cycle-262 fast-fail.
//
// THE CONTRACT THESE TESTS PIN (the auditor's remediation direction, and the
// production API Builder must create — RED today = compile failure):
//
//	recovery: func (d *FatalPaneDetector) Signatures() []string
//	    the seeded substrings, so the stripper's protect-list can never drift
//	    from the registry it is protecting.
//	bridge:   func strippedForFatalPaneScan(pane, injectedPrompt string, protected []string) string
//	    the fatal-pane twin of strippedForExhaustionScan. Removes agent-diff
//	    AND prompt-echo content by BLANKING each matched line in place —
//	    never deleting it — so every surviving line keeps its leading "\n"
//	    (D1). A line containing any TrimSpace'd entry of protected is left
//	    untouched by the prompt-echo half (D2): a prompt quoting a fatal
//	    signature must never suppress that signature on-pane. The diff half
//	    is NOT protect-listed — diff-prefixing is proof of agent authorship
//	    by construction (cycle-314), and suppressing agent-authored seed text
//	    is the entire point of this task (see C1117_003 below).
//	bridge:   StopEvent.InjectedPrompt string
//	    populated at the driver's checkpoint construction site from the
//	    already-resolved prompt (the same source strippedForExhaustionScan
//	    uses), and consumed by fatalPaneVerdict.
//
// DO NOT MODIFY THESE TESTS to make them pass — they are the acceptance
// criteria. The parameter NAMES of strippedForFatalPaneScan are part of the
// contract: go/acs/cycle1117 mutates the function by overlay to prove these
// tests are load-bearing, and a renamed parameter fails that predicate loudly.
```

### `go/internal/bridge/fatalpane_strip_c1117_test.go:83` — above `var c1117AnchoredSeeds = []string{"\nquote>", "\nbquote>", "\ndquote>", "\nheredoc>"}`

```text
// c1117AnchoredSeeds are the four newline-anchored dead-shell signatures. The
// anchor is the whole defence against a bare word false-positive, so line
// POSITION is load-bearing for every one of them (cycle-274/277).
```

### `go/internal/bridge/fatalpane_strip_c1117_test.go:138` — above `func TestC1117_PromptQuotingSeedDoesNotSuppressBanner(t *testing.T) {`

```text
// TestC1117_PromptQuotingSeedDoesNotSuppressBanner — AC2, the D2 regression.
// The pane shows the CLI's REAL model-invalid banner while the injected prompt
// happens to quote that same sentence (a detector-hardening cycle, a retro,
// this todo). Substring-keyed echo stripping would eat the banner and silence
// the cycle-262 fast-fail; the protect-list must keep it.
```

### `go/internal/bridge/fatalpane_strip_c1117_test.go:166` — above `func TestC1117_AgentDiffSeedTextDoesNotFastFail(t *testing.T) {`

```text
// TestC1117_AgentDiffSeedTextDoesNotFastFail — AC1, the NEGATIVE axis and the
// load-bearing proof that fatalPaneVerdict consults a STRIPPED pane at all.
// An agent editing the fatal registry renders seed text on numbered diff lines;
// that is the agent's own content, not the CLI's chrome, and must not kill it.
// On the raw pane (today's code, or any pass-through "strip") this fast-fails.
//
// Note the deliberate asymmetry with C1117_002: diff lines are NOT protect-
// listed. Diff-prefixing is proof of agent authorship by construction
// (cycle-314), whereas "appears in the prompt" is the weak signal D2 showed can
// swallow a genuine banner.
```

### `go/internal/bridge/fatalpane_strip_c1117_test.go:206` — above `func TestC1117_StopEventCarriesInjectedPromptFromDriver(t *testing.T) {`

```text
// TestC1117_StopEventCarriesInjectedPromptFromDriver — AC3, the wiring proof.
// The behavioral tests above pass on a field production never populates (the
// exact gap that left cycle-1115's helper inert). This drives the real
// production path — Engine.LaunchArgs → runTmuxREPL → the checkpoint's
// StopEvent construction — with a reviewer that records the event, and asserts
// the prompt actually arrived.
```

### `go/internal/bridge/fatalpane_test.go:3` — above `import (`

```text
// fatalpane_test.go — ADR-0044 C2 (Slice 2) RED tests: the fatal-pane
// fast-fail seam at the stop-review checkpoint.
//
// cycle-262 mechanism: a dead pane (codex self-update → bare zsh; claude
// --model auto boot error) never produces an artifact, but the bridge's own
// nudge text echoes into the pane, reads as "progress" next interval, and
// buys extension after extension — ~20 min per phase against maxExtends on a
// state that was fatal on sight. The fix consults the deterministic
// recovery.FatalPaneDetector BEFORE the reviewer at each checkpoint:
//
//   stage=off     → detector not consulted; byte-identical legacy flow
//   stage=shadow  → detect + log the would-be fast-fail; legacy verdict still
//                   decides (behavior-neutral soak; the DEFAULT)
//   stage=enforce → a fatal match on a non-Busy pane preempts the reviewer
//                   with ReviewStop; the wait exits this interval and the
//                   runner's exit-81 fallback chain takes over immediately
//
// A Busy pane is NEVER preempted regardless of stage — the prime directive of
// the stop-review layer (never kill a working agent) outranks fast-fail.
```

### `go/internal/bridge/fatalpane_test.go:133` — above `func TestRecoveryStageFromEnv(t *testing.T) {`

```text
// TestRecoveryStageFromEnv pins the bridge-side stage resolution via
// Deps.RecoveryStage (policy-injected, ADR-0044): unset → shadow (the
// behavior-neutral default), a typo → off (never silently enabling a
// kill-path), explicit values normalized case-insensitively.
```

### `go/internal/bridge/fatalpane_test.go:155` — above `func TestFatalPaneStageOf(t *testing.T) {`

```text
// TestFatalPaneStageOf (F27) pins the fatal-pane dial's resolution through the
// SAME normalizer as the program dial — and reads ONLY Deps.FatalPaneStage:
// with the program dial at enforce, an unset fatal-pane dial still resolves to
// shadow (an unwired Deps stays observe-only; the composition root injects the
// policy default), and a typo resolves to off (never a silent kill-path).
```

### `go/internal/bridge/git_evidence_test.go:14` — above `type fakeGit struct {`

```text
// git_evidence_test.go — the ADR-0027 git-evidence completion contract:
// completion = a NEW commit in baseline..HEAD whose trailer verifies the phase
// + challenge token. Driven through the gitCmd seam (no real repo).
```

### `go/internal/bridge/hastokenresolver_test.go:3` — above `import (`

```text
// hastokenresolver_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96).
//
// Deps.TokenResolver is a DI seam nothing outside this package can inspect
// today (deps is unexported) — the two production composition roots
// (adapters/bridge.Adapter, subagent.defaultExecAdapter) build a gobridge.Deps
// and have no way to prove, in a test, that the field they set actually
// reached the constructed Engine. HasTokenResolver gives both call sites (and
// their tests — see adapters/bridge/tokenresolver_wiring_test.go and
// subagent/tokenresolver_wiring_test.go) that seam. It is undefined today, so
// this file fails to compile — the intended RED signal. Builder implements:
//
//	func (e *Engine) HasTokenResolver() bool { return e.deps.TokenResolver != nil }
```

### `go/internal/bridge/interaction_e2e_test.go:3` — above `import (`

```text
// interaction_e2e_test.go — ADR-0045 §8 "Integration (the self-correction
// proofs)". The per-slice tests prove each rung in isolation; these thread the
// rungs together end-to-end. The fourth §8 proof —
// TestE2E_MisplacedArtifact_SalvagedNoRedispatch — lives in core
// (TestSalvage_RelocatesThenVerifiesDESTINATION drives a full RunCycle), so it
// is not duplicated here.
```

### `go/internal/bridge/interaction_rules.go:3` — above `import (`

```text
// interaction_rules.go — ADR-0045 I4 CONSUMPTION side: merge promoted,
// enforce-stage auto-respond rules into a launch's active rule set. The
// PROMOTION side (the quarantined advisor that mints a rule from a novel
// escalation) reuses interaction.PromoteRule and is wired with the in-bridge
// advisor tail (a named follow-up — the same deferred-LLM plumbing the I3
// advisor needs). This file makes the registry LIVE: once a rule exists under
// .evolve/instincts/interaction-rules/, it fires here, re-validated against
// the immutable healthy-pane corpus at every load (corpus-rot demotes a rule
// whose pattern a new CLI version's banner now matches — threat S3).
```

### `go/internal/bridge/interaction_rules.go:42` — above `func interactionRulesDir(projectRoot string) string {`

```text
// interactionRulesDir is the durable promoted-rule registry, a sibling of the
// fatal-signature registry (ADR-0044 Slice 5) under the same instincts root.
```

### `go/internal/bridge/interaction_rules_test.go:3` — above `import (`

```text
// interaction_rules_test.go — ADR-0045 I4 consumption side: enforce-stage
// promoted rules become live auto-respond prompts; shadow rules do not; the
// embedded healthy corpus parses and a corpus-matching rule is demoted at load.
```

### `go/internal/bridge/interaction_telemetry_test.go:3` — above `import (`

```text
// interaction_telemetry_test.go — ADR-0045 I1 (slice 1): the bridge's two
// existing interactions (one-shot nudge, auto-respond sends) must record a
// typed Outcome in <workspace>/<phase>-interactions.ndjson, resolved against
// external evidence only (artifact presence, pane pattern state). cycles
// 263–269: `nudgeSent=true` and nothing measures whether any nudge ever
// worked — these tests pin that the measurement now exists and is honest.
```

### `go/internal/bridge/launch.go:133` — above `sessionMode := "ephemeral"`

```text
// Realize the launch intent against this CLI's manifest (ADR-0022). The
// *-tmux drivers build their launch command from this rather than
// constructing model/permission flags inline, so a claude-origin profile's
// raw flags realize only for the matching CLI (RawByCLI[agy/codex] = nil).
// permMode is still carried on Config for the safety gates and the headless
// claude-p driver; the realizer is what the tmux launch command consumes.
```

### `go/internal/bridge/launch.go:230` — above `if pf, ok := driver.(CLIPreflight); ok {`

```text
// Cycle-124 G3: per-CLI Preflight seam (ADR-0022 extension). Drivers
// that implement the optional CLIPreflight interface get a uniform
// hook BEFORE Launch — today codex-tmux uses it to pre-trust the
// worktree + workspace paths in ~/.codex/config.toml (cycle-122 Fix 1).
// Best-effort: a non-nil error is logged but does NOT abort Launch
// (matches the prior inline behavior; Fix 2's extended fallback
// trigger list defends downstream). A driver opts in by declaring the
// method; nothing changes for drivers that don't.
```

### `go/internal/bridge/launch_error_repro_test.go:41` — above `func TestLaunchFailurePersistsLaunchErrorFile(t *testing.T) {`

```text
// TestLaunchFailurePersistsLaunchErrorFile — R3.6 acceptance (inbox
// bridge-launch-validation-stderr-lost): a launch dying in the validate
// gauntlet (here: LoadProfile on a missing profile, the cycle-270 shape)
// must leave the cause string in the run dir as <agent>-launch-error.txt,
// because the failure precedes per-agent stderr-log creation.
```

### `go/internal/bridge/launch_outcome_fixture_test.go:3` — above `import (`

```text
// launch_outcome_fixture_test.go — ADR-0103 unit 10: the ONE scripted driver
// the launch-outcome goldens, the stream golden and the Launch step tests
// drive the real Engine.Launch → LaunchArgs → driver path with. The script
// rides ExtraFlags (after `--`, so it reaches Config.ExtraFlags through the
// same argv every launch field takes): "exit=<code>" is the exit the driver
// returns, "stderr=<fixture>" names the captured-stderr fixture it prints,
// "artifact=both" makes it write DISTINCT bytes to the artifact and the
// stdout log (the Completion == "stdout" read path). Registered on demand
// (ensureLaunchFixtureDriver) rather than at init: the registry-reset tests
// restore only the seven builtins, and TestDriverRegistry counts them
// strictly. The script is per-call, never package state.
```

### `go/internal/bridge/launch_outcome_golden_test.go:3` — above `import (`

```text
// launch_outcome_golden_test.go — ADR-0103 unit 10, the characterization
// goldens captured on the pre-extraction code (8e8f080f) and replayed after:
// what one launch exit MEANS (the error string, the sentinel it wraps, the
// attempt ledger's cause code), the full argv vector Launch serializes, and
// the ordered signal stream per Launch path. A missing golden is captured and
// the test FAILS ("captured — re-run"), so an absent file is never a silent
// green; the committed file is the oracle.
```

### `go/internal/bridge/launch_outcome_golden_test.go:262` — above `func TestEngineLaunch_SignalStream_Golden(t *testing.T) {`

```text
// Test 9 — the ordered {module, kind, code} sequence per Launch path equals
// testdata/launch-signals.golden.json (captured on 8e8f080f: the
// construction-time resolver warning only). The landing commit EDITS the
// golden with the unit's declared additions — the diff is the declaration.
```

### `go/internal/bridge/launch_outcome_seam_test.go:3` — above `import (`

```text
// launch_outcome_seam_test.go — ADR-0103 unit 10, the host seam: the exit
// numerics, the marker and the timeout-cause vocabulary are ONE belief with
// the leaf (consumer pins), the classifier is called from exactly one host
// site per projection, and the driver's marker line round-trips through the
// classifier's vocabulary (design §6 tests 36-38).
```

### `go/internal/bridge/launch_test.go:157` — above `func newTestEngine(d Deps) *Engine {`

```text
// newTestEngine builds an Engine for driver tests with a HERMETIC environment
// by default: a test that does not pin Deps.LookupEnv gets the EMPTY lookup
// rather than falling through to os.LookupEnv (lookupEnv, driver_common.go),
// carrying the `run`/`runLookup` convention above across to the tmux-REPL
// suite. Deps.Env is consulted first by lookupEnv and is unaffected, so a test
// that MEANS to exercise an env branch still opts IN explicitly.
//
// Why this exists: runTmuxREPL reads ipcenv.FleetKey ("EVOLVE_FLEET") through
// lookupEnv, and under a fleet supervisor with no --worktree it fails closed
// with the CB.2 refusal errWorktreeRequired -> ExitBadFlags(10) BEFORE the
// artifact wait loop runs. With LookupEnv nil that read reached the AMBIENT
// process env, so 20 driver tests passed in a developer shell and failed under
// the ACS/EGPS runner, which inherits the orchestrator's EVOLVE_FLEET=1
// (internal/acsrunner/runner.go does not sanitize). That cost cycle-1252 and
// cycle-1254 a run each. The guard is correct; the fixtures were porous.
```

### `go/internal/bridge/launchintent.go:3` — above `type LaunchIntent struct {`

```text
// launchintent.go — the CLI-agnostic launch abstraction (ADR-0022).
//
// A phase agent describes HOW it wants to be launched in high-level terms
// (which model tier, what permission posture, project-only settings, an
// ephemeral or named session). A per-CLI Realizer translates that intent into
// the concrete realization for ONE CLI — launch flags it actually defines,
// post-boot REPL input, controller (tmux lifecycle) hints — never a flag the
// target CLI does not understand. This decouples intent from realization so
// the same intent drives claude, codex, and agy without leaking one CLI's
// argv vocabulary into another.
```

### `go/internal/bridge/liveness_amplify_reviewer_test.go:9` — above `func TestAmp_Reviewer_IdleStateNotExtend(t *testing.T) {`

```text
// Adversarial amplification tests for the LivenessState → reviewer integration
// (cycle-423). Written by the Test Amplifier — black-box view, spec only.
//
// Coverage gaps targeted (not reached by ACS predicates C423_007–C423_010):
//   B1  State=Idle → never ReviewExtend (ACS only specs Converging and Hung paths)
//   B2  State=BusyButStagnant under maxExtends → ReviewExtend (bounded budget path)
//   B3  State=BusyButStagnant at/past maxExtends → NOT ReviewExtend (backstop, cycles 254/255)
//   B4  State=Hung at attempt=0 → fast-fail (AC10 only tests attempt=1)
//   B5  StopEvent.State zero-value ignores the retired legacy Progressed/Busy
//       fields (S3: the boolean fallback is retired, not backward-compat)
//   B6  Converging with non-positive maxExtends still extends (mirrors pre-existing Progressed test)
```

### `go/internal/bridge/liveness_amplify_reviewer_test.go:56` — above `func TestAmp_Reviewer_BusyButStagnantAtMaxExtendsBackstops(t *testing.T) {`

```text
// TestAmp_Reviewer_BusyButStagnantAtMaxExtendsBackstops verifies that
// State=BusyButStagnant at attempt >= maxExtends produces a non-extend verdict.
// This is the critical backstop for cycles 254/255: a busy-but-stagnant agent
// must be killed when its extension budget is exhausted. Unlike Converging, which
// extends unconditionally (AC9), BusyButStagnant IS bounded by maxExtends.
```

### `go/internal/bridge/liveness_matrix_test.go:25` — above `{"converging attempt=0", StopEvent{State: panestream.LivenessConverging, Attempt: 0}, ReviewExtend},`

```text
// Converging → ReviewExtend unconditionally (cycles 311/312: no cap on real output)
```

### `go/internal/bridge/liveness_matrix_test.go:30` — above `{"busy-stagnant attempt=0", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: 0}, ReviewExtend},`

```text
// BusyButStagnant → extend under cap, pause at/over cap (cycles 254/255)
```

### `go/internal/bridge/liveness_matrix_test.go:67` — above `func TestLivenessMatrix_ConvergingUnconditionalPastMaxExtends(t *testing.T) {`

```text
// TestLivenessMatrix_ConvergingUnconditionalPastMaxExtends pins the unconditional-
// extend invariant: Converging at attempt=9 (past maxExtends=2) still returns
// ReviewExtend. This closes cycles 311/312: a producing scout was killed at
// the maxExtends backstop even while emitting real output.
```

### `go/internal/bridge/livenesscenter_wedge_invariant_test.go:3` — above `import (`

```text
// livenesscenter_wedge_invariant_test.go — cycle-431 slice S3, Task B: pins
// every wedge-incident invariant against the center-authoritative liveness
// path (Task A). Each case names a real panestream.Liveness* constant and
// drives the actual production deterministicReviewer / runTmuxREPL — no
// stubs standing in for the reviewer under test (AC6 anti-gaming).
//
// Incident corpus:
//
//	cycle-311/312 — a producing agent is NEVER capped (Converging → extend
//	  unconditionally, past any maxExtends bound).
//	cycle-254/255 — a busy-but-silent pane extends UP TO maxExtends, then
//	  pauses (the bound a producing agent never reaches).
//	cycle-262      — a dead/echoing pane must classify Hung, not Converging
//	  (Hung fast-fails before the maxExtends backstop; Converging never
//	  stops extending on its own).
//	cycle-286/288  — non-empty pane evidence (StopEvent.StdoutTail) survives
//	  across a checkpoint whose live capture comes back empty (session death
//	  after the last good frame).
```

### `go/internal/bridge/livenesscenter_wedge_invariant_test.go:32` — above `func TestWedgeCorpus_Converging_ProducingNeverCapped(t *testing.T) {`

```text
// TestWedgeCorpus_Converging_ProducingNeverCapped (AC1, cycle-311/312):
// LivenessConverging must extend UNCONDITIONALLY — an attempt count far
// past maxExtends must still extend, because real output is never "stuck".
```

### `go/internal/bridge/livenesscenter_wedge_invariant_test.go:43` — above `func TestWedgeCorpus_BusyStagnant_BoundedThenPause(t *testing.T) {`

```text
// TestWedgeCorpus_BusyStagnant_BoundedThenPause (AC2, cycle-254/255):
// LivenessBusyButStagnant extends up to maxExtends, then pauses — the
// bound that distinguishes a silently-working agent from a genuinely stuck
// one.
```

### `go/internal/bridge/livenesscenter_wedge_invariant_test.go:57` — above `func TestWedgeCorpus_DeadPane_HungIsNotConverging(t *testing.T) {`

```text
// TestWedgeCorpus_DeadPane_HungIsNotConverging (AC3, cycle-262): a dead or
// self-echoing pane must classify LivenessHung and pause BEFORE the
// maxExtends backstop — never LivenessConverging, which the cycle-262
// bridge nudge-echo would otherwise ride to an unconditional, indefinite
// extend.
```

### `go/internal/bridge/livenesscenter_wedge_invariant_test.go:70` — above `func TestWedgeCorpus_EvidenceSurvivesEmptyCapture(t *testing.T) {`

```text
// TestWedgeCorpus_EvidenceSurvivesEmptyCapture (AC4, cycle-286/288): once
// the tmux server dies, every later capture returns empty — the
// checkpoint's StopEvent.StdoutTail must still carry the last NON-EMPTY
// pane, not go blank, or an escalation report built from it loses the only
// evidence of what the agent was doing.
```

### `go/internal/bridge/livenesscenter_wedge_invariant_test.go:112` — above `func TestStopReview_RenderWedgeOverride(t *testing.T) {`

```text
// TestStopReview_RenderWedgeOverride (cycle-432 S4, AC4, edge): the
// cycle-291 render-wedge override must survive the migration to
// center-sourced Busy — a blank pane from a LIVE session still classifies
// LivenessBusyButStagnant (never Idle) AND StopEvent.Busy stays true via the
// `center.Busy(session) || renderWedged` term, so a working agent is never
// paused purely on a pane-render failure. Pre-existing GREEN is acceptable
// here (the `|| renderWedged` term already exists pre-migration); this test
// pins it as a regression guard so S4 cannot silently drop the term while
// relocating the Busy source.
```

### `go/internal/bridge/livesmoke.go:22` — above `func LiveSmokeTest(ctx context.Context, driverName string, cfg *Config, deps Deps) (rc int, pattern, scrollback string) …`

```text
// LiveSmokeTest performs a REAL launch of the given *-tmux driver that
// SUBMITS one trivial contracted prompt and waits (briefly) for the artifact.
// It is the only probe shape that can see a quota wall: BootSmokeTest passes
// against a rate-limited CLI because provider walls appear only after work is
// submitted (cycle-283 — every codex phase re-discovered the wall the
// expensive way). Used by `evolve doctor live` and the loop's per-cycle bench
// canary.
//
// Returns the bridge exit code, the escalation pattern name when the launch
// died on a classified interactive wall ("rate_limit" — empty otherwise), and
// the captured pane scrollback (carries the wall text for reset-hint parsing).
```

### `go/internal/bridge/livesmoke.go:67` — above `rc, _ = d.Launch(ctx, cfg, deps)`

```text
// Dead-shell guard is armed by the real driver constructor (guardDeadShell),
// so smoke boots get the same cycle-274 rejection a phase launch gets.
```

### `go/internal/bridge/livesmoke_test.go:3` — above `import (`

```text
// RED contract for LiveSmokeTest (cycle-283): the boot smoke-test passes
// against a quota-walled CLI because the wall only appears AFTER work is
// submitted. LiveSmokeTest is the probe that can actually see it: a real
// launch that submits one trivial contracted prompt and reports whether the
// REPL produced the artifact (healthy), hit a classified wall (pattern name
// from the escalation report), or failed otherwise.
```

### `go/internal/bridge/livesmoke_test.go:45` — above `base := &FakeTmuxController{CaptureFrames: []string{"❯", "❯", "working ❯", "working ❯", "done ❯", "cleanup"}}`

```text
// Two leading "❯" frames: claude-tmux now ticks the auto-responder during
// boot (tickDuringBoot), so the first boot iteration reads the pane twice
// (boot loop + tick) before the marker check breaks.
// The extra "working ❯" is the settling tick: under the cycle-1233 cross-poll
// stability window (completion.go) the artifact completes one tick after it
// first appears, so the pane is captured once more before cleanup.
```

### `go/internal/bridge/livesmoke_test.go:65` — above `func TestLiveSmokeTest_QuotaWallClassified(t *testing.T) {`

```text
// TestLiveSmokeTest_QuotaWallClassified: the cycle-283 replay — the pane shows
// the provider wall after submission; the autoresponder classifies rate_limit,
// escalates (85), and LiveSmokeTest surfaces the pattern name.
```

### `go/internal/bridge/manifest.go:149` — above `ChatGPTSafeModels []string 'json:"chatgpt_safe_models,omitempty"'`

```text
// ChatGPTSafeModels lists the concrete model IDs a ChatGPT/subscription
// account can reliably use for this CLI. When the resolved auth mode is
// "chatgpt" and the realized -m model is NOT in this set, the driver clamps
// it to ChatGPTDefaultModel. Empty → no clamp (API-key-only CLIs, or no
// constraint). codex's model picker/docs advertise models that the live
// backend 400-rejects on ChatGPT accounts by plan tier (gpt-5.4/5.5 in 2026-06; multiple open OpenAI issues); this set is the proven-safe
// subset. See docs/incidents/cycle-142-* and the codex-chatgpt-model-support
// research dossier.
```

### `go/internal/bridge/manifest.go:170` — above `Params map[string]ParamSpec 'json:"params,omitempty"'`

```text
// Params is the declarative per-CLI realization table: how each high-level
// LaunchIntent parameter maps to this CLI's launch flags / REPL input /
// controller hints. Absent param → no-op. See ADR-0022 + realizer.go.
```

### `go/internal/bridge/manifest.go:290` — above `if len(m.ModelTierMap) == 0 {`

```text
// v1 → v2 schema compat (cycle-124 followup): a manifest declaring the
// legacy `tier_aliases` key — with the Anthropic-leaked vocabulary
// `{haiku|sonnet|opus → native}` — is read into a sidecar struct,
// translated to the canonical `fast|balanced|deep` keys, and merged
// into ModelTierMap. We only translate when ModelTierMap is empty
// (v1-only); a manifest declaring both keys keeps ModelTierMap as the
// source of truth without warnings. One deprecation line per manifest.
```

### `go/internal/bridge/manifest.go:301` — above `_ = json.Unmarshal(data, &v1)`

```text
// Second Unmarshal of the same bytes: an error here is impossible
// given the first Unmarshal into `m` already validated the JSON
// shape; a struct-tag mismatch just leaves v1.TierAliases at nil.
// Explicit discard documents the intent for future readers (per
// cycle-124 PR 2 review).
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:10` — above `func TestClaudeTmuxExhaustedRegex_PerModelWording(t *testing.T) {`

```text
// manifest_exhaustion_wording_test.go — regression lock for the claude-tmux
// usage.exhausted_regex against REAL captured quota-wall wording.
//
// Cycles 904–911 (2026-07-17) each burned ~20 min and shipped nothing: the
// auditor launched on `--model fable`, hit the Fable 5 per-model quota, and
// Claude Code rendered "You've reached your Fable 5 limit. Run /usage-credits
// to continue or switch models with /model." — then parked at its prompt.
// The manifest's exhausted_regex only matched the LEGACY wording
// ("reached your (usage|weekly) limit"), so the model-name-interpolated message
// slipped through, the pane never classified LivenessExhausted, the always-on
// exit-85 failover in driver_tmux_repl.go was bypassed, and the phase fell
// through to the 600s artifact-wait → exit 81 → self-heal relaunch → same
// exhausted quota → repeat (8 cycles).
//
// These fixtures are the ACTUAL strings captured in
// .evolve/runs/cycle-{904,905,908,909,910,911}/audit-escalation-report.json
// (final_pane). The prior exhaustion test used a SYNTHETIC "reached your usage
// limit" fixture that matched the old regex and passed — false confidence that
// let the upstream wording drift go uncaught. Verify against real data.
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:55` — above `"You've reached your Fable 5 li" + "mit. Run /usage-cre" + "dits to continue or switch models with /model.",`

```text
// Exact string captured in cycle-910/911 final_pane. The per-model wall
// ALWAYS carries its "/usage-credits" companion on the same line — the
// regex requires it (see the notWalls rationale), so every real-wall
// fixture must include it too:
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:69` — above `"You've hit your weekly li" + "mit · resets Jul 26 at 9pm (Asia/Taipei)",`

```text
// Exact string captured in cycle-1096 tmux-final-scrollback.txt
// (2026-07-23, batch-11 tail): the weekly wall drifted from "reached"
// to "hit", slipped past the regex, and cycles 1077–1096 each burned
// ~40 min at the 600s artifact-wait (exit 81, never 85) against a dead
// provider — the FOURTH wording-drift instance of this class. The
// "/usage-credits" companion renders on the NEXT pane line, so the
// weekly branch cannot require same-line adjacency:
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:86` — above `notWalls := []string{`

```text
// NOT a wall — must NOT match. Matching any of these would fast-fail a
// WORKING agent (the cycle-254/255/314/641 false-FAIL sin). The stop-review
// checkpoint scans the pane through strippedForExhaustionScan (unified-diff
// +/- lines removed, driver_tmux_repl.go), but NON-diff renderings — Read-tool
// cat-n output, plain prose, log lines — reach the regex unstripped, so a
// false match on those kills a healthy agent. Two-round
// review hardened the per-model branch: it requires BOTH Claude Code's
// second-person chrome ("you(?:.?ve| have) reached your … limit") AND the
// wall-specific "/usage-credits" companion adjacent on the same line — so
// neither third-person prose NOR ordinary second-person prose NOR narration
// that merely quotes the wall phrase (without the companion) can match.
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:108` — above `"Claude Code now emits You've reached your Fable 5 limit wording",`

```text
// narration that quotes the wall phrase but is NOT the wall (no companion) —
// e.g. an agent reading/discussing this very fix's artifacts (review round 2):
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:111` — above `"do you think you have reached your daily commit limit for the day",`

```text
// ordinary SECOND-person prose — the "you've/you have" anchor alone is not
// enough; requiring "/usage-credits" adjacency excludes it (review round 2):
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:135` — above `func TestClaudeSessionWallUsesGuardedExhaustion(t *testing.T) {`

```text
// Captured from live cycles 1607–1609; concatenation avoids triggering agents
// reading this test as ordinary tool output.
```

### `go/internal/bridge/manifest_exhaustion_wording_test.go:185` — above `func TestClaudeSessionWall_NBSPIndentIsRecognised(t *testing.T) {`

```text
// TestClaudeSessionWall_NBSPIndentIsRecognised pins the wording drift lanes
// 1676 and 1677 hit (2026-09-14): tmux renders Claude Code's "⎿" indentation
// with U+00A0, and the session-wall branch of exhausted_regex allowed only
// [ \t] between the glyph and the sentence — so the live wall never
// classified, the guarded fast-fail never armed, and both audits ran the full
// 40-minute artifact window with "POSSIBLE EXHAUSTION-REGEX DRIFT" in the
// bridge stderr. Concatenation keeps agents reading this test from matching.
```

### `go/internal/bridge/manifest_prompt_regex_compile_test.go:8` — above `func TestManifestInteractivePromptRegexesCompile(t *testing.T) {`

```text
// manifest_prompt_regex_compile_test.go — every interactive_prompts regex, in
// every manifest, must COMPILE.
//
// decideAutoRespond swallows a compile error and moves on (autorespond.go:
// `re, err := regexp.Compile(p.Regex); if err != nil { continue }`), so a rule
// whose pattern is invalid is not a loud failure — it is a rule that silently
// does nothing, and the symptom is the hang it was written to prevent.
//
// Found the hard way 2026-08-27: a plan-mode rule was authored with a `{0,1200}`
// bound. Go's RE2 caps repeat counts at 1000, so the pattern never compiled and
// the rule was dead on arrival — visible only because a new test happened to
// assert the rule fired. `controls.exhausted_regex` has had a compile guard
// since manifest_controls_test.go; interactive_prompts, the field whose silent
// death costs a lane, had none.
```

### `go/internal/bridge/manifest_v1_compat_test.go:9` — above `func TestManifestV1Compat_TranslatesTierAliasesKeys(t *testing.T) {`

```text
// manifest_v1_compat_test.go — pins the cycle-124-followup v1→v2 schema
// migration for the per-CLI parameter mapping table. v1 manifests use the
// Anthropic-leaked vocabulary `tier_aliases: {haiku|sonnet|opus → native}`;
// v2 manifests use the provider-neutral vocabulary `model_tier_map:
// {fast|balanced|deep → native}` matching what profiles already declare.
//
// The contract: a v1-shape manifest installed in EVOLVE_BRIDGE_MANIFEST_DIR
// (or hand-rolled by an operator) continues to work for one release after
// the migration. parseManifest detects the legacy key shape, translates
// haiku→fast / sonnet→balanced / opus→deep on read, populates the new
// ModelTierMap field, and emits ONE stderr deprecation line per manifest.
// After the deprecation window the v1 path is removed; this test fails
// loudly when the planned removal happens.
```

### `go/internal/bridge/model_freshness_test.go:5` — above `func TestModelFreshness_ClaudeDeclaresAlias(t *testing.T) {`

```text
// TestModelFreshness_ClaudeDeclaresAlias: claude-tmux declares the alias
// freshness fact — the claude CLI resolves a bare family alias ("opus") to
// that family's newest release at LAUNCH (verified live 2026-07-27:
// --model opus → canonicalModel claude-opus-5), so the alias is strictly
// fresher than any concrete id a catalog could cache. This is a fact about
// the claude binary, declared as manifest DATA beside model_tier_map — the
// same category as chatgpt_safe_models, and zero "claude" conditionals in Go.
```

### `go/internal/bridge/model_tier_parity_test.go:3` — above `import (`

```text
// model_tier_parity_test.go — cycle-447 Task 2 (model-tier-matrix-parity-pin):
// every embedded *-tmux CLI must translate the abstract intent.ModelTier
// through SOME effective channel — flag emits the flag+value, repl emits
// REPLInput, and ollama is classified effective-POSITIONAL (its model is the
// positional arg of `ollama run <model>`, never force-migrated to a flag).
// A multi-model CLI with channel:"noop" silently drops the tier — the exact
// defect agy-tmux carried until cycle-447 — so the parity rule REJECTS it
// (negative fixture below). The manifest glob is completeness-driven: a
// future *-tmux CLI added without an effective channel fails here, no
// hardcoded CLI list.
```

### `go/internal/bridge/pane_capture_error_reanchor_test.go:3` — above `import (`

```text
// pane_capture_error_reanchor_test.go — cycle-1580 audit-repair RED test for
// defect L1. The transient-dwell refactor hoisted the completion-wait's
// CapturePane out of the `if channelOn` block to make ONE canonical frame per
// tick, and in doing so dropped the old `if rendered, cerr := …; cerr == nil`
// guard (driver_tmux_repl.go:697-703). An errored capture now yields "" and is
// still fed to recordTokens and PaneDelta.Next — and Next("") re-anchors the
// delta (emitted=0, anchor=""), so the NEXT successful frame re-emits the whole
// stable pane to <agent>-pane.live. Pre-refactor a capture error skipped the
// delta entirely and the stream stayed monotone.
```

### `go/internal/bridge/paste_settle.go:3` — above `import (`

```text
// paste_settle.go — the ONE home of "paste, let the TUI ingest it, press
// Enter": the prompt paste (human_input.go) and the mid-turn inject
// (tmux_inject.go) both deliver through settlePasteThenEnter, so the timing
// belief the 2026-09-14 wedge incident corrected cannot survive in a second
// copy (docs/incidents/2026-09-14-codex-prompt-submit-wedge.md).
//
// A multi-KB bracketed paste is ingested by the TUI for seconds; an Enter that
// lands mid-ingestion is swallowed into the paste and the submission is later
// declared wedged. So: settle by size, wait for the pane to stop changing,
// then Enter — each bounded, each said out loud when it gives up, and the
// evidence returned so the submit-verify ledger records it on the SUCCESS path
// too (a recovered stall is a rate only with a denominator).
```

### `go/internal/bridge/pidfile_test.go:31` — above `rc, err := execRunner(context.Background(), "sh", "",`

```text
// execRunner necessarily writes the pidfile AFTER cmd.Start() (the child PID
// does not exist before Start), so a consumer must poll for it — exactly
// what the real reader (the auto-spawn observer's CPU-liveness probe) does.
// The child mirrors that: poll up to ~2s for the file to appear before
// reading it. Without the poll the child can `cat` before the parent's write
// lands, yielding an empty OUT — the cycle-274-class 0.00s CI flake.
```

### `go/internal/bridge/preflight_test.go:13` — above `func TestCLIPreflight_CodexTmuxImplementsIt(t *testing.T) {`

```text
// preflight_test.go — cycle-124 G3 contract: the optional CLIPreflight
// interface lets a Driver hook pre-launch prep work (today only codex-tmux,
// to pre-trust the worktree+workspace in ~/.codex/config.toml before the
// REPL boots — cycle-122 Fix 1 promoted out of an inline call). The tests
// pin three properties:
//
//  1. codex-tmux IS a CLIPreflight (the driver type assertion the Engine
//     does at launch.go would otherwise fall through silently and the
//     pretrust never run).
//  2. Drivers WITHOUT preflight work (claude-tmux, agy-tmux, ollama-tmux,
//     claude-p, codex headless, agy headless) MUST NOT accidentally
//     implement the interface — the absence is the OPT-OUT mechanism,
//     and a no-op stub on every concrete driver would be the wrong
//     pattern (the comment in driver.go documents this).
//  3. codex-tmux's Preflight returns an error from pretrustCodexProjects
//     unchanged (best-effort: error gets logged by Engine.Launch but does
//     not abort the phase; the contract is "do something useful or return
//     a logged-but-non-fatal error").
```

### `go/internal/bridge/preflight_test.go:280` — above `func TestCLIPreflight_OptOutDoesNotPanicOnTypeAssertion(t *testing.T) {`

```text
// TestCLIPreflight_OptOutDoesNotPanicOnTypeAssertion pins the Engine's
// `if pf, ok := driver.(CLIPreflight); ok` short-circuit: a driver that
// doesn't implement CLIPreflight must evaluate `ok=false`. The comma-ok
// form of type assertion is documented as never-panicking by the Go spec,
// so no recover() is needed (cycle-124 test-review LOW). The second
// assertion uses an anonymous interface to triple-check the underlying
// type genuinely lacks Preflight — catches accidental embedded promotion
// a refactor might introduce.
```

### `go/internal/bridge/profile.go:27` — above `ExtraFlagsByCLI map[string][]string`

```text
// ExtraFlagsByCLI is the per-CLI raw-flag escape hatch (ADR-0022). Flags
// are keyed by the CLI they belong to ("claude-tmux": [...]) and realized
// ONLY for the matching CLI, so a claude-origin profile switched to
// agy/codex realizes none of claude's argv. Replaces the flat extra_flags
// that forwarded one CLI's vocabulary verbatim to every CLI.
```

### `go/internal/bridge/prompt_echo_escalation_c654_test.go:3` — above `import "testing"`

```text
// RED regression test for cycle-654 top_n task `infra-classifier-echo-veto`
// at the ESCALATION / agy-tmux auto-responder layer (lesson cycle-641
// preventiveAction #4, cycle-642 defect D2): the exhaustion/escalation matcher
// MUST NOT fire on captured pane text that is a verbatim echo of the injected
// prompt/instruction body. decideAutoRespond's signature is pinned (autorespond.go),
// so the fix pre-strips echoed prompt lines from the pane in the caller — a
// stripPromptEchoLines helper mirroring the existing stripAgentDiffLines. This
// test drives that helper and asserts on matchExhausted over its output.
//
// RED today: stripPromptEchoLines does not exist (compile failure). GREEN once
// Builder adds it (drop pane lines that appear verbatim in the injected prompt)
// and wires it into the tick pane-cleaning ahead of the exhaustion / escalate scan.
//
// Ported (renamed C653→C654) from the fix-of-record RED suite preserved in
// .evolve/worktrees/cycle-21f9f7ae-653; do not author duplicate C653 copies.
```

### `go/internal/bridge/prompt_echo_wiring_c672_test.go:3` — above `import (`

```text
// RED wiring tests for cycle-672 top_n task `echo-veto-wiring-completion`,
// AC2: stripPromptEchoLines (landed cycle-654, TestC654_004 green) has ZERO
// production call sites — autoResponder.tick() still hands the RAW pane to the
// exhaustion scan (autorespond.go, ExhaustedOf) and to decideAutoRespond, so
// an agent echoing its own Deliverable-Contract exhaustion instructions can
// escalate rc 85 / classify rate_limit (cycle-656 retro D3 fired live).
//
// The fix (scout map): add an injectedPrompt field to autoResponder, populate
// it from the already-resolved prompt at BOTH construction sites
// (driver_tmux_repl.go, recipe_adapter.go), and strip the pane via
// stripPromptEchoLines in tick() ahead of the exhaustion check.
//
// RED today: autoResponder has no injectedPrompt field — compile failure.
// DO NOT MODIFY THESE TESTS (echo-veto intent). C672_004 is the negative guard
// (genuine banner must STILL escalate) and must be GREEN after the wiring lands.
// C672_005 is the discriminating anti-gaming check for the construction-site
// half — the behavioral tests alone could be satisfied by a field nothing
// populates in production (exactly the class of gap that let cycles 654/656 slip).
//
// UPDATE (exhaustion-gate, 2026-07): C672_004's tick COUNT was revised from 1 to
// exhaustionPersistObservations because the exhaustion fast-fail is now
// persistence-gated (exhaustion_persistence.go) — a genuine wall still escalates (intent
// preserved: it survives prompt-echo stripping), it just does so on the
// threshold-th consecutive tick, not the first. The echo-veto behavior C672_003
// and C672_005 pin is unchanged.
```

### `go/internal/bridge/realizer.go:10` — above `type ParamSpec struct {`

```text
// realizer.go — the Go engine half of the hybrid Realizer (ADR-0022). The
// per-CLI mapping data lives declaratively in each manifest's `params` table;
// this engine interprets it. Flags-first: an intent realizes to a launch flag
// when the CLI declares one, to REPL injection when declared `repl`, to a
// controller hint for session lifecycle, or to nothing (no entry / `noop`).
```

### `go/internal/bridge/realizer.go:30` — above `Default string 'json:"default,omitempty"'`

```text
// Default is the value realized when the intent leaves this param empty —
// data-driven, per CLI (2026-08-15: codex's two-layer model config runs at
// the CLI's own reasoning-effort default unless the second layer is set;
// the manifest now pins effort=high by default, operator directive).
```

### `go/internal/bridge/realizer.go:37` — above `var unresolvedModelTokens = append([]string{"auto", "high"}, modelcatalog.CanonicalTiers...)`

```text
// unresolvedModelTokens is the closed vocabulary that never names a model on
// ANY CLI: the "auto" resolve-me sentinel (ADR-0044 C2/D3, cycle-262), every
// canonical tier, and "high" (the input alias of "deep", translateV1TierKey).
// A value still in this set after Manifest.ModelTierMap translation means the
// manifest declared no entry for the tier and the fallback ladder left the tier
// NAME in place — translateV1TierKey passes unknown keys through verbatim,
// which is what lets one reach an emit point at all.
//
// Derived from modelcatalog.CanonicalTiers so the tier vocabulary has exactly
// one source: a tier added there is covered here, in every driver, and in the
// tests that sweep this var — with no parallel list to keep in sync.
//
// Note what is deliberately ABSENT: haiku/sonnet/opus. translateV1TierKey maps
// them as legacy tier aliases, but they are also real claude model ids, so
// suppressing them would disable model routing outright.
```

### `go/internal/bridge/realizer.go:104` — above `if len(m.DefaultArgs) > 0 {`

```text
// Manifest-level default_args land FIRST so per-param flags + raw
// escape-hatch flags append after them. This is the "always-on" hook
// each CLI uses for unconditional launch flags (e.g. codex-tmux's
// --yolo to short-circuit the per-edit-approval modal that stalled
// cycle-123 tdd — see docs/incidents/cycle-123-codex-edit-approval-
// modal-and-empty-fallback-chain.md G1a). The field has existed on
// Manifest since manifest.go:63 but was previously unread; wired in
// cycle-124 Fix G1a.
```

### `go/internal/bridge/realizer.go:150` — above `r.LaunchFlags = dedupeLaunchFlags(combinedFlags)`

```text
// Dedupe LaunchFlags (cycle-124 G1a wire-up consequence): a manifest's
// default_args may declare a flag that one of its params ALSO emits when
// a particular intent value is set (e.g. agy-tmux declares
// --dangerously-skip-permissions in default_args AND in
// params.permission.values.bypass; both fire under
// intent.Permission="bypass"). Dedupe is order-preserving (keep first
// occurrence) so the operator-declared default still takes the leading
// position. Idempotent for the already-unique case.
```

### `go/internal/bridge/realizer.go:163` — above `func dedupeLaunchFlags(in []string) []string {`

```text
// dedupeLaunchFlags returns a copy of in with subsequent duplicate UNITS
// removed, preserving order. A unit is a flag-value PAIR when a `-`-prefixed
// token is followed by a non-flag token (`-c key=val`, `-m model`), otherwise
// the single token (`--yolo`). So a flag repeated with distinct values is kept
// in full, while an identical pair or a repeated boolean collapses.
//
// LIMITATION, deliberate and pinned: pairing keys on "the next token does not
// start with `-`", NOT on flag arity, which nothing here can know. So a value
// that itself begins with `-` is not recognised as a value, and that pair
// degrades to the old token-wise behaviour — `--min -1 --max -1` still yields
// `--min -1 --max`, dropping the second `-1` and leaving `--max` dangling. No
// manifest or tracked profile emits a `-`-leading value today; the case is
// pinned in realizer_dedupe_pairs_test.go so a future one is caught here rather
// than in a lane. An empty next token is likewise not treated as a value (see
// the guard below).
//
// It used to dedupe individual tokens, which did neither thing its own comment
// claimed. `-m gpt-5.4 -m gpt-5.5` became `-m gpt-5.4 gpt-5.5` — the repeated
// FLAG dropped, both VALUES kept, i.e. a different command line rather than a
// deduplicated one, with the second value silently demoted to a positional.
// Latent until codex's effort param needed two `-c` overrides
// (model_reasoning_effort + plan_mode_reasoning_effort) and the realized argv
// came out as `-c model_reasoning_effort=high plan_mode_reasoning_effort=high`.
// Caught by an exact-argv pin, which is the argument for pinning argv exactly.
//
// Its original purpose (cycle-124 G1a) was a manifest's default_args declaring
// a flag that a param also emits — e.g. agy-tmux declaring
// --dangerously-skip-permissions in both. That collision no longer exists on
// any manifest (claude-tmux and agy-tmux both ship default_args: []), so the
// live jobs today are the internal-duplicate typo guard and, above all, NOT
// corrupting codex's two `-c` overrides. Order-preserving, keep-first, so an
// operator-declared default retains the leading position. Idempotent.
//
// One asymmetry versus the old behaviour, recorded rather than fixed: a bare
// flag no longer dedupes against the same flag used as a pair, so
// ["-c","a=1","-c"] keeps the dangling "-c" where token-wise dedupe dropped it.
// Only reachable through an operator typo in extra_flags_by_cli, and a dangling
// -c is a loud codex parse error rather than a silent wrong command line.
```

### `go/internal/bridge/realizer.go:206` — above `out := make([]string, 0, len(in))`

```text
// Fresh backing array (cycle-124 review HIGH): `out := in[:0]` would
// alias `in`'s storage. The function's contract is "returns a copy"
// and the call site relies on that — keeping `in` intact lets the
// caller hold a pre-dedupe reference for diagnostics without
// witnessing in-place writes through it.
```

### `go/internal/bridge/realizer.go:241` — above `func legacyTierAlias(value string) string {`

```text
// legacyTierAlias translates the deprecated Anthropic-named tier vocabulary
// (haiku/sonnet/opus) into the canonical abstract vocabulary (fast/balanced/
// deep) used by ModelTierMap keys after the cycle-124 schema migration.
// Pass-through for already-canonical names AND for raw model identifiers
// (which fall through to the realizer's identity-fallback at the call site).
// Delegates to manifest.translateV1TierKey so the 3-entry mapping has a
// single source of truth (avoids silent drift if a fourth legacy alias is
// ever added — see ADR-0022 PR 2 addendum).
```

### `go/internal/bridge/realizer.go:276` — above `if spec.From == "model_tier_map" || spec.From == "tier_alias" {`

```text
// ParamSpec.From identifies the manifest sidecar table to translate
// through. "model_tier_map" is canonical (cycle-124 followup); the
// legacy spelling "tier_alias" is accepted unchanged for one release
// so operator-installed v1 override manifests keep working.
```

### `go/internal/bridge/realizer.go:283` — above `if param == "model_tier" && isUnresolvedModelToken(resolved) {`

```text
// ModelFlagPolicy (ADR-0044 C2 / D3, generalized): a vocabulary token here
// means model_tier_map translation fell through, so the value names no
// model on any CLI and `<cli> --model <token>` is the cycle-262 fatal boot.
// Omit the param — the CLI's own default always beats a fatal boot. This is
// the emit point for every flag/repl CLI; the headless drivers guard their
// own argv against the same vocabulary (claudePArgs, driver_codex.go).
```

### `go/internal/bridge/realizer.go:311` — above `func resolveTierModel(m Manifest, value string) string {`

```text
// resolveTierModel is the ONE tier→model ladder for every transport: the
// realizer's flag/repl emit (realizeScalar) and the headless codex driver's
// own -m composition both resolve through it, so a change here (e.g. the
// queued within-tier fallback axis) reaches every dispatch. Fallback ladder
// for the cycle-124 deprecation window: try the raw intent value first
// (synthetic test fixtures + operator v1 manifests where keys are still
// haiku/sonnet/opus); if that misses, try the canonical translation
// (parseManifest's v1-shimmed manifests where keys are now fast/balanced/deep).
// Native ids and genuinely unknown values pass through unchanged — the
// vocabulary guard at each emit point then decides whether to omit the flag.
```

### `go/internal/bridge/realizer_dedupe_pairs_test.go:8` — above `func TestDedupeLaunchFlags_KeepsDistinctValuesForRepeatedFlag(t *testing.T) {`

```text
// realizer_dedupe_pairs_test.go — dedupeLaunchFlags must dedupe flag-VALUE
// PAIRS as units, not individual tokens.
//
// The function's doc comment used to state a contract it did not implement:
// "a flag with distinct values (e.g. -m gpt-5.4 vs -m gpt-5.5) is correctly
// kept twice because the token values differ" and "flag-value pairs that
// legitimately repeat should NOT be deduped this way". Token-wise dedupe does
// neither: the repeated FLAG token is dropped while both value tokens survive,
// silently reassembling `-m a -m b` into `-m a b` — a different command line,
// not a deduplicated one.
//
// Latent until 2026-08-27, when codex's effort param needed two `-c` overrides
// (model_reasoning_effort and plan_mode_reasoning_effort). The realized argv
// came out as `-c model_reasoning_effort=high plan_mode_reasoning_effort=high`,
// passing the second key as a bare positional. Caught by an existing argv pin
// rather than in production, which is the argument for pinning argv exactly.
```

### `go/internal/bridge/realizer_modelpolicy_test.go:3` — above `import "testing"`

```text
// realizer_modelpolicy_test.go — ADR-0044 C2/D3 (Slice 2) RED tests: the
// ModelFlagPolicy "omit-on-auto" guard at the realizer chokepoint.
//
// cycle-262 (2026-06-09): retro was dispatched with model "auto" — the loop's
// resolve-me sentinel, never a valid concrete model for ANY CLI. The
// claude-tmux manifest realizes model_tier as `--model <value>` with
// ModelTierMap pass-through for unmapped values, so the sentinel sailed
// straight into `claude --model auto`, which boots into the fatal
// "There's an issue with the selected model (auto)" pane (verified in the
// cycle-262 tmux-final-scrollback). The headless codex driver already guards
// this (omit -m on auto, pinned in coverage_batch7_test.go); the realizer is
// the single emit point for every flag/repl-channel CLI, so ONE guard here
// covers claude-tmux, codex-tmux, and any future manifest (matrix-wide fix,
// not a per-driver patch).
//
// Contract: when post-resolution model_tier is still the "auto" sentinel, the
// realizer emits NO model param at all — the CLI's own default model is
// always preferable to a fatal boot. Concrete tiers and raw model names are
// unaffected.
```

### `go/internal/bridge/realizer_realmanifest_test.go:8` — above `func TestRealizeFor_RealManifests_NoCrossCLILeak(t *testing.T) {`

```text
// realizer_realmanifest_test.go — RealizeFor against the REAL embedded
// manifests (not constructed fixtures). This is the contract the cycle-1 boot
// failure violated: the SAME intent must realize to each CLI's own launch
// flags and never leak one CLI's vocabulary into another. Flags-first: model
// is a launch flag for claude (--model), codex (-m), and — since the
// cycle-447 live probe of agy 1.0.15 — agy (--model, display-name tokens).
```

### `go/internal/bridge/realizer_realmanifest_test.go:36` — above `want := []string{"--model", "Gemini 3.7 Flash (High)", "--dangerously-skip-permissions"}`

```text
// agy 1.0.15 selects its model via --model (cycle-447 live probe;
// 1.0.3 had no model flag — incident cycle-154, `-m` is still
// undefined). Tier "sonnet" resolves via the legacy ladder to
// balanced → the manifest's offline display-name default. The scalar
// order (model before permission) is part of the pin; settings_scope
// stays a no-op for agy.
```

### `go/internal/bridge/realizer_realmanifest_test.go:50` — above `if !reflect.DeepEqual(r.LaunchFlags, []string{"--yolo", "-m", "gpt-5.6-terra", "-c", "model_reasoning_effort=high", "-c"…`

```text
// codex resolves the tier via its manifest tier map (sonnet→balanced→gpt-5.6-terra)
// and emits it as the -m launch flag (flags-first); no permission flag.
// Cycle-124 G1a: --yolo from manifest.default_args lands FIRST (defuses
// the per-edit-approval modal that hung cycle-123 tdd by setting
// approval=never + sandbox=danger-full-access at boot — undocumented in
// codex --help 0.134 but parsed by clap; verified empirically). The
// order is load-bearing: default_args before per-param scalars.
// The second -c is plan_mode_reasoning_effort, added 2026-08-27: codex's
// plan mode does NOT fall back to model_reasoning_effort, so without it
// entering plan mode silently drops to codex's built-in preset
// (observed live: gpt-5.6-sol xhigh -> medium). This exact-argv pin is
// what caught the realizer dropping the repeated -c flag, so keep it
// exact rather than relaxing it to a Contains check.
```

### `go/internal/bridge/realizer_test.go:8` — above `func claudeTmuxManifest() Manifest {`

```text
// realizer_test.go — the heart of ADR-0022: ONE CLI-agnostic LaunchIntent must
// realize correctly AND differently per CLI, via each manifest's declarative
// `params` table. The same intent that yields `--dangerously-skip-permissions
// --model sonnet --setting-sources project` for claude must yield
// `--dangerously-skip-permissions --model "Gemini 3.5 Flash (High)"` for agy
// (display-name model tokens, no settings flag; cycle-447) and `-m gpt-5.4`
// for codex — and NEVER a flag the target CLI does not define. An intent with
// no manifest entry is a no-op (the property that makes foreign params unable
// to break a launch).
```

### `go/internal/bridge/realizer_test.go:48` — above `ModelTierMap: map[string]string{"fast": "Gemini 3.5 Flash (Low)", "balanced": "Gemini 3.5 Flash (High)", "deep": "Gemini…`

```text
// Mirrors the real manifest post-cycle-447: agy 1.0.15 grew a --model
// launch flag whose selectable tokens are display names (spaces/parens).
```

### `go/internal/bridge/realizer_test.go:178` — above `func TestRealize_DefaultArgs_LandFirst(t *testing.T) {`

```text
// TestRealize_DefaultArgs_LandFirst pins the cycle-124 G1a wire-up: manifest
// `default_args` is the always-on launch-flag channel (was a dead field
// before cycle-124 — declared in manifest.go but never read). Tokens land in
// LaunchFlags BEFORE per-param scalars, so a manifest can prepend
// unconditional boot-time switches (e.g. codex-tmux --yolo, ollama-tmux
// --experimental-yolo) without competing with intent-driven flags. An empty
// or nil default_args remains a no-op (regression guard for the agy/claude
// migration that emptied their default_args and let params.permission be
// the sole emitter).
```

### `go/internal/bridge/realizer_test.go:201` — above `mEmpty := Manifest{`

```text
// Empty default_args produces unchanged behavior — regression guard for the
// cycle-124 agy/claude migration that emptied default_args and let
// params.permission be the sole emitter of --dangerously-skip-permissions.
```

### `go/internal/bridge/realizer_test.go:225` — above `func TestRealize_DefaultArgs_Deduped(t *testing.T) {`

```text
// TestRealize_DefaultArgs_Deduped covers the cycle-124 G1a wire-up's
// order-preserving dedupe: when a manifest declares the same token in
// default_args AND one of its params channels emits the same token, the
// duplicate is silently dropped (the operator-declared default keeps the
// leading position). This is the documented invariant for boolean-style flags;
// flag/value PAIRS with different VALUES are preserved because dedupe keys on
// the PAIR.
//
// This comment previously said the opposite — that dedupe was "token-level, so
// `--model gpt-5.4` and `--model gpt-5.5` would both survive — neither matches
// the other as tokens". That was the exact belief the assertion below encoded,
// and it was false in a way the sentence hid: token-wise dedupe dropped the
// second `--model` and kept BOTH values, yielding `--model gpt-5.4 gpt-5.5` and
// demoting a model name to a positional argument (i.e. into the prompt, since
// the tmux launch line carries no other positional). Corrected 2026-08-27 with
// the assertion, so the prose and the pin can no longer disagree.
```

### `go/internal/bridge/realizer_test.go:266` — above `if !reflect.DeepEqual(got2.LaunchFlags, []string{"--model", "gpt-5.4", "--model", "gpt-5.5"}) {`

```text
// FLIPPED 2026-08-27 (header explains why): both pairs survive, and
// last-one-wins lets the param-resolved tier take effect — which is what a
// caller declaring the param wanted. Declaring the same flag in both
// default_args and params remains poor hygiene, but is no longer harmful.
```

### `go/internal/bridge/realizer_test.go:275` — above `func TestDedupeLaunchFlags_Edges(t *testing.T) {`

```text
// TestDedupeLaunchFlags_Edges directly exercises the helper that the cycle-124
// HIGH review-finding had us re-write with a fresh backing slice. The Realize
// integration tests above exercise the helper indirectly; this table drives
// the pure function across every plausible input shape so future refactors
// surface here first. Order-preservation (first occurrence wins) is the
// load-bearing contract — flags-first reflects the operator-declared default,
// per-param scalars deduplicate against it, and raw extras come last.
```

### `go/internal/bridge/realizer_test.go:300` — above `"flag-value pair with DISTINCT values → both pairs kept intact",`

```text
// FLIPPED 2026-08-27: this pinned a documented FOOTGUN — token-wise
// dedupe dropped the repeated flag and kept both values, silently
// rewriting `--model x --model y` into `--model x y` and demoting
// the second value to a positional. dedupeLaunchFlags now dedupes
// flag-value PAIRS as units, which is what its doc comment always
// claimed. Exposed by codex's effort param needing two `-c`
// overrides; see codex_plan_effort_test.go.
```

### `go/internal/bridge/realizer_test.go:347` — above `func TestDedupeLaunchFlags_AliasSafety(t *testing.T) {`

```text
// TestDedupeLaunchFlags_AliasSafety pins the cycle-124 review HIGH fix:
// dedupeLaunchFlags MUST return a slice that does not share backing storage
// with its input. Pre-fix `out := in[:0]` would alias, so a caller holding
// the original could see the deduped result through their reference. The
// fix `out := make([]string, 0, len(in))` allocates fresh. This test
// captures the invariant so a future "optimization" can't silently
// regress it.
```

### `go/internal/bridge/realizer_tier_sentinel_test.go:3` — above `import "testing"`

```text
// realizer_tier_sentinel_test.go — the generalized unresolved-model-token
// invariant. ADR-0044 C2/D3 (cycle-262) established that the "auto" resolve-me
// sentinel must never reach a CLI: `claude --model auto` boots into the fatal
// "There's an issue with the selected model (auto)" pane. An ABSTRACT TIER NAME
// reaching the same emit point is the identical failure for the identical
// reason — it means model_tier_map translation fell through, so the value is a
// vocabulary token, not a model.
//
// This is live today: claude-tmux.json declares no "top" entry, and
// translateV1TierKey("top") is a pass-through, so realizeScalar leaves
// resolved=="top" and the flag channel emits `claude --model top`. The tier is
// reachable — universalTierFloor (router/model_routing_clamp.go:27) has
// Max:"top" and 60+ profiles carry no envelope, so the advisor may propose it
// for any of them.
//
// The rule is glob-driven over ManifestNames() so a future *-tmux CLI added
// without full tier coverage fails here — no hardcoded CLI list.
```

### `go/internal/bridge/realizer_wiring_test.go:16` — above `func writeIntentProfile(t *testing.T, dir, name, cli string, extraByCLI map[string][]string) string {`

```text
// realizer_wiring_test.go — ADR-0022 Phase 2b/3 acceptance. Proves the tmux
// drivers build their launch command from the per-CLI Realization (the single
// owner of model+permission+raw flags), so a claude-origin profile's flags
// never leak into agy/codex. This is the contract the cycle-1 multi-CLI boot
// failure violated: profile.extra_flags were claude argv forwarded verbatim to
// every CLI.
//
// The migrated profile shape: extra_flags_by_cli keyed per CLI (claude flags
// live under "claude-tmux"), and NO permission_mode (the bypass posture is the
// realized default). A profile switched to agy/codex therefore realizes to
// that CLI's own flags only — RawByCLI[agy/codex] is nil.
```

### `go/internal/bridge/realizer_wiring_test.go:93` — above `want:   "agy --model 'Gemini 3.7 Flash (High)' --dangerously-skip-permissions",`

```text
// agy 1.0.15 selects its model via --model (cycle-447 live probe);
// the display-name token is shell-quoted by launchCmdLine. The
// undefined -m short flag stays in `absent` (space-delimited so the
// substring can't match inside --model) — the cycle-154 regression
// lock. Model "sonnet" → legacy ladder → balanced → offline default.
```

### `go/internal/bridge/realizer_wiring_test.go:105` — above `want:   "codex --yolo -m gpt-5.6-terra -c 'model_reasoning_effort=high' -c 'plan_mode_reasoning_effort=high'",`

```text
// Cycle-124 G1a: --yolo from codex-tmux.json:default_args lands
// FIRST per the realizer's wire-up order (default_args before
// per-param scalars), then -m from the params.model_tier tier_alias
// (sonnet → gpt-5.4). Behavior contract: --yolo at boot sets
// approval=never AND sandbox=danger-full-access, short-circuiting
// the per-edit-approval modal that hung cycle-123 tdd. (Codex's
// --help 0.134 omits --yolo from its option list, but clap parses
// it; verified empirically.)
// Cycle-142: this launch runs with no OPENAI_API_KEY → codexAuthMode
// == "chatgpt"; the clamp seam stays armed but no longer fires —
// the whole gpt-5.6 family (2026-08-14 operator-confirmed refresh)
// is in chatgpt_safe_models, so the realized balanced tier
// (gpt-5.6-terra) passes through. The leak-absent assertions below
// are what this case actually guards; the model value rides along.
// The second -c is the plan-mode effort override (2026-08-27). This
// is the END-TO-END launch string reaching tmux, so it is also the
// wiring proof that the flag survives realization, dedupe and
// quoting — the three places it could have been dropped.
```

### `go/internal/bridge/recipe_adapter.go:117` — above `if d.ar != nil {`

```text
// Echo-veto (cycle-672): the recipe path has no single resolved prompt
// file — every body we inject is prompt text the pane may echo back, so
// accumulate it for tick()'s stripPromptEchoLines exhaustion guard.
```

### `go/internal/bridge/recipe_adapter.go:148` — above `if strings.HasPrefix(cli, "codex") {`

```text
// CB.3: the recipe path bypasses the engine's CLIPreflight chokepoint, so
// codex-family sessions must pretrust their worktree/workspace here or the
// first boot in a fresh worktree renders the cycle-122 trust modal. Same
// best-effort semantics as the driver preflight (the boot auto-responder
// remains the downstream defense).
```

### `go/internal/bridge/recipe_adapter.go:219` — above `if ctx.Err() != nil {`

```text
// the caller's interrupt wins over the settle ticks (F20: the usage probe sat here)
```

### `go/internal/bridge/render_wedge_test.go:11` — above `type jiggleTmux struct {`

```text
// render_wedge_test.go — claude ≥2.1.173 BLANK-PANE render wedge (inbox
// claude-2.1.173-blank-pane-after-interval): the Ink renderer can blank a
// detached pane mid-turn while the agent keeps working (cycle-291: healthy
// frames to 12:10:36, capture-rc-0-but-empty from 12:11:06, interval-2
// still saw stdout growth). A blank capture with a LIVE session is a
// render wedge, not idleness — the legacy reviewer read it as a stall and
// paused, burning interval×attempts to exit=81 on a working agent.
//
// Contract:
//  1. blank pane + live session ⇒ the driver jiggles the window width
//     (SIGWINCH → Ink full re-render) and re-captures;
//  2. still blank ⇒ the stop event reads Busy (extend; never pause a live
//     agent on a pane that stopped rendering), bounded by maxExtends;
//  3. the jiggle recovering content ⇒ the recovered frame feeds the normal
//     progressed/busy evaluation.
```

### `go/internal/bridge/render_wedge_test.go:51` — above `func TestRunTmuxREPL_BlankPaneWedge_JigglesAndExtends(t *testing.T) {`

```text
// TestRunTmuxREPL_BlankPaneWedge_JigglesAndExtends — the cycle-291 shape:
// pane boots, renders once, then goes permanently blank while the session
// stays alive. The driver must jiggle and EXTEND (busy) every interval —
// never the legacy "stalled; pause for investigation" — until the
// maxExtends backstop exhausts.
```

### `go/internal/bridge/request.go:3` — above `import (`

```text
// request.go — the in-process launch request's required-field gauntlet
// (ADR-0103 unit 10, review fold: a pre-launch rule lives in the host beside
// the Launch that runs it, not in the launch-outcome classifier). The
// production Adapter (adapters/bridge) projects the same function, so the
// engine and the adapter reject one request with one string.
```

### `go/internal/bridge/request_test.go:3` — above `import (`

```text
// request_test.go — ADR-0103 unit 10, review fold (architecture MEDIUM 2):
// the required-field gauntlet lives in the HOST — a pre-launch rule beside
// the Launch that runs it, not in the outcome classifier — and the Adapter
// projects it through gobridge.ValidateRequest. Test 32 moved verbatim from
// launchoutcome/request_test.go.
```

### `go/internal/bridge/sandbox_wrap.go:3` — above `import (`

```text
// sandbox_wrap.go — default SandboxWrap implementation (Workstream B).
//
// CLI-agnostic confinement: every driver (claude/codex/agy/ollama) running a
// source-writing phase gets wrapped in the host's OS sandbox
// (sandbox-exec on macOS, bwrap on Linux). The non-Claude drivers historically
// bypassed the trust kernel entirely (Issue 2 from cycle 119).
//
// This file owns ONLY the decision + prefix-argv synthesis. Drivers (tmux +
// headless) call deps.SandboxWrap at their launch site and either prepend the
// returned argv or enforce the profile's confinement requirement when no
// wrapper is available. Mandatory profiles fail closed without an explicit opt-out.
//
// Probe is cached behind sync.Once because it shells to LookPath; the cached
// result is captured in the closure that withDefaults returns.
```

### `go/internal/bridge/sandbox_wrap.go:48` — above `switch mode {`

```text
// Normalize any UNRECOGNIZED value to auto, mirroring config.applyEnv's
// validation contract. Pre-fix, an unknown value (operator typo like
// "1") was neither "off" nor "auto", so it slipped past the
// nested-claude skip below and forced a sandbox-exec wrap — which hangs
// claude's REPL boot on nested macOS (exit=80 ExitREPLBootTimeout; the
// 2026-06-13 soak burned cycles 324-326 on exactly this). Treat an
// unknown value as auto (the safe default) and WARN so it's observable.
```

### `go/internal/bridge/sandbox_wrap.go:159` — above `sbplDir := req.Workspace`

```text
// ADR-0049 S0 / gap G6: write the SBPL to a PER-INVOCATION profile
// dir, not a shared <workspace>/sandbox-<phase>.sb. Two same-phase
// dispatches sharing a workspace (a re-dispatch, two fan-out workers,
// or two runs reusing a cycle number) otherwise write the same file —
// and if their WritePaths differ, B's profile landing between A's
// write and A's sandbox-exec read confines A to B's allow-list (A's
// legit source writes EPERM-denied). A mktemp -d (0o700) per
// invocation isolates them — the per-invocation sandbox-profile
// pattern (CERT FIO21-C; Codex generates a profile per launch). A
// mkdir failure degrades to the shared workspace profile (confinement
// preserved, isolation lost) rather than running unconfined. No-op for
// the live sequential loop: a lone dispatch just gets its own subdir.
```

### `go/internal/bridge/sandbox_wrap.go:378` — above `ok, optOut, reason := sandbox.ConfinementSatisfied(`

```text
// The three-cell decision projects from its single home (Specification —
// sandbox.ConfinementSatisfied); preflight's host-capabilities check
// displays the same predicate, so the two can no longer diverge (the
// 2026-09-01 nested-HALT divergence class).
```

### `go/internal/bridge/sandbox_wrap_test.go:70` — above `fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "/tmp/sb.sb"}, available: true}`

```text
// Source-writing phases pass their absolute Worktree+Workspace+RepoRoot
// to the wrapper — the SBPL profile depends on every path being absolute
// (matches Workstream A's invariant; relative paths broke cycle-119).
```

### `go/internal/bridge/sandbox_wrap_test.go:222` — above `deps := Deps{`

```text
// depEnvGetter consults the Env map FIRST, then falls back to deps.LookupEnv.
// The sibling nested tests cover the Env-map branch; this pins the LookupEnv
// fallback branch (a nested signal present only via LookupEnv must still be
// detected, so the wrap is skipped). Audit follow-up (cycle-990613 LOW).
```

### `go/internal/bridge/sandbox_wrap_test.go:253` — above `ws := t.TempDir()`

```text
// Darwin path: the SBPL profile is materialized into the workspace
// (per-phase file) AND the prefix uses `-f` (file path), NOT `-p` (which
// passes the inline SBPL string). This pins the cycle-119 fix: a `-p
// <path>` would silently leave the phase unconfined because sandbox-exec
// would parse the path AS the profile.
```

### `go/internal/bridge/sandbox_wrap_test.go:342` — above `deps := Deps{Env: map[string]string{`

```text
// Regression (2026-06-13 soak, cycles 324-326): an UNRECOGNIZED
// EVOLVE_SANDBOX value (operator typo "1" instead of auto|on|off) was
// neither "off" nor "auto", so it slipped past the nested-claude skip
// (which only fired for the literal "auto") and forced a sandbox-exec
// wrap on nested macOS. That hung claude's REPL boot >60s
// (exit=80 ExitREPLBootTimeout), failing every cycle at scout. An
// unrecognized value MUST normalize to auto so the nested-skip applies.
//
// Workspace must be writable: with the bug, the darwin branch writes an
// SBPL file there and returns the wrap prefix — t.TempDir() ensures that
// write SUCCEEDS, so a missing fix is a true RED (wrap returned), not a
// false pass via os.WriteFile failure.
```

### `go/internal/bridge/sandbox_wrap_test.go:432` — above `func TestDefaultSandboxWrap_Darwin_PerInvocationProfileDir(t *testing.T) {`

```text
// TestDefaultSandboxWrap_Darwin_PerInvocationProfileDir pins ADR-0049 S0 / gap
// G6: two same-phase dispatches sharing ONE workspace must NOT write the same
// sandbox-<phase>.sb. Pre-fix both wrote <workspace>/sandbox-build.sb, so if
// their WritePaths differed, B's profile landing between A's write and A's
// sandbox-exec read confined A to B's allow-list (A's legit source writes
// EPERM-denied). A per-invocation profile dir isolates them. This is a true RED
// before the fix (identical paths) and GREEN after (distinct mktemp -d dirs).
```

### `go/internal/bridge/sandbox_write_subpaths_test.go:3` — above `import (`

```text
// sandbox_write_subpaths_test.go — profile.sandbox.write_subpaths is honored.
//
// Every phase profile declares its sandbox write surface in
// sandbox.write_subpaths, and the persona renders its instructions from the
// same profile. The bash-era dispatcher (subagent-run.sh, v8.21–v8.23) granted
// those paths; the Go port kept the SBPL glob-widening they fed
// (adapters/sandbox: "Globs widen to parent dir (bash:520)") but dropped the
// consumer: launch.go projects deny_subpaths and deny_read_subpaths from the
// profile and never reads write_subpaths, so the wrapper invents its own
// allow set (worktree + workspace + /tmp) plus a Go-literal copy of ONE
// profile's declaration (retrospective's lesson dir).
//
// The consequence is the P0 in inbox triage-sandbox-denies-its-own-claim-write:
// triage declares ".evolve/inbox/processing", its persona instructs
// `evolve inbox-mover claim` as the atomic hand-off, the profile never grants
// it, mkdir fails, and cycle 1623 ran twelve phases and shipped 922 lines
// against an EMPTY commitment. The same gap is latent for every other
// declared write no literal happens to cover (doc-sync's docs/ and
// knowledge-base/, orchestrator's ledger, scout's eval materialization).
//
// These tests read the REAL profiles, so a declaration that drifts from its
// grant fails here rather than in a live cycle.
```

### `go/internal/bridge/sandbox_write_subpaths_test.go:70` — above `func TestDefaultSandboxWrap_GrantsTriagesDeclaredInboxClaimDir(t *testing.T) {`

```text
// TestDefaultSandboxWrap_GrantsTriagesDeclaredInboxClaimDir is the P0 pin: the
// triage profile's own write_subpaths declaration must appear as a write
// grant in the rendered profile. Before the fix the request carries the
// declaration and the wrapper ignores it — the exact shape of cycle 1623's
// sandbox-triage.sb (deny on the plane, allow-back set without the inbox).
```

### `go/internal/bridge/sandbox_write_subpaths_test.go:97` — above `claimDir := filepath.Join(canonicalRoot, ".evolve", "inbox", "processing", "cycle-1623")`

```text
// SBPL (subpath X) covers everything below X, so the claim dir may be
// granted by its own line or by a parent's (since PR-0 of ADR-0100 the
// profile grants .evolve/inbox — the rename's SOURCE directory too).
```

### `go/internal/bridge/sandbox_write_subpaths_test.go:319` — above `func TestTriageProfile_GrantsTheClaimMove(t *testing.T) {`

```text
// TestTriageProfile_GrantsTheClaimMove pins the grant against the OPERATION
// the triage persona performs, not just a path. `evolve inbox-mover claim`
// renames <inbox>/<item>.json → <inbox>/processing/cycle-N/<item>.json; a
// rename writes BOTH directories (unlink at the source, create at the
// destination). Batch cycle 1630 (2026-09-12) had the destination granted and
// the source denied: mkdir succeeded, the rename got EPERM, and the cycle
// terminated with an empty commitment.
```

### `go/internal/bridge/scratch_cwd_test.go:74` — above `func TestIsDir_MatchesTheLaunchGuardPredicate(t *testing.T) {`

```text
// IsDir is the guard predicate driver_tmux_repl.go refuses a launch on
// (ExitBadFlags) and, since cycle-1278, the one the retro phase tests a
// candidate worktree against before dispatching. Both directions matter: a
// false negative strands a live lane in a repo-less scratch dir, a false
// positive hands the bridge a path it will refuse. Regular files are NOT dirs —
// the guard must reject them, which is why this asserts the file case too.
```

### `go/internal/bridge/signal.go:3` — above `import (`

```text
// signal.go — ADR-0101 S3: the bridge module's Signal Center codes and the
// producers' shared shape. Producers: NewEngine (a missing token resolver),
// attemptLogContext.warn (telemetry warnings), attemptLogContext.tripwire
// (a successful-but-silent attempt beyond the threshold),
// attemptLogContext.launchWarn (ADR-0103 unit 10: the Launch spine's four
// step failures registered here and the BRIDGE_EXIT_* classification the
// launchoutcome leaf registers) and paneLivenessHandler (liveness edges the
// LivenessCenter dispatches — module liveness). Each replaces a hand-written
// "[engine] WARN" line 1:1; the WARN-filtered stderr sink at the root renders
// them in the one line format.
```

### `go/internal/bridge/signal.go:28` — above `CodeBootStrikeClearFailed    signalcenter.Code = "BRIDGE_BOOT_STRIKE_CLEAR_FAILED"`

```text
// The Launch spine's step failures (ADR-0103 unit 10): each names the host
// step that could not do its best-effort work; the launch's classified
// error is unchanged by any of them.
```

### `go/internal/bridge/signal.go:57` — above `type dispatchIdentity struct {`

```text
// dispatchIdentity is what every bridge signal of one dispatch carries — ONE
// derivation: cycle and run from the request (the driver's Config carries the
// same values), phase = the agent role. No fallback: every production
// dispatcher stamps Cycle; a request without one is an operator probe whose
// signals stay at cycle 0 (kept in Recent, never filed under a cycle).
```

### `go/internal/bridge/signal_test.go:3` — above `import (`

```text
// signal_test.go — ADR-0101 S3: the bridge engine receives the Signal Center
// at construction (Deps.Signals, proven by SignalsWired), its telemetry
// warnings and tripwires are bridge.warning / bridge.tripwire events with the
// call identity in fields, and the tmux-pane liveness edges the LivenessCenter
// dispatches become pane.liveness events. Hand-written "[engine] WARN" lines
// for those facts are gone; the WARN-filtered stderr sink renders them.
```

### `go/internal/bridge/signal_test.go:208` — above `func TestDispatchIdentity_OneRuleFromTheRequestAndTheConfig(t *testing.T) {`

```text
// One identity rule for every bridge signal of a dispatch: the request's
// cycle/run and its agent role as the phase; the driver's Config carries the
// same values. No fallback — a request without a Cycle stays at cycle 0.
```

### `go/internal/bridge/stop_review_ledger_test.go:10` — above `type stopReviewRec struct{ phase, action, reason string }`

```text
// stop_review_ledger_test.go — RED contract for cycle-188 Task 1
// (stop-review-ledger-trail), bridge side: the *-tmux REPL driver must
// surface EVERY stop-review verdict (extend AND pause) through a nil-safe
// Deps.OnStopReview(phase, action, reason) callback, so the orchestrator
// can append a kind=stop_review ledger entry (ADR-0026 Stage 1 #5).
//
// Before the Builder adds Deps.OnStopReview these tests FAIL TO COMPILE
// ("unknown field OnStopReview in struct literal") — the correct RED for a
// field-add AC in Go. Once the field exists and the driver calls it, the
// assertions below pin the behavior.
```

### `go/internal/bridge/stopreview.go:58` — above `State panestream.LivenessState`

```text
// State carries the per-CLI liveness detector's structured verdict — the
// reviewer's SOLE decision input (ev.livenessState()). Populated by the
// driver via panestream.LivenessCenter.Observe+Aggregate (ADR-0068, S3); the
// pre-S3 Progressed+Busy boolean fallback is retired (an actually-unset
// State carries no liveness signal, never a boolean-derived extend).
// Progressed/Busy stay populated for fatalpane.go's C2 detector and
// checkpoint logging — they are evidence fields, not a decision path.
```

### `go/internal/bridge/stopreview.go:91` — above `const artifactTimeoutMarker = launchoutcome.ArtifactTimeoutMarker`

```text
// artifactTimeoutMarker prefixes the ONE self-describing summary line the
// artifact wait emits before returning ExitArtifactTimeout, and is the token
// Engine.Launch matches on to lift that line into the exit-81 error
// (artifactTimeoutSummary). It exists because a timeout death otherwise carries
// no reason beyond the code: the reader of a dead cycle cannot tell "the agent
// was still working and ran out of budget" (raise bridge.phase_artifact_timeout_s)
// from "the pane was wedged" (fix the wedge). Marker-driven rather than
// position-driven on purpose — real launches emit `[bridge] WARN:` sandbox
// chatter BEFORE the wait, which a first-`[bridge]`-line heuristic would report
// as the timeout's cause. The marker's ONE spelling is the parser's
// (launchoutcome.ArtifactTimeoutMarker, ADR-0103 unit 10); the emitters here
// project it.
```

### `go/internal/bridge/stopreview.go:115` — above `func livenessOrUnknown(s panestream.LivenessState) string {`

```text
// livenessOrUnknown renders a LivenessState as the stable snake_case word the
// timeout summary carries: the vocabulary's ONE spelling (LivenessState.String,
// ADR-0101 S3) with "-" folded to "_". The zero value means "no checkpoint
// observed liveness" — itself the signal — and renders "unknown" through the
// same path, so a new state can never be spelled twice.
```

### `go/internal/bridge/stopreview.go:162` — above `switch ev.livenessState() {`

```text
// Reviewer decides from LivenessState → ReviewAction alone (S3: the
// Progressed+Busy boolean fallback is retired; zero State falls through to
// the default case below and pauses).
//
// Invariants preserved from the boolean era:
//  Converging → Extend UNCONDITIONALLY (cycles 311/312: producing scout killed
//    mid-work by the backstop — real output is never "stuck").
//  BusyButStagnant → Extend BOUNDED by maxExtends (cycles 254/255: quiet-Opus
//    extended-thinking paused at interval 0).
//  Hung → fast-fail BEFORE maxExtends×interval backstop (new: the detector
//    declares Hung after stallThreshold consecutive busy-stagnant intervals).
//  Idle → Pause (no liveness signal at all).
```

### `go/internal/bridge/stopreview_test.go:14` — above `func TestDeterministicReviewer(t *testing.T) {`

```text
// TestDeterministicReviewer covers the Stage-0 review decision: extend while
// the agent produces SUBSTANTIVE output (State=Converging) — extend without
// bound; the maxExtends backstop applies only to a busy-but-STALLED pane
// (spinner, no new content). The key property — a genuinely-progressing agent
// is NEVER told to stop (cycle-311/312: a scout producing output for >30min was
// killed at the backstop) — is what keeps a slow-but-working phase alive.
// Cost/budget caps bound a pathological infinite-producer, not this wait
// reviewer. StopEvent.State is set explicitly (S3: the driver's
// panestream.LivenessCenter is the sole liveness source; the pre-S3
// Progressed/Busy boolean fallback is retired).
```

### `go/internal/bridge/stopreview_test.go:46` — above `func TestDeterministicReviewer_BusyPaneIsLiveness(t *testing.T) {`

```text
// TestDeterministicReviewer_BusyPaneIsLiveness pins the fix for the Opus
// recovery-audit false-FAIL (cycles 254/255): a pane with no substantive delta
// but a visible per-CLI busy affordance (State=BusyButStagnant) is a WORKING
// agent — extended-thinking models (Opus) render only the stripped
// "Deliberating Ns"/token-counter lines, so PaneHasSubstantiveChange reads false
// while the agent is demonstrably alive. Such an agent must be EXTENDED (bounded
// by maxExtends), never paused/killed at interval 0 — that kill recorded a PASS
// audit report as FAIL and halted the batch. StopEvent.State is set explicitly
// (S3: verdict is a pure function of State, not the retired Progressed/Busy
// booleans — Progressed/Busy still ride along on the event as evidence for
// fatalpane.go + logging, but the reviewer no longer reads them).
```

### `go/internal/bridge/stopreview_test.go:143` — above `func TestRunTmuxREPL_ContextCancelledBreaks(t *testing.T) {`

```text
// TestPaneHasSubstantiveChange (cycle-432 S4): relocated to
// panestream.TestPaneHasSubstantiveChange alongside the function it tests —
// panestream is now the single home for both (single-source-with-projection;
// see panedelta.go and panedelta_test.go).
```

### `go/internal/bridge/tmux.go:39` — above `type PaneCommander interface {`

```text
// PaneCommander is an OPTIONAL TmuxController capability: the foreground
// process name of the session's active pane (`#{pane_current_command}`).
// The boot handshake and post-paste spill check type-assert for it — a
// controller without it degrades to the marker-only behavior (cycle-274
// fix, inbox codex-update-menu-swallows-injection). Optional so existing
// test doubles keep compiling.
```

### `go/internal/bridge/tmux_codex_paste_test.go:26` — above `func TestTmuxPromptCodexPasteChipRecovery(t *testing.T) {`

```text
// The submit-verify payload also carries the paste delivery's own evidence
// (paste_settle, stability) since the 2026-09-14 wedge fix; the fixture prompt
// is under the stability floor, so it settles 1 s and skips the capture.
```

### `go/internal/bridge/tmux_inject.go:16` — above `func emitChannelBreadcrumb(w io.Writer, channel, corrID string) {`

```text
// emitChannelBreadcrumb writes one structured channel marker to w. The producer's
// correlator parses these to bracket an injected ask's answer span (ADR-0037).
// Empty corrID is a no-op so non-correlated injects add no noise. The caller
// chooses w: the <agent>-breadcrumbs.live file when the channel is on (enforce),
// else io.Discard (the producer tails the FILE — RT2 moved these off the in-memory
// stderr stream a discarded producer never read).
```

### `go/internal/bridge/tmux_inject.go:29` — above `func channelEnabled(deps Deps) bool {`

```text
// channelEnabled reports whether the live bidirectional channel (ADR-0037) is
// on. ADR-0045 I6 folded the rollout into EVOLVE_PHASE_RECOVERY: the channel is
// implied by the stage (enforce → on; off/shadow → off, byte-identical).
// channel.Enabled is the single source for both this driver and the observer adapter.
```

### `go/internal/bridge/tmux_inject.go:51` — above `if env.Kind == inbox.KindKeystroke {`

```text
// Cycle-124 F4 / ADR-0023 addendum: the "full tmux control" hatch the
// operator asked for. Body is one tmux key-spec (literal text and/or
// space-separated named keys like "Enter" / "Escape" / "C-c" / "Up" /
// "y Enter") sent verbatim via SendKeys with enter=false. NO idle-gate
// (operator may need to send keys precisely BECAUSE the agent isn't
// idle — e.g. dismissing a modal that hung mid-turn), NO ESC prefix
// (unlike interrupt), NO automatic Enter append. Empty body is a no-op
// to match the existing SendKeys contract (line 59 of tmux.go skips
// empty key strings). The operator is fully responsible for what they
// inject; the bridge does not interpret the body.
```

### `go/internal/bridge/tmux_inject.go:69` — above `if err := deps.Tmux.SendKeys(ctx, lp.session, env.Body, false); err != nil {`

```text
// Surface a failed send instead of logging success unconditionally
// (cycle-124 review MEDIUM): a vanished session / killed pane would
// otherwise show as `injected keystroke "Enter"` on stderr while
// nothing actually reached the REPL.
```

### `go/internal/bridge/tmux_pane_checks.go:21` — above `if m, err := LoadManifest(lp.name); err == nil {`

```text
// Project the manifest's quota/rate-limit pattern into the profile
// (single-source, ADR-0047): the LivenessCenter's ExhaustionProbe reads
// ExhaustedRegex to detect a mid-phase wall through the SAME abstraction as
// liveness. manifestExhaustedPattern is the one maintained source ("what a
// wall looks like") shared with the usage probe (usageclassify.go), so the
// probe-time and phase-execution detections can never drift. Best-effort: an
// unloadable manifest leaves ExhaustedRegex empty (detection off, fail-open —
// p is a value copy, so this never mutates the shared Profiles map).
```

### `go/internal/bridge/tmux_pane_checks.go:35` — above `func detectorFor(lp tmuxLaunch) panestream.LivenessProbe {`

```text
// detectorFor returns a per-run LivenessProbe for the tmux driver identified by
// lp. Co-located with paneProfileFor (single-source-with-projection, ADR-0047):
// the profile drives both the content-boundary extractor and the strategy
// registry without duplicating the CLI→name mapping.
```

### `go/internal/bridge/tmux_pane_checks.go:79` — above `func paneLooksLikeShellSpill(pane string) bool {`

```text
// paneLooksLikeShellSpill reports the cycle-274 paste-spill signatures: a
// shell continuation prompt (quote>/bquote>/dquote>/heredoc>) as the LAST
// non-blank line (continuation prompts only ever render at the cursor), or
// zsh's command-not-found echo anywhere. Callers MUST pair this with the
// authoritative paneShellProcess check — agent output may legitimately quote
// these strings.
```

### `go/internal/bridge/tmux_pane_checks_exhaustion_test.go:8` — above `func TestPaneProfileFor_ProjectsExhaustedRegex(t *testing.T) {`

```text
// paneProfileFor must project the per-CLI manifest's quota/rate-limit pattern
// into PaneProfile.ExhaustedRegex (single-source), so the LivenessCenter's
// ExhaustionProbe detects a mid-phase wall. Acceptance: the projected pattern
// must match the REAL Gemini incident wording — "Individual quota reached" —
// the exact message that hung the agy router phase (usage-probe pattern already
// matches it; the bug was it was never wired into phase execution).
```

### `go/internal/bridge/tmux_project_root_env_test.go:3` — above `import (`

```text
// tmux_project_root_env_test.go — the pane shell carries the plane's root.
//
// Headless drivers hand the inner CLI driverEnv(deps) — the process env plus
// Deps.Env — but a tmux pane is a shell the bridge did not start: it inherits
// the tmux server's environment, and the bridge only ever sends it `cd
// <worktree>` and the launch command. So every `evolve` subcommand an agent
// runs in that pane resolves its root through cmdutil.EnvOrCwd → the cycle
// worktree — whose .evolve/inbox is a git-tracked snapshot of the plane's
// queue. Batch cycle 1631 (2026-09-12) claimed an inbox item in that copy;
// the plane's item never moved. core/phase.go documents ProjectRoot as
// "what a subprocess sees as EVOLVE_PROJECT_ROOT"; this pins that the pane
// sees it, in the one shell that survives into the agent's commands.
```

### `go/internal/bridge/tmux_repl_fixture_test.go:97` — above `base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "final scrollback", "cleanup scrollba…`

```text
// Two "working ❯" frames: the cycle-1233 cross-poll stability window
// (completion.go) completes the artifact one tick after it first appears,
// so the pane is captured once more while the deliverable settles.
```

### `go/internal/bridge/tmux_repl_fixture_test.go:130` — above `{`

```text
// The repeated "idle ›" is the settling tick the cycle-1233 cross-poll
// stability window adds before the artifact completes (see completion.go).
```

### `go/internal/bridge/tmux_repl_integration_test.go:187` — above `if strings.Contains(stderr.String(), "re-sending Enter") {`

```text
// The fake models a CLEAN real REPL (marker redrawn after every consumed
// line): submit-verify must see a clear input line, not lean on the
// ground-truth belt. A resend here means the harness regressed to the
// parked shape that produced the v22.20.0 release red.
```

### `go/internal/bridge/tmux_repl_livecli_test.go:16` — above `type liveCLISpec struct {`

```text
// tmux_repl_livecli_test.go — the REAL-CLI tier of the tmux REPL suite,
// grounded in the ground-truth capture in
// knowledge-base/research/tmux-repl-cli-behavior-2026-05-26.md.
//
// These drive the ACTUAL claude / codex / agy binaries inside a real tmux
// server, so they validate the one thing neither the fake-tmux unit tests
// nor the scripted-fake integration tests can: that the boot markers the
// drivers grep for (❯ / › / "? for shortcuts") actually appear in the real
// CLI's pane, at the real version installed on this host.
//
// Two tiers, both OFF by default (so `go test` / CI needs neither the CLIs
// installed nor any LLM spend):
//
//	EVOLVE_BRIDGE_LIVE_CLI=1            → boot-marker tier (cheap: launches
//	                                      the CLI, asserts the marker, exits.
//	                                      No prompt delivered → no inference).
//	EVOLVE_BRIDGE_LIVE_CLI_ROUNDTRIP=1 → full round-trip tier (real LLM
//	                                      spend: prompt → artifact write).
//
// The specs below MUST match the production driver constants. If a CLI
// upgrade moves a marker, the boot-marker test fails loudly here before it
// breaks a real cycle — that is the point.
```

### `go/internal/bridge/tmux_session.go:30` — above `nonce := strconv.FormatUint(ephemeralSessionNonce.Add(1), 36)`

```text
// ADR-0049 N15: a per-process monotonic nonce GUARANTEES uniqueness even
// when two ephemeral sessions are minted in the same wall-clock second
// (concurrent fleet cycles, or a same-phase retry within a cycle). The
// second-granularity timestamp alone collided; pid covers cross-process and
// the nonce covers within-process. It sits BEFORE the timestamp so
// truncate64 (tmux's 64-char ceiling) degrades the recency hint, never the
// uniqueness — for a long agent like build-planner the tail timestamp may be
// clipped, but n<nonce> always survives.
```

### `go/internal/bridge/tmux_session.go:43` — above `var ephemeralSessionNonce atomic.Uint64`

```text
// ephemeralSessionNonce is the process-global counter behind resolveSession's
// per-session nonce (ADR-0049 N15). atomic so concurrent fleet dispatches in
// one process never read the same value.
```

### `go/internal/bridge/tmux_session.go:79` — above `func recoverBlankPane(ctx context.Context, deps Deps, session string, scrollback int, pane, pfx string) (string, bool) {`

```text
// recoverBlankPane handles the claude ≥2.1.173 BLANK-PANE render wedge
// (inbox claude-2.1.173-blank-pane-after-interval): an EMPTY capture while
// the session is alive is the Ink renderer wedging, not idleness —
// cycle-291's agent kept working behind a blank pane and the stall-pause
// burned interval×attempts to exit=81. Recovery: jiggle the window width
// (two SIGWINCHes → full repaint, windowJiggler optional capability) and
// re-read. Returns the freshest pane and whether the wedge persisted — a
// still-blank pane must read BUSY (extend; never pause a live agent on a
// pane that stopped rendering; the maxExtends backstop still bounds it).
```

### `go/internal/bridge/tmux_session.go:108` — above `func tmuxCleanup(ctx context.Context, deps Deps, name, session, scrollbackFile string, named bool, scrollback int) {`

```text
// tmuxCleanup captures final scrollback then kills the session — unless it
// is a named session, which is preserved for resume. It runs on a context
// DETACHED from the launch context's cancellation and bounded by
// tmuxCleanupTimeout: the deferred cleanup fires exactly when the launch
// context has been canceled (operator pause, batch stop, watchdog), and a
// canceled context would make every tmux command below refuse to run — the
// provider session then survives the orchestrator (2026-09-09 token-waste
// root cause #1). Cancellation is the reason to clean up, not a reason to skip it.
```

### `go/internal/bridge/tokencount_test.go:13` — above `func TestExtractTokenCount(t *testing.T) {`

```text
// tokencount_test.go — RED contract for cycle-256 task `token-counter-extraction`.
//
// The bridge already compiles `rxTokens` (↓ N.Nk tokens) only to STRIP volatile
// counter lines from cleanPane(). This task repurposes that same pattern to
// EXTRACT the peak observed token count into a structured value, write a
// `token-usage.json` sidecar after a tmux phase, and surface it on bridge.Report.
//
// These tests are behavioral: TestExtractTokenCount exercises the pure parser
// across positive/fractional/multi/no-match/malformed inputs (the strongest
// anti-no-op signal — a presence-only impl that returns a constant fails the
// peak + zero cases); TestTmuxPhase_WritesTokenUsage runs the real REPL engine
// and asserts the sidecar the agent's pane produced; TestBuildReport_TokenUsage
// asserts the report field is populated from (and tolerant of) the sidecar.
```

### `go/internal/bridge/tokencount_test.go:46` — above `{"unified extractor: plain-integer → 5200", "↓ 5200 tokens", 5200},`

```text
// Reconciled (cycle-429 S1): the old k-only extractor returned 0 here; the
// unified ExtractResponseTokens superset correctly yields 5200 (plain-integer
// path). Production panes always render the k-form; this form only appears in
// synthetic test frames where 5200 is the correct value.
```

### `go/internal/bridge/tokendriver_test.go:3` — above `import (`

```text
// tokendriver_test.go — cycle-779 AC2 plumbing contract (named by ACS
// predicate C779_005): recordTokenUsage must forward the launch's CLI/driver
// identity into the tokenusage.Window it hands the resolver. Without it the
// resolver cannot dispatch per driver and uncovered drivers (agy/codex)
// surface as silent zeros — the 2026-07-13 all-zeros baseline defect.
```

### `go/internal/bridge/tokenfallback_red_test.go:3` — above `import (`

```text
// tokenfallback_red_test.go — RED contract for cycle-754 task
// token-resolver-production-wiring (composition-root half; the tokenusage half
// is internal/tokenusage/fallbackchain_test.go).
//
// recordTokenUsage (engine.go) currently builds a tokenusage.Window carrying
// only Worktree/ArtifactPath/Start/End, so the lower fallback tiers can never
// fire and every tmux-driven launch records "source":"none" with zero tokens
// (confirmed live across 124 .evolve/runs/*/llm-calls.ndjson files). This
// contract requires the engine to thread the launch's events-log context —
// the workspace's <agent>-events.ndjson — into the Window it hands the
// resolver, so the REAL production resolver (tokenusage.DefaultResolver)
// recovers usage for a launch with no transcript. DO NOT modify these tests;
// make them pass by wiring context into the Window at the call site.
```

### `go/internal/bridge/tokenresolver_bootwarn_test.go:11` — above `func stubResolver(tokenusage.Window) (tokenusage.Result, error) {`

```text
// Cycle-745 task token-resolver-boot-warn: Deps.TokenResolver is a fail-open
// seam. Historically nil disabled the whole token record, which is how the
// all-zeros first telemetry batch shipped unnoticed. Lifecycle telemetry now
// remains active, but token usage is unavailable, so fail-open must still be
// loud. Constructing an Engine with a nil TokenResolver emits one WARN naming
// TokenResolver; a wired resolver stays silent.
```

### `go/internal/bridge/transient_dwell_shortcircuit_test.go:3` — above `import (`

```text
// transient_dwell_shortcircuit_test.go — the cycle-1580 RED contract for
// `transient-artifact-timeout-shortcircuit-the-silence-budget`.
//
// The defect. `classifyTransientPane` already recognizes a family's
// manifest-declared transient upstream error, but it is consulted ONLY after
// the artifact wait has already timed out (driver_tmux_repl.go, the
// `!completed` block) — where it merely annotates the marker line. Nothing
// reads it DURING the wait, so a session parked on "API Error: 529 Overloaded
// … usually temporary" burns the entire silence budget (3 of 4 observed router
// stalls, cycles 1523/1524/1526, at ~600s each) before anything reacts.
//
// The contract these tests pin (scout AC 1-7):
//
//	AC-1  the transient pattern is resolved ONCE, in newAutoResponder, from the
//	      launched CLI's manifest — mirroring exhaustedRegex.
//	AC-2  no new exit code: the shortcircuit exits through ExitArtifactTimeout.
//	AC-3  a dwell tracker fires only after the pattern has held for 60s, and
//	      ANY non-matching frame resets it.
//	AC-4  a BUSY pane is never preempted (the stop-review prime directive).
//	AC-5  the ADR-0044 RecoveryStage dial gates the ACTION (off/shadow/enforce,
//	      default shadow) while shadow still records would_fast_fail evidence.
//	AC-6  in enforce the dwell sets a ReviewStop verdict and breaks into the
//	      EXISTING `!completed` machinery — escalation report, marker line,
//	      exit 81 — rather than duplicating any of it.
//	AC-7  the enforce stop applies a deliberate re-dispatch delay so the retry
//	      does not land in the same upstream weather window.
//
// CADENCE CONTRACT (load-bearing for the Builder). The dwell MUST be measured
// on the wait loop's own ~2s poll cadence — an observation counter, exactly
// like exhaustionGate/exhaustionPersistObservations — NOT on wall-clock
// deps.Now(). Deps.Sleep is documented as "tests inject a no-op so the loops
// iterate instantly (the loop bound is an iteration counter, not wall clock)",
// so a wall-clock dwell is unreachable in every fixture in this package and
// would make the acceptance criteria unsatisfiable (the cycle-644 shape).
// 60s dwell == 30 consecutive matching ticks of the 2s poll.
```

### `go/internal/bridge/transient_dwell_shortcircuit_test.go:69` — above `func runTransientDwell(t *testing.T, fx launchFixture, paneSeq []string, stage string, timeoutS int) transientDwellRun {`

```text
// runTransientDwell drives a REAL claude-tmux launch through Engine.LaunchArgs
// with a scripted pane sequence and an explicit ADR-0044 stage.
//
// It does not reuse runTmuxOnStopReview because that helper overwrites
// Deps.Sleep with a no-op unconditionally — and Deps.Sleep is the only seam
// through which AC-7's re-dispatch delay is observable. The sleeps are
// recorded, not slept, so the loops still iterate instantly.
```

### `go/internal/bridge/transient_dwell_shortcircuit_test.go:107` — above `func (r transientDwellRun) transientOutcomes(t *testing.T) []map[string]any {`

```text
// transientOutcomes reads the DURABLE interaction ledger this launch wrote
// (<workspace>/<phase>-interactions.ndjson) and returns the transient-dwell
// records. Reading the emitted artifact — not an in-memory spy — is the point:
// the soak reporter and the would/did parity check read exactly this file, and
// stderr-only evidence left ADR-0044 C2's soak blind by construction (R8.3).
```

### `go/internal/bridge/transient_dwell_shortcircuit_test.go:343` — above `func TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted(t *testing.T) {`

```text
// TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted — AC-4: the pane
// shows the 529 text AND the live interrupt affordance ("esc to interrupt"),
// i.e. the agent is working while provider chatter sits on screen. Never kill a
// working agent (the cycle-254/255 false-FAIL prime directive, mirrored from
// fatalPaneVerdict's ev.Busy guard) — the reviewer decides, as today.
```

### `go/internal/bridge/transient_dwell_shortcircuit_test.go:367` — above `func TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing(t *testing.T) {`

```text
// TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing — AC-5: shadow is
// the DEFAULT stage and must be behavior-neutral (the reviewer still decides)
// while still leaving durable would_fast_fail evidence for the soak's
// false-positive measurement. Evidence without action is the whole point of the
// ADR-0044 dial.
```

### `go/internal/bridge/wallcorroborate.go:3` — above `import (`

```text
// wallcorroborate.go — out-of-band corroboration for the exhaustion fast-fail
// (2026-08-15 false-wall incident; inbox exhaustion-scan-needs-corroboration).
//
// The pane exhaustion scan can never distinguish STATE from SUBJECT MATTER: a
// lane fixing the exhaustion regexes renders the true wall phrases from its
// own test fixtures, the file persists on-pane exactly like a real wall, both
// existing guards (prompt-echo strip, persistence gate) are structurally
// defeated, and every fallback family "walls" on the same content — a false
// all-families quota checkpoint while the plans have headroom.
//
// Pattern: Strategy via DI. WallCorroborator is a Deps seam (like Runner/Now):
// on a persistence-gate cross the site asks the corroborator; only a
// corroborated wall escalates rc 85. The default strategy sends ONE cheap
// headless request to the family — a truth the pane's content cannot forge:
// a provider that answers is not walled, whatever the terminal shows. nil
// corroborator = legacy behavior byte-identical (every pre-existing test and
// caller unaffected); the production composition root wires
// DefaultWallCorroborator explicitly.
```

### `go/internal/bridge/wallcorroborate.go:40` — above `var wallProbeRecipes = map[string]struct {`

```text
// wallProbeRecipes is the per-family probe DATA: the cheapest one-shot
// headless request each CLI supports. Families without an entry cannot be
// corroborated and stay conservative (walled=true — legacy behavior). Argv
// forms verified live 2026-08-15 (CODEX-ALIVE / CLAUDE-ALIVE probes).
```

### `go/internal/bridge/wallcorroborate_test.go:3` — above `import (`

```text
// wallcorroborate_test.go — contracts for the wall-corroboration seam. The
// incident narrative and design rationale live ONCE, in wallcorroborate.go's
// header (2026-08-15 false-wall class; Strategy via DI, nil = legacy).
```

### `go/internal/bridge/wallcorroborate_test.go:102` — above `hang := func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {`

```text
// NOT parallel — this test WRITES the package-level wallProbeTimeout var;
// Go runs serial tests to completion before any t.Parallel test resumes,
// which is the only ordering that makes the write race-free (the -race
// CI run on PR #466 caught the parallel variant racing every reader).
```

### `go/internal/bridge/wallcorroborate_test.go:128` — above `func TestTick_ContentWallSuppressedWhenCorroboratorSaysHealthy(t *testing.T) {`

```text
// A pane whose wall text is FILE CONTENT (not prompt echo — the 2026-08-15
// class: fixtures under edit persist frame after frame) must NOT escalate
// when the corroborator proves the provider healthy; the suppression is loud
// and the scan disables for the rest of the phase (exactly ONE probe, not one
// per tick).
```
