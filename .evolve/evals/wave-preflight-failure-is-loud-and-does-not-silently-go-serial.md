---
score_cap:
  - criterion: "A pool preflight refusal emits a WARN loop.wave signal (LOOP_WAVE_DISPATCH_FAILED, path=pool, step=preflight) naming the reason"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestPoolPreflight_UncommittedControlPlaneFileEmitsAWarnSignalNamingTheReason$' ./cmd/evolve"
  - criterion: "wave status names the pool preflight failure reason as a WARN line and a JSON warning"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestWaveStatus_NamesThePoolPreflightFailureReason|TestWaveStatus_JSONCarriesThePoolPreflightFailureReasonAsAWarning)$' ./cmd/evolve"
  - criterion: "the errors channel carries the signal so events watch wakes on it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPoolPreflight_TheErrorsChannelCarriesTheSignalAndEventsWatchWakesOnIt$' ./cmd/evolve"
---

# Eval: A failed pool preflight is loud

> A dirty control-plane file at pool start made the pool fall back to one sequential cycle with only a console line, so the console and the wave status never saw why the fleet went serial. This eval pins the typed WARN signal, its reason in wave status, and the wake of the events watch. Source: cycle 2171, inbox item wave-preflight-failure-is-loud-and-does-not-silently-go-serial.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| signal-emitted | WARN signal with the reason | 5/10 | `go test -run TestPoolPreflight_UncommittedControlPlaneFileEmitsAWarnSignalNamingTheReason ./cmd/evolve` |
| status-names-reason | wave status shows the reason | 6/10 | `go test -run TestWaveStatus_NamesThePoolPreflightFailureReason ./cmd/evolve` |
| watch-wakes | errors channel wakes events watch | 6/10 | `go test -run TestPoolPreflight_TheErrorsChannelCarriesTheSignalAndEventsWatchWakesOnIt ./cmd/evolve` |
