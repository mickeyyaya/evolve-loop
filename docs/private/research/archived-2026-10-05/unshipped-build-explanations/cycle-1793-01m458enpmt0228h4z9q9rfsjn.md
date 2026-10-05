# Build Explanation — Cycle 1793

## Build Binding
- Cycle: 1793
- Base SHA: 9761f517835683b2568022238370efd987d65cef

## Summary
The `evalqualitycheck` lint now flags an ACS predicate that cannot pass on any tree, and the host runs that lint automatically at the end of every tdd phase. A predicate is flagged in two cases. In the first, a self-reporting positive `acsassert` primitive (`FileContains`, `FileMatchesRegex`, `FileExists`, `JSONFieldEquals`) is used as a bare failure condition. In the second, a negated positive primitive's failure text demands absence. This is the cycle-1488 `TestC1488_003` class, which burned cycles 1488, 1492 and 1495. Cycle 1788 built the lint and wired it into the hand-run CLI, then failed on its build report; this cycle carries the lint forward and wires it into the host reviewer. The CLI wiring was withdrawn because `go/internal/cli/guardcmd` is a protected control-plane path (ADR-0064) that a cycle may not edit.

## Rationale
The lint has to reach the agent before build dispatch to be worth anything. Before this cycle it ran only when an agent hand-ran `evolve eval quality-check -predicates`, so an unsatisfiable predicate still reached build. The existing `evalgate` reviewer already runs Gate D (the flaky-shape lint) at end-of-tdd over the same predicate directory. Adding a parallel Gate E there reuses the production seam (`go/cmd/evolve/cmd_cycle.go` composes `evalgate.NewReviewer`) and adds no new plumbing. The lint stays advisory and never blocks. A false positive would otherwise fail a healthy cycle, and the corpus calibration below is one sweep, not a per-class breakdown.

## Changed Areas
- `go/internal/evalqualitycheck/unsatisfiable.go` — the lint. It adds `FileExists` and `JSONFieldEquals` to the positive primitives, each with a remedy fitted to its primitive. A bare-condition finding now requires the primitive's TB to be a `testing` parameter of the function or a closure, so swallowing probes are exempt. Only a direct failure-reporter statement of the `if` body counts as a failure condition. The string literals of one failure call are joined before the absence-intent test.
- `go/internal/evalqualitycheck/unsatisfiable_test.go` — table-driven unit tests for each flagged and each exempt shape, plus the per-primitive remedy.
- `go/internal/evalqualitycheck/apicover_named_test.go` — names every exported lint symbol in an executing assertion (carried from cycle 1788).
- `go/internal/evalgate/unsatisfiable.go` — Gate E, `unsatisfiable-predicate-shape`. It runs at tdd, never blocks, and returns a reason on every path: findings, clean, no package, or stand-down.
- `go/internal/evalgate/unsatisfiable_test.go` — tests for the gate's outcomes, its phase scope, its wiring into `NewReviewer`, and the end-to-end reviewer log at enforce.
- `go/internal/evalgate/reviewer.go` — composes Gate E into `NewReviewer`'s gate list, the production seam.
- `go/internal/evalgate/gate_wiring_registry_test.go` — registers Gate E's wiring pin, which the registry test requires of every composed gate.
- `go/acs/cycle1793/predicates_test.go` — this cycle's ACS predicates, authored by the tdd phase.
- `.evolve/evals/acs-absence-primitive-and-unsatisfiability-lint.md` — the task's eval, retargeted by tdd from cycle 1788's predicates to cycle 1793's.
- `docs/architecture/packages/internal-evalqualitycheck.md` — documents the lint's primitive set, its three false-positive cuts with the corpus shapes behind each, its one production caller (Gate E), and the CLI wiring left as console work.
- `docs/architecture/packages/internal-evalgate.md` — documents Gate E and updates the gate count.
- `docs/private/research/archived-2026-10-05/superseded-predicate-packages/cycle1788/predicates_test.go` — archives cycle 1788's predicates, which the cycle-1793 package supersedes.
- `docs/private/research/archived-2026-10-05/unshipped-build-explanations/cycle-1788-01m3spwpfvp5qqqyycdbnpn0ct.md` — archives cycle 1788's explanation draft, which never shipped.

## Design Decisions
- **Self-reporting is decided by the TB argument's declared type.** The lint flags a primitive only when its TB is an identifier declared as `*testing.T`, `*testing.B`, `*testing.F` or `testing.TB`. Narrower rules would miss subtests and helpers; wider ones (any `acsassert.TB`) would flag the swallowing `recordingTB` probes in `go/acs/cycle1702`. Shadowing a `t` parameter with a probe is not tracked; the corpus has no instance.
- **Only an unconditional failure counts.** `go/acs/regression/cycle99` uses `if FileExists(t, p) { …nested check… }` as a guard. Collecting every nested `Errorf` produced a false positive there, so only direct statements of the `if` body count.
- **The message is judged whole.** `go/acs/cycle25` and `cycle49` split messages across `+` concatenation, and fragment-by-fragment intent testing produced false positives on "remains"/"retained". Joining the literals of one call keeps the call's "not"/"must" in scope.
- **Gate E mirrors Gate D rather than generalising it.** A shared "lint this cycle's predicate dir" helper would touch Gate D's tested messages for a two-gate saving.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1793`: 9 pass and 1 fail. The failure is `TestC1793_006`, which needs the protected CLI wiring. The passing set includes the live cycle-1488 bytes red-first (002) and the whole-corpus sweep (008): 530 packages, 555 `.go` files linted, 0 findings.
- `go test -count=1 ./internal/evalqualitycheck ./internal/evalgate`: all PASS. Predicate 009 also runs these under `-race` with `go vet`.
- The full `evolve acs suite --cycle 1793` and `evolve selfcheck build` results are in the cycle's build report.

## Compatibility
The CLI is unchanged. Gate E never blocks, so no previously passing tdd phase can now fail. The lint flags strictly fewer shapes than cycle 1788's version on bare conditions (swallowing TBs and guards are exempt) and strictly more primitives (`FileExists`, `JSONFieldEquals`).

## Limitations
The lint is syntactic. It does not resolve a TB passed through a variable or a struct field, follow helpers across functions, or track shadowing. The absence-intent test is a keyword heuristic tuned on the current corpus. `evolve eval quality-check -predicates` does not run the lint; that wiring is console work in `internal/cli/guardcmd`. Gate E stays advisory until a per-class false-positive breakdown justifies enforcement.
