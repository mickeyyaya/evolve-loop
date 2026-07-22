# ADR-0075: Rewire the inert `evolve guard phase` PreToolUse hook to the Agent/Task tool

- **Status**: Accepted (decision); **implementation is operator-gated** — the load-bearing
  `.claude/settings.json` change is pipeline control plane (see "Why this cannot land in a cycle")
  and must be applied via human-gated `evolve ship --class manual` OUTSIDE any cycle.
- **Date**: 2026-07-22
- **Cycle**: 1035 (fleet task `rewire-or-retire-guard-phase-hook`, inbox item `guard-phase-hook-inert`, weight 0.89)
- **Supersedes / relates to**: ADR-0064 (pipeline-integrity boundary), rule 5 in
  `docs/operations/runtime-reference.md` (phase agents via the native bridge).

## Context

`go/internal/guards/phase.go` (`Phase.Decide`) returns a non-Allow decision **only** when
`in.ToolName == "Agent"` and a cycle is active (`cycle-state.json` `cycle_id != 0`): it denies the
in-process `Agent` subagent-dispatch tool, forcing phase agents through the native bridge
(`evolve subagent run` / `evolve loop`). The Go logic is correct and fully unit-tested
(`go/internal/guards/phase_test.go`: `TestPhase_AgentDuringCycleDenied`, `…OutsideCycleAllowed`,
`…BypassAllowsAgent`, `…NonAgentToolsPass`, `…ReadCycleStateErrorDenies`, `…NilStorageDenies`).

**The defect is the wiring, not the logic.** `.claude/settings.json` wires `evolve guard phase`
**exclusively** under the `"Bash"` PreToolUse matcher. Claude Code therefore only ever invokes it with
`tool_name == "Bash"`, which takes the `ToolName != "Agent"` early-return and Allows unconditionally.
The guard's one real branch can never fire: it is a **wired no-op**. Meanwhile several doc surfaces
assert that `evolve guard phase` enforces phase transitions / denies in-process `Agent` — enforcement
that does not happen.

Verified live on the cycle-1035 worktree: `go/internal/guards/phase.go:26-49`,
`.claude/settings.json` (`guard phase` appears only in the `"Bash"` matcher block). `readGuardInput`
(`go/internal/cli/guardcmd/guard.go`) reads `tool_name` from the PreToolUse stdin JSON, so a matcher
covering the `Agent` tool would make `Phase.Decide`'s Agent branch fire end-to-end — this is a genuine
wiring fix, not a magic-string dodge.

## Decision

**Rewire (chosen) over retire.**

- **Rewire** — move `evolve guard phase` out of the `"Bash"` matcher into a PreToolUse matcher that
  covers the in-process subagent-dispatch tool (`"Agent|Task"`). The existing, already-tested
  `Phase.Decide` logic then fires on the tool it was written for.
- **Retire** — delete `go/internal/guards/phase.go`, unwire it from `.claude/settings.json`, and
  re-point docs at the state machine as the sole enforcement point.

### Why rewire, not retire

The state machine (`go/internal/core`, `SpineSatisfiedUpTo` + the legality graph) is the authoritative
enforcer of phase **ORDER** — that is not in question and does not change. But the phase guard enforces
a **different** invariant: it denies the *in-process `Agent` dispatch mechanism* during a live cycle, so
phase agents are forced through the native bridge (rule 5, runtime-reference.md; CLAUDE.md autonomous
rule 5). The state machine cannot see individual tool calls, so it cannot deny in-process `Agent`. The
two are **complementary, not redundant** — so `[[never_duplicate_centralize_via_design_patterns]]` does
**not** argue for retire here (scout flagged the "duplication" hypothesis as needing Builder-time
confirmation; confirmed false). Retiring would delete the only hook-layer enforcement of a real,
documented invariant. Rewiring restores it with the smallest possible diff and no dead code.

## Why this cannot land inside a cycle (operator action required)

`.claude/settings.json` (`PreToolUse hook wiring`) is listed in
`guards.ProtectedSurfaceManifest` (`go/internal/guards/integrity_surface.go`) — the compiled,
non-config-softenable pipeline **control plane**. The `role` PreToolUse guard denies any Edit/Write to
it while a cycle is active (`role.go`, integrity-violation, no bypass), because "a cycle must never be
able to edit the gate that judges it" (the cycle-20 self-blessing breach). `go/internal/guards/` is
likewise protected, so **retire is equally blocked in-cycle** (both dispositions require a control-plane
edit). This is exactly why the prior autonomous attempts (cycle-763, 1028, 1032) never landed. The
decision is therefore recorded here as an ADR, and the wiring change is handed to an operator — matching
the standing `pipeline_fixes_console_first` rule (pipeline-integrity defects are fixed hands-on in a
console session, never inside an autonomous cycle).

## Operator patch (apply via `evolve ship --class manual`, outside any cycle)

In `.claude/settings.json`, **remove** the `guard phase` hook from the `"Bash"` matcher block and
**add** a new `PreToolUse` entry:

```json
{
  "matcher": "Agent|Task",
  "hooks": [
    {
      "type": "command",
      "command": "B=\"$CLAUDE_PROJECT_DIR/go/bin/evolve\"; [ -x \"$B\" ] || B=\"$CLAUDE_PROJECT_DIR/go/evolve\"; \"$B\" guard phase --evolve-dir \"$CLAUDE_PROJECT_DIR/.evolve\""
    }
  ]
}
```

Also update the `_comment` segment (2) to describe the guard as phase-gate on `Agent|Task`. No Go code
changes are required — `Phase.Decide` and its tests are already correct.

### Verification after the operator applies the patch

- `go test -tags acs ./go/acs/cycle1035/... -run TestC1035_001` → GREEN (matcher now covers `Agent`).
- `go test ./go/internal/guards/... -run TestPhase -race` → GREEN (unchanged, already passing).

## Consequences

- **Positive**: the documented "in-process `Agent` denied during a cycle" invariant becomes real; no
  dead config; smallest diff; Go code untouched.
- **Negative / risk**: adds one PreToolUse hook invocation to every `Agent`/`Task` call (negligible
  latency; identical pattern to the four existing guards). Requires an operator step — it cannot be
  fully automated by design, which is the correct trade for a control-plane change.
- **Doc lockstep** (this cycle, non-control-plane, landed with this ADR): `CLAUDE.md`,
  `docs/operations/runtime-reference.md`, and `skills/loop/SKILL.md` are corrected so no surface
  asserts the guard currently enforces phase order — they now credit `go/internal/core` (state machine)
  for phase ORDER and describe `evolve guard phase` as the in-process-`Agent`-dispatch backstop rewired
  per this ADR.
