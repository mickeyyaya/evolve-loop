# Comment history: `internal/cycleclassify`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/cycleclassify/classify.go:1` — above `package cycleclassify`

```text
// Package cycleclassify ports classify_cycle_failure from
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:548-637. Given a
// cycle workspace it inspects orchestrator-report.md plus the unified
// *-events.ndjson stream (ADR-0020) and returns one of six canonical
// classifications. The result feeds the failure-adapter (M3) and
// drives the cmd_loop dispatcher policy decision (RETRY vs STOP).
//
// The infrastructure signal that the legacy classifier found by re-scanning
// raw *-stdout.log/*-stderr.log now comes from the normalizer's
// kind==infra_failure events — one owner of the infra-marker vocabulary.
//
// Determinism: classification is order-sensitive. Infrastructure beats
// ship-gate-config beats audit-fail beats build-fail. The order
// matters because a ship-gate-deny report can mention the audit verdict
// in passing, and we want the more specific (and lower-severity)
// ship-gate-config label rather than the broad audit-fail label.
```

### `go/internal/cycleclassify/classify.go:62` — above `ClassShipGateConfig Classification = "ship-gate-config"`

```text
// ClassShipGateConfig — audit declared PASS but ship-gate refused
// (v8.27.0). Distinct from audit-fail because the audit itself
// succeeded; the rejection is in the post-audit gate config/logic.
```

### `go/internal/cycleclassify/classify.go:120` — above `func Classify(workspace string) Result {`

```text
// Classify scans the cycle workspace and returns the resolved
// classification. The workspace path is .evolve/runs/cycle-<N>/ —
// caller is responsible for constructing it.
//
// Pass order (the ordering contract, stated once): 0b's record is read first
// so that 0 can be recency-aware — (0) the stopping phase's own classed FAIL
// sentinel; (0b) a coded FAIL on the LAST outcome of the C1 record
// (phase-timing.json) ⇒ phase-refusal; (1) infrastructure in the
// orchestrator report; (2) infrastructure in the typed events stream; (3) the
// post-audit ship gate; (4) the audit verdict; (5) build never GREEN; (6) the
// hang reclassifier; then the fallbacks below. Structured passes precede
// prose; a sentinel from an EARLIER phase than the record's last outcome is
// stale and yields to 0b.
//
// Scanning order per cycle:
//
//  1. orchestrator-report.md — infra patterns
//  2. *-events.ndjson — kind==infra_failure (the normalizer's typed infra
//     signal; catches the API 529s that landed in memo-stdout.log per
//     cycle-61 forensics, now sourced from the clean stream)
//  3. orchestrator-report.md — ship-gate, audit-fail, build-fail
//
// Returns ClassIntegrityBreach with empty Marker/Source when
// orchestrator-report.md is missing, OR when it exists but no pattern
// hits.
```

### `go/internal/cycleclassify/classify.go:153` — above `record, recordOK := classifyFromRecord(workspace)`

```text
// Pass 0 (ADR-0039 §7): a phase that self-reported a structured failure
// class (sentinel v2) is the authority on WHY it failed — the regex
// passes below are heuristics over prose. Only classes that normalize
// into the canonical taxonomy are trusted; an out-of-taxonomy agent
// string falls through to the regex passes (never UnknownClassification,
// never blind trust).
// Pass 0b is read first because pass 0 must be recency-aware: the record
// (phase-timing.json, the C1 chokepoint's own ledger) names the phase the
// cycle STOPPED on; a classed FAIL sentinel from an earlier phase (a scout
// that wedged, retried and passed) is stale evidence once a later gate
// refused the cycle with a code. A sentinel from the stopping phase itself
// keeps its authority (the phase's own class beats the gate's code).
```

### `go/internal/cycleclassify/classify.go:183` — above `if src, marker, ok := scanEventsForInfra(workspace); ok {`

```text
// Pass 2: infrastructure in the unified events stream (ADR-0020). The
// normalizer owns the infra-marker vocabulary and emits
// kind==infra_failure for markers it sees on stdout OR stderr, so
// cycleclassify just filters that kind rather than re-scanning the raw
// *-stdout.log/*-stderr.log files. Catches the API 529 / sandbox EPERM
// signal (cycle-61 forensics) from the clean stream.
//
// Note: a transient stream-json rate_limit_event normalizes to
// kind==rate_limit (non-fatal backoff), deliberately distinct from
// kind==infra_failure — a recovered rate-limit no longer forces an
// infrastructure verdict on the retry/stop decision.
```

### `go/internal/cycleclassify/classify.go:426` — above `type infraEventEnvelope struct {`

```text
// infraEventEnvelope is the subset of a phasestream envelope this scan
// needs: the kind discriminator, the emitting phase, plus the marker and the
// raw excerpt the normalizer recorded. Excerpt + phase drive the cycle-641/642
// prompt-echo veto (isPromptEchoSelfReport).
```

### `go/internal/cycleclassify/classify.go:461` — above `func isPromptEchoSelfReport(workspace, phase, excerpt string) bool {`

