# Comment history: `internal/adapters/observer`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/adapters/observer/core_adapter.go:3` — above `import (`

```text
// core_adapter.go — cycle-122 Fix 3 / ADR-0030: bridges this package's
// per-phase Observer + Watch loop to the orchestrator's core.Observer
// interface so `evolve loop` can auto-spawn a stall detector for every
// phase without requiring the operator to run `evolve phase-observer`
// as a separate subprocess.
//
// The adapter is intentionally thin: it translates core.PhaseRequest
// → Config, derives the stdout-log and events paths from the engine's
// one layout (observerengine.PathsFor — <workspace>/<phase>-stdout.log,
// matching the runner's convention at go/internal/phases/runner/runner.go,
// and <workspace>/<phase>-observer-events.ndjson), opens/creates the
// append-only events sink, and runs the existing Observer.Watch goroutine.
// The returned cancel function signals the watcher to stop + closes the
// events sink. Its two faults — the sink could not be opened, the watcher
// did not exit — are observer.warning signals through the engine's
// Reporter (ADR-0103 unit 12), never stderr lines.
```

### `go/internal/adapters/observer/core_adapter.go:58` — above `RecoveryStage string`

```text
// RecoveryStage is the ADR-0044 Unified Phase Recovery stage, injected
// by the orchestrator from cfg.PhaseRecovery (policy-resolved). Empty →
// channel.ResolveStage returns "shadow" (behavior-neutral default).
```

### `go/internal/adapters/observer/core_adapter.go:66` — above `Signals func() *signalcenter.Center`

```text
// Signals is the orchestrator's Signal Center, read live at every use
// (the root builds the Center before the adapter; nil = the Null Object).
// ADR-0103 unit 12: the adapter's own faults are observer.warning signals.
```

### `go/internal/adapters/observer/core_adapter.go:109` — above `a.reporter().Report(originAdapterStart, req.Cycle, phase, observerengine.CodeEventsSinkOpenFailed,`

```text
// Best-effort: signal + degrade to no-op. Per ADR-0030, observer
// failure must NOT block the phase — it runs unobserved.
```

### `go/internal/adapters/observer/core_adapter.go:127` — above `WorkspaceDir: req.Workspace,`

```text
// Treat fresh writes anywhere in the workspace as progress — tmux-driver
// agents write live output to the tmux scrollback (not the stdout-log),
// so workspace artifact writes are the only filesystem liveness signal
// until clean exit. Without this the observer falsely stalls a working
// tmux build agent (cycle-141).
```

### `go/internal/adapters/observer/core_adapter.go:133` — above `LivenessProbe: anyProbe(`

```text
// cycle-190 + headless follow-up: when even workspace writes go quiet
// (a long single "Incubating" turn that thinks for minutes then dumps
// its artifact), filesystem signals are blind. Two ground-truth probes,
// consulted ONLY at the stall threshold and OR-ed (alive if either):
//   - tmux pane-hash: the live pane spinner/token-counter advancing
//     (tmux-driver phases; a non-tmux phase finds no session → no-op);
//   - process CPU time: the agent subprocess accruing CPU (HEADLESS
//     phases, which have no pane; PID written by the bridge at launch).
```

### `go/internal/adapters/observer/core_adapter.go:146` — above `obs := New(cfg, sink)`

