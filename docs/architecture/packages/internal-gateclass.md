# internal/gateclass

> Decision: [ADR-0155](../adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md). Spec of record: [llm-isolation.md](../llm-isolation.md) §20 and §21. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X15. The callers are the ship check `refuseWeakenedGates` (`go/internal/phases/ship/gate_classifier_gate.go`) and the build floor check `core.GateClassifierFloorChecks` (`go/internal/core/gate_classifier_floor.go`).

## Purpose

`internal/gateclass` holds invariant I-10: no agent changes a kernel surface or weakens a gate. It has two parts:

- the surface tiers T0, T1 and T2 of spec §20.1 (`TierOf`);
- a deterministic classifier of a cycle diff (`Classify`). It flags each change of spec §20.3.

The package is pure Go. It reads no file and runs no command. `Read` takes a git function from the caller and returns the diff text and the untracked paths. The caller adds the cycle number and two predicates: protected and constitution.

## A worked example: a skipped guard test

A loop cycle adds one line to `go/internal/widget/widget_guard_test.go`:

```diff
+	t.Skip("flaky")
```

The path is not on the protected list of ADR-0064, so the P2 check of ship (`verifyNoControlPlaneEdits`) lets the diff pass. Before X15 the cycle shipped. `Classify` now returns one flag:

| Pattern | Path | Detail |
|---|---|---|
| `added-skip` | `go/internal/widget/widget_guard_test.go` | `t.Skip("flaky")` |

The build floor fails the build with this flag. If the flag reaches ship, ship refuses the cycle with `GATE_WEAKENED` and emits `HEALING_I10_VIOLATION`. `TestRefuseWeakenedGates_ACycleDiffThatSkipsAGuardTestIsRefused` pins this example.

## Design

- **Tiers** (`TierOf`).

  | Tier | Rule |
  |---|---|
  | T0 | the protected predicate of the caller is true (`guards.IsProtectedSurface` in production) |
  | T1 | the path is under `go/internal/core/`, `go/internal/bridge/`, `go/internal/cliroute/`, `go/internal/gc/` or `agents/` |
  | T2 | each other path |

  The protected manifest (`go/internal/guards/integrity_surface.go`) is the one home of the T0 paths. An entry with `Constitution: true` is also a healing constitution path (`guards.IsHealingConstitution`): today `go/internal/gateclass/`, `go/internal/policy/healing.go` and `go/internal/phasecontract/diagnosis.go`. Spec §20.1 puts the catalog, the budgets and the tier table in T0, so the action catalog of X14 gets an entry with `Constitution: true`.
- **Patterns** (`Classify`). `Read` owns the git arguments that the parser needs: `-c core.quotePath=false diff --no-color --no-ext-diff -U0 --no-renames <base>` and `ls-files --others --exclude-standard -z`. Thus the two callers cannot drift. Each pattern reads only the changed lines of one file.

  | Pattern | Flag when |
  |---|---|
  | `protected-path` | the protected predicate is true for a changed or untracked path |
  | `removed-guard-test` | a guard test file, or a test with `Contract` in its name, loses a `func Test…(` line. No added line has that test name. |
  | `added-skip` | an existing `_test.go` file gets a `.Skip(`, `.Skipf(` or `.SkipNow(` call that the diff does not also remove |
  | `changed-threshold` | a removed and an added line set one name to two different numbers. The name has `threshold`, `budget`, `floor`, `halt`, `ratchet` or `coverage`. |
  | `stage-off-enforce` | a removed line sets a stage key to `enforce`, and an added line sets the same key to `shadow` or `off` |
  | `frozen-check` | a file under `go/acs/` changes or goes, and it is not under `go/acs/cycle<N>/` of the shipping cycle |
  | `healing-constitution` | the constitution predicate is true for a changed or untracked path |

  The patterns `changed-threshold` and `stage-off-enforce` skip a `_test.go` file, because a test fixture sets these values freely.
  A guard test file is in `guards/`, `repocontract/`, `sizeratchet/`, `rawgitratchet/` or `go/acs/regression/`. A `_test.go` file with `guard` in its name is also a guard test file.
- **A changed threshold, not only a lower one.** Spec §20.3 names a lower threshold. A raised halt count is also weaker (`zero_ship_halt` from 2 to 3), and the diff does not show the direction of the rule. Thus the classifier flags each change of such a number. The operator ships a correct change with `evolve ship --class manual`.
- **A new test is not a skip of a test that ran.** A skip in a new test file, or a skip that the diff moves, gives no flag.
- **The signal** (`ViolationEvent`). The code is `HEALING_I10_VIOLATION` in the module `healing`, with the kind `gate.rejected` and the severity WARN. `fields.patterns` and `fields.paths` name each flag.
- **Stage.** The classifier is at `enforce` and has no stage key. The build floor fails a flagged build. The ship check refuses a flagged cycle diff with `GATE_WEAKENED`. A manual ship does not run the check.
- **The build floor** gives the classifier no protected and no constitution predicate. `ProtectedSurfaceFloorChecks` already reports each protected path, and each constitution path is protected, so one path does not get two failures.
- **Ship** gives both predicates. Today the P2 check refuses a protected path first, so the two path patterns are a second line of defence there.

## Invariants

- Each pattern of spec §20.3 has one case in `TestGateClassifier_FlagsEachWeakeningPattern`.
- An ordinary change gives no flag (`TestGateClassifier_PassesAnOrdinaryChange`). A moved skip and a moved guard test give no flag (`TestGateClassifier_KeepsAMovedSkipAndAMovedTest`).
- The composed build floor runs the classifier (`TestProductionBuildFloorChecks_TheComposedFloorRunsTheGateClassifier`), and a cycle ship runs it before the audit binding (`TestVerifyClass_ACycleShipRunsTheGateClassifier`).
- The package and its two callers are on the protected list, so a cycle cannot change the classifier that grades it.

## Limits

- The classifier reads text, not Go syntax. A threshold in a call argument, or a stage value in a variable, gives no flag.
- The `frozen-check` pattern has no digest yet. Item `tdd-predicates-frozen-at-handoff` adds the digest of `go/acs/cycle<N>` at the TDD handoff. Until then, the classifier flags each change of a predicate package of another cycle, and the cycle can still change its own.
- The T1 judge rule is `cliroute.Decision.IndependentOf` ([internal-cliroute.md](internal-cliroute.md)). X16 calls it when it routes the audit of a T1 repair cycle.
- No production code calls `TierOf` yet. X16 routes a repair item by its tier: T0 to the operator, T1 and T2 to the loop.
- A moved test is a removed test whose name comes back in a guard test file or in the same directory. A test of the same name in an unrelated package does not excuse the removal.
