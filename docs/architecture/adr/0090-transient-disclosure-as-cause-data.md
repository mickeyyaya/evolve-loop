# ADR-0090 — A transient upstream failure is disclosed as DATA on the artifact-timeout cause, never as a reclassification

- **Status:** Accepted (2026-08-21, PR #478)
- **Driving incidents:** cycles 1523 / 1524 / 1526 (batches 20260819a / 20260820a-verify) —
  3 of 4 observed router stalls burned the full 600s silence budget on a pane that
  stated its own cause verbatim. Full trace:
  [2026-08-18-transient-529-inside-artifact-timeout.md](../../incidents/2026-08-18-transient-529-inside-artifact-timeout.md).
- **Preserves:** eval `transient-bridge-retry` AC-1 (cycle 173) — "ErrArtifactTimeout
  (exit 81) must NOT classify as transient". This ADR exists precisely because that
  contract is correct and must not be weakened.
- **Related:** [ADR-0047](0047-surface-classification-and-channel-separation.md) —
  the governing principle here: a detector that reads a surface co-mingling agent
  content with infrastructure chrome will match the wrong thing. Decisions 2 and 6
  below are direct applications of it (read the pane, not the bridge's own stderr;
  strip agent content before matching).
- **Related:** [ADR-0029](0029-cli-fallback-chain-and-per-agent-overrides.md) —
  dispatch-level `cli_fallback_on_exit` DOES include 81 (amended cycle-122), which
  is a *different* mechanism from `internal/core`'s `isTransientBridgeError` retry
  classification (which excludes it). Keep the two straight: this ADR touches
  neither, it removes the need to spend the budget at all.
- **Related:** [ADR-0070](0070-signal-center-exhaustion-signal.md) — the quota-wall
  detector this deliberately does NOT extend.

## Problem

Three recognition paths exist for "the phase died for a reason outside the agent's
control", and a transient server error falls between all three:

1. **`exhausted_regex`** (manifest-driven, `usageclassify.go`) recognizes the
   PERMANENT quota wall. A 529 is not a quota wall, and widening the wall pattern
   to catch it would corrupt the wall signal — walls escalate, blips should not.
2. **`isTransientBridgeError`** (`internal/core`, sentinel `ErrTransientBridgeFailure`
   in `errors.go`) classifies exits 80 / 85 / 86 / 124 and a ctx-cancel signal-death
   as retryable. Note 124 is already handled — it does NOT fall between the paths the
   way 529 does. A 529 produces none of these; the pane simply goes quiet and the
   artifact never lands, so the death is a generic exit 81.
3. **Exit 81 itself** is contractually non-retryable, and correctly so: a generic
   artifact timeout means "the agent stalled, investigate", and blind retry burns
   another full budget.

Nothing was wrong with any of those rules. The defect was that the information
needed to tell a blip from a wedge was **already captured** — it is in the pane,
and it is in the escalation report's `final_pane` — and was never consulted.

Cost is per ROUTER INVOCATION, not per lane: cycle-1526 paid 600s twice in one
cycle (initial plan + post-scout re-plan), which is why it trailed cycle-1525 by
three phases and ended on the static spine.

## Decision

**1. Discrimination rides the cause as data; the exit code is untouched.**
The driver adds ONE field, `transient=true|false`, to the single self-describing
`artifact-timeout:` marker line it already emits before `return ExitArtifactTimeout`.
Exit 81 remains non-transient by exit code, so AC-1 stays green by construction
rather than by careful re-reading.

**2. The classifier reads the PANE, never the stderr buffer.**
Every `deps.Stderr` write on the exit-81 path is a bridge-authored, `pfx`-prefixed
note; the provider's error text renders in the PANE, which goes to the
`cfg.StderrLog` / `cfg.StdoutLog` FILES — a different sink. A stderr-buffer
classifier would pass fixture-based acceptance 3/3 GREEN while never firing on a
real API error. This was the cycle-1528 premise-challenge's decisive objection
(severity CRITICAL) and is the reason acceptance must observe a real captured pane.

**3. The field EXTENDS the existing marker line; it never competes with it.**
`artifactTimeoutSummary` selects ONE cause line by marker match. Adding a second
candidate would resurrect the displacement class that the drift alarm (which
contains the word "quota") and the workspace file listing (`"%s (%d bytes)"`)
already caused once. Extending the line makes that class structurally unreachable
rather than merely tested-against.

**4. The pattern is per-family config, resolved from the LAUNCHED cli.**
`transient_regex` is declared in each `manifests/<cli>.json` and resolved via
`lp.name`, so every CLI family carries its own provider's signature and no
provider vocabulary is hard-coded in Go (phases-are-config-only). A family that
declares none is fail-open — `transient=false`, never a fabricated cause.

**5. The key is TOP-LEVEL, not a sibling of `controls.usage.exhausted_regex`.**
The two read different surfaces: `exhausted_regex` classifies the OUTPUT OF the
`/usage` control, while `transient_regex` classifies the working pane. The sibling
placement (which the original design proposed) also leaves `ollama-tmux`
structurally unable to declare recognition — a local model has no usage control at
all, yet its server still returns 500s.

**6. The scan runs on the AGENT-STRIPPED pane.**
Same treatment as the exhaustion detector and the drift alarm
(`strippedForExhaustionScan`). A raw scan fires on error text the agent merely
quoted — and the sharpest case is this repo itself, where a cycle whose task IS
"handle API Error: 529" would mislabel every one of its own timeouts.

**7. The field is a bool, never pane text.**
Driver-authored values only. Agent-chosen filenames and provider prose can never
reach the recorded cause through it, so the indirect-prompt-injection surface stays
closed rather than being re-opened by a diagnostic.

## Alternatives considered and rejected

| Alternative | Why rejected |
|---|---|
| Make exit 81 retryable when the pane looks transient | Regresses `transient-bridge-retry` AC-1, a contract that is correct. A generic timeout must stay "investigate", or every genuine wedge costs a second full budget. |
| Widen `exhausted_regex` to match 529 | Corrupts the wall signal. Walls escalate (the fallback chain is consumed); blips should not. Path collision is asserted against in both directions. |
| Classify from the stderr buffer in `Engine.Launch` | The buffer never contains provider text (see Decision 2). Passes synthetic fixtures, never fires live. |
| Emit a second cause candidate and let `artifactTimeoutSummary` choose | Re-opens the cause-displacement regression class. |
| One shared transient pattern in Go, with per-family overrides | A Go-literal pattern is the anti-pattern this repo names explicitly. No manifest-inheritance mechanism exists, and inventing one inside a bugfix is out of scope — recorded as a follow-up if the four patterns converge. |

## Consequences

**Gained.** An exit-81 death now says whether the pane carried a recognized
temporary upstream failure. The operator playbook keys on it
([full-tmux-control.md §7a](../full-tmux-control.md)): `transient=true` → the provider
failed, re-dispatch is reasonable and the phase budget is not the problem;
`transient=false` + `busy=true` + `extends_used == max_extends` → raise
`bridge.phase_artifact_timeout_s`; `transient=false` + `liveness=idle` → a genuine
wedge.

**NOT gained (deliberate).** The 600s is still burned. This ADR delivers the data
that makes the burn diagnosable, not the control flow that avoids it. Acceptance #1
of the driving inbox item (no 600s pause, no static-spine degrade) is a follow-on
that now has a signal to key on.

**Evidence asymmetry, stated.** `claude-tmux`'s pattern is grounded in a verbatim
live pane (checked in as `internal/bridge/testdata/cycle-1523-router-529-pane.txt`).
The codex / agy / ollama patterns come from those providers' documented error
shapes; no captured transient pane exists for them in the runtime logs. Fail-open
means a miss costs only the label. The next non-Claude occurrence is the evidence
that confirms or corrects them.

**Enforcement.** `TestEveryTmuxFamilyDeclaresTransientRecognition` iterates the
embedded manifest set, so a future tmux family added without a `transient_regex`
fails the build rather than silently shipping a blind spot.

**Known interaction — a console-first landing does not retire its scope.**
This fix was landed console-first (per the operating policy for pipeline-integrity
defects), which means it never passed through any of the loop's retirement sites.
[ADR-0089](0089-continuation-retirement-and-live-scope-guard.md) made retirement
transactional for the paths that take an item out of the pending pool
(`inboxmover.Promote`, `ReconcileSuperseded`, ship-time `consumeCommittedItems`) —
but a console landing touches none of them. The scope-keyed binding therefore
survives the work:

```
$ evolve continuation list
  transient-api-error-invisible-inside-artifact-timeout  branch=cycle-42824668-1528  cycle=1528
```

Because the wave planner mints lane scopes directly from registry bindings, a
resumed loop can mint a lane on a scope that has already shipped and burn it
re-deriving landed work — the ADR-0089 failure mode reached by a path ADR-0089 does
not cover. The sanctioned release is
`evolve continuation release <scope-id>` (which preserves the salvage pointer into
the scope's inbox item first); the inbox item itself is retired with
`evolve inbox-mover promote`. Hand-moving the inbox file is NOT equivalent — it
leaves the binding live, which is the original defect.

**Generalization worth tracking:** any console-first landing of an inbox-sourced
scope has this gap, not just this one. Whether retirement should be wired into a
console-landing site (or whether the operator runbook is the right place) is not
decided here.
