# Comment history: `internal/continuation`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d2

### `go/internal/continuation/authority.go:3` — above `import (`

```text
// authority.go — the operator-authority gate every path that can DROP a
// continuation binding shares.
//
// A binding is the lineage the defect-ledger gate reads as anti-tamper
// evidence (ADR-0085/0089, cycle-1285), and `evolve continuation release`
// shipped in cycle 1515 with no gate at all: its FlagSet declared only
// -project-root and the command dropped straight from arg-parsing to an
// unconditional release. The phase guard (internal/guards/phase.go) denies
// in-process Agent/Task dispatch during a cycle but says nothing about a Bash
// invocation of the evolve binary, so ANY Bash-capable process — an in-cycle
// agent included — could erase a live scope's lineage. The registry's
// "ORCHESTRATOR-side only" authority invariant (DeleteRegistryEntry's contract)
// was widened by omission, not by decision.
//
// The check lives HERE rather than in cmd/evolve so a future in-process caller
// cannot route around the gate the CLI honours — a private copy in the command
// is exactly the drift that produced audit cycle-1507's H2.
```

### `go/internal/continuation/continuation.go:1` — above `package continuation`

```text
// Package continuation is the single source for the ADR-0076 slice C
// continuation binding: the record that lets a FAILed cycle's preserved,
// snapshot-committed work be resumed by a later cycle instead of restarted
// cold. Three consumers share this one type — the orchestrator (writes the
// per-cycle manifest at the preserve decision), the inbox mover (stamps
// released items from the manifest, transactionally with the release), and the
// batch loader (parses the stamp back off the item) — so the schema can never
// drift between writer and readers.
//
// Work resumes; grades do not: a continuation carries no verdict state, and
// every phase re-runs on adoption.
```

### `go/internal/continuation/continuation.go:33` — above `const ManifestName = "continuation-manifest.json"`

```text
// ManifestName is the per-cycle workspace artifact the orchestrator writes at
// the preserve decision and the inbox mover reads at release time. Exported
// (ADR-0103 unit 09) so the audit teardown hold, the citation self-cite
// denylist and the defect ledger name this file through the owner, never by
// a literal of their own.
```

### `go/internal/continuation/continuation_test.go:48` — above `func TestManifestName_IsTheFileTheManifestRoundTripsThrough(t *testing.T) {`

```text
// TestManifestName_IsTheFileTheManifestRoundTripsThrough — ADR-0103 unit 09
// review fold (F5): the manifest's file name is exported so its three outside
// consumers (the audit teardown hold, the citation self-cite denylist and the
// ledger's prompt-degrade path field) name the SAME file WriteManifest
// publishes and ReadManifest loads — a rename here fails to compile there.
```

### `go/internal/continuation/registry.go:3` — above `import (`

```text
// registry.go — ADR-0076 slice C, G2: the scope-id-keyed continuation registry.
// G1 binds preserved work to an INBOX CLAIM (the stamp on
// .evolve/inbox/processing/cycle-N/<item>.json), so a lane whose scope came
// from the wave planner rather than a claim had nothing to stamp — cycle-1078's
// preserved snapshot was orphaned for exactly that reason. This is the second
// scope-identity class: a root-owned map of lane-scope todo id → the SAME
// Continuation value the manifest carries. One schema, two identity classes; a
// registry-only field or a forked format would reopen the drift this package
// exists to prevent.
```

### `go/internal/continuation/registry.go:123` — above `func DeleteRegistryEntry(projectRoot, scopeID string) error {`

```text
// DeleteRegistryEntry releases scopeID's binding unconditionally. The missing
// half of the registry lifecycle (2026-08-10 stall: entries were immortal, so
// a binding whose snapshot had landed or whose worktree was reaped re-armed
// the defect-ledger gate's out-of-band check on every future lane of that
// scope — the absorbing-FAIL state, cycles 1412/1418). Deleting an absent
// scope is a clean no-op; an empty scope id is rejected to mirror
// WriteRegistryEntry. Callers are ORCHESTRATOR-side only — lane agents have
// no write path to the root-owned registry, which is what keeps the gate's
// cycle-1285 anti-tamper property intact. When the release is conditional on
// WHICH ancestor the binding names, use DeleteRegistryEntryIfCycle — a
// read-then-delete across two locks loses a sibling lane's concurrent rebind.
```

### `go/internal/continuation/registry.go:206` — above `func RedactHostPaths(c Continuation) Continuation {`

```text
// RedactHostPaths returns c with the operator's home directory collapsed to
// "~". The preserved-pointer annotations written by the retire/consume paths
// ride the ship commit into TRACKED .evolve/inbox items on a public remote, and
// Worktree/FindingsPath are absolute host paths that carry the account name
// (audit cycle-1507 M1). The tilde form stays operator-usable and stays a
// stable salvage pointer; SnapshotSHA/BaseSHA/Branch — the fields salvage
// actually resolves work from — are untouched. An unresolvable home is a
// no-op rather than a failure: redaction must never block a release.
```

### `go/internal/continuation/registry.go:248` — above `func AppendReleased(doc map[string]any, c Continuation, reason string) []any {`

```text
// AppendReleased returns doc's released_continuations[] with binding c
// appended — the ONE shape every consumption path writes (ship's in-commit
// consume, the operator `evolve inbox consume`, and the consumed-corpus
// reconciler), so the salvage pointer a release preserves can never drift
// between routes. Host paths are redacted here because consumed items are
// TRACKED and ride commits to the public remote (audit cycle-1507 M1).
```

### `go/internal/continuation/registry_delete_test.go:3` — above `import (`

```text
// registry_delete_test.go — pins DeleteRegistryEntry, the missing half of the
// registry lifecycle (2026-08-10 stall investigation: 84 immortal entries,
// ~60 pointing at dead state, no delete API anywhere). A binding the adopter
// declines must be releasable, or every lane on that scope auto-FAILs at the
// defect-ledger gate's out-of-band check forever (cycles 1412/1418 — the
// absorbing-FAIL state).
```

### `go/internal/continuation/registry_operator_test.go:52` — above `func TestRedactHostPaths(t *testing.T) {`

```text
// TestRedactHostPaths pins the M1 fix (audit cycle-1507): released pointers ride
// the ship commit into TRACKED .evolve/inbox items on a public remote, so the
// absolute host paths must collapse to "~" — while the refs salvage actually
// resumes from (snapshot, base, branch) stay byte-identical, since redacting
// those would turn a privacy fix into a data-loss bug.
```

### `go/internal/continuation/registry_test.go:3` — above `import (`

```text
// registry_test.go — ADR-0076 slice C, G2 (cycle-1104): the scope-id-keyed
// continuation registry. G1 binds preserved work to an INBOX CLAIM (the stamp
// on .evolve/inbox/processing/cycle-N/<item>.json). Cycle-1078's failing lane
// (`chain-boundary-loop`) had its scope come from the wave planner, not a
// claim, so there was no item file to stamp and the preserved snapshot was
// orphaned. The registry is the second scope-identity class: a root-owned
// <projectRoot>/.evolve/continuation-registry.json mapping lane-scope todo id →
// the SAME Continuation value the manifest carries.
//
// The hard invariant these tests pin: ONE schema. The registry stores
// continuation.Continuation verbatim — no forked manifest format, no
// registry-only fields — so writer and every reader can never drift.
```
