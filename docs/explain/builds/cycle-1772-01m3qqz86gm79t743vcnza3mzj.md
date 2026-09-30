# Build Explanation — Cycle 1772

## Build Binding
- Cycle: 1772
- Base SHA: 6aa43b70d6d0f68302ce12e2ab2fe54180394d79

## Summary
A composed-tree gate decline now produces one coded Signal Center event,
`ORCHESTRATOR_COMPOSED_GATE_DECLINED`, under `signalcenter.ModuleOrchestrator`. This holds at
all three `ciparity.MissingComposedGates` trip sites:

- RUNG 0 `compositionCarryForward`
- RUNG 2 `scopedMergeCarryForward`
- the ADR-0105 B3 `identityCarryForward`

The event names the cycle, lists every failing gate in `Reason`, and carries each failing gate's
output tail in its own `tail_<gate>` field. The tail is no longer written to stderr as a
multi-line block. The stderr lines that remain are single lines, and each is tied to its cycle.

## Rationale
The filed item (console, 2026-09-29, C6 architecture review M3) does not name trip sites. It
names three files and gives two acceptance criteria:

1. "A carry decline is one coded event naming the cycle, each failing gate and its output tail"
2. "No untagged multi-line stderr block remains for a decline"

Its `files` list includes `identity_carry_forward.go`. The identity carry declines on
`MissingComposedGates` exactly as the two rungs do, so it is a third trip site and falls inside
the item's scope. Round 1 of this cycle covered only RUNG 0 and RUNG 2. Round 1's audit
rejected that, and this build covers all three sites.

The fix mirrors the in-repo precedent `CodeRebaseReentryAborted`
(`go/internal/core/ship_recovery_debugger.go`). It registers one `signalcenter.Code` and emits
it through `o.signals.Emit`.

The Signal Center bounds values in two ways:

- `log.SanitizeField` caps `Reason` and every field value at 512 runes, and it keeps the FRONT
  of a longer value.
- `MaxLineBytes` caps the whole rendered JSON line at 4096 bytes. Over that cap it drops the
  largest fields first.

Round 1 packed all tails into `Reason`, so a realistic 20-line tail lost its end, and a second
gate's tail was lost entirely. This build trims each tail from its front (`boundedTail`), which
keeps the last lines where a failure is reported. Each trimmed tail fits both bounds:

- at most 512 runes;
- at most `MaxLineBytes/2` divided by the number of failing gates, measured in rendered JSON
  bytes. That is 512 bytes each when all four gates fail, as they do after a compile break.
  JSON escaping (`<`, `>` and `&` render as six bytes each) is counted.

## Changed Areas
- `go/internal/ciparity/composedgates.go` — adds GateOutcome{Status, Tail} and the GateStatuses projection, so the gate runner can return each gate's output tail while MissingComposedGates and the ledger's GateResults keep their status-only map[string]string shape.
- `go/internal/ciparity/composedgates_test.go` — names GateOutcome and GateStatuses in a real assertion, because the export-naming floor requires every new export to be exercised by a test.
- `go/cmd/evolve/cmd_composition_wiring.go` — the composed gate runner now returns each gate's last 20 output lines in GateOutcome.Tail and logs one single line per failing gate (gate, make target, worktree, exit error, pointer to the coded event), removing the untagged multi-line tail dump the item forbids.
- `go/internal/core/composition_carryforward.go` — registers CodeComposedGateDeclined and adds runGateSet (per-call context sink returning statuses plus outcomes), declineComposedGates (the one decline shape: one coded event plus one cycle- and code-tagged stderr line) and boundedTail; RUNG 0 and RUNG 2 now decline through it instead of the untagged line, so every decline is one coded event.
- `go/internal/core/identity_carry_forward.go` — gatesOnIntactTree runs the gates through runGateSet under the same tree fence, and a MissingComposedGates miss declines through declineComposedGates with origin Orchestrator.identityCarryForward, because the item lists this file and it is the third trip site; non-gate declines keep their existing tagged decline() line.
- `go/internal/core/composed_gate_tail_bound_test.go` — table test for boundedTail (end kept, JSON escape bytes counted, invalid UTF-8, byte budget) plus a four-gate decline with HTML-escape-dense tails, proving every tail_<gate> field is delivered without fields.truncated.
- `go/internal/core/composed_gate_decline_test.go` — TDD-authored round-2 contract: realistic tails reach the delivered event, unreported gates are still named, tagged stderr at RUNG 0 and RUNG 2, the identity-carry event, and no event for non-gate identity outcomes.
- `go/internal/core/composition_carryforward_signal_test.go` — TDD-authored RUNG 0 coded-event test, asserting the cycle, failing gates and tails over Reason plus Fields.
- `go/internal/core/scoped_merge_carryforward_signal_test.go` — TDD-authored RUNG 2 coded-event test, asserting the same event shape as RUNG 0 so both rungs share one decline contract.
- `go/cmd/evolve/cmd_composition_gates_tail_test.go` — TDD-authored tests proving the runner returns each gate's tail in its outcome and keeps the gate output out of the log.
- `go/cmd/evolve/cmd_composition_gates_env_test.go` — the failing gate's output is now asserted in its outcome's Tail instead of the log, while the log must still name the gate and the worktree.
- `go/internal/core/composition_carryforward_wired_test.go` — updated to the widened WithCompositionGateRunner return type (map[string]ciparity.GateOutcome); no behavioral assertion changed.
- `go/internal/core/ship_recovery_composition_test.go` — updated to the widened WithCompositionGateRunner return type; no behavioral assertion changed.
- `go/internal/core/ship_recovery_runid_seam_test.go` — updated to the widened WithCompositionGateRunner return type; no behavioral assertion changed.
- `go/internal/core/identity_carry_forward_test.go` — the harness moves to the widened return type and gains an optional Signal Center (no-op when nil) so identity-carry decline events can be asserted.
- `docs/architecture/signal-codes.md` — regenerated so the ORCHESTRATOR_COMPOSED_GATE_DECLINED row names all three trip sites and the tail_<gate> fields, keeping the code registry doc in sync with the code.
- `docs/architecture/packages/internal-core.md` — the Composition carry-forward entry now describes the shared decline shape, the per-call tail sink and the tail bound, because the new helpers carry no code comments and the package page owns that design detail.
- `.evolve/evals/composed-gate-decline-coded-signal.md` — this item's eval, written by the TDD phase, listing the graders that prove the coded decline at all three sites.

