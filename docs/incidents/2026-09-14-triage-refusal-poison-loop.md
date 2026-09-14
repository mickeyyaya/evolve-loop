# 2026-09-14 — one inbox item drew nine lanes: the triage refusal had no failure class

**Status:** fixed (this change) · **Severity:** P1 pipeline (tokens burned with zero progress, one FAIL per wave guaranteed) · **Found by:** the Signal Center stream of the first post-decomposition verification wave (cycle 1675), confirmed from `.evolve/ledger.jsonl`.

## What the stream said (three lines)

```
[bridge]       bridge.warning WARN BRIDGE_EXIT_ARTIFACT_TIMEOUT cycle=1675 phase=scout  … cause=submit_wedged
[orchestrator] phase.outcome  WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=1675 phase=triage … top_n card "verdict-sentinel-as-tool-call" names protected surface "go/internal/bridge/streamjson_verdict.go" — control-plane changes go through the console route
[orchestrator] cycle.sealed   WARN ORCHESTRATOR_CYCLE_FAILED cycle=1675 … phases_run=2
[inbox]        inbox.warning  WARN INBOX_CLAIM_NOT_FOUND cycle=1675 origin=Mover.Claim — claim: task 'verdict-sentinel-as-tool-call' not found in …/.evolve/inbox
```

The fourth line was the tell: the closeout could not find an item that was right there. The ledger then showed the loop:

| cycle | 1650 | 1653 | 1655 | 1658 | 1661 | 1665 | 1669 | 1672 | 1675 |
|---|---|---|---|---|---|---|---|---|---|
| action on `verdict-sentinel-as-tool-call` | recover | recover | recover | recover | recover | recover | recover | recover | recover |

Nine waves, the same item, `failure_count` never written. Every wave: a scout dispatch, a triage dispatch, a FAIL — and the item back at the inbox root for the next wave.

## Root cause (mechanism, not symptom)

1. `phases/triage.Classify` refuses a top_n card that names a protected surface (`guards.IsProtectedSurface`) — correct, by design (ADR-0074: control-plane changes are operator-owned). It recorded verdict FAIL with a **prose-only** diagnostic.
2. `cycleclassify.Classify` had no class for a phase gate's own refusal: no `orchestrator-report.md` exists on the abnormal path, the agent's sentinel carried no failure class, so the cycle fell to the default reading — **system-level** (not the item's fault).
3. `cycleoutcome.FailureInputsFor` therefore set `SystemLevel=true`, and `lifecycle.Mover.Release`'s `parkAtCeiling` — which gates the `failure_count` bump on `!SystemLevel` (ADR-0072 AC4, rightly: an infra failure is not the todo's fault) — released the item untouched. The S5 quarantine ceiling was unreachable for this whole class.
4. Meanwhile `ClaimLaneScope` re-claimed the committed id from the inbox **root** while the lane's own claim had already moved it into `processing/cycle-N/` → the false `INBOX_CLAIM_NOT_FOUND` on every closeout (unit 06 had documented it as "the declared benign WARN").

## Fix (structured end to end — the decomposition and Signal Center design applied)

