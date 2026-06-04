# Design — Orchestrator contract-correction retry

> Spec for the queued feature: when the orchestrator finds a phase agent's output violates the
> deliverable contract, it sends a correction directive back to the agent and re-dispatches the
> phase (bounded to 2 retries) instead of aborting the cycle on the first violation.
>
> Date: 2026-06-04 · Status: approved design, pre-implementation (TDD to follow) · Gate:
> `EVOLVE_CONTRACT_CORRECTION_RETRIES` (default 2; `0` = today's immediate-abort).

---

## 1. The request / requirement

> *"When the orchestrator checks the phase agent output and there is something wrong, it should
> send the command to the agent and ask it to correct the error and follow the contract to
> generate the output. Build this recovery flow as retry for 2 times. Follow TDD."*

### Requirements

- **R1.** When a finished phase's deliverable violates its contract, re-dispatch the phase with
  a correction directive describing exactly what was wrong, asking the agent to fix it and
  satisfy the contract.
- **R2.** Bound the corrections to **2** attempts; if still failing after that, abort the cycle
  exactly as today (the current behavior is the preserved floor).
- **R3.** Scope = **deliverable-contract well-formedness only** (missing / wrong-path /
  missing-section / unparseable deliverable) — the `DeliverableReviewer` `Approve=false` case.
  Semantic / auditor-FAIL correctness is explicitly out of scope (documented follow-up).
- **R4.** Default-on, with a single config knob to disable (`=0`); off is byte-identical to today.
- **R5.** TDD; ≥95% coverage on changed packages; spec + quality review per task.
- **R6.** Within-phase retries are NOT new cycles (AGENTS.md Rule 4 — never fabricate cycles);
  each correction is observable (ledger + diagnostics + loud logging).

---

## 2. Current behavior (the integration point)

`go/internal/core/orchestrator.go:1531-1534`, after each phase's `runner.Run` + the bridge-timeout
retry loop:

```go
rr := o.reviewer.Review(ctx, rin)
if !rr.Approve {
    return result, fmt.Errorf("review gate: phase %q deliverable rejected: %s", next, rr.Reason)
}
```

The code comment says verbatim: *"Retry/N is a follow-up — today reject = abort."* This feature
is that follow-up.

- `o.reviewer` is the `DeliverableReviewer` (default `noopReviewer` → always approve; the real one
  is `deliverable.NewReviewer`, chained at the per-phase seam via `core.ChainReviewers`).
- `rr.Reason` is produced by `deliverable.summarize`:
  `"<phase> deliverable failed contract: [missing_section] required section 'Verdict' not found; …"`
  — i.e. the precise, deterministic correction signal, already in hand.
- **Architectural constraint:** by the time `Review` runs, the phase agent has exited and its tmux
  session is torn down (`tmuxCleanup` is deferred in `runTmuxREPL`). So "send a command to the
  *running* agent" is not available here. The robust, CLI-agnostic mechanism is to **re-dispatch
  the phase** with the correction injected into the prompt. (Live-inject into a *resumable* tmux
  session is a possible future enhancement; it does not work for headless `claude -p` and is
  CLI-specific, so it is out of scope.)

---

## 3. Approaches considered

### 3.1 Correction delivery mechanism (the main axis)

| | Mechanism | Verdict |
|---|---|---|
| **A** | New typed `core.BridgeRequest.CorrectionDirective string`, prepended as a `## Correction` block at the existing prompt-prefix seam (`bridge.go:125`, beside Rules/Policy/SystemPrompt) | ✓ **chosen** — typed, CLI-agnostic, testable at a proven seam, one small field; empty = no-op |
| B | Thread via `req.Env["EVOLVE_<PHASE>_CORRECTION"]`, bridge reads + prepends | rejected — stringly-typed, clutters the env overlay |
| C | Write a correction file the prompt references | rejected — indirect; relies on the agent choosing to read it |

### 3.2 Re-run worktree state

- **Fix-in-place (chosen):** leave the per-cycle worktree intact so the agent sees its prior
  (rejected) output + the precise correction directive and *fixes* it. Matches "correct the
  error", cheaper and more reliable than full regen for well-formedness defects (missing section,
  wrong path).
- Clean-slate (rejected): deleting the malformed deliverable before re-dispatch forces a fresh
  generation but discards the agent's prior work context.

### 3.3 Counter

- **Separate counter (chosen):** `EVOLVE_CONTRACT_CORRECTION_RETRIES` (default 2). Distinct from
  `EVOLVE_PHASE_MAX_ATTEMPTS`, which bounds *bridge-timeout / transient* retries — a different
  trigger. Overloading one knob conflates two failure classes.

---

## 4. Chosen design

### 4.1 Control flow (orchestrator)

Wrap the existing run → bridge-retry → review block in a bounded correction loop:

