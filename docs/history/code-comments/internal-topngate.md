# Comment history: `internal/topngate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/topngate/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported topngate symbol by identifier (apicover counts field
// access as "uses", not "names"). NewReviewer is the package's sole export;
// each assertion pins a REAL contract (the composite gate reviewer's blocking
// and fail-open behaviour), not a magic string. Helpers writeTriageReport /
// writeBuildReport are shared from reviewer_test.go in this package.
```

### `go/internal/topngate/apicover_named_test.go:35` — above `if res := enforce.Review(context.Background(), in); !res.Approve {`

```text
// Advisory since 2026-07-22: label drift approves at every stage (see
// reviewer_test.go rationale); the apicover contract here is the NAMED
// exercise of the reviewer types, not the old fatal policy.
```

### `go/internal/topngate/builder_authority_test.go:22` — above `func TestBuilderPromptNamesTopNAsSoleTaskAuthority(t *testing.T) {`

```text
// TestBuilderPromptNamesTopNAsSoleTaskAuthority is the AC-2 regression test:
// the Builder's own instructions must name triage-report.md's ## top_n as its
// sole task authority, demoting scout-report.md to background context only.
//
// Before this fix, agents/evolve-builder.md instructed the opposite: "Read
// task from workspace/scout-report.md" and "the `## Task: <slug>` line ...
// MUST be ... copied verbatim from the scout-report's `## Selected Tasks`" —
// the root cause of the cycle-640 wrong-task build
// (builder-task-binding-topn-gate, 7th recurrence: cycles 282, 310, 522, 575,
// 577, 599, 640).
```

### `go/internal/topngate/gate.go:1` — above `package topngate`

```text
// Package topngate implements the build->audit BLOCKING gate that enforces the
// Builder's task-slug binding to triage-report.md's ## top_n commitment (inbox
// builder-task-binding-topn-gate, 8th recurrence of the wrong-task-build
// defect: cycles 282, 310, 522, 575, 577, 599, 640, 645). The root cause is two
// competing task-identity sources — scout-report.md's ## Selected Tasks vs
// triage-report.md's ## top_n — that can diverge; when Builder binds to the
// wrong one, audit grades the delivered (wrong) diff while the committed task's
// ACS suite fails, burning a whole audit+ship phase pair on a doomed cycle.
// This gate makes triage ## top_n the single authority at the build->audit
// transition. It mirrors internal/evalgate's gate/reviewer shape.
```

### `go/internal/topngate/gate.go:74` — above `return "label drift (advisory since 2026-07-22): build-report labels its task '" + claimed + "' but triage committed {" …`

```text
// Label drift is ADVISORY, not fatal (2026-07-22, cycles 916 + 1012): both
// recorded rejections discarded CORRECT work whose report merely described
// the committed task under a different label — two LLM outputs string-
// compared. The dispatch is plan-driven by construction (the lane exists
// BECAUSE triage committed these ids), so the binding authority is the
// committed set, not the prose. The non-empty reason with block=false
// routes through the reviewer's single structured logf seam (testable);
// real fraud protection (deliverable file-scope vs the committed item's
// declared scope) is the queued construction-level check.
```

### `go/internal/topngate/gate.go:86` — above `type tddScopeGate struct{}`

```text
// tddScopeGate binds the TDD phase's AUTHORED set to triage's ## top_n
// commitment (inbox tdd-topn-binding-gate; cycle-660, 3rd recurrence). The
// defect: triage commits an empty ## top_n, TDD reads scout-report.md instead
// of triage-report.md and still authors RED scaffolds for a slug triage
// explicitly declined; build then honours the empty top_n correctly and chokes
// on the orphan scaffolds. topNBindingGate covers build->audit; this covers
// TDD->Build, the transition one phase earlier.
```

### `go/internal/topngate/gate.go:100` — above `func (tddScopeGate) check(in core.ReviewInput) (string, bool) {`

```text
// check reconciles complete multi-member declarations first. For legacy
// zero/single-member commitments it blocks certain orphan authoring and fails open on every
// ambiguity (missing/unparseable report, no claimed slug, nothing authored):
//
//  1. empty committed top_n + a non-empty authored set — triage committed
//     nothing, so the only compliant TDD deliverable is a no-op. FATAL.
//  2. non-empty committed top_n + an authored set claimed for a slug with zero
//     overlap against it — ADVISORY, mirroring the build-side gate's
//     label-drift carve-out (see topNBindingGate.check).
//
// The two cases differ in kind, not degree: under an empty top_n there is no
// committed item the authored files could be a differently-labelled response
// to, so case 1 is unambiguous and stays fatal. Case 2 compares two
// LLM-authored strings for equality against a set that is non-empty by
// construction — the same false-rejection risk #348 closed one phase later.
```

### `go/internal/topngate/gate.go:122` — above `return "", false`

```text
// no test-report.md → nothing to bind → fail open (cycle-1620 audit L1)
```

### `go/internal/topngate/gate.go:124` — above `if committed := normalizedSlugs(core.ContractTaskIDs(in.Workspace)); len(committed) > 1 {`

```text
// Multi-member lanes need complete set equality before Build starts; a
// single matching label cannot account for the second member (cycle-1480).
// The committed set is the CONTRACT's (core.ContractTaskIDs: the lane pin,
// else the triage decision's top_n, minus deferrals) — the same ids the
// Task Contract handed TDD — never triage-report.md's markdown ## top_n,
// whose working-id decomposition sub-ids would falsely block a lane whose
// TDD declared exactly what it was contracted for. The markdown top_n
// stays the authority for the legacy zero/single-member paths below. Known
// asymmetry: a complete multi-member reconciliation returns here, before
// the single-member file-scope advisory (inbox
// multi-member-file-scope-advisory).
```

### `go/internal/topngate/gate.go:150` — above `return fileScopeAdvisory(in.Workspace, claimed, authored), false`

```text
// In-lane by label. The construction-level check runs ALONGSIDE the
// label check on exactly this path (cycle-1111): the label proves
// nothing about what was actually authored, so compare the authored
// files against the committed item's declared scope. Advisory only.
```

### `go/internal/topngate/gate.go:157` — above `return "label drift (advisory since 2026-07-23): TDD authored test file(s) {" + strings.Join(authored, ", ") +`

```text
// Label drift is ADVISORY here for the same reason it is on the build side
// (cycles 916 + 1012): the lane exists BECAUSE triage committed these ids, so
// the committed set — not the TDD report's prose — is the binding authority,
// and a differently-labelled RED scaffold for the committed item is correct
// work. The non-empty reason at block=false still routes through the
// reviewer's single structured logf seam. (Out-of-lane labels keep reporting
// label drift; the file-scope advisory below covers the in-lane path.)
```

### `go/internal/topngate/gate.go:169` — above `func fileScopeAdvisory(workspace, slug string, authored []string) string {`

```text
// fileScopeAdvisory compares the files TDD actually authored against the file
// scope the committed item declares in scout-report.md, and returns a non-empty
// ADVISORY reason only when both sets are non-empty and share nothing. Every
// ambiguity — no scout-report.md, the committed slug absent from it, no
// declared scope, nothing authored — returns "" (fail open), matching this
// gate family's convention. It is advisory rather than fatal because a
// legitimate deliverable can touch a shared helper or an incidental file that
// scout never named; shadow evidence decides whether it ever becomes fatal.
```

### `go/internal/topngate/gate.go:321` — above `if json.Unmarshal([]byte(strings.Join(block, "\n")), &payload) == nil && (len(payload.Slugs) > 0 || len(payload.TestFile…`

```text
// The declaration is the first fence that DECLARES something: a
// JSON fence carrying neither slugs nor testFiles (RED-run output,
// a status object) is not the handoff and must not shadow the
// real one that follows (cycle-1620 audit M1).
```

### `go/internal/topngate/gate_test.go:1` — above `package topngate`

```text
// Package topngate implements the build->audit BLOCKING gate that enforces
// Builder task-slug binding to triage-report.md's ## top_n (inbox
// builder-task-binding-topn-gate, weight 0.96, 7th recurrence: cycles 282,
// 310, 522, 575, 577, 599, 640). This file drives topNBindingGate.check
// directly (white-box, same package — mirrors internal/evalgate/gates_test.go
// for materializationGate).
```

### `go/internal/topngate/gate_test.go:60` — above `ws := t.TempDir()`

```text
// POLICY CHANGE (operator-directed, cycles 916 + 1012): both recorded
// fatal rejections discarded CORRECT work whose report merely labeled
// the committed task differently — two LLM strings compared. The lane
// is plan-driven by construction, so label drift WARNs loudly and the
// binding authority is the committed set. Scope-based fraud
// verification is the queued construction-level replacement.
```

### `go/internal/topngate/gate_test.go:154` — above `func TestTDDScopeGate_LabelDriftIsAdvisory(t *testing.T) {`

```text
// TestTDDScopeGate_LabelDriftIsAdvisory is the cycle-1073 crux. tddScopeGate's
// case 2 (non-empty committed top_n + an authored slug with zero overlap) is
// the SAME "two LLM-authored strings compared for exact equality" defect that
// #348 (cbd088a1) converted to an advisory for the sibling topNBindingGate,
// one phase later in the pipeline. Two recorded false rejections (cycles 916,
// 1012) discarded correct work over a label; the triage->TDD transition carries
// the identical risk and must warn, not block.
```

### `go/internal/topngate/gate_test.go:277` — above `type scoutTask struct {`

```text
// --- cycle-1111: file-scope binding (tdd-file-scope-binding-check) -----------
//
// The slug check above compares two LLM-authored PROSE labels, which is why
// both drift cases are advisory. The gate's own comments (gate.go:73-75,
// 132-135) name the missing complement as "the queued construction-level
// check": the committed item's DECLARED file scope vs what TDD actually
// authored. A deliverable can carry the right label and touch nothing the
// committed item names — today that passes silently.
//
// This second check runs ALONGSIDE the slug check on the in-lane path (it
// never replaces or tightens it) and is ADVISORY (block=false), matching this
// gate family's fail-open convention: a scope mismatch has legitimate causes
// (shared helper, incidental file), so it warns and lets shadow evidence
// decide whether it ever becomes fatal.
```

### `go/internal/topngate/gate_test.go:336` — above `func TestTDDScopeGate_FileScopeDriftIsAdvisory(t *testing.T) {`

```text
// TestTDDScopeGate_FileScopeDriftIsAdvisory is the cycle-1111 crux: the label
// matches the committed item exactly, so every existing check passes silently,
// yet the authored file lives in a tree the committed item never names. That
// is the wrong-work shape the slug check provably cannot see, and the one the
// gate's own comments defer to this check.
```

### `go/internal/topngate/reviewer_test.go:13` — above `ws := t.TempDir()`

```text
// POLICY CHANGE 2026-07-22 (cycles 916 + 1012): label drift is advisory —
// even at enforce, a drifted label WARNs and passes; the committed set is
// the binding authority. See gate_test.go's advisory case for rationale.
```

### `go/internal/topngate/reviewer_test.go:100` — above `func TestReplayCycle640Shape(t *testing.T) {`

```text
// TestReplayCycle640Shape is a direct regression test for the 7th (and
// intended-final) recurrence of this defect: cycle 640 triage committed
// exactly "statefile-rmw-flock-single-source", TDD authored predicates for
// it, but Builder instead implemented "fix-token-resolver-transcript-source"
// (the OTHER fleet lane's goal). Audit graded the delivered diff PASS 0.93
// while the ACS suite bound to the committed task returned FAIL, red_count=9,
// ship_eligible=false (.evolve/runs/cycle-640/retrospective-report.md +
// stage-lesson-1.yaml). This test replays that exact shape and asserts the
// gate now blocks BEFORE audit ever ran, instead of consuming a full
// audit+ship phase pair on a cycle doomed from the build->audit transition.
```

### `go/internal/topngate/reviewer_test.go:111` — above `ws := t.TempDir()`

```text
// HISTORICAL REPLAY, updated 2026-07-22: cycle-640's wrong-lane build now
// passes with a loud WARN instead of a fatal block — the 916/1012
// evidence showed the fatal form discarded CORRECT work over label drift
// between two LLM strings, while the cycle-640 fraud class is covered by
// the queued scope-verification (deliverable files vs committed item
// scope), which catches REAL wrong-work regardless of its label.
```

### `go/internal/topngate/scope_audit_probe_test.go:10` — above `func TestTDDScopeGate_CompleteDeclarationAfterUnrelatedJSONFenceProceeds(t *testing.T) {`

```text
// Cycle-1620 audit M1 (the salvage's discovered defect): the rewritten
// handoff parser accepted ANY JSON fence inside "## Handoff to Builder" as the
// declaration, so a complete multi-member declaration that followed an
// unrelated JSON fence (RED-run output, a status object) was shadowed and the
// gate falsely blocked a compliant TDD deliverable. The declaration is the
// first fence that actually DECLARES something.
```

### `go/internal/topngate/scope_audit_probe_test.go:53` — above `func TestTDDScopeGate_MissingReportFailsOpenForMultiMemberLane(t *testing.T) {`

```text
// Cycle-1620 audit L1: readTDDScope's ok was never consulted on the
// multi-member path, so an ABSENT test-report.md (the documented fail-open
// ambiguity) blocked a two-member lane as "missing both members".
```

### `go/internal/topngate/scope_reconciliation_test.go:3` — above `import (`

```text
// scope_reconciliation_test.go — the red-first contract for inbox item
// `multi-slug-lane-scope-reconciliation` (cycle-1620).
//
// The defect (cycle-1480, batch-20260815c wave-2, recurred cycle-1483). A lane
// bundled two slugs: `minted-phase-verdict-contract-unsatisfiable` and
// `dead-api-sweep`. TDD minted a cycle-wide predicate suite covering BOTH; the
// Builder's deliverable contract bound only the FIRST. Nothing reconciled the
// two scopes, so the lane ran the full ~12-phase spine and FAILed at audit with
// slug 2 entirely undelivered. Verbatim audit H1: "TDD minted a cycle-wide
// predicate suite covering both slugs while the Builder contract bound only the
// first slug; nothing reconciles the two scopes."
//
// The contract these tests freeze. When triage's ## top_n commits TWO OR MORE
// members, the TDD deliverable's DECLARED member set must EQUAL the committed
// set; a missing committed member is a CERTAIN scope mismatch and must abort at
// the TDD->Build boundary with a reason containing the marker `scope-mismatch`
// and NAMING every omitted member. A one-member commitment keeps today's
// fail-open / label-drift-advisory behavior exactly (no regression).
//
// The declared member set is stated BOTH ways a TDD report can state it — the
// "## Task:" header (comma-separated when plural) and the "## Handoff to
// Builder" JSON's slugs[] — so these tests bind to the DECLARATION, never to
// one syntax. JSON shown inside an OUTER `~~~markdown` example fence is
// illustration, not declaration, and must never be read as the handoff
// (recovery review 2026-09-09, the false-accept control).
```

### `go/internal/topngate/scope_reconciliation_test.go:96` — above `func TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks(t *testing.T) {`

```text
// TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks is the cycle-1480
// shape verbatim: two committed members, one declared. Build must not start.
```

### `go/internal/topngate/scope_reconciliation_test.go:126` — above `func TestTDDScopeGate_TwoSlugDeclarationOrderIsIrrelevant(t *testing.T) {`

```text
// TestTDDScopeGate_TwoSlugDeclarationOrderIsIrrelevant pins SET equality, not
// list equality: the members are unordered work items, and a gate that keyed on
// order would false-block correct work (the cycles 916/1012 destruction class).
```

### `go/internal/topngate/scope_reconciliation_test.go:168` — above `func TestTDDScopeGate_OuterFenceFakeCompleteHandoffStillBlocks(t *testing.T) {`

```text
// TestTDDScopeGate_OuterFenceFakeCompleteHandoffStillBlocks is the false-accept
// control from the 2026-09-09 recovery review. The report DOCUMENTS a complete
// handoff inside an outer `~~~markdown` example fence while actually declaring
// one member. Illustration is not declaration: the lane must still block.
```

### `go/internal/topngate/tdd_declaration_prompt_test.go:10` — above `func TestTDDPromptDeclaresHandoffSlugs(t *testing.T) {`

```text
// TestTDDPromptDeclaresHandoffSlugs — cycle-1620 salvage (architecture
// CRITICAL 2): the TDD->Build scope gate reads `slugs[]` from the handoff JSON
// and a comma-separated `## Task:` header, so the persona that PRODUCES the
// report must instruct exactly that shape, bound to the `## Task Contract`
// block's ids. Without this the only reports that pass the multi-member gate
// are the ones tests author themselves.
```
