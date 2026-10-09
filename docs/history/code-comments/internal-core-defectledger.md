# Comment history: `internal/core/defectledger`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## ADR-0124 Q2: the ledger leaf declares reportdoc

### `go/internal/core/defectledger/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under core (ADR-0103 unit 09
// §2): stdlib plus the five named internal packages, never internal/core,
// carryover or phasecontract (the compiler is the cycle guard; this is the
// leaf-ness declaration — signalcenter/importgraph_test.go idiom). The leaf
// also never writes stderr: its failure modes are codes, not prose lines.
```

## phase 3d r2 (defectledger, router)

### `go/internal/core/defectledger/defectledger.go:1` — above `package defectledger`

```text
// Package defectledger is unit 09 of the component breakdown (ADR-0103): the
// audit phase's anti-laundering record — the defect/prescription ledger's
// schema and vocabulary (the ONE home audit, carryover and the adoption seeder
// project from), its Center-free reader and writer, and the continuation
// disposition gate: EMIT (a rejecting audit's OPEN rows), ARM (is this cycle a
// continuation, and which record says so — the workspace manifest or the
// root-owned registry keyed by the lane-scope pin), GRADE (the disposition
// diff against the ancestor's ledger, written back BEFORE the verdict),
// PREFLIGHT (the artifact-level MISSING / INCOMPLETE finding) and the prompt
// block that tells a continuation its inherited ids.
//
// A Ledger takes its two hidden couplings as explicit collaborators — the
// lane-identity reader and the citation-resolution policy (a Strategy the
// audit package keeps) — and the Signal Center through an accessor. It never
// writes stderr, never persists beyond the two artifacts it owns, takes no
// context, and reports its failure modes as audit.warning under module audit
// while the graded diagnostics stay the byte-identical wire the dossier and the
// bookkeeping regrade read. Design:
// docs/architecture/decomposition/09-defectledger.md.
```

### `go/internal/core/defectledger/defectledger.go:117` — above `type Ledger struct {`

```text
// Ledger owns the gate. Its collaborators are explicit and REQUIRED at
// construction: a forgotten reader would silently re-open the cycle-1285 F2
// deletion hole, a forgotten resolver would grade FIXED without touching
// disk — so a nil one panics at first use rather than disarming.
```

### `go/internal/core/defectledger/dispositions.go:48` — above `func (l *Ledger) Preflight(req Request, ancestorCycle int, ancestor []Entry, claims map[string]Entry) []cyclestate.Diagn…`

```text
// Preflight grades the disposition ARTIFACT's completeness against the
// ancestor's OPEN set, before the per-id reconcile runs (cycle-1342 F4): the
// per-id switch blocks correctly but never failed loudly BY NAME on the file
// itself. Silent — necessarily, as the anti-no-op half — whenever the ancestor
// carries no OPEN entries or every one of them is covered; a pre-flight that
// fires on every continuation proves nothing.
```

### `go/internal/core/defectledger/emit_test.go:11` — above `func TestEmit_AppendsOpenRowsDedupedByText(t *testing.T) {`

```text
// Test 10 — two Emits of the same rejection append one row each, OPEN, the
// existing rows kept: the G1 bytes (captured on 8e8f080f) both times.
```

### `go/internal/core/defectledger/helpers_test.go:3` — above `import (`

```text
// helpers_test.go — the leaf's test fixtures: a Ledger over stub collaborators
// reporting into a recording Center, the continuation lineage the goldens were
// captured on (8e8f080f, zz_golden_capture_test.go), and the golden readers.
```

### `go/internal/core/defectledger/helpers_test.go:93` — above `type fixture struct {`

```text
// fixture is <root>/.evolve/runs/cycle-1270 continuing cycle-1255 — the
// lineage the goldens were captured on.
```

### `go/internal/core/defectledger/helpers_test.go:169` — above `type scenario struct {`

```text
// scenario is one row of diagnostics.golden.json (captured on 8e8f080f).
```

### `go/internal/core/defectledger/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 09 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/core/defectledger/lineage.go:13` — above `func (l *Ledger) arm(req Request) (continuation.Continuation, bool, Verdict) {`

```text
// arm decides whether this cycle is a continuation and which record
// establishes that — ONE decision with one set of rules, separate from
// grading (the cycle-1285 F2 defect was entirely in the arming half while
// every defense lived in the grade). It returns the ancestor binding and true
// when the grade must run; otherwise the Verdict to return as-is (zero for a
// plain cycle, blocked for a corrupt manifest or a manifest/registry
// disagreement). A registry-only binding arms the grade AND blocks: the
// Verdict carries the missing-manifest finding for Reconcile to prepend.
```

### `go/internal/core/defectledger/lineage.go:27` — above `v := blockedOn(fmt.Sprintf("defect ledger: continuation manifest is unreadable (%s) — a continuation cannot be graded ag…`

```text
// cycle-1285 F2: one byte of garbage in a 0644 file in the graded
// agent's own workspace used to retire every inherited defect. The
// manifest decides whether the gate runs AT ALL, so it blocks, like an
// unparseable defect-dispositions.json already did.
```

### `go/internal/core/defectledger/lineage.go:115` — above `func vouchedCycles(cont continuation.Continuation, ancestor Doc) []int {`

```text
// vouchedCycles is the lineage a graded, unblocked reconcile vouches for —
// the immediate ancestor plus the ledger's origin cycle — so the closure-
// citation gate's prose demand demotes to advisory for claims WITHIN it
// (cycle-1502). Vouch ONLY when the ancestor ledger exists with entries: the
// unblocked missing/empty branch verified NOTHING, and a deleted ancestor
// ledger must not convert the closure gate's backstop into a demotion. Reads
// the Doc the grade already loaded — no second read.
```

### `go/internal/core/defectledger/prompt_test.go:13` — above `func promptFixture(t *testing.T) (root, ws, ancestorWS string) {`

```text
// promptFixture is the cycle-1431 continuation of cycle-1425 the G5 golden
// was captured on: one OPEN row with embedded newlines, one FIXED row, one
// 250-rune OPEN row.
```

### `go/internal/core/defectledger/reconcile.go:89` — above `func mergeInherited(current, ancestor []Entry, claims map[string]Entry, resolve func(string) (bool, string)) ([]Entry, […`

```text
// mergeInherited MERGES the ancestor's rows onto this workspace's ledger. The
// inherited rows are rebuilt from the ANCESTOR on every pass and their status
// derives ONLY from the claims, never from the row already in this workspace
// (cycle-1282 DEF-1: `current` is a file the graded agent may write, so a
// pre-planted FIXED row must satisfy nothing). What `current` contributes is
// this cycle's own emitted rows. Entries transition; they are never deleted.
```

### `go/internal/core/defectledger/reconcile_test.go:14` — above `func TestReconcile_DiagnosticsAreByteIdenticalToTheGoldens(t *testing.T) {`

```text
// G4 replay — every diagnostic scenario captured on 8e8f080f
// (diagnostics.golden.json) rebuilt on the same fixture and graded through the
// leaf: the messages, severities, order, block decision and lineage are
// byte-identical.
```

### `go/internal/core/defectledger/schema.go:35` — above `const (`

```text
// MaxEntries and TextMaxRunes bound the ledger against an agent-authored
// verdict sentinel carrying thousands of defects or a megabyte-long defect
// line (cycle-1282 DEF-6). The ledger is re-read and re-written on every
// Classify, so unbounded growth is quadratic work on the audit hot path as
// well as an unreadable artifact. Overflow is RECORDED as a synthetic entry,
// never silently dropped — a cap that erases defects would be the laundering
// primitive wearing a resource-limit costume.
```

### `go/internal/core/defectledger/schema.go:57` — above `const DispositionsSchemaExample = '{"dispositions": [`

```text
// DispositionsSchemaExample is the ONE canonical defect-dispositions.json
// example, surfaced inline on rejection (cycle-1403 Task 3): the agent
// re-authoring the file on the next dispatch does not read Go. It is
// byte-for-byte the same document (as JSON) as the examples in
// agents/evolve-auditor.md and docs/architecture/continuation-defect-ledger.md
// — the audit package's defect_ledger_doc_example_test.go holds the three in
// sync, so there is one schema with three projections.
```

### `go/internal/core/defectledger/schema.go:159` — above `func ID(text string) string {`

```text
// ID derives an entry id from the defect TEXT alone. A positional id re-binds
// the same id string to different text as soon as a list is reordered, so a
// disposition keyed on it closes something other than what it claims —
// laundering by renumbering. A content hash is stable across cycles, chains
// and re-emissions. SIXTEEN bytes, not four (cycle-1282 DEF-3): the preimage
// is chosen by the agent authoring the sentinel, and a 32-bit id is a ~2^32
// brute force away from a benign defect that collides with an inherited
// CRITICAL; the merge additionally cross-checks TEXT per id.
```

### `go/internal/core/defectledger/schema.go:194` — above `type dispositionEvidence struct {`

```text
// dispositionEvidence is the wire type of a disposition's `evidence` field: a
// single citation STRING or a JSON ARRAY of citation strings (cycle-1399: the
// auditor cited `["a.go:1", "b.go:2"]`, a string-typed field refused the whole
// document and the gate blocked a correct claim). Tolerance is widened for the
// SHAPE only, never the CLAIM: an object, number or bool is still rejected
// outright rather than degraded to "" (cycle-1285 F2 — a silent degrade is the
// gate's cheapest bypass), and every citation must still resolve on its own.
```
