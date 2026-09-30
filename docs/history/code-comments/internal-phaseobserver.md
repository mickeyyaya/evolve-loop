# Comment history: `internal/phaseobserver`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phaseobserver/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names AND
// exercises the exported symbols apicover flagged UNCOVERED:
//
//	type  Scope, const ScopeCycle, const ScopePhase — via the observer's scope
//	      handling: Run defaults an empty Scope to ScopePhase (phaseobserver.go
//	      L207-209) and stamps string(cfg.Scope) into the observer_started event
//	      (L240). We drive Run end-to-end for each enum value and read the field
//	      back out of the events.ndjson the observer actually wrote.
//	func  DefaultProcessAlive — the production R3.4 liveness probe; invoked
//	      against a real, live process group (alive) and a bogus pgid (dead).
//
// (The dead ExitFatal=1 enum member — zero consumers tree-wide; Run returns only
// ExitOK/ExitInvalidArgs — was deleted rather than pinned with a value test.)
```

### `go/internal/phaseobserver/coverage_test.go:13` — above `func fixedClock(at time.Time) func() time.Time {`

```text
// coverage_test.go targets branches the behavior suite does not yet reach:
// the zero-value config defaults, the empty-stdoutPath default, the heartbeat
// emit, the EOF-grace shutdown and the two fault paths the engine reports
// (the tail/processLine edges moved to internal/observerengine, ADR-0103
// unit 12). Behavior-pinned, no real
// sleeps > ~150ms, deterministic clocks and t.TempDir only.
```

### `go/internal/phaseobserver/coverage_test.go:26` — above `func TestRun_AppliesZeroValueDefaults(t *testing.T) {`

```text
// === Run applies zero-value defaults ========================================
// Passing a Config with all the tunable knobs left at their zero values forces
// every `if cfg.X == 0 { cfg.X = default }` branch in withDefaults (PollS,
// StallS, EOFGraceS, HeartbeatEvery, Scope — the five never-read defaults
// LoopN/LoopWindowS/ErrorRate/CostSigma/ThrottleN fell with ADR-0103 unit
// 12) plus the empty-stdoutPath default. A quick SIGUSR1
// shutdown keeps it deterministic.
```

### `go/internal/phaseobserver/defaults_pin_test.go:3` — above `import (`

```text
// defaults_pin_test.go — ADR-0103 unit 12 step 0: the two owners of the
// observer's default thresholds are the host's zero-value defaults (Run) and
// policy's compiled ObserverConfig() (the manual subcommand dereferences the
// latter into Config). They agree on PollS 5 / StallS 600; NudgeS is the NAMED
// divergence (policy 300 — the subcommand nudges because phasecmd feeds the
// policy value; host 0 — a bare Run never nudges); EOFGraceS 10 is host-only
// (policy leaves it 0). An external package so the pin can import policy
// without a cycle. Kills M19 (a host default drifting from policy's).
```

### `go/internal/phaseobserver/order_pins_test.go:3` — above `import (`

```text
// order_pins_test.go — ADR-0103 unit 12 step 0: the poll-tick order and the
// rule invariants of Run, pinned on the pre-extraction code (8e8f080f) BEFORE
// the tick body moved into internal/observerengine. Each test names the
// one-line mutant it kills; every one was proven red against that mutant by
// hand before the move. They drive the kept Run facade, so they stay in the
// host as the regression net after the move.
```

### `go/internal/phaseobserver/order_pins_test.go:340` — above `func TestRun_UnknownScopeRunsNoRules(t *testing.T) {`

```text
// TestRun_UnknownScopeRunsNoRules — characterization of the :351 tautology: a
// Scope that is neither phase nor cycle runs NO stall rule (no incident, no
// kill) while observer_started still records it verbatim. The :37 comment's
// "cycle-scope runs only stall_no_output" was never implemented; any third
// value disables ALL rules. Kills M12 (`==` → `!=` on the scope check).
```

### `go/internal/phaseobserver/phaseobserver.go:1` — above `package phaseobserver`

```text
// Package phaseobserver ports the core behavior of
// legacy/scripts/dispatch/phase-observer.sh (v10.18.0+ default stall detector).
//
// SCOPE NOTE: this port covers the primary use case — stall detection +
// NDJSON event tracking + report writing on shutdown. The 4 secondary
// detection rules (infinite_loop, error_spike, cost_anomaly, rate_limit) were
// never ported; their Config fields stay for the literal shape (ADR-0103
// unit 12, operator question 5).
//
// Since ADR-0103 unit 12 this file is the SEAM: validation, defaults, the
// real ticker and the select loop stay here and drive the engine in
// internal/observerengine one Tick at a time — the tail, the decoder, the
// stall rules, the incident responder and the envelope/report sinks live
// there. Run's spelling is kept for the phasecmd subcommand and the by-name
// tests (Strangler Fig). The observer emits to:
//   - {agent}-observer-events.ndjson  (live, append-only, one envelope/line)
//   - {agent}-observer-report.json    (atomic write at shutdown)
```

### `go/internal/phaseobserver/phaseobserver.go:78` — above `StallPolicy recovery.StallPolicy`

```text
// StallPolicy (ADR-0044 C4) maps a stall INCIDENT to a recovery action
// (extend | kill_retry | escalate), decoupling action from detection.
// nil — the default until the C3 composition slice wires a real policy —
// preserves the legacy inline behavior byte-for-byte: Enforce → SIGTERM,
// unenriched INCIDENT envelope. With a policy injected, its verdict
// outranks Enforce (extend/escalate suppress the kill; kill_retry kills
// even without Enforce) and the decision + justification are recorded
// inside the INCIDENT envelope (action / action_reason).
```

### `go/internal/phaseobserver/phaseobserver.go:88` — above `ProcessAlive func(pgid int) bool`

```text
// ProcessAlive probes whether the observed agent's process group still
// exists (R3.4, cycles 274/277: pane/log echo is NOT liveness — a wedged
// shell read as alive for 25+ min after the CLI died). A dead group
// fires a "process_dead" INCIDENT once, resolved by the stall policy
// (chain: kill_retry) regardless of idle budgets. nil DISABLES the probe
// (legacy byte-identical — the same nil-seam convention as StallPolicy);
// the composition root wires DefaultProcessAlive, so fixture Configs
// with fake pgids never see a false "dead".
```

### `go/internal/phaseobserver/process_dead_test.go:1` — above `package phaseobserver`

```text
// process_dead_test.go — R3.4: the observer's liveness signal must include
// the agent PROCESS, not just pane/log echo (inbox codex-update-menu,
// cycles 274/277: a wedged shell read as alive for 25+ min). A dead process
// group fires a "process_dead" INCIDENT within one poll tick — once, not
// per-tick — and the stall policy resolves it to kill_retry regardless of
// idle budgets.
```

### `go/internal/phaseobserver/process_dead_test.go:28` — above `PollS: 1, StallS: 99999, EOFGraceS: 9999,`

```text
// StallS is huge so no idle stall can fire — any incident in this
// test comes from the process probe alone.
```

### `go/internal/phaseobserver/seam_test.go:3` — above `import (`

```text
// seam_test.go — ADR-0103 unit 12 §6 tests 37-38: the host is the unit's ONE
// construction site of the engine, and its Nudge port is the inbox append the
// original spelled inline.
```

### `go/internal/phaseobserver/sequence_golden_test.go:3` — above `import (`

```text
// sequence_golden_test.go — ADR-0103 unit 12 step 0, G3: the ordered event
// sequence of one idle scenario driven through Run with a count-stepping
// clock, captured on 8e8f080f. The clock is indexed by CALL: construction (1),
// observer_started (2), one tick that ingests four lines (3-6: one read per
// line; 7: the idle rule; 8: the no-progress rule; 9: the heartbeat), a tick
// at +400 s (10: idle → the nudge; 11: inbox.Append's own read — the SEVENTH
// clock site, host-side; 12: soft_stall_nudge; 13: no-progress; 14: heartbeat),
// a tick at +700 s (15: idle → stuck_no_output under an extend policy; 16: the
// INCIDENT emit; 17: no-progress; 18: heartbeat — the clock closes ShutdownSig
// here), the shutdown (19) and the report (20). One extra or missing clock
// read shifts the shutdown point and every `ts` after it — the clock-parity
// tripwire the leaf's replay (its test 31) must match call for call.
//
// Q12 note: under a frozen clock two emits in one tick share an `id` —
// eventCount is a line count, not an emit sequence; the golden documents it.
```

### `go/internal/phaseobserver/stallpolicy_test.go:3` — above `import (`

```text
// stallpolicy_test.go — ADR-0044 C4 (Slice 4) RED tests: the observer's
// StallPolicy Strategy seam.
//
// cycle-262 D5: the observer DETECTS stalls (stuck_no_output /
// stuck_no_progress INCIDENT events) but its only action is an inline
// "Enforce → SIGTERM" branch — detection and action are welded together, so
// recovery policy can't evolve without editing the detector (SRP violation).
// C4 extracts the action decision into recovery.StallPolicy injected via
// Config: nil policy ⇒ byte-identical legacy behavior (Enforce branch,
// unenriched envelope); a policy maps each typed StallEvent →
// extend | kill_retry | escalate, the decision is recorded INSIDE the
// INCIDENT envelope (action + action_reason — every recovery decision is
// justified, ADR-0044), and only kill_retry touches the process group.
//
// The policy is wired by the C3 composition slice; this slice ships the seam
// default-nil (behavior-neutral), pinned by the legacy tests above plus
// TestRun_StallPolicyNil_EnvelopeUnenriched below.
```

### `go/internal/phaseobserver/stallpolicy_test.go:33` — above `type scriptedStallPolicy struct {`

```text
// scriptedStallPolicy returns a fixed action for every stall incident and
// records the events it was consulted with.
```
