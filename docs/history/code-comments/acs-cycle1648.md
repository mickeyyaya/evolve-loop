# Comment history: `acs/cycle1648`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1648/harness_test.go:3` — above `package cycle1648`

```text
// harness_test.go — the cycle-1648 predicate harness: the real `evolve` binary
// built once, the fixture corpus (five valid pairs, five named exclusions, two
// out-of-sample entries), and the Markdown report-shape matchers. Split from
// predicates_test.go so the contract file stays readable; both carry the acs
// build tag so neither compiles into the normal suite.
```

### `go/acs/cycle1648/harness_test.go:33` — above `func TestMain(m *testing.M) {`

```text
// TestMain compiles go/cmd/evolve ONCE for the whole predicate package (the
// cycle-1439 buildEvolve shape, hoisted so nine CLI predicates don't relink nine
// times under fleet load). `go -C` pins the module root to this worktree so the
// binary is built from the tree the ship lands, never from process cwd.
```

### `go/acs/cycle1648/harness_test.go:136` — above `func shadowJSON(cycle int, narrative, chain, shipped string, overrodeBy []string) string {`

```text
// shadowJSON renders an audit-chain-shadow.json in the exact shape
// internal/auditchain.ShadowRecord marshals (cycle-1640's record is the model).
// shipped=="" omits the field (omitempty), the legacy-record shape.
```

### `go/acs/cycle1648/harness_test.go:219` — above `func fixtureCorpus(t *testing.T) corpus {`

```text
// fixtureCorpus is the canonical corpus every representation predicate reads.
//
//	VALID PAIRS (5)                narrative chain   gate  shipped  overrode_by
//	  101 clean pass                PASS      PASS    PASS  PASS     —
//	  102 force-override (AC4 edge) PASS      PASS    FAIL  FAIL     explanation review
//	  103 WARN under a red gate     WARN      absent  FAIL  FAIL     EGPS
//	  104 two gates                 PASS      PASS    FAIL  FAIL     EGPS + apicover
//	  105 narrative FAIL, gates     FAIL      FAIL    PASS  FAIL     —   (shipped_verdict
//	      green                                                          absent → dossier)
//	EXCLUSIONS (5)
//	  106 dossier, run dir with no shadow            → missing-shadow
//	  107 dossier, shadow is not JSON                → malformed-shadow
//	  108 shadow, no dossier                         → missing-dossier
//	  109 shadow, dossier is truncated JSON          → malformed-dossier
//	  110 dossier + shadow, fail-reason is not JSON  → malformed-fail-reason
//	OUT OF SAMPLE (neither pair nor exclusion)
//	  cycle-1623.reset-20260911T180942 (non-canonical run name, valid shadow)
//	  cycle-111.json (a dossier with no run dir — no narrative can exist)
//
// Expected matrix (narrative × gate): (PASS,PASS)=1 (PASS,FAIL)=2 (WARN,FAIL)=1
// (FAIL,PASS)=1. Expected classes: EGPS=2 (103,104), verdict-conflict=2 (102,103).
```

### `go/acs/cycle1648/predicates_test.go:3` — above `package cycle1648`

