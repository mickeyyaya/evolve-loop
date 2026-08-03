package regressiontia

// apicover_named_test.go — the ADR-0069 repo-wide apicover gate's second half.
// Enrolling ./internal/regressiontia in go/.apicover-enforce is not enough:
// every exported symbol must be NAMED here in an assertion that EXECUTES it,
// or the enrolled-but-unnamed shape fails the gate.
//
// These are not restatements of regressiontia_test.go's contract tests. Each
// one exercises a path that contract file leaves cold — the enforce stage, real
// dependency resolution against this repository's own regression corpus, the
// scope-driven partition, and the marshalling round trip through Emit's real
// filesystem write.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestArtifactName_IsTheWorkspaceEvidenceFile names ArtifactName and asserts the
// value the audit phase and every operator reading a cycle workspace agree on.
func TestArtifactName_IsTheWorkspaceEvidenceFile(t *testing.T) {
	if ArtifactName != "acs-tia-shadow.json" {
		t.Errorf("ArtifactName = %q, want \"acs-tia-shadow.json\" — the audit phase and its reachability test key off this exact name", ArtifactName)
	}
}

// TestCompute_EnforceStageIsAlsoArmed names Compute on the one stage the
// contract tests do not reach. "enforce" is in the closed policy vocabulary, so
// it must compute (and therefore leave evidence) rather than fall into the
// dormant default and silently produce nothing on the very stage an operator
// promoted to.
func TestCompute_EnforceStageIsAlsoArmed(t *testing.T) {
	mod := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mod, "acs", "regression", "routing"), 0o755); err != nil {
		t.Fatal(err)
	}

	d := Compute("enforce", "", mod, nil)

	if d.Stage != "enforce" {
		t.Errorf("Compute(stage=enforce).Stage = %q, want \"enforce\" — a promoted stage must not fall into the dormant default", d.Stage)
	}
	if !reflect.DeepEqual(d.Selected, []string{"./acs/regression/routing"}) {
		t.Errorf("Selected = %v, want the discovered corpus", d.Selected)
	}
}

// TestListCorpusDeps_ResolvesTheRealCorpus proves dependency resolution
// actually EXECUTES against this repository: `go list -tags acs -test` over
// acs/regression/... The `-test` half is load-bearing and was found the hard
// way — a regression predicate package holds only _test.go files, so its plain
// .Deps is empty and a listing without -test reports the whole corpus as
// depending on nothing.
func TestListCorpusDeps_ResolvesTheRealCorpus(t *testing.T) {
	moduleDir := repoModuleDir(t)
	const pkg = "./acs/regression/apicover"

	raw, modPath := listCorpusDeps(moduleDir, []string{pkg})

	if modPath == "" {
		t.Fatalf("listCorpusDeps resolved no module path under %s", moduleDir)
	}
	if len(raw[pkg]) == 0 {
		t.Fatalf("listCorpusDeps returned no dependencies for %s — resolution silently produced nothing, which the fail-safe would mask as 'unresolvable' forever", pkg)
	}
	if !containsStr(raw[pkg], modPath+"/pkg/acsassert") {
		t.Errorf("deps of %s = %d entries without %s/pkg/acsassert — the predicate's own import edge is missing, so the listing is not reading the acs-tagged test files", pkg, len(raw[pkg]), modPath)
	}
}

// TestResolveDeps_DropsPredicatesThatReachOutsideTheImportGraph is the
// CORRECTNESS crux of this whole mechanism, and it is a real defect this build
// hit, not a hypothetical. go/acs/regression/apicover fails a cycle for adding
// an unenrolled internal package — by READING go/.apicover-enforce and shelling
// out to `go list`, importing none of the code it grades. Its static dependency
// set is disjoint from nearly every diff, so an import-graph selector marks it
// skippable on exactly the change it exists to catch. It must never be
// derivable, and therefore never a skip candidate.
func TestResolveDeps_DropsPredicatesThatReachOutsideTheImportGraph(t *testing.T) {
	moduleDir := repoModuleDir(t)
	const pkg = "./acs/regression/apicover"

	if _, derivable := resolveDeps(moduleDir, []string{pkg})[pkg]; derivable {
		t.Errorf("%s was classified as import-graph derivable, but it reads repo files and spawns `go list` — selection built on that would hide the very class this predicate guards", pkg)
	}
}

