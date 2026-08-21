# A transient upstream error inside an artifact timeout is recognized by nothing

**Window:** cycles 1523 / 1524 / 1526 (batches `20260819a-verify`, `20260820a-verify`), 2026-08-18 → 2026-08-20.
**Symptom:** 3 of 4 observed router stalls burned the full 600s silence budget, then degraded routing to the static spine.
**Cost:** per ROUTER INVOCATION, not per lane — cycle-1526 paid it **twice in one cycle** (initial plan + post-scout re-plan), which is why it trailed cycle-1525 by three phases.
**Status:** RESOLVED (partially) — PR #478 ([ADR-0090](../architecture/adr/0090-transient-disclosure-as-cause-data.md)): the diagnostic is delivered; the control-flow fix that avoids the spend is still open (§6).

---

## 1. The evidence

`.evolve/runs/cycle-1523/router-escalation-report.json`, key `final_pane`, verbatim:

```
⏺ API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment.
  If it persists, check https://status.claude.com.

✻ Sautéed for 3m 28s
```

The agent then produced no further output. The bridge's generic silence detector
fired at `waited=600s interval=300s extends_used=1 last_review=pause liveness=idle`
→ exit 81 → the orchestrator logged `WARN phase advisor Plan failed (degrading to
static spine)` / `WARN post-scout re-plan failed (keeping initial plan)`.

The same text appears in the cycle-1524 and cycle-1526 reports. A
`grep -rniE '529|overloaded'` over `go/internal/` at the time returned **zero**
functional handling — every hit was a cycle number or a comment about overloaded
Go types.

## 2. Why all three recognition paths missed it

| Path | What it recognizes | Why 529 slipped |
|---|---|---|
| `exhausted_regex` (`usageclassify.go`, manifest-driven) | the PERMANENT quota wall | 529 is not a wall. Widening the wall pattern would corrupt the wall signal — walls consume the fallback chain and escalate; blips should not. |
| `isTransientBridgeError` (`internal/core`, sentinel `ErrTransientBridgeFailure`) | exits 80 / 85 / 86 / 124 + ctx-cancel signal-death | 529 produces none of these. The pane goes quiet, the artifact never lands, the death is a generic exit 81. (124 is already covered here — unlike 529, it does not fall between the paths.) |
| exit 81 itself | "the agent stalled — investigate" | Contractually non-retryable, and **correctly so** (eval `transient-bridge-retry` AC-1, cycle 173). Blind retry of a real wedge costs a second full budget. |

None of these rules was wrong. The defect was that the distinguishing evidence was
**already captured** — in the pane, and in the escalation report's `final_pane` —
and was simply never consulted.

**Precision worth keeping straight.** Exit 81 *is* present in the dispatch-level
CLI-fallback trigger set (the runner logs `fallback=[codex-tmux] triggers=[80 81 85 124 127]`,
ADR-0029 as amended in cycle-122). So 81 is not unhandled. The defect is that the
fallback fires only AFTER the full 600s is already spent, and for the router the
chain is by then exhausted (agy → claude, both consumed), leaving degrade-to-static-spine
as the only remaining move. Two distinct mechanisms — dispatch-level fallback triggers
(include 81) vs orchestrator-level `isTransientBridgeError` (excludes 81) — and this
incident targets neither.

## 3. Two designs that were tried and killed

Both are recorded because each was *plausible*, would have passed its own acceptance
criteria, and was falsified only by going and looking.

### 3a. The stderr-buffer classifier — killed by the cycle-1528 premise-challenge (severity CRITICAL)

The obvious implementation reads the stderr buffer `Engine.Launch` inspects on an
exit-81 death. **That buffer never contains provider error text.** Exit 81 has
exactly one production emitter (`driver_tmux_repl.go`), and every `deps.Stderr`
write on that path is a bridge-authored, `pfx`-prefixed note; the provider's text
renders in the PANE, which goes to the `cfg.StderrLog` / `cfg.StdoutLog` FILES — a
different sink.

Such a classifier would have passed fixture-based acceptance **3/3 GREEN while never
firing once on a real API error.** Worse, the only lines it *would* have matched on
that path are (a) the exit-81 drift alarm, which contains the word "quota"
(`exhaustion_drift.go`), and (b) workspace file sizes (`"%s (%d bytes)"`,
`driver_common.go`) — both of which would have displaced the self-describing timeout
summary and regressed the earlier `engine.go` cause-selection fix.