```text
// Package cycle1648 materializes the acceptance criteria for the one task this
// fleet lane committed (lane-scope.json todo_ids; triage-report.md ## top_n):
// `auditor-calibration-report`. Per R9.3 nothing here binds to the deferred
// `auditor-persona-rubric-adjustment` item — it gets ZERO predicates.
//
// The ask (inbox 2026-07-30T09-02-00Z-auditor-calibration-report.json): mine
// the corpus of (auditor narrative, deterministic gate) verdict pairs that the
// verdict-conflict machinery already records — knowledge-base/cycles/cycle-N.json
// dossiers beside .evolve/runs/cycle-N/audit-chain-shadow.json (+ optional
// audit-fail-reason.json) — into a deterministic Markdown agreement matrix and
// per-defect-class breakdown, through a read-only `evolve audit calibration`
// command that takes explicit corpus/output paths and fails loudly on bad input.
// Reproduced RED in .evolve/runs/cycle-1648/bug-reproduction-report.md: the
// dispatcher rejects `audit` as an unknown command (exit 2).
//
// Vocabulary pinned here (the contract the Builder implements — see
// test-report.md ## Handoff to Builder for the full Markdown shape):
//
//	Narrative  shadow.narrative_verdict               (PASS|WARN|FAIL)
//	Chain      shadow.chain_verdict, preserved as-is  (PASS|WARN|FAIL|absent)
//	Gate       the DETERMINISTIC SHIP-GATE outcome: FAIL when shadow.overrode_by
//	           is non-empty (a gate forced the verdict), else PASS. This — not
//	           chain_verdict — is what the inbox calls "the gate", because the
//	           question is "narrative PASS while a gate was red"; cycle-1640
//	           (narrative PASS, chain PASS, shipped FAIL by override) must land
//	           in a DISAGREEMENT cell, never in (PASS,PASS).
//	Shipped    shadow.shipped_verdict; the dossier's final_verdict when absent
//	Sample     every `runs/cycle-<N>` directory (that is where a narrative can
//	           exist); its dossier `cycle-<N>.json` is the required counterpart.
//	           A missing/malformed counterpart is a NAMED exclusion, never a pair.
//	           Non-canonical run names (cycle-N.reset-…, archive/) are not cycles.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 deterministic Markdown matrix + defect-class breakdown   → 002, 006, 007
//	AC2 narrative/gate/shipped kept distinct; malformed/missing
//	    pairs are counted exclusions, never agreement              → 002, 003, 005
//	AC3 CLI takes explicit corpus/output paths; non-zero on bad    → 001, 004, 008
//	AC4 tests include the negative pair case + the force-override → 003, 005, 011
//	House rule 1 (new package → repo-wide apicover gate)           → 010
//	House rule 2 (wiring proof through the production caller)      → 001, 009
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 004 (six invalid
// inputs must each exit non-zero AND name the offender AND leave no report
// behind), 005 (five malformed/missing shapes must each be a named exclusion and
// the matrix total must not absorb them). EDGE/OOD: 005 (empty corpus, a
// `cycle-N.reset-…` run dir, an orphan dossier), 003 (the force-override shape,
// with a CONTROL corpus that differs only in the override so a report that
// ignores overrode_by is caught by the delta, not just by a magic string).
// SEMANTIC: reachability (001), representation (002), override semantics (003),
// input rejection (004), exclusion accounting (005), determinism (006),
// defect-class normalisation (007), path defaulting (008), the real corpus (009),
// the apicover obligation (010), the Builder's own unit tests (011) — eleven
// distinct behaviours, not one restated.
//
// No grep-only predicates (cycle-85 ban): every predicate runs the REAL
// `evolve` binary (built once from this worktree's go/cmd/evolve in TestMain)
// through its dispatcher — the production caller — and asserts on exit code,
// stderr, and the emitted report; 010 executes the coverage toolchain and the
// in-process apicover gate; 011 binds the Builder's unit tests by their
// `--- PASS:` markers (one named package, -run-narrowed — the cycle-976/1587
// precedent). The single FileContains in 010 is an explicit config-check waiver
// and auxiliary to the executed gate.
//
// Reachability probe (cycle-644 rule): this package imports only
// internal/apicover (a leaf) and pkg/acsassert; acs/cycle1648 is itself a leaf,
// so no package-qualified pin here can close an import cycle.
```

### `go/acs/cycle1648/predicates_test.go:363` — above `func TestC1648_009_RealCorpusProducesInterpretableReport(t *testing.T) {`

```text
// TestC1648_009 — house rule 2 on the REAL corpus (scout build plan step 4):
// the state root's runs beside the committed dossiers produce a non-empty,
// interpretable report in which cycle-1640's recorded force-override (narrative
// PASS → shipped FAIL by the explanation review) is listed as such. SKIPs — never
// false-reds — when the runtime state is absent (archived run, bare export).
```

### `go/acs/cycle1648/predicates_test.go:452` — above `func assertBoundTestsPass(t *testing.T, pkg string, names ...string) {`

```text
// assertBoundTestsPass runs ONE named package narrowed to exactly the bound
// test names (`-run ^(A|B)$`) from the module root and requires each
// `--- PASS:` marker — the cycle-976/1587 binding shape. cmd/evolve is a
// known-slow suite; the -run narrowing is what keeps this predicate cheap.
```
