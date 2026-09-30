# Comment history: `internal/reachabilityprobe`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/reachabilityprobe/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — repo-wide apicover public-API coverage (House
// Rule 1 / ADR-0069's second gate): names and exercises every exported symbol
// of this package (ImportGraph, CallSite, Violation, CheckCallSite,
// BuildImportGraph, FrozenTestFiles, ExtractFrozenPins, CheckFrozenPins) by
// identifier.
```

### `go/internal/reachabilityprobe/apicover_named_test.go:15` — above `func TestExportedSymbols_Named(t *testing.T) {`

```text
// TestExportedSymbols_Named names every exported identifier of this package
// and pins the two load-bearing contracts: CheckCallSite detects the
// cycle-644 shape (non-nil Violation with a populated Cycle and Error()) and
// leaves an acyclic pin unchanged (nil).
```

### `go/internal/reachabilityprobe/apicover_named_test.go:84` — above `func TestFrozenPinExports_Named(t *testing.T) {`

```text
// TestFrozenPinExports_Named names and exercises the frozen-pin seam
// (FrozenTestFiles, ExtractFrozenPins, CheckFrozenPins) end to end against a
// real throwaway module carrying the cycle-644 shape: storage imports core, so
// a frozen test pinning `storage.UpdateStateMap(` into a core file demands
// core -> storage -> core.
```

### `go/internal/reachabilityprobe/frozenpins.go:1` — above `package reachabilityprobe`

```text
// frozenpins.go closes the deterministic half of inbox item
// tdd-structural-test-reachability-probe (root cause cycle-644): turning the
// probe from a library an agent MAY call by hand into one the phase-gate
// pipeline calls automatically.
//
// CheckCallSite (reachabilityprobe.go) answers the question for ONE call site
// the caller already knows. The missing piece was deriving those call sites
// from the artefact the TDD phase actually produces: a test-report.md whose
// handoff JSON freezes a set of test files (`doNotModifyTests: true`), each
// pinning package-qualified call sites into named production files. This file
// walks that path end to end — deliverable -> frozen files -> pins -> import
// graph -> violations — so `evolve phase verify tdd` can refuse a permanently
// unsatisfiable acceptance criterion BEFORE the build phase burns on it.
//
// Fail-open discipline throughout: an unreadable file, an underivable module,
// a `go list` failure or an unresolvable package identifier yields NO
// violation. Only a compiler-provable cycle is a confirmed violation, because
// a false HALT here taxes every cycle.
```

### `go/internal/reachabilityprobe/frozenpins.go:41` — above `func FrozenTestFiles(reportPath string) ([]string, error) {`

```text
// FrozenTestFiles returns the test files a tdd deliverable froze: the handoff
// JSON's testFiles when doNotModifyTests is true, and nil when it is false
// (unfrozen tests are not permanent commitments, so their pins are not the
// cycle-644 failure mode). Paths are returned exactly as written — worktree
// relative, slash separated. An unreadable deliverable is an error; a
// deliverable with no parseable handoff block yields nil, nil.
```

### `go/internal/reachabilityprobe/frozenpins.go:88` — above `var (`

```text
// A structural pin is a single line naming BOTH the production file the
// requirement lands in and the package-qualified call site required inside it
// — the `acsassert.FileContains(t, "go/internal/core/state.go",
// "storage.UpdateStateMap(")` idiom, which is literally the cycle-644
// artefact. Both halves are Go string literals, so scanning literals (not raw
// line text) keeps the surrounding assertion call from matching as a pin.
```

### `go/internal/reachabilityprobe/frozenpins.go:250` — above `func CheckFrozenPins(worktreeRoot string, frozenTestFiles []string) ([]Violation, error) {`

```text
// CheckFrozenPins reports every frozen pin in frozenTestFiles that would close
// an import cycle — the cycle-644 shape — by resolving each pin against the
// real import graph of the module owning its pinned source file. It returns no
// violation for pins it cannot prove: an unresolvable package identifier, a
// module whose graph `go list` will not produce, or a pinning package absent
// from that graph (CheckCallSite's own rule: absence of evidence is not
// evidence of a cycle).
```

### `go/internal/reachabilityprobe/frozenpins.go:292` — above `func resolvePackage(graph ImportGraph, ident, pinning string, aliases map[string]string) (string, bool) {`

```text
// resolvePackage maps a bare package identifier as written at a call site to a
// full import path present in graph, preferring the candidate sharing the
// longest prefix with the pinning package (the nearest neighbour in the same
// module) and breaking ties lexically so the verdict is deterministic.
//
// Precedence is exact path, then base name, then alias — and the ORDER is the
// load-bearing part. The identifier is compiled in the PINNED PRODUCTION file's
// scope, not the frozen test file's, so an alias declared in the test file is a
// hint about intent, never an authoritative binding. Consulting it first (as
// this resolver did until cycle-1248) makes the map able to SUPPRESS: one import
// line in the frozen test file rebinding `storage` to some benign package
// silently redirects a `storage.UpdateStateMap(` pin away from the real
// internal/storage that base-name matching would have found, and an alias
// binding anything outside the module graph killed resolution outright. Either
// turns the gate blind to the exact cycle-644 shape it exists to catch — and a
// frozen test file is agent-authored, so that suppression is one plausible-
// looking edit away.
//
// Consulted LAST, the alias can only ever ADD reach: it resolves identifiers
// (`st`, `lf`) that match no package's base name and would otherwise fail open,
// and it can no longer displace a real graph package that does match. An alias
// binding a package absent from graph resolves nothing, which is the same
// fail-open verdict as no alias at all.
```

### `go/internal/reachabilityprobe/frozenpins_test.go:3` — above `import (`

```text
// frozenpins_test.go — the DURABLE (non-acs) regression guard for cycle-1246's
// task `reachabilityprobe-alias-resolution`.
//
// Gap (scout-report.md Key Finding #2): resolvePackage maps the bare identifier
// written at a pinned call site to a full import path by matching
// path.Base(pkg) == ident. An identifier introduced by an IMPORT ALIAS matches
// no package's base name, so the pin is silently skipped — a genuine cycle-644
// shape written as
//
//	import st "example.com/fixture/internal/storage"
//	... acsassert.FileContains(t, "go/internal/core/state.go", "st.UpdateStateMap(")
//
// fails OPEN today and the permanently-unsatisfiable acceptance criterion sails
// through `evolve phase verify tdd`.
//
// Where the alias binding can live: NOT in the pinned production file. If
// core/state.go already imported storage while storage imports core, the module
// is already cyclic and `go list` refuses to produce a graph at all (the gate
// then fails open on infra ambiguity, by design). The one place the binding can
// exist while the module still lists cleanly is the FROZEN TEST FILE that
// carries the pin — `go list -deps -json` ignores _test.go imports — and that
// is exactly where a structural test that also exercises the symbol declares it.
// So: aliases are resolved from the import block of the frozen test file the pin
// was extracted from.
```

### `go/internal/reachabilityprobe/frozenpins_test.go:61` — above `func aliasFixtureWorktree(t *testing.T) string {`

```text
// aliasFixtureWorktree builds a throwaway worktree whose go/ subdirectory is a
// real, `go list`-resolvable module carrying both shapes the gate must tell
// apart:
//
//	internal/storage  imports internal/core  → pinning it inside a core file
//	                                           is the cycle-644 shape.
//	internal/leafutil imports nothing        → pinning it inside a core file
//	                                           closes no cycle.
//
// Every frozen test file below pins a call site into go/internal/core/state.go;
// the pins differ only in how the referenced package's identifier is bound.
```

### `go/internal/reachabilityprobe/frozenpins_test.go:87` — above `writeAliasFixtureFile(t, wt, aliasCyclicFrozenTest,`

```text
// ALIASED cycle-644 shape: the binding `st` -> internal/storage is declared
// in this frozen test file's own import block.
```

### `go/internal/reachabilityprobe/frozenpins_test.go:143` — above `func TestCheckFrozenPins_AliasedImportResolved(t *testing.T) {`

```text
// TestCheckFrozenPins_AliasedImportResolved is the CRUX predicate (scout Task 1
// verifiableBy): a frozen pin written through an import alias must be resolved
// against the real import graph and reported as a violation, not silently
// skipped.
//
// RED today: resolvePackage matches path.Base(pkg) == "st", no package in the
// fixture module is named "st", so ok=false and CheckFrozenPins returns zero
// violations on a compiler-provable cycle-644 shape.
```

### `go/internal/reachabilityprobe/reachabilityprobe.go:1` — above `package reachabilityprobe`

```text
// Package reachabilityprobe is a deterministic compiler-probe check for the
// TDD structural-test-freeze step (inbox
// tdd-structural-test-reachability-probe, weight 0.92, root cause cycle-644).
//
// Cycle-644 froze a `doNotModifyTests:true` structural test that pinned
// `storage.UpdateStateMap(` inside a `core`-package file, while `storage`
// already imported `core` — a compiler-proven import cycle. The acceptance
// criterion was permanently unsatisfiable and burned the whole cycle before
// anyone noticed the shape was unbuildable.
//
// CheckCallSite answers, from a package import graph alone (no `go build`
// invocation required — the caller supplies the graph, typically derived from
// `go list -deps`), whether pinning a package-qualified call site would
// introduce exactly that shape: the referenced package already (transitively)
// imports the pinning package, so the pinning package importing the referenced
// package back would be an import cycle.
```

### `go/internal/reachabilityprobe/reachabilityprobe_test.go:3` — above `import (`

```text
// reachabilityprobe_test.go — table-driven coverage for CheckCallSite,
// closing the inbox tdd-structural-test-reachability-probe (weight 0.92)
// acceptance criterion 3: "a reachable pinned call site passes the check
// unchanged (no false positives - table-driven)". The prior coverage in
// apicover_named_test.go exercised exactly one 2-node cycle and one acyclic
// case as hardcoded assertions, not a table; this file adds the
// multi-hop/edge shapes the package's own doc comment (cycle-644) promises.
```

### `go/internal/reachabilityprobe/reachabilityprobe_test.go:25` — above `name: "direct_2node_cycle",`

```text
// cycle-644 shape, re-asserted here in table form per the AC.
```

### `go/internal/reachabilityprobe/resolvepackage_test.go:3` — above `import "testing"`

```text
// resolvepackage_test.go covers resolvePackage's BASE-NAME FALLBACK — the branch
// every frozen pin written the ordinary way (`storage.UpdateStateMap(`, no
// import alias) flows through, and the one branch of the resolver that had zero
// tests before cycle-1248. The alias branch is covered from the outside in
// frozenpins_test.go; this file drives the resolver directly because the
// interesting inputs are graph SHAPES (two packages sharing a base name, two
// equidistant candidates) that would take a whole throwaway module each to
// stage through CheckFrozenPins.
//
// ImportGraph is a plain map[string][]string, so a graph literal here is the
// real input type `go list` produces — no fake, no seam.
```

### `go/internal/reachabilityprobe/resolvepackage_test.go:30` — above `func resolveGraph() ImportGraph {`

```text
// resolveGraph is the shared fixture graph. Edges are irrelevant to
// resolvePackage (it maps identifier -> path; CheckCallSite walks the edges), so
// they stay empty except where a reader would expect the cycle-644 shape.
```

### `go/internal/reachabilityprobe/resolvepackage_test.go:155` — above `func TestResolvePackage_AliasNeverSuppressesBaseNameMatch(t *testing.T) {`

```text
// TestResolvePackage_AliasNeverSuppressesBaseNameMatch is the cycle-1248
// regression guard for the audit defect: resolvePackage consulted the frozen
// test file's alias map FIRST and unconditionally.
//
// The alias map comes from a test file, but the identifier is compiled in the
// PRODUCTION file's scope — so the alias is a hint, not an authority. Consulted
// first it could SUPPRESS: one import line in an agent-authored frozen test file
// rebinding `storage` to a benign package redirected a genuine cycle-644 pin
// away from the real internal/storage, and an alias pointing outside the module
// graph vetoed resolution outright. Both made the gate blind to the shape it
// exists to catch. Consulted last, the alias can only ADD reach.
```