## Design Decisions
`orchestrator.go`'s `compositionGateRunner` field is a protected control-plane surface, and its
type is pinned to a status-only `map[string]string`. For that reason:

- `WithCompositionGateRunner` takes the richer closure and projects it down to statuses.
- The closure hands the full outcomes back through a context value that `runGateSet` attaches
  for exactly one call and reads right after.

The identity carry now uses the same `runGateSet`. That is how its decline gets the tails
without touching the field.

One code covers all three sites, told apart by `Origin`, the same way `CodeRebaseReentryAborted`
covers one multi-branch flow. Gate names stay in `Reason` because they are short, always fit and
are what a reader scans first. Tails go in separate fields, so each one gets its own 512-rune
budget and no gate's tail can push out another's.

The stderr lines that remain on a gate decline:

1. `[orchestrator] cycle <N> <site>: composed-tree gates not green (<gates>), ORCHESTRATOR_COMPOSED_GATE_DECLINED; falling back to full re-audit`
   - Written by core, one line per decline.
   - Names the cycle and the code, so a loop log carrying several lanes can join it to the event.
2. `[orchestrator] composed-tree gate <gate> (make <target>) failed in <worktree>: <err>; its output tail rides in the ORCHESTRATOR_COMPOSED_GATE_DECLINED event`
   - Written by the cmd gate runner, one line per failing gate.
   - The runner does not know the cycle number. The lane worktree path it names
     (`.evolve/worktrees/cycle-<id>-<N>`) ties the line to its cycle.

## Verification
- The eval graders in `.evolve/evals/composed-gate-decline-coded-signal.md` all pass. They cover
  RUNG 0, RUNG 2, identity carry, realistic tails, tagged stderr and the runner log.
- `TestBoundedTail` and `TestCompositionCarryForward_HTMLEscapedTailsOfAllFourGatesFitOneEventLine`
  pass.
- The full `go test -count=1 ./...`, `gofmt -l .` and `go vet ./...` results are recorded in
  `build-report.md`.

## Compatibility
These are unchanged:

- `ciparity.MissingComposedGates`'s signature;
- the ledger's `CompositionVerdictInput.GateResults` type, `map[string]string`;
- the composition-verdict record format;
- `orchestrator.go`'s field type.

What an operator sees changes in three ways:

- One new Signal Center code.
- The stderr tail dump is gone. The tail now lives in the event's `tail_<gate>` fields.
- The decline line gains the cycle and the code.

## Limitations
- A tail is the gate's last 20 output lines, trimmed further from the front to its per-gate
  budget. When all four gates fail, each keeps about the last 500 bytes. Earlier output survives
  only in the gate's own re-run.
- The runner's per-gate log line names the worktree, not the cycle number.
- Other decline diagnostics are outside this item and still write their pre-existing stderr
  lines with no coded event:
  - in `composition_carryforward.go`: snapshot unavailable, composed diff unavailable, patch-id
    mismatch, writer failed;
  - in `scopedMergeCarryForward`: incompatible disposition, resolution re-verify failure.

  Triage recorded these as the deferred items `composed-gate-decline-other-stderr-diagnostics`
  and `composed-gate-decline-scoped-merge-nongate`.