```
maxCorrections = EVOLVE_CONTRACT_CORRECTION_RETRIES (default 2, clamp [0,5])
correctionDirective = ""
for correction := 0; ; correction++ {
    phaseReq.CorrectionDirective = correctionDirective      // "" on the first pass
    resp, err = <existing run + bridge-timeout retry loop>  // unchanged
    if err != nil { <existing error handling> }
    rr = o.reviewer.Review(ctx, rin)
    if rr.Approve { break }                                  // success (incl. after a fix)
    if correction >= maxCorrections {
        return result, fmt.Errorf("review gate: phase %q deliverable rejected after %d correction(s): %s",
            next, correction, rr.Reason)                     // preserved floor (today's abort)
    }
    correctionDirective = composeCorrection(rr.Reason)       // ledger + log the attempt
}
```

- `maxCorrections == 0` ⇒ the loop body runs once and the `correction >= 0` guard fires
  immediately on reject ⇒ byte-identical to today's abort. (Off path proven by a test.)
- The bridge-timeout retry loop (the inner `for attempt`) is untouched — it is orthogonal.

### 4.2 `composeCorrection`

```
"Your previous output for this phase was REJECTED by the deliverable contract check:

<rr.Reason>

Fix the deliverable so it satisfies the contract — write it at the EXACT contracted path with all
required sections / valid structure — then finish. Do not change unrelated files."
```

(Pure function of `rr.Reason`; unit-tested.)

### 4.3 Delivery seam (runner + bridge)

- `core.PhaseRequest` gains `CorrectionDirective string` (orchestrator → runner).
- `core.BridgeRequest` gains `CorrectionDirective string` (runner → bridge); the runner copies it
  through alongside `SystemPrompt`.
- `bridge.go:125` becomes
  `injectCorrectionPrefix(injectRulesPrefix(injectPolicyPrefix(body, …), req.SystemPrompt), req.CorrectionDirective)`
  — a new `## Correction` block, empty ⇒ identity (off path byte-identical). Same CLI-agnostic
  seam as Rules.

### 4.4 Observability

- One ledger entry per correction attempt: `kind:"contract_correction"`, `role:<phase>`, body =
  the violation summary + attempt index. (Real within-phase event; NOT a new cycle number.)
- `corrections_used int` surfaced in the phase diagnostics / `PhaseResponse` so a cycle summary
  shows it.
- Loud `[orchestrator] phase <p>: contract violation (attempt k/N) — re-dispatching with correction: <reason>`.

### 4.5 Circuit-breaker interaction

The `deliverable` reviewer's breaker increments on a block and resets on a clean review
(`reviewer.go: resetBreaker`). A corrected→passing phase therefore resets it naturally — corrections
must not trip the enforce→advisory demotion. A test asserts a fail-then-pass sequence leaves the
breaker reset.

---

## 5. Module boundaries (independently testable units)

| Unit | Responsibility | Test |
|---|---|---|
| `composeCorrection(reason) string` (orchestrator) | turn a violation summary into a directive | pure unit test |
| correction loop (orchestrator) | bound retries, re-dispatch, preserve the abort floor | table test with a stub reviewer (reject×k then approve / always-reject) + a stub runner capturing the injected directive |
| `injectCorrectionPrefix(prompt, directive) string` (bridge) | prepend the `## Correction` block; empty = identity | pure unit test |
| `PhaseRequest`/`BridgeRequest` field plumbing | carry the directive end-to-end | runner test asserting the directive reaches the BridgeRequest |

---

## 6. Edge cases & invariants

- **`maxCorrections=0`** → byte-identical to today (one run, abort on reject).
- **Non-numeric / out-of-range** env → clamp to default 2 (range [0,5]); same discipline as
  `EVOLVE_PHASE_MAX_ATTEMPTS`.
- **`VerdictSKIPPED`** phases skip the reviewer entirely (unchanged) → no correction loop.
- **A correction that bridge-times-out** is handled by the inner bridge-retry loop first; the
  correction loop only re-enters on a *review reject*, not a transient error.
- **Floor preserved:** after the budget, the abort error message is a superset of today's (adds the
  correction count) so existing "rejected" handling still matches.
- **No fabricated cycles:** corrections are within-phase; `lastCycleNumber` is untouched.

---

## 7. Out of scope (documented follow-ups)

1. Semantic / auditor-FAIL correction (only well-formedness here).
2. Live-inject correction into a *resumable* tmux session instead of re-dispatch (CLI-specific;
   no headless support).
3. Adaptive directive wording / LLM-composed correction (the deterministic summary is the v1 signal).

---

## 8. References

- `go/internal/core/orchestrator.go:1414-1535` — the run/review block this wraps.
- `go/internal/core/reviewer.go` — `DeliverableReviewer` / `ReviewResult` / `ChainReviewers`.
- `go/internal/deliverable/{deliverable.go,reviewer.go}` — `Verify`, `summarize`, the breaker.
- `go/internal/adapters/bridge/bridge.go:125,189-237` — the prompt-prefix injection seam.
- `go/internal/phases/runner/runner.go:383-415` — where `SystemPrompt` is resolved + the
  `BridgeRequest` is built (the model for `CorrectionDirective`).
- CLAUDE.md — `EVOLVE_PHASE_MAX_ATTEMPTS` (the distinct transient-retry counter), the deliverable
  contract gate + circuit breaker rows.
