# Quota-detection regex drift

**Period:** 2026-05-27 → 2026-07-26 (class history; the two July storms 07-17 and 07-23→26) · **Status:** shipped (anchor-hardening + cross-CLI parity queued)
**Primary artifacts:** commits `9c17dc6a` (#328), `bbd275eb` (#332), `1b8e9c35` · `go/internal/bridge/manifests/claude-tmux.json` · `go/internal/bridge/manifest_exhaustion_wording_test.go` · CHANGELOG v22.4.0, v22.8.0

## Problem

The loop detects a provider quota wall by matching the tmux pane against
`usage.exhausted_regex` in the CLI's bridge manifest. When the wall is detected, the phase
fast-fails **exit 85** and the always-on failover re-dispatches on another model/family. When the
wall is *missed*, the phase waits out the artifact timeout and dies **exit 81**, the self-heal
relaunches into the same exhausted quota, and the loop livelocks against a dead provider. The
regex is a contract with wording **Anthropic can change in any CLI release** — and it drifted
repeatedly:

- **Cycles 904–911 (2026-07-17):** Claude Code v2.1.212 introduced a per-model wall — "You've
  reached your Fable 5 limit. Run /usage-credits …" — which the legacy
  `reached your (usage|weekly) limit` regex missed. 8 audit cycles died as exit-81 artifact
  timeouts, ~20 min each, zero ships: "the root cause of the post-v22.3.0 no-ship streak"
  (commit `9c17dc6a`).
- **Cycles 1077–1096 (2026-07-23, batch-11):** the weekly wall drifted from "reached" to "hit"
  ("You've hit your weekly limit · resets Jul 26 at 9pm") and ended batch-11 in a **19-cycle
  exit-81 livelock, ~40 min burn each** (commit `1b8e9c35`; the real wall string is preserved at
  runtime `.evolve/runs/cycle-1096/tmux-final-scrollback.txt:93`).

The commit subject calls 1077–1096 the "5th-gen drift"; the fixture comment in
`manifest_exhaustion_wording_test.go` counts it as "the FOURTH wording-drift instance of this
class" — the generation count depends on where you start counting, but both sources agree the
class is serial.

## Context & evidence

The class has two opposite failure modes, and the repo hit both before July:

- **False-negative (miss the wall):** the two July storms above. Chain, spelled out in the #328
  message: pane never classifies `LivenessExhausted` → the always-on exit-85 failover in
  `driver_tmux_repl.go` is bypassed → 600s artifact-wait → exit 81 → self-heal relaunch → same
  exhausted quota → repeat.
- **False-positive (see a wall that isn't there):** far worse per incident, because it kills a
  *working* agent. `bd3a6bfc` (2026-05-27) tightened the rate-limit auto-respond regex after a
  false-positive escalation; `bbd275eb` (#332) records the "cardinal false-FAIL" cycles
  254/255/314/641 — wall-shaped text that a healthy agent momentarily renders (a cat/grep/diff
  quoting a provider's limit message) crossing the detector.
- **Why the synthetic-fixture era hid the drift:** "The prior exhaustion test used a SYNTHETIC
  'reached your usage limit' fixture that matched the old regex and passed — false confidence that
  let the upstream wording drift go uncaught" (`manifest_exhaustion_wording_test.go`, header
  comment). The 904–911 fixtures are the actual strings captured in the cycles' own
  `audit-escalation-report.json` final panes.

## Approaches considered

- **Broaden the regex until it can't miss.** Rejected — the false-positive asymmetry forbids it.
  #328's landed shape is deliberately narrow: a companion-anchored branch requiring the CLI's
  second-person chrome AND the `/usage-credits` companion "so ordinary prose about limits cannot
  false-fail a working agent," hardened over two rounds of adversarial go-review (`9c17dc6a`).
  The 5th-gen fix had to *relax* one anchor with evidence: the weekly wall renders its
  `/usage-credits` companion on the NEXT pane line, so the weekly branch cannot require same-line
  adjacency (fixture comment, `manifest_exhaustion_wording_test.go`).
- **Fire on one observation.** Rejected in #332: the persistence gate requires the wall to persist
  for 2 consecutive observations (fast-poll and checkpoint both) before exit-85, so momentary
  wall-shaped text never kills an agent while "a re-printing real wall still crosses."
- **Detect drift instead of chasing wording.** Shipped as a *diagnostic* in #332: a broad
  `drift_probe_regex` at exit-81 teardown emits a loud `POSSIBLE-EXHAUSTION-REGEX-DRIFT` line when
  a pane looks quota-walled but `exhausted_regex` missed it — "so the next wording drift is caught
  in one cycle, not eight. Never drives the verdict" (`bbd275eb`). See Results for how that
  worked out.
- **Fix the class, not the wording.** Queued rather than shipped, explicitly: strip the raw pane
  before the checkpoint exhaustion check (`exhaustion-checkpoint-raw-pane-stripping`, filed by
  #328), regex anchor-hardening + codex/agy manifest parity (filed by `1b8e9c35`). The durable
  end-state — not depending on provider prose at all — has no shipped mechanism yet.

## Decision & reasoning

Three standing rules came out of the two storms, each traceable to a specific cost:

1. **False-positive ≫ false-negative for quota detection.** A missed wall costs a batch of
   exit-81 timeouts; a false wall discards a healthy agent's work — the cardinal false-FAIL class.
   So the regex stays narrow and anchored, the persistence gate stays at 2 observations, and every
   widening must carry a real capture as evidence.
2. **Validate against REAL pane captures, never synthetic fixtures.** The synthetic fixture passed
   while production burned. Both July fixes pin the exact captured strings (cycles 904/905/908–911
   escalation reports; cycle-1096 scrollback) as regression fixtures.
3. **Fixtures must be concatenation-split.** Discovered while writing those very fixtures: an
   agent that `Read`s the test file gets a cat-n rendering (digit+tab prefix, NOT a diff line)
   which `stripAgentDiffLines` does not strip — "a verbatim fixture would then match the live
   exhausted_regex through the persistence gate … and exit-85 a healthy session reviewing this
   very file." Every wall fixture is built as `"li" + "mit"` so the literal wall text never
   appears on one source line (`manifest_exhaustion_wording_test.go`, SELF-TRIGGER GUARD comment).
   The detector's own test suite was a false-positive vector.

## Implementation

- **v22.4.0 (2026-07-18):** `9c17dc6a` (#328) — per-model wall branch in
  `go/internal/bridge/manifests/claude-tmux.json` `exhausted_regex`; real-data regression test;
  synthetic e2e fixtures replaced with real strings. `bbd275eb` (#332) — persistence gate
  (2 consecutive observations, both poll paths; checkpoint liveness Observe decoupled from the
  exhaustion decision) + `drift_probe_regex` fail-loud alarm; `exhaustion_gate.go` renamed
  `exhaustion_persistence.go`.
- **v22.8.0 (2026-07-28):** `1b8e9c35` — `(?:reached|hit) your (usage|weekly) limit` widening;
  weekly branch without same-line companion anchoring; concatenation-split fixture suite pinning
  the cycle-1096 capture plus legacy-parity wordings; follow-ups queued
  (`exhaustion-regex-anchor-hardening`, `untrack-goevolve-binary` — both visible in the commit's
  inbox diff).

## Results (measured)

- **904–911 class closed:** the per-model wall now classifies `LivenessExhausted` and takes the
  exit-85 failover; the 8-cycle/zero-ship streak ended with v22.4.0 (commit `9c17dc6a`; CHANGELOG
  v22.4.0).
- **1077–1096 class closed:** the "hit your … limit" wording matches; the 19-cycle livelock shape
  is regression-pinned against the real capture (commit `1b8e9c35`; CHANGELOG v22.8.0).
- **The drift alarm did not save batch-11.** The honest negative result: the pre-fix
  `drift_probe_regex` (at `1b8e9c35^`) contains the literal alternative `weekly limit`, so the
  probe *would* match the cycle-1096 wall — yet 19 cycles burned anyway. The alarm is
  diagnostic-only by design ("Never drives the verdict"), a stderr line at teardown with no
  automated consumer, and an unattended overnight batch reads no stderr. The one-cycle-not-eight
  promise assumed a reader.
- **No cardinal false-FAIL recurrence** of the wall-quoting class is recorded in the July
  operations docs after the persistence gate landed; the fixture self-trigger guard closed the
  last known in-repo vector.

## Retrospective — what we learned

- **A regex against provider prose is a treadmill, and each missed step costs a batch.** Two
  storms, 27 burned cycles, ~14 hours of provider-facing burn between them. The class-level fixes
  (pane stripping before the exhaustion check, anchor hardening, cross-CLI parity) are queued but
  the treadmill is still the mechanism.
- **Fail-loud without an actuator is fail-quiet.** The drift alarm matched and changed nothing.
  A detector whose output nobody consumes has the same operational value as no detector; wiring an
  alarm into a halt/failover decision is a different (and riskier) design that was consciously not
  taken — but the gap between "we will see it in one cycle" and "the loop will act on it in one
  cycle" was not named at ship time.
- **Your test suite runs inside the thing it tests.** The concatenation-split lesson generalizes:
  in a harness that watches its own agents' panes, any verbatim reproduction of a trigger string —
  in fixtures, docs, or a chronicle entry like this one — is a live input. (Every wall string
  quoted in this entry is paraphrased or truncated for exactly that reason.)
- **Real captures or it didn't happen.** The synthetic-fixture false confidence is the same
  disease as trusting a recorded verdict over on-disk artifacts
  ([false-FAIL storm entry](2026-07-false-fail-storm.md)): a validation surface decoupled from
  production reality validates nothing.

## Links

- Commits: `bd3a6bfc` (2026-05-27 rate-limit tighten) · `9c17dc6a` #328 · `bbd275eb` #332 · `1b8e9c35`
- [CHANGELOG.md](../../CHANGELOG.md) v22.4.0, v22.8.0
- Test/manifest: `go/internal/bridge/manifest_exhaustion_wording_test.go` · `go/internal/bridge/manifests/claude-tmux.json`
- Runtime evidence (read-only): `.evolve/runs/cycle-1096/tmux-final-scrollback.txt` (line 93)
- Sibling entries: [False-FAIL storm](2026-07-false-fail-storm.md) (the false-positive cost model) ·
  [Fingerprint identity](2026-07-fingerprint-identity.md) (exit-81 storms as a breaker input class) ·
  [LLM output stability](2026-07-llm-output-stability.md)
