---
score_cap:
  - criterion: "gcWorkspaceSweep in go/cmd/evolve/cmd_gc.go is <=50 lines (sizeratchet ceiling; was 60)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_002_GCWorkspaceSweepWithinSizeRatchetLimit ./acs/cycle1753/..."
  - criterion: "The project-root resolution block is extracted into a resolveGCProjectRoot helper that gcWorkspaceSweep actually calls, not merely deleted or inlined elsewhere"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_004_GCWorkspaceSweepCallsExtractedProjectRootResolver ./acs/cycle1753/..."
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists cmd/evolve.gcWorkspaceSweep (the ratchet only tightens)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_006_OffendersJSONNoLongerListsGCWorkspaceSweep ./acs/cycle1753/..."
  - criterion: "resolveGCProjectRoot has its own unit tests covering: empty+mutating (refused, exit 1, stderr names the refusal), empty+dry-run (falls back to a readable cwd, exit 0), and a non-empty value (passthrough, exit 0, no stderr) regardless of dryRun"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestResolveGCProjectRoot_' ./cmd/evolve/..."
  - criterion: "Existing evolve gc CLI behavior is unchanged: all pre-existing cmd/evolve gc workspace-sweep tests still pass (dry-run preview, explicit apply, unmerged-branch preservation, mutating-run refusal without --project-root)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestRunGC_' ./cmd/evolve/..."
  - criterion: "cmd_gc.go is gofmt clean and go vet ./cmd/evolve/... is clean after the extraction"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1753_007_TouchedFilesAreGofmtClean|TestC1753_008_TouchedPackageVetsClean' ./acs/cycle1753/..."
  - criterion: "The explanation document and both task evals describe the new helper test files as they are declared (no table-driven claim for a file with no case table, no wrong test count) and the document still describes cmd_gc_projectroot_test.go"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_009_ExplanationDocAndEvalsDescribeNewTestFilesAsDeclared ./acs/cycle1753/..."
  - criterion: "The explanation document's Verification section reports the cycle predicate count go/acs/cycle1753 declares, and its evolve acs suite line carries the (cycle= regression= red-team=) breakdown with a cycle scope equal to that count, figures that add up (green+red+skip = total = cycle+regression+red-team) and red=0 under verdict=PASS"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_010_ExplanationDocVerificationCountsMatchDeclaredPredicates ./acs/cycle1753/..."
---

# Eval: shrink-gc-workspace-sweep

> Pins the size-ratchet extraction of `gcWorkspaceSweep`
> (`go/cmd/evolve/cmd_gc.go`), 60 lines at cycle-1753 baseline, down to the
> `sizeratchet` ceiling of 50 by extracting its `--project-root` resolution
> block — the mutating-run refusal, the dry-run-only cwd fallback, and the
> explicit-value passthrough — into a standalone `resolveGCProjectRoot`
> helper. The deliberate asymmetry documented on `runGC`'s own flag help
> text (an explicit operator `evolve gc` applies the sweep; the in-loop hook
> defaults to shadow) must survive unchanged: refusing a mutating run with no
> `--project-root`, falling back to cwd only under `--dry-run`, and passing
> an explicit value straight through in either mode. Source: cycle-1753
> scout/triage carve-out of `go/internal/sizeratchet/offenders.json` lane
> work, task `shrink-gc-workspace-sweep`.
>
> Source incident: this is a mechanical, low-risk refactor task class; the
> score-cap pins both the structural outcome (line count, offenders.json
> entry removed) and each of the three resolver branches individually, since
> a naive extraction can easily collapse the dry-run-only cwd fallback into
> an unconditional one (silently permitting a mutating sweep with no
> explicit aim). Cycle-1753 audit round 1 (H1) failed the build on a
> narrative defect: the explanation document misdescribed the structure of
> the resolver's four standalone Test functions, a label copied from the TDD
> report; the narrative-accuracy entry checks that wording against the test
> file's AST. Round 2 (H1) failed again on the same document: its suite line
> kept the round-1 totals after a predicate was added, contradicting its own
> predicate count; the verification-counts entry ties both figures to the
> predicates go/acs/cycle1753 declares.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | gcWorkspaceSweep <=50 lines | 8/10 | `go test -run TestC1753_002...` |
| wiring | gcWorkspaceSweep calls resolveGCProjectRoot | 8/10 | `go test -run TestC1753_004...` |
| offenders-cleanup | offenders.json entry removed | 7/10 | `go test -run TestC1753_006...` |
| helper-tested | resolveGCProjectRoot refused/fallback/passthrough covered | 7/10 | `go test -run 'TestResolveGCProjectRoot_'` |
| behavior-preserved | pre-existing evolve gc tests still pass | 8/10 | `go test -run 'TestRunGC_'` |
| format-vet-clean | gofmt + go vet clean on cmd/evolve | 6/10 | `go test -run 'TestC1753_007...\|TestC1753_008...'` |
| narrative-accuracy | doc + evals describe the new test files as declared | 6/10 | `go test -run TestC1753_009...` |
| verification-counts | doc Verification predicate and suite counts match the declared predicates | 6/10 | `go test -run TestC1753_010...` |
