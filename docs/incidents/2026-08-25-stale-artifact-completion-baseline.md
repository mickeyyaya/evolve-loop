# 2026-08-25 — Completion detector certifies the prior attempt's stale artifact (cycle-1550 family)

## Symptom

A correction re-dispatch whose prior failed attempt left its report at the
canonical artifact path had those UNCHANGED bytes certified as completion:
the fresh detector saw the file, watched it "stay stable" for two ticks
(nothing was writing it — of course it was stable), and completed. The prior
attempt's verdict re-graded itself on every retry — no post-dispatch write
required. Cycle-1550's audit FAILed on exactly this shape's red-first pin
(`TestArtifactDetector_PreExistingArtifactRequiresPostDispatchWrite`, authored
by the wave-2 cycle-1554 lane and salvaged from its ADR-0076 continuation
snapshot; the lane exhausted its correction budget before greening it).

## Root cause

`artifactDetector`'s stability window starts on FIRST SIGHT of any
located-and-stable file. It had no concept of "what was already on disk
before this dispatch's prompt went out," so pre-dispatch leftovers and
post-dispatch deliverables were indistinguishable. The finality short-circuit
("artifact on disk at the buzzer is the evidence") compounded it: even a
timeout run would complete on the stale bytes.

## Fix

A PRE-DISPATCH BASELINE (`artifactBaseline`: path+size+mtime — the same key
the stability window uses) captured inside `runTmuxREPL` BEFORE prompt
delivery (captured later, an instant-writing agent's fresh artifact could be
mistaken for leftovers) and threaded into the detector via
`Deps.CaptureBaseline` (nil → real capture; `withDefaults` wires it, pinned):

- an observation identical to the baseline never begins a stability window;
- the finality concession stops at the baseline — byte-identical leftovers do
  NOT complete at the buzzer; timeout is the honest outcome for an agent that
  wrote nothing (an artifact the agent DID rewrite keeps the concession);
- size alone counts as post-dispatch evidence (coarse-mtime filesystems);
- capture errors degrade to an absent baseline (fail-open, never a new
  refusal class).

The baseline snapshots the WHOLE artifactCandidatePaths set (canonical +
fallbacks, single-sourced with artifactLocate) — a stray at a fallback,
shadowed by the canonical at capture time, must not certify later when the
canonical vanishes mid-session (design-review note 1, closed in this change).

Known accepted costs, both chosen over re-graded stale verdicts: (1) an
attempt that finished writing just after its buzzer leaves a COMPLETE
artifact the next attempt's baseline refuses — the agent redoes the work;
(2) a stale canonical that shadows fresh work the agent wrote ONLY at a
fallback path times out despite real work — strictly better than pre-fix,
which completed the STALE file in that shape, and rung-1 salvage/correction
owns the recovery.

Test harnesses whose fake sessions cannot write files pre-seed the artifact
as a stand-in for a mid-session write; they declare that intent with
`zeroBaselineCapture` at their Deps assembly — production never sets the hook.

## The second door (go-review CRITICAL, closed in the same change)

Refusing completion alone did NOT close the class end-to-end: the runner's
reconcile-on-teardown (`internal/phases/runner/runner.go`) independently
re-reads the canonical artifact after `ErrArtifactTimeout` and trusts any
well-formed deliverable — and a prior attempt's leftover is well-formed AND
carries the cycle-scoped challenge token (minted once per cycle), so both the
well-formed fall-through and the ACS deterministic floor would have
resurrected the stale verdict five minutes later. The runner now takes its own
PRE-DISPATCH snapshot of the canonical artifact (`statArtifactSnapshot`, the
same size+mtime key) and refuses BOTH reconcile doors for anything still
byte-identical to it, with the refusal cause in the FAIL diagnostics. The
cycle-254/255 contract is untouched: a deliverable written DURING the session
still reconciles.

## Regression pins

`internal/bridge/completion_baseline_test.go`: the salvaged 1554 red test,
finality-refuses-unchanged + finality-still-completes-rewrites, coarse-mtime
size-only pin, capture unit, `withDefaults`-is-real tripwire, and the
full-engine wiring replay (`LaunchArgs` → stale artifact + silent live session
→ `ExitArtifactTimeout`).
`internal/phases/runner/runner_reconcile_stale_test.go`: stale leftover not
reconciled (mandatory FAIL + cause in diagnostics), rewritten leftover still
reconciles, optional degrades to WARN, ACS floor refuses the stale leftover.
Mutation-tested: 12 mutants across both layers, all killed (bridge: both
gates, multi-candidate capture, both key components, the deps hook, the
default; runner: snapshot, both door gates, the unchanged predicate).
