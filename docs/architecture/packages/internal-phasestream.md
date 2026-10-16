# internal/phasestream

## Purpose

`internal/phasestream` turns one phase's raw CLI output (stream-json from claude-p, codex or agy, or tmux scrollback) into one unified envelope stream, `<phase>-events.ndjson`. Every downstream reader (cost, observer rules, cycle classification) reads that stream instead of re-parsing raw logs. The raw log is never modified. Design: ADR-0020.

## Design

- `Classifier` turns one line into zero or more envelopes. It keeps the per-phase state: the coalesced progress counter, the monotonic `seq`, and the correlation tracker.
- `Normalizer` tails the stdout and stderr logs of a live phase, runs the classifier, applies the stall rule and writes to a single-writer sink.
- `Produce` is the post-phase path for the synchronous bridge, where the whole log exists at once. It reads each log in full and writes the file atomically.
- `MaskStaleObservations` is a pure transform for the context budget. It keeps the newest N tool observations and masks older bulky ones.
- `marker.go` owns the infrastructure-failure vocabulary. The normalizer detects it, and consumers read the typed `infra_failure` events.

## Invariants

- One `Classifier` and one `Normalizer` per phase, driven from a single goroutine. Neither is safe for concurrent use.
- Call `Classifier.SetInjectedPrompt` one time, before the first line is fed. An empty prompt turns off the prompt-echo filter. The setter has no lock, so a call during feeding is a data race.
- Infrastructure markers are tried in declared order and the first match wins. Specific markers come before broad ones: `api_429` comes before `rate_limit`. The `api_529` marker matches both `529` and `overloaded`. Pinned by `TestDetectInfraMarker_SpecificMarkerBeatsBroadOne`.
- An infrastructure-marker line that is a verbatim (trimmed) substring of the phase's own injected prompt is the agent quoting its prompt, not a runtime failure. It is suppressed. An empty injected prompt disables this. The reason is cycle 641/642: an adversarial reviewer's checklist quoted "rate limits" and was read as a 429.
- `stream_event` deltas are never dropped. They are coalesced into one progress envelope per poll, so the observer keeps a liveness signal.
- Interactive tools (`AskUserQuestion`, `ExitPlanMode`) bypass the 200-byte input clamp. Their payload is the signal.
- A line that looks like JSON but does not parse, or has no `type`, falls back to plaintext. No line is lost silently.
- The live tail holds back an unterminated final line until its newline arrives. A shrunk file (truncation or rotation) restarts from offset zero.
- The post-phase `Produce` reads each log in full, so the trailing `result` event, which carries billing, is never held back. Its write is temp file plus rename. The temp name is dot-prefixed, so the consumers' `*-events.ndjson` glob never matches it. A line is capped at 16 MiB, because a result envelope can embed a large payload.
- A missing stdout log is a no-op: the phase produced no output.
- The stall rule fires at most once per stall window, and any activity re-arms it. The outcome of a kill is recorded on the envelope (`kill_result`), so a failed enforcement (bad pgid, permissions) is visible.
- A correlation breadcrumb whose `corr_id` has no open inject (duplicate or out of order) is ignored.
- `MaskStaleObservations` never mutates its input. With `windowTurns <= 0` it returns an identical copy, so a new knob is off by default. Non-evictable kinds (verdicts, errors, thinking, interactions, correlation) are never masked.

## Findings

- The correlation Data keys are a wire contract. `sub` is `request`, `response_complete` or `response_timeout`, and each form carries its own keys.
