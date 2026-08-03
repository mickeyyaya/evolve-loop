# ADR-0082 — Regression test-impact selection, staged off → shadow → enforce

- **Status:** Accepted (shadow stage landed cycle-1260; enforce NOT recommended — see Finding)
- **Date:** 2026-08-04
- **Supersedes / relates:** ADR-0069 (apicover CI-parity gate), cycle-1250 router/routingtest miss,
  cycle-1253 `changedpkgs.ImporterClosure` (shipped green with zero callers)

## Context

The EGPS Go regression corpus (`go/acs/regression/<sub>`, 40 packages today) runs in full on every
cycle. The standing inbox item `egps-regression-tia-selection` (P1, weight 0.91, 3rd live instance)
asked for deterministic test-impact selection over that corpus so a cycle runs `O(change)` rather
than `O(corpus)`.

Two prior findings shape the design:

1. **cycle-1250.** A change confined to `internal/router` never selected `internal/routingtest`,
   which imports it and holds the keystone parity invariant. Forward-only changed-package derivation
   hid that class and kept main red for 5 commits. `changedpkgs.ImporterClosure` was written to fix
   it — and then shipped with **no production caller**, so the fix has never executed once.
2. **Selection is the only mechanism here that can hide a regression.** Under-selecting is a missed
   red that reaches main; over-selecting is only slower. Every fail-safe must therefore resolve
   toward RUNNING a predicate.

## Decision

### 1. A separate package, not a change to the gate runner

`go/internal/acssuite` is protected control plane (`guards.ProtectedSurfaceManifest`:
`{"/go/internal/acssuite/", "the gate runner"}`) — a cycle may not edit the gate that grades it.

That constraint yields the better architecture rather than an obstacle to route around: the shadow
stage **changes nothing about what the gate runs** (that is what shadow means), so it needs no code
in the runner at all. It is observability computed *beside* the suite by the suite's own production
caller — `internal/phases/audit.generateACSVerdict`. Only the future `enforce` stage, which actually
skips packages, must live inside `acssuite`, and that change is human-gated
`evolve ship --class manual` outside a cycle by construction.

### 2. Config-as-code, closed vocabulary, fail-safe default

`.evolve/policy.json` gains an optional `regression_tia` block (`{"stage": "off"|"shadow"|"enforce"}`),
resolved through `policy.RegressionTIAConfig()` / `policy.RegressionTIAStageFor(root)`. No flag, no Go
literal. Absent block, unreadable file, malformed JSON, or any unrecognized stage all resolve to
`"off"`. The checked-in `policy.json` carries no block, so the live production path is dormant and
byte-identical to its pre-change self.

### 3. Fail-safes, all pointing the same direction

| Unknown | Resolution |
|---|---|
| Underivable changed set (no repo, git error, fleet `index.lock` race) | empty scope ⇒ skip nothing |
| Empty changed scope | skip nothing (unknown impact ≠ zero impact) |
| Unresolved dependency data for a package | that package always runs |
| Unrecognized policy stage | `off` |
| Unwritable evidence sink | swallowed; the audit never fails on observability |

`ChangedScope` routes through `changedpkgs.ImporterClosure`, giving that function its first
production caller and closing the cycle-1250 class.

### 4. Evidence artifact

At `shadow`/`enforce` the decision lands at `<workspace>/acs-tia-shadow.json` — stage, changed
packages, selected, would-skip, and a would-skip count that is a *projection* of the list.

## Finding — why `enforce` is NOT recommended today

Building this surfaced a defect the design had to absorb. **A regression predicate's evidence
usually lives outside the import graph.** `go/acs/regression/apicover` is the gate that fails a cycle
for adding an unenrolled internal package — and it does so by *reading* `go/.apicover-enforce` and
shelling out to `go list`. It imports none of the code it grades. Its static dependency set is
disjoint from nearly every diff, so a naive importer-graph selector marks it **skippable on exactly
the change it exists to catch**.

`resolveDeps` therefore classifies any corpus package whose test closure touches an *escape hatch*
(`os`, `os/exec`, `os/user`, `syscall`, `net`, `net/http`) as **underivable**, and underivable means
always-run.

Measured against this repository (scope `./internal/apicover/...`):

```
corpus=40  selected=40  would_skip=0
```

Every predicate in the corpus reads files or spawns processes. **Import-graph selection cannot
safely narrow this corpus at all as it stands.** That is the honest shadow result, and it is exactly
the evidence a shadow stage exists to collect: arming `enforce` on the naive graph would have
silently disarmed the corpus. Promoting past `shadow` requires first giving predicates a declared
impact surface (an explicit manifest of the files/commands each one observes) — not a wider import
graph.

## Consequences

- Live behavior is unchanged: no block ⇒ `off` ⇒ no computation, no artifact, no new failure mode on
  the path that grades every cycle.
- `changedpkgs.ImporterClosure` now has a production caller and executes under `shadow`.
- The dependent follow-on `egps-regression-tia-boundary-fullsweep` (an enforce-stage safety valve) is
  premature until the manifest work above exists; it stays deferred.
- New package `go/internal/regressiontia` is enrolled in `go/.apicover-enforce` with an
  `apicover_named_test.go` naming every exported symbol (ADR-0069 dual edit).