// TestCompute_NeverSkipsAnEscapeHatchPredicate is the same invariant at the
// production seam, through Compute against the REAL corpus with a real changed
// scope: whatever selection concludes, a file-reading/subprocess predicate
// stays in Selected, and the partition never loses a package.
func TestCompute_NeverSkipsAnEscapeHatchPredicate(t *testing.T) {
	moduleDir := repoModuleDir(t)

	// Empty repoRoot keeps ImporterClosure's `go list` out of this test: the
	// scope is supplied verbatim, so what is under test is selection, not
	// closure (TestChangedScope_ covers that).
	d := Compute("shadow", "", moduleDir, []string{"./internal/apicover/..."})

	if len(d.Selected)+len(d.WouldSkip) == 0 {
		t.Fatalf("Compute discovered no regression corpus under %s — the enumeration is looking in the wrong place", moduleDir)
	}
	if d.WouldSkipCount != len(d.WouldSkip) {
		t.Fatalf("WouldSkipCount = %d but len(WouldSkip) = %d", d.WouldSkipCount, len(d.WouldSkip))
	}
	if !containsStr(d.Selected, "./acs/regression/apicover") {
		t.Errorf("Selected = %v, want ./acs/regression/apicover retained — a predicate whose evidence lives outside the import graph is never safely skippable", d.Selected)
	}
	if containsStr(d.WouldSkip, "./acs/regression/apicover") {
		t.Errorf("WouldSkip = %v contains the predicate that guards unenrolled packages — selection is hiding exactly the class it must protect", d.WouldSkip)
	}
}

func repoModuleDir(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return filepath.Join(repoRoot, "go")
}

// TestSelect_ParentScopeEntryStillIntersects names Select on the prefix axis:
// a scope naming a parent directory ("./internal/...") must still hold every
// package beneath it, or a coarse scope would read as disjoint and mark
// everything skippable.
func TestSelect_ParentScopeEntryStillIntersects(t *testing.T) {
	patterns := []string{"./acs/regression/routing"}
	deps := map[string][]string{"./acs/regression/routing": {"./internal/router/..."}}

	selected, wouldSkip := Select(patterns, []string{"./internal/..."}, deps)

	if len(wouldSkip) != 0 {
		t.Errorf("wouldSkip = %v under the parent scope ./internal/..., want empty — a coarser scope may only ever select MORE", wouldSkip)
	}
	if !reflect.DeepEqual(selected, patterns) {
		t.Errorf("selected = %v, want %v", selected, patterns)
	}
}

// TestChangedScope_DropsNoInputUnderClosure names ChangedScope and pins the
// never-narrow invariant against a pattern with no importers at all.
func TestChangedScope_DropsNoInputUnderClosure(t *testing.T) {
	in := []string{"./internal/nosuchpackage/..."}
	if got := ChangedScope("", in); !reflect.DeepEqual(got, in) {
		t.Errorf("ChangedScope = %v, want the input %v retained — closure only ever widens", got, in)
	}
}

// TestEmit_RoundTripsEveryDecisionField names Emit and Decision (every field)
// and proves the JSON tags round-trip. A field that marshals but does not
// unmarshal reads to an operator as absent evidence.
func TestEmit_RoundTripsEveryDecisionField(t *testing.T) {
	d := Decision{
		Stage:           "enforce",
		ChangedPackages: []string{"./internal/router/...", "./internal/routingtest/..."},
		Selected:        []string{"./acs/regression/routing"},
		WouldSkip:       []string{"./acs/regression/apicover", "./acs/regression/buildselfcheck"},
		WouldSkipCount:  2,
	}

	path, err := Emit(t.TempDir(), d)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Error("emitted artifact does not end in a newline — it is read by line-oriented tooling")
	}
	var back Decision
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back, d) {
		t.Errorf("round trip = %+v, want %+v", back, d)
	}
}

// TestEmit_ReportsAnUnwritableWorkspace pins the loud half of Emit's contract:
// the audit caller chooses to swallow this error, which is only defensible
// because Emit actually RETURNS one.
func TestEmit_ReportsAnUnwritableWorkspace(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does", "not", "exist")
	if _, err := Emit(missing, Decision{Stage: "shadow"}); err == nil {
		t.Error("Emit into a nonexistent workspace returned nil error — the caller cannot distinguish written evidence from lost evidence")
	}
}
