# Comment history: `internal/phasestream`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/phasestream/apicover_named_test.go:7` — above `func TestKindError_Defined(t *testing.T) {`

```text
// TestKindError_Defined names the KindError taxonomy member (envelope.go) and
// pins its wire token. KindError is part of the public ADR-0020 normalized-event
// Kind enum; downstream consumers (cycleclassify, the observer rules) filter on
// these exact string values, so the token is a contract — mirrors the existing
// TestKindCorrelation_Defined idiom in envelope_test.go.
```

### `go/internal/phasestream/classify.go:37` — above `injectedPrompt string`

```text
// injectedPrompt is the phase's own prompt/instruction text. When set,
// an infra-marker line that is a verbatim substring of it is a self-echo
// of the agent's OWN prompt (e.g. an Adversarial Reviewer's exploit
// checklist "...missing rate limits.") — not a runtime infra signal — and
// is NOT emitted as infra_failure (cycle-641/642 lesson). Empty = no
// suppression (every marker emits, the pre-fix behaviour).
```

### `go/internal/phasestream/classify.go:46` — above `func (c *Classifier) SetInjectedPrompt(prompt string) {`

```text
// SetInjectedPrompt threads the phase's prompt/instruction text into the
// Classifier so infra-marker lines that merely echo it are recognized as the
// agent quoting its own prompt and suppressed (cycle-641/642 fix-of-record). A
// no-op when prompt is empty. Not concurrency-safe — set once before feeding
// lines (the normalizer owns one Classifier per phase, single goroutine).
```

### `go/internal/phasestream/classify.go:77` — above `func (c *Classifier) Emit(kind Kind, sev Severity, data map[string]any) Envelope {`

```text
// Emit builds an envelope for a normalizer-originated event (e.g. a stall
// incident) using the same monotonic seq + source as line events, so the
// unified stream stays gap-free across both line- and rule-events.
```

### `go/internal/phasestream/classify.go:152` — above `func (c *Classifier) infraEnvelope(marker, source, line string) Envelope {`

```text
// infraEnvelope builds the canonical infra_failure envelope for a marker
// hit, tagging the channel it surfaced on ("stdout" | "stderr"). Shared by
// Stderr and classifyPlain so both channels carry an identical wire shape —
// cycleclassify filters kind==infra_failure regardless of source (ADR-0020).
```

### `go/internal/phasestream/classify.go:283` — above `func (c *Classifier) formatToolUse(blk assistantContent) Envelope {`

```text
// formatToolUse special-cases interactive tools BEFORE the generic
// 200-byte input clamp: AskUserQuestion / ExitPlanMode carry the
// question, options, recommendation, or plan — that IS the signal, so
// it must survive at full fidelity (ADR-0020).
```

### `go/internal/phasestream/classify.go:382` — above `if marker := detectInfraMarker(line); marker != "" && !c.isPromptEcho(line) {`

```text
// Plaintext infra markers (CLI error lines, tmux scrollback) surface as
// infra_failure, symmetric with Stderr — the legacy cycleclassify
// scanned *-stdout.log too (cycle-61 memo-stdout.log 529).
```

### `go/internal/phasestream/classify_test.go:21` — above `const realResultLine = '{"type":"result","subtype":"success","is_error":false,"duration_ms":177003,"num_turns":22,"total…`

```text
// Real claude-p result-event shape (cycle-107 build-stdout.log, redacted text).
```

### `go/internal/phasestream/classify_test.go:180` — above `func TestClassifier_Plaintext_InfraMarkerSurfaces(t *testing.T) {`

```text
// TestClassifier_Plaintext_InfraMarkerSurfaces guards the parity contract
// for cycleclassify's hard-switch to the events stream (ADR-0020 task 4):
// the legacy classifier scanned *-stdout.log too, so infra markers that
// land on STDOUT (the cycle-61 memo-stdout.log 529) must surface as
// infra_failure, symmetric with Stderr(). Without this, the events-only
// filter would silently miss stdout-borne infra.
```

### `go/internal/phasestream/envelope.go:1` — above `package phasestream`

```text
// Package phasestream is the single normalizer for evolve-loop phase
// output. It classifies raw CLI stdout/stderr (stream-json from
// claude-p/codex/agy, or plaintext scrollback from claude-tmux) into a
// unified envelope stream — one <agent>-events.ndjson per phase that
// every downstream consumer (cyclecost, the observer rules, and
// cycleclassify) reads instead of re-parsing raw logs.
//
// Design: ADR-0020. The raw .log is never touched; this package only
// produces the clean event stream.
```