```text
// isPromptEchoSelfReport reports whether an infra_failure event for phase is a
// self-echo of that phase's OWN prompt on an otherwise-successful phase, and so
// must NOT drive an infrastructure verdict (cycle-641/642 fix-of-record; retro
// recommendation #2 — deliverable-PASS + clean-exit are source-of-truth, a bare
// keyword echo cannot override them). All three must hold:
//
//  1. the event excerpt is a verbatim substring of <phase>-prompt.txt — the
//     agent quoting its own instruction text (e.g. an Adversarial Reviewer's
//     exploit checklist "...missing rate limits."), not a runtime banner;
//  2. the phase's deliverable <phase>-report.md carries a PASS verdict sentinel;
//  3. the phase's driver exited 0 in llm-calls.ndjson.
//
// Any missing/unreadable artifact fails the check CLOSED (no veto), so a genuine
// runtime infra signal — non-zero exit, or an excerpt absent from the prompt —
// still classifies as infrastructure. An empty phase or excerpt never vetoes.
```

### `go/internal/cycleclassify/classify_test.go:107` — above `ws := writeReport(t, "## Verdict\nNo errors detected")`

```text
// Report is clean; the events stream carries a stdout-borne 429. Per
// cycle-61 forensics the classifier must catch infra on stdout — now
// sourced from the unified events stream, not raw *-stdout.log.
```

### `go/internal/cycleclassify/classify_test.go:172` — above `func TestClassify_ParityViaProduce(t *testing.T) {`

```text
// TestClassify_ParityViaProduce is the ADR-0020 cutover gate for failure
// classification: a raw <phase>-stderr.log written by the bridge, run through
// the actual production path (phasestream.Produce as runner.go calls it),
// yields an events.ndjson from which cycleclassify recovers ClassInfrastructure
// — the end-to-end parity guarantee for the no-runtime-fallback collapse.
```

### `go/internal/cycleclassify/classify_test.go:316` — above `func TestClassify_Pass0_StructuredClassBeatsRegex(t *testing.T) {`

```text
// --- ADR-0039 §7 item 6: Pass 0 — structured sentinel beats regex guess ---
```

### `go/internal/cycleclassify/echo_veto_c654_test.go:3` — above `import (`

```text
// RED regression tests for cycle-654 top_n task `infra-classifier-echo-veto`
// (the fix-of-record for lesson cycle-641-infra-incident-classifier-matches-
// echoed-prompt-keywords, byte-for-byte recurred in cycle-642). These encode the
// SOURCE-OF-TRUTH gate on cycle discard (retro recommendation #2): when a phase's
// driver exited 0 AND its deliverable sentinel is PASS AND the only infra_failure
// event's excerpt is a verbatim echo of the injected prompt text, cycleclassify
// MUST NOT return ClassInfrastructure. The paired negative test proves the fix is
// not a blanket disable of infra detection: a genuine runtime signal (non-zero
// exit, excerpt NOT in the prompt) MUST still classify as infrastructure.
//
// Both exercise the real system under test — cycleclassify.Classify over an
// on-disk cycle-642-shape workspace — and assert on its returned Classification.
// Neither is a source-grep. TestC654_001 is RED today (Classify's Pass 2
// scanEventsForInfra returns ClassInfrastructure on the keyword-only event,
// ignoring the PASS deliverable + exit-0 + prompt-echo). TestC654_002 is a
// GREEN-now regression guard (genuine infra already vetoes) that the fix must
// keep green.
//
// Ported verbatim (renamed C653→C654) from the fix-of-record RED suite preserved
// in .evolve/worktrees/cycle-21f9f7ae-653; do not author duplicate C653 copies.
```

### `go/internal/cycleclassify/echo_veto_c654_test.go:30` — above `const echoedReviewerLine = "unbounded allocation or recursion; TOCTOU / race windows; missing rate limits."`

```text
// echoedReviewerLine is verbatim adversarial-review-prompt.txt:46 — the
// Reviewer's own exploit checklist the tmux driver echoes into the pane/stderr,
// which the normalizer keyword-matched as marker:"rate_limit" in cycle-641/642.
```

### `go/internal/cycleclassify/echo_veto_c654_test.go:88` — above `func TestC654_001_PassDeliverableExit0EchoNotInfraVeto(t *testing.T) {`

```text
// TestC654_001_PassDeliverableExit0EchoNotInfraVeto — AC1 (source-of-truth gate).
// A cycle-642-shape workspace: adversarial-review driver exited 0, its deliverable
// carries a PASS sentinel, and the only infra_failure event's excerpt is a verbatim
// echo of the injected prompt. Classify MUST NOT veto this to infrastructure.
// RED today: scanEventsForInfra returns ClassInfrastructure on the keyword alone.
```

### `go/internal/cycleclassify/empty_output_test.go:88` — above `ws := t.TempDir()`

```text
// The cycle-120 signature: a mid-cycle quota abort never writes an
// orchestrator-report.md. The pass must still recover, from per-phase
// artifacts alone, instead of short-circuiting to breach.
```

### `go/internal/cycleclassify/hang_coverage_test.go:58` — above `dir := t.TempDir()`

```text
// Initialize a temp git repo with one commit whose message
// mentions "cycle 42".
```

### `go/internal/cycleclassify/refusal_test.go:3` — above `import (`

```text
// refusal_test.go — the C1-record pass: a cycle whose LAST recorded phase
// outcome is a FAIL carrying a diagnostic Code is a phase-refusal — the phase's
// own deterministic gate stopped the cycle (triage refusing a protected-surface
// card), which is task-attributable and re-occurs on every retry. Read from
// phase-timing.json, never from prose; ranked after the agent's sentinel class
// (pass 0) and before the regex passes, because a coded refusal IS the reason
// the cycle stopped (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).
```