```text
// Cycle-124 Task 6 — KNOWN GAP: the operator's "active liveness
// nudging" mechanism is wired into the STANDALONE `evolve phase-
// observer` (phasecmd; policy NudgeS defaults to 300s) but the
// AUTO-SPAWN path here does NOT yet emit nudge envelopes — this
// adapter's Observer is a thin Watch-only implementation. The full
// nudge logic (inbox append + nudged dedupe + soft_stall_nudge event),
// the dead-process probe and the no-progress backstop now live in
// `internal/observerengine` (ADR-0103 unit 12) as a clock-stepped
// Engine the standalone host drives; folding it in here is the
// unit-12 follow-up F8 (operator question 1 — it would start nudging
// autonomous runs). DefaultNudgeS stays as the threshold that fold
// reads. For autonomous `evolve loop` runs, nudging is currently
// effectively opt-out (no nudge fires unless an operator runs the
// standalone phase-observer alongside the loop).
```

### `go/internal/adapters/observer/core_adapter.go:170` — above `var prodCancel func()`

```text
// ADR-0037 + ADR-0045 I6: spawn the live channel producer beside the
// observer when the channel is on. The channel rides EVOLVE_PHASE_RECOVERY
// (enforce implies it). channel.Enabled is the single source shared with the
// bridge driver. Off → byte-identical to pre-channel behavior (no producer,
// no feed file).
```

### `go/internal/adapters/observer/core_adapter.go:178` — above `stdoutPath, stderrPath := a.channelSourcePaths(req, phase)`

```text
// Transport-aware source (ADR-0037 RT3): a tmux-family driver streams its
// live answer to <agent>-pane.live and breadcrumbs to
// <agent>-breadcrumbs.live (its stdout.log is empty until the at-exit
// dump), so point the Producer there. Headless → empty paths → the
// Producer keeps its legacy <phase>-stdout/-stderr.log defaults.
```

### `go/internal/adapters/observer/core_adapter.go:249` — above `func (a *CoreAdapter) channelSourcePaths(req core.PhaseRequest, phase string) (stdout, stderr string) {`

```text
// channelSourcePaths returns the (stdout, stderr) files the channel Producer
// should tail for this phase (ADR-0037 RT3). A tmux-family driver streams its
// live answer to <agent>-pane.live + correlation breadcrumbs to
// <agent>-breadcrumbs.live, so the producer reads that pair. A headless driver
// streams live to <phase>-stdout.log, so empty strings are returned and the
// producer keeps its legacy defaults. The family is resolved best-effort from
// the per-phase CLI env (profile.cli pins not surfaced in env are not seen
// here — a wrong guess only degrades that phase's live feed, never the phase).
```

### `go/internal/adapters/observer/core_adapter_signals_test.go:3` — above `import (`

```text
// core_adapter_signals_test.go — ADR-0103 unit 12 §6 tests 43-46: the live
// adapter's two faults reach the Signal Center through the engine's Reporter
// (module observer, kind observer.warning), its paths project from the
// engine's layout, and it writes neither os.Stderr nor reads the environment.
```

### `go/internal/adapters/observer/core_adapter_sinkclose_test.go:3` — above `import (`

```text
// core_adapter_sinkclose_test.go — unit contract for the fable5 deep-scan
// finding observer-sink-close-race (inbox weight 0.92, cycle-618 scout;
// fix landed cycle 669).
//
// Context. Start's returned cancel closure (core_adapter.go) historically
// waited for the watcher goroutine with a bounded 10s timeout, then
// UNCONDITIONALLY closed the events sink regardless of which select arm
// fired. When the watcher goroutine is genuinely wedged (e.g. a hung
// liveness probe or a stuck sink write) past the 10s bound, the timeout arm
// fires but the leaked goroutine is still running and may still be mid-write
// to the sink. Closing the sink out from under it is a use-after-close race
// — the leaked goroutine's next Write can return an error, or (on some
// platforms/sink implementations) corrupt concurrent state, and the returned
// error is swallowed, so the race is invisible until it manifests as a flake.
//
// The landed fix extracts the wait+close decision into an isolated, directly
// testable primitive:
//
//	func closeSinkAfterWait(done <-chan struct{}, timeout time.Duration, closer io.Closer) bool
//
// which closes `closer` ONLY when `done` fires within `timeout` — never on
// the timeout arm, so a still-running leaked goroutine's sink is never
// closed under it (the accepted fd leak is documented by Start's WARN log;
// the OS reclaims the fd at process exit). Start's cancel closure delegates
// to this helper with the real done channel, sinkCloser, and the production
// 10s bound. These tests pin that contract against regression.
```

### `go/internal/adapters/observer/core_adapter_test.go:77` — above `func TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows(t *testing.T) {`

```text
// TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows is the
// cycle-122 shape regression test: workspace exists but the phase's
// stdout-log never appears (codex hung at modal). The observer must
// emit a stall_no_output INCIDENT.
```

### `go/internal/adapters/observer/core_adapter_test.go:171` — above `func TestCoreAdapter_Start_DegradesToNoopWhenEventsFileUnopenable(t *testing.T) {`

```text
// TestCoreAdapter_Start_DegradesToNoopWhenEventsFileUnopenable covers the
// os.OpenFile error branch in Start: when Sink is nil and the events file
// cannot be created, the adapter must degrade to a no-op cancel (never block
// the phase, per ADR-0030) rather than panic. We force the failure by pointing
// Workspace at a path that is a regular FILE, so the events path's parent is
// not a directory and OpenFile fails.
```

### `go/internal/adapters/observer/cpu_probe.go:3` — above `import (`

```text
// cpu_probe.go — a CPU-delta LivenessProbe for HEADLESS phases. The tmux
// pane-hash probe (tmux_probe.go) covers tmux-driver agents, but a headless
// `claude -p` phase that thinks for minutes in one turn with no streamed output
// has no pane to probe and its stdout-log + workspace both go flat — the same
// false-stall shape as cycle-190, one layer over. This probe reads the agent
// process's accumulated CPU time: a computing agent (even silently thinking)
// accrues CPU; a deadlocked one does not. The bridge writes the agent PID to a
// per-phase file at launch (engine.go execRunner, gated by bridgePidfileEnv);
// this probe reads it.
//
// NOT absolute ground truth — it is a better PROXY than stdout-flatness, with
// its own blind spots: an agent blocked on a slow API response mid-turn accrues
// little CPU (false-negative), and a wedged busy-loop reads alive (false-pos).
// It fails SAFE: a no-claim (false) leaves the caller's existing stall logic
// unchanged, and the first sighting grants one window. The true fix remains a
// bridge wall-clock heartbeat envelope independent of agent output.
```

### `go/internal/adapters/observer/events_golden_test.go:3` — above `import (`

```text
// events_golden_test.go — ADR-0103 unit 12 step 0, G4: the live adapter's
// events file (started / stall_no_output / stopped, its OWN lowercase
// envelope — observer.go:57-66, :249-269) byte-equal to a golden captured on
// 8e8f080f BEFORE core_adapter.go / observer.go were edited. The stall is
// scripted: no stdout growth, no workspace activity, a liveness probe that
// answers false, a clock that jumps past StallS from the third read on. Kills
// M20 (a severity uppercased), M21 (an Event field added or renamed).
```

### `go/internal/adapters/observer/observer.go:31` — above `DefaultStallS = 600 * time.Second`

```text
// DefaultStallS is the hard-kill threshold — after this many seconds of
// no stdout-log output the observer signals the runner to SIGTERM the
// subagent. ADR-0030; configured by ObserverPolicy.StallS.
```

### `go/internal/adapters/observer/observer.go:35` — above `DefaultNudgeS = 300 * time.Second`

```text
// DefaultNudgeS is the soft-stall nudge threshold (cycle-124 Task 6 /
// operator redirect): when an agent emits no fresh output for this many
// seconds, the observer appends ONE nudge envelope to the agent's inbox
// asking it to summarize state + continue OR finalize, BEFORE the hard
// SIGTERM at DefaultStallS. Default is half of DefaultStallS so the
// agent gets a clear "still alive?" prompt with enough time to recover.
// Opt-out: set ObserverPolicy.NudgeS=0. See ADR-0023 facet A.
```

### `go/internal/adapters/observer/observer.go:44` — above `observerEventsSuffix = observerengine.EventsSuffix`

```text
// observerEventsSuffix names the observer's own NDJSON sink file (the
// adapter writes <phase>-observer-events.ndjson into WorkspaceDir). It is
// excluded from the workspace-activity scan so the observer's own
// started/stall/stopped writes can never reset the stall timer and mask a
// genuine stall. Projected from the engine's one layout (ADR-0103 unit 12).
```

### `go/internal/adapters/observer/observer.go:62` — above `Severity string 'json:"severity"'`

```text
// info | warn | incident
```

### `go/internal/adapters/observer/observer.go:77` — above `WorkspaceDir string`

```text
// WorkspaceDir, when non-empty, makes a fresh write anywhere under it
// count as progress alongside stdout-log growth. This is load-bearing for
// tmux-driver agents (claude-tmux/codex-tmux/agy-tmux): their live output
// goes to the tmux scrollback and reaches the stdout-log only on clean
// exit, so the stdout-log stays flat while the agent is productively
// writing artifacts. Without this signal the observer falsely reports
// stall_no_output for a working tmux agent (cycle-141). Empty → stdout-log
// growth is the only progress signal (byte-identical to the pre-fix path).
```

### `go/internal/adapters/observer/observer.go:87` — above `LivenessProbe func() bool`

```text
// LivenessProbe, when non-nil, is consulted ONLY at the stall threshold,
// before a stall_no_output incident is emitted. It reports whether the
// agent is still alive by a signal the filesystem cannot see: for
// tmux-driver phases, whether the live tmux pane changed since the last
// check (the spinner / token-counter advancing during a long single
// "Incubating" turn that commits no scrollback lines and writes no
// artifact yet). On true, the observer treats the agent as alive — it
// resets the stall clock and emits a benign stall_probe_active info event
// instead of a false incident (cycle-190). The probe is consulted at most
// once per StallS window, so a subprocess-backed probe (tmux capture-pane)
// costs nothing on the common no-stall path. Nil → no probe; stdout-log +
// workspace growth govern alone (byte-identical to the pre-probe path).
```

### `go/internal/adapters/observer/observer.go:174` — above `if o.cfg.LivenessProbe != nil && o.cfg.LivenessProbe() {`

```text
// Before declaring a stall, consult the liveness probe (if any).
// A tmux agent mid-"Incubating" turn produces no filesystem
// growth but is alive (pane spinner advancing) — holding the
// kill here avoids the cycle-190 false-positive. The probe is
// only reached at the threshold, so it never runs on the common
// healthy path.
```

### `go/internal/adapters/observer/observer_test.go:102` — above `func TestWatch_StallEmitsIncident(t *testing.T) {`

```text
// TestWatch_StallEmitsIncident drives a deterministic stall: with a
// virtual clock advanced past StallS, Watch emits an "incident" event
// reporting stall_no_output. Mirrors bash phase-observer.sh:stall rule.
```

### `go/internal/adapters/observer/observer_test.go:169` — above `func TestWatch_LivenessProbeSuppressesFalseStall(t *testing.T) {`

```text
// TestWatch_LivenessProbeSuppressesFalseStall — cycle-190 regression: a
// tmux-driver agent in a long single "Incubating" turn (extended thinking +
// one big tool call) commits NO scrollback lines and writes NO workspace
// artifact for minutes, then dumps everything at the end. Both filesystem
// liveness signals (stdout-log size, workspace mtime) stay flat, so the
// observer falsely fires stall_no_output. When a LivenessProbe reports the
// agent is still alive (e.g. the tmux pane spinner/token-counter advancing),
// the observer must HOLD the stall: reset the clock and emit a benign
// stall_probe_active info event instead of a false incident.
```

### `go/internal/adapters/observer/observer_test.go:182` — above `if err := os.WriteFile(logFile, []byte("start"), 0o644); err != nil {`

```text
// Flat log, no growth, no WorkspaceDir activity — exactly the cycle-190
// think-then-dump window.
```

### `go/internal/adapters/observer/observer_test.go:257` — above `func TestWatch_WorkspaceActivityResetsStallTimer(t *testing.T) {`

```text
// TestWatch_WorkspaceActivityResetsStallTimer — cycle-141: a tmux-driver
// agent writes its live output to the tmux scrollback, NOT the stdout-log,
// so the stdout-log stays flat while the agent is productively writing
// artifacts (worktree commit, reflection.yaml) into the workspace tree. When
// WorkspaceDir is set, a fresh write anywhere under it counts as progress and
// resets the stall timer — so a working tmux agent is not falsely killed.
```

### `go/internal/adapters/observer/tmux_probe.go:3` — above `import (`

```text
// tmux_probe.go — cycle-190 fix: a concrete LivenessProbe for tmux-driver
// phases. The auto-spawn observer's filesystem signals (stdout-log size,
// workspace mtime) both go flat while a tmux agent is in a long single
// "Incubating" turn — extended thinking plus one large tool call that commits
// no scrollback lines and writes no artifact until the turn ends. The live
// tmux pane is the only liveness signal in that window (its spinner /
// token-counter advances every second). This probe reads it.
```

### `go/internal/adapters/observer/tmux_probe_runscope_test.go:1` — above `package observer`

```text
// tmux_probe_runscope_test.go — CB.6 contract (concurrency campaign W4):
// the observer's pane-liveness probe asserts RUN ownership before making a
// liveness claim. The probe's match grants stall-clock extensions; matching
// ANOTHER run's session would keep a dead agent's clock fresh forever (the
// cross-run variant of the cycles-254/255 false-liveness class). Fail-closed:
// a probe that knows its run id refuses any session without the run token.
```

### `go/internal/adapters/observer/tmux_probe_test.go:36` — above `func TestTmuxPaneProbe_PaneAnimatingIsAlive(t *testing.T) {`

```text
// TestTmuxPaneProbe_PaneAnimatingIsAlive — the cycle-190 case: a matching
// session whose pane content changes between checks (spinner / token counter
// advancing) reports alive. The first sighting also reports alive (grace
// window); an unchanged pane afterward reports not-alive (possibly hung).
```