### `go/internal/phasestream/envelope.go:25` — above `type Kind string`

```text
// Kind enumerates the normalized event taxonomy (ADR-0020).
```

### `go/internal/phasestream/envelope.go:41` — above `KindCorrelation Kind = "correlation"`

```text
// KindCorrelation Data keys (ADR-0037):
//   sub: "request"           + corr_id, at_seq
//   sub: "response_complete" + corr_id, start_seq, end_seq
//   sub: "response_timeout"  + corr_id, waited_s
```

### `go/internal/phasestream/marker.go:5` — above `type infraMarker struct {`

```text
// infraMarker is the system-wide infrastructure-failure vocabulary.
// Defined ONCE here (ADR-0020): the normalizer owns detection, and
// cycleclassify consumes the typed kind==infra_failure events rather
// than re-scanning raw text. Mirrors the markers cycleclassify's
// reInfrastructure regex caught (classify.go:57).
```

### `go/internal/phasestream/mask_test.go:8` — above `func TestMaskStaleObservations_MasksOldEvictablePreservesRest(t *testing.T) {`

```text
// TestMaskStaleObservations_MasksOldEvictablePreservesRest is the durable
// (normal-suite) unit test for the observation-masking transform. It pins the
// core contract the cycle-530 ACS predicates bind to and gives apicover an
// executed reference for MaskStaleObservations. It also confirms purity: the
// caller's input envelope Data is never mutated.
```

### `go/internal/phasestream/normalizer_test.go:126` — above `func TestNormalizer_PollEmitsInfraFailureFromStderr(t *testing.T) {`

```text
// 3. A stderr line carrying an infra marker becomes one infra_failure incident.
```

### `go/internal/phasestream/normalizer_test.go:153` — above `func TestNormalizer_PollEmitsStallOnceAfterThreshold(t *testing.T) {`

```text
// 4. With no output, once the clock crosses StallS a single stall incident fires; it does not re-fire.
```

### `go/internal/phasestream/produce.go:20` — above `InjectedPrompt string`

```text
// InjectedPrompt is the phase's composed prompt text. When set, the
// Classifier suppresses infra-marker lines that are a verbatim echo of it
// (cycle-641/642 lesson; wired cycle-672). Empty = no suppression.
```

### `go/internal/phasestream/produce.go:31` — above `func Produce(cfg ProduceConfig) error {`

```text
// Produce reads a finished phase's complete raw <phase>-stdout.log (and
// optional <phase>-stderr.log) and writes the unified <phase>-events.ndjson
// through the same Classifier the live Normalizer uses (ADR-0020). This is the
// post-phase path for the synchronous-bridge runner, where the whole log
// exists at once.
//
// Unlike the live Poll tail — which deliberately holds back an unterminated
// final line to avoid splitting a still-being-written record — Produce reads
// each log in full, so the billing-critical trailing `result` event is never
// dropped. The write is atomic (temp + rename) so the post-cycle readers
// (cyclecost, cycleclassify) never observe a half-written events file, and the
// temp never pollutes their *-events.ndjson glob.
//
// A missing stdout log is a no-op (returns nil): the phase produced no output.
```

### `go/internal/phasestream/prompt_echo_c654_test.go:14` — above `func TestClassifier_SetInjectedPrompt(t *testing.T) {`

```text
// Preserve both the API contract and cycle-654's infra-classifier-echo-veto
// incident input (the cycle-641/642 reviewer-checklist regression). The direct
// SetInjectedPrompt call also binds its exported identifier for apicover.
```

### `go/internal/phasestream/prompt_echo_wiring_c672_test.go:3` — above `import (`

```text
// RED wiring test for cycle-672 top_n task `echo-veto-wiring-completion`
// (third cycle on this defect class: 654 landed the leaf helpers, 656's
// wiring attempt was quota-killed, 672 completes the consumption wiring).
//
// TestClassifier_SetInjectedPrompt proves the echo veto works when called
// directly — but grep shows ZERO production call sites: Produce() constructs
// the Classifier (produce.go) and never threads the phase prompt in, so the
// live emit path still classifies echoed prompt text as infra_failure
// (cycle-656 retro D3: a 100%-echoed pane classified rate_limit).
//
// This test pins the WIRING: ProduceConfig must carry the injected prompt and
// Produce must hand it to the Classifier before classifying. RED today:
// ProduceConfig has no InjectedPrompt field — compile failure. DO NOT MODIFY;
// Builder adds the field + the SetInjectedPrompt call to make it GREEN.
```
