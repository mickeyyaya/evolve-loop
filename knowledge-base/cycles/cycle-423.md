# Cycle 423 Dossier

**Goal:** Reduce evolve-loop cycle latency by making phase-agent liveness detection more accurate, designed as a CLEAN ABSTRACT LAYER on the agent bridge so each LLM CLI can detect liveness DIFFERENTLY behind one interface — driver-agnostic, no per-CLI branching in the consumer. Follow TDD (red→green→refactor). No regression of any wedge-incident safeguard (cycles 254/255, 262, 274/277, 286/288, 311/312 are hard-won; changes additive and test-pinned).

DESIGN (Strategy + Dependency Inversion, single-source-with-projection):
  1. Define a driver-agnostic LivenessDetector interface (the abstract layer) — e.g. Probe(paneFrames) → LivenessState{Converging | BusyButStagnant | Idle | Hung} (+ confidence). The stop-review reviewer (internal/bridge/stopreview.go, driver_tmux_repl.go) depends ONLY on this interface; it must contain ZERO CLI-specific parsing or if/switch-on-CLI. Selecting the per-CLI detector goes through the EXISTING per-driver registry/projection (PaneProfile + panestream.ClassifyLine channel separator, ADR-0047) — adding/retuning a CLI is a new strategy + profile entry, never an edit to the reviewer.
  2. Per-CLI concrete strategies (each may detect differently):
     - A DEFAULT strategy usable by ANY CLI out of the box: stable-content GROWTH VELOCITY from panestream.PaneDelta (sustained new stable content = Converging → extend like Progressed/unconditional; growth stalled + busy-only for N intervals = Hung → fast-fail before the maxExtends×interval backstop). This alone must give codex (no busy affordance today) accurate liveness — closing its weak-signal degradation.
     - claude strategy MAY layer its monotonic `↓ Nk tokens` counter as a higher-confidence Converging signal; ollama MAY use its token stream / "Thinking…" header; agy its spinner. Each is its OWN strategy implementation, composed over (not replacing) the default — an enhancement, never a requirement.
  3. Today extractTokenCount is recorded for reporting but unused in the decision, and the reviewer extends on coarse Progressed/Busy bounded by maxExtends=6 over interval=300s (up to 30 min on a hung-but-busy agent). The new interface replaces that coarse path uniformly.

TDD + PARITY (mandatory):
  - Write failing tests FIRST: per-CLI LivenessDetector tests over panestream testdata frames (thinking vs answer per CLI) asserting the right LivenessState; a CLI×state matrix test proving every driver (claude/codex/agy/ollama) routes through a strategy and none is left on the old coarse path; regression tests pinning each prior incident invariant. Then implement to green; refactor.
  - Driver-agnostic per [[any_cli_any_phase_any_model_invariant]] and [[driver_agnostic_model_routing]]; design-pattern not flags per [[no_feature_flags_use_design_patterns]]; single-source-with-projection per [[never_duplicate_centralize_via_design_patterns]].

SECONDARY latency levers (general, validate — do not flip blindly):
  - Bench ANY driver that repeatedly hits ExitREPLBootTimeout (exit 80 = 60s wasted boot) for the rest of the batch so its pinned phases skip the dead driver — keyed on the driver-agnostic exit code, not codex by name (~15 wasted boots in one 4-cycle run).
  - Evaluate ParallelEvaluate=enforce (currently StageOff, config.go:463) to run independent post-build checks concurrently (~11%/cycle). Soak-gated — validate via shadow/soak before enforce.

CONSTRAINTS: keep every invariant green (Progressed extends unconditionally; a bare spinner buys extensions not immortality; fatal-pane fast-fail preempts; ticking-clock hole stays closed). Incident-prone code — smallest correct change, clean code, ultrathink the design, strong review, behavioral tests per CLI, zero per-CLI branching in the reviewer.
**Final verdict:** PASS
**Run ID:** 01KWBAX8KMF5AQ4JD0H923S9ZB

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | PASS |  | cycle completed; ledger walk deferred to future slice |