| Layer | Change | Test (red first) |
|---|---|---|
| `cyclestate.Diagnostic` | `code` + `subject` (both `omitempty`); `DiagCodeTriage*` vocabulary; `ErrorCodes` projection | `diagnostic_code_test.go` (wire shape, vocabulary, projection) |
| `phases/triage` | the three deterministic refusals stamp their code; the protected-surface one names the card as `subject` | `refusal_codes_test.go` (all three refusals + PASS carries none) |
| `core` C1 chokepoint | `ORCHESTRATOR_PHASE_VERDICT_FAIL` carries `fields.diagnostic_codes` | `signal_refusal_codes_test.go` |
| `cycleclassify` | pass 0b reads the C1 record (`phase-timing.json`): a coded FAIL on the LAST outcome ⇒ `phase-refusal` (marker = code, detail, subject); ranked after the sentinel pass, before prose | `refusal_test.go` (six cases: coded, uncoded, not-last, outranks regex, sentinel still wins, corrupt record) |
| `cycleoutcome` | `phase-refusal` is task-level; `FailureInputs` carries the refusal; a `TRIAGE_PROTECTED_SURFACE` subject (one in this cycle's committed set) is routed console-manual **before** the drain, then the drain runs with `Routed` set for that cycle; other coded refusals bump toward the ceiling per `cyclestate.RefusalDisposition` | `refusal_route_test.go` (route + breaker closes; bump + quarantine at ceiling; uncoded stays system-level) |
| `inboxmover/lifecycle` (unit 06) | `Mover.RouteConsole` — rewrite in place wherever the claim left the item; `INBOX_ITEM_ROUTED_CONSOLE` / `INBOX_ROUTE_NOT_FOUND`; `INBOX_ITEM_REWRITE_FAILED` gains `step=route` | `route_test.go` (root item, processing item, not found, rewrite fault, claim refused afterwards); leaf stays 100 % lines / 53/53 API |
| `inboxmover` host | `RouteConsole` facade; `ClaimLaneScope` skips an id the lane already holds in `processing/cycle-N/` | `route_console_test.go`, `claim_lane_scope_already_claimed_test.go`; three goldens re-captured (only the two false WARN lines per already-claimed id vanish) |

Why route rather than count: a protected-surface refusal is deterministic — the same card produces the same refusal on every retry — and the triage gate's own sentence already says who owns the item. Counting to a ceiling would spend two more lanes to reach the same disposition. The ceiling stays as the second breaker (a route that cannot happen says so on the stream and falls back to the bump).

## What the operator now sees

```
[orchestrator] phase.outcome WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=N phase=triage … diagnostic_codes=TRIAGE_PROTECTED_SURFACE
[inbox]        inbox.warning WARN INBOX_ITEM_ROUTED_CONSOLE cycle=N origin=Mover.RouteConsole — route-console: '<id>' is now console-manual — top_n card "<id>" names protected surface "<path>" …
```

One WARN names the item, the surface and the disposition; the item carries `route`, `routed_reason`, `routed_cycle`, `routed_at`; no lane draws it again; the console worklist shows it under `route:console-manual`.

## Mitigation applied on the plane before the fix landed

`2026-07-30T13-04-00Z-verdict-sentinel-as-tool-call.json` was routed `console-manual` by hand (with the same reason) so the running wave stopped drawing it.

## Follow-ups

- The scout `submit_wedged` artifact timeouts (two of three lanes at attempt 1 on codex-tmux in the same wave) are a separate reliability defect; tracked next.
- `cycleclassify`'s prose passes remain for the classes no phase codes yet; each producer that stamps a code retires a regex.

## Review folds (diff-scoped fleet: code-simplifier → architecture-reviewer ∥ go-reviewer)

Simplifier: no edits. Go review: PASS (two MINORs, both pre-existing package conventions — the leaf's one WARN producer for notable outcomes; chmod fault injection vacuous as root). Architecture review: FIX_THEN_MERGE, every finding folded red-first:

| Finding | Fold |
|---|---|
| HIGH-1 — an agent-authored `Subject` reached a durable, operator-owned disposition unchecked | `ApplyFailure` routes only a Subject that is in this cycle's committed set (a mis-copied id logs `route-console skipped … not in the committed set` and falls back to the bump); the leaf refuses a `Location` held by another cycle's claim (`INBOX_ROUTE_NOT_FOUND` with `held_by_cycle`) |
| HIGH-2 — "whose fault" decided at the class level charged I/O faults (`TRIAGE_COMMITMENT_INVALID`) to the queue | `cyclestate.RefusalDisposition(code)` — the table beside the vocabulary: protected-surface = task + route, empty top_n = task, commitment-invalid = system; `cycleoutcome.IsTaskLevelResult` reads it; `IsTaskLevelFailure(phase-refusal)` is false by class alone |
| MEDIUM-1 — `SystemLevel` overloaded to suppress the bump | a separate `Routed` knob (`inboxmover.CycleOutcome.Routed` → `lifecycle.Policy.Routed`) — the drain bumps and parks nothing for a routed cycle while `SystemLevel` stays the fact it is |
| MEDIUM-2 — pass 0 was not recency-aware: a stale earlier-phase sentinel could disarm the fix | `Classify` reads the record first; a sentinel from a phase other than the record's last outcome yields to 0b; the 0b-before-pass-2 precedence is stated where the ordering lives |
| MEDIUM-3 — the new decisions had no negative tests | two committed ids with one refused (only the Subject routed, the other untouched); empty / uncommitted Subject → bump; commitment-invalid → system-level |
| LOW — the header still documented a three-pass scan | the pass order, stated once, on `Classify` |