**Lesson:** the premise-challenge's decisive objection was that all three proposed
acceptance criteria were falsifiable only against synthetic fixtures while the
production outcome stayed untouched. Acceptance must observe a REAL captured pane.
The fix ships one: `internal/bridge/testdata/cycle-1523-router-529-pane.txt` is the
unedited `final_pane`, with a fixture guard that fails loudly if it is ever truncated.

### 3b. `controls.usage.transient_regex` — falsified by ollama during implementation

The design doc proposed hanging the new pattern off `controls.usage`, as a sibling
of `exhausted_regex`. Implementing it revealed that **`ollama-tmux` has no `usage`
control at all** — a local model has no `/usage` command and no quota concept — yet
its server still returns 500s. The sibling placement left one family structurally
unable to declare recognition.

The deeper reason the placement was wrong: the two patterns read **different
surfaces**. `exhausted_regex` classifies the OUTPUT OF the `/usage` control;
`transient_regex` classifies the working pane. Co-locating them looked like
single-sourcing but coupled a phase-execution concern to a control not every CLI has.
Resolved by making `transient_regex` a top-level manifest key.

**Lesson:** this only surfaced because the work was driven to be LLM-agnostic across
all four CLI families rather than proven on Claude and generalized later. A
claude-only implementation would have shipped the wrong schema and the wrong tests.

## 4. The fix

The single self-describing `artifact-timeout:` marker line now carries a
driver-authored `transient=true|false` field, matched from a per-family
`transient_regex` against the AGENT-STRIPPED captured pane. Exit 81 is untouched —
discrimination rides the cause as **data**, not as a reclassification. Full rationale
and the rejected alternatives: [ADR-0090](../architecture/adr/0090-transient-disclosure-as-cause-data.md).
Operator playbook: [full-tmux-control.md §7a](../architecture/full-tmux-control.md).

## 5. Regression coverage map

| Failure mode | Pinned by |
|---|---|
| Transient error invisible on a real captured pane | `artifact_timeout_transient_test.go::TestRunTmuxREPL_ArtifactTimeout_MarkerFlagsTransientOnLivePane` (verbatim cycle-1523 pane + fixture guard) |
| Field degenerates to a constant | `…_SilentPaneIsNotTransient` (wedged pane must report `transient=false`) |
| Recognition is claude-only / pattern hard-coded in Go | `…_TransientFieldIsFamilyAgnostic` (drives codex, whose transient text contains no "529") + `TestEveryTmuxFamilyDeclaresTransientRecognition` |
| A new tmux family ships with no recognition | `TestEveryTmuxFamilyDeclaresTransientRecognition` iterates `manifestFS` — the build fails, it does not silently blind-spot |
| Pattern collides with that family's quota wall (either direction) | `TestEveryTmuxFamilyDeclaresTransientRecognition` asserts disjointness both ways |
| Pattern matches bridge-authored chatter (drift alarm's "quota", workspace listing, 429 burst) | same test, `bridgeAuthoredChatter` table |
| Classifier reads the RAW pane → agent's echoed task text mislabels its own timeouts | `TestClassifyTransientPane_IgnoresEchoedPromptText` (unit) **and** `…_EchoedPromptIsNotTransient` (driver — the unit test alone does NOT catch the driver handing over the raw pane) |
| Unknown/manifest-less driver fabricates a cause | `TestClassifyTransientPane_UnknownDriverFailsOpen` |
| Exit 81 becomes retryable | `internal/core::TestIsTransientBridgeError` (unchanged, verified green) |

All five designed mutations were confirmed to bite, including "hard-code Claude's
pattern in Go", which is killed by the codex driver test.

## 6. Still open

1. **The 600s is still burned.** This fix delivers the diagnostic data, not the
   control flow that avoids the spend. Acceptance #1 of the driving inbox item
   (no 600s pause, no static-spine degrade) is a follow-on that now has a signal to
   key on. It must not be implemented by making exit 81 retryable — see §2.
2. **codex / agy / ollama patterns are unvalidated against live panes.** Only
   `claude-tmux`'s is evidence-grounded; the other three come from those providers'
   documented error shapes. No captured transient pane for them exists in the runtime
   logs (a sweep found 6 "Overloaded" hits, all Claude). Fail-open means a miss costs
   only the label. **The next non-Claude occurrence is the evidence** — when one
   appears with `transient=false` on a pane that clearly shows a server error, that
   family's regex is the thing to correct.
3. **Four near-identical regexes across four manifests.** Follows the existing
   `drift_probe_regex` precedent (codex and agy already carry byte-identical copies),
   but there is no manifest-inheritance mechanism. If the four converge further,
   centralizing them is worth its own ADR.
