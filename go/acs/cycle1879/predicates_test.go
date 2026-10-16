//go:build acs

package cycle1879

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1879_001_ScopeDeltaIsASubcommandOfCycle(t *testing.T) {
	w := newWave(t)
	w.change(t, inScopeFile)
	r := scopeDelta(t, w.projectRoot, "--cycle", strconv.Itoa(laneCycle))
	if strings.Contains(r.stderr, "unknown subcommand") {
		t.Fatalf("RED: evolve cycle does not recognise scope-delta:\n%s", r)
	}
	if r.code != exitClean {
		t.Errorf("RED: scope-delta on an in-scope fixture exited %d, want %d:\n%s", r.code, exitClean, r)
	}
}

func TestC1879_002_UndeclaredPathIsListedAndExits1(t *testing.T) {
	w := newWave(t)
	w.change(t, inScopeFile, undeclaredFile)
	r := scopeDelta(t, w.projectRoot, "--cycle", strconv.Itoa(laneCycle))
	if r.code != exitOutOfScope {
		t.Errorf("RED: an undeclared changed path exited %d, want %d:\n%s", r.code, exitOutOfScope, r)
	}
	if !strings.Contains(r.stdout, undeclaredFile) {
		t.Errorf("RED: the human report does not list the undeclared path %s:\n%s", undeclaredFile, r)
	}
	if lineWith(r.stdout, inScopeFile, "opportunistic") {
		t.Errorf("RED: the declared path %s is reported as out of scope:\n%s", inScopeFile, r)
	}
}

func TestC1879_003_SiblingLanePathIsCrossLane(t *testing.T) {
	w := newWave(t)
	w.change(t, inScopeFile, siblingFile, unrelatedFile)
	r := scopeDelta(t, w.projectRoot, "--cycle", strconv.Itoa(laneCycle), "--json")
	if r.code != exitOutOfScope {
		t.Errorf("RED: a sibling lane's path exited %d, want %d:\n%s", r.code, exitOutOfScope, r)
	}
	report := decodeReport(t, r)
	sibling, ok := entryFor(report, siblingFile)
	if !ok || sibling.Class != "cross-lane" {
		t.Errorf("RED: %s in the same-wave sibling's scope is %+v, want class cross-lane:\n%s", siblingFile, sibling, r)
	}
	unrelated, ok := entryFor(report, unrelatedFile)
	if !ok || unrelated.Class == "cross-lane" {
		t.Errorf("RED: %s belongs to a lane of another wave (goal_hash) and must not be cross-lane, got %+v:\n%s", unrelatedFile, unrelated, r)
	}
	if report.Summary.ByClass["cross-lane"] != 1 {
		t.Errorf("RED: summary by_class[cross-lane]=%d, want 1:\n%s", report.Summary.ByClass["cross-lane"], r)
	}
}

func TestC1879_004_InScopeAndClosureOnlyExits0(t *testing.T) {
	w := newWave(t)
	w.change(t, inScopeFile, declaredDoc, closureFile)
	r := scopeDelta(t, w.projectRoot, "--cycle", strconv.Itoa(laneCycle), "--json")
	if r.code != exitClean {
		t.Fatalf("RED: in-scope plus closure changes exited %d, want %d:\n%s", r.code, exitClean, r)
	}
	report := decodeReport(t, r)
	if report.Summary.Carved != 0 || report.Summary.Refused != 0 || report.Summary.Kept != 3 {
		t.Errorf("RED: summary %+v, want kept=3 carved=0 refused=0:\n%s", report.Summary, r)
	}
	closure, ok := entryFor(report, closureFile)
	if !ok || closure.Class != "closure" {
		t.Errorf("RED: %s is %+v, want class closure (cycle-artifact rule):\n%s", closureFile, closure, r)
	}
}

func TestC1879_005_JSONEmitsSummaryAndEntries(t *testing.T) {
	w := newWave(t)
	w.change(t, inScopeFile, undeclaredFile)
	r := scopeDelta(t, w.projectRoot, "--cycle", strconv.Itoa(laneCycle), "--json")
	report := decodeReport(t, r)
	if report.Cycle != laneCycle {
		t.Errorf("RED: json cycle=%d, want %d", report.Cycle, laneCycle)
	}
	if report.Summary.ByClass["in-scope"] != 1 || report.Summary.ByClass["opportunistic"] != 1 {
		t.Errorf("RED: summary by_class=%v, want in-scope=1 opportunistic=1:\n%s", report.Summary.ByClass, r)
	}
	if report.Summary.Kept != 1 || report.Summary.Carved != 1 {
		t.Errorf("RED: summary %+v, want kept=1 carved=1", report.Summary)
	}
	stray, ok := entryFor(report, undeclaredFile)
	if !ok || stray.Disposition != "carve" {
		t.Errorf("RED: %s entry %+v, want disposition carve", undeclaredFile, stray)
	}
	if len(report.Entries) != 2 {
		t.Errorf("RED: %d entries, want exactly the 2 changed paths: %+v", len(report.Entries), report.Entries)
	}
}

func TestC1879_006_UnreadableCycleExits2(t *testing.T) {
	w := newWave(t)
	missing := scopeDelta(t, w.projectRoot, "--cycle", "99999999")
	if missing.code != exitIO {
		t.Errorf("RED: a cycle with no run directory exited %d, want %d:\n%s", missing.code, exitIO, missing)
	}
	if err := os.WriteFile(filepath.Join(w.projectRoot, ".evolve", "runs", "cycle-"+strconv.Itoa(laneCycle), "cycle-state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := scopeDelta(t, w.projectRoot, "--cycle", strconv.Itoa(laneCycle))
	if corrupt.code != exitIO {
		t.Errorf("RED: a corrupt cycle-state.json exited %d, want %d:\n%s", corrupt.code, exitIO, corrupt)
	}
}

func TestC1879_007_UsageErrorsExit10(t *testing.T) {
	w := newWave(t)
	cases := []struct {
		name string
		args []string
	}{
		{"no cycle", nil},
		{"unknown flag", []string{"--cycle", strconv.Itoa(laneCycle), "--bogus"}},
		{"non-numeric cycle", []string{"--cycle", "abc"}},
		{"zero cycle", []string{"--cycle", "0"}},
		{"negative cycle", []string{"--cycle", "-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := scopeDelta(t, w.projectRoot, tc.args...)
			if strings.Contains(r.stderr, "unknown subcommand") {
				t.Fatalf("RED: the exit came from cycle's unknown-subcommand path, not scope-delta's usage check:\n%s", r)
			}
			if r.code != exitUsage {
				t.Errorf("RED: %s exited %d, want %d:\n%s", tc.name, r.code, exitUsage, r)
			}
		})
	}
}

func TestC1879_008_EmptyDiffIsCleanAndPositionalCycleWorks(t *testing.T) {
	w := newWave(t)
	r := scopeDelta(t, w.projectRoot, "--json", strconv.Itoa(laneCycle))
	if r.code != exitClean {
		t.Fatalf("RED: an empty diff via positional cycle exited %d, want %d:\n%s", r.code, exitClean, r)
	}
	report := decodeReport(t, r)
	if len(report.Entries) != 0 || report.Summary.Kept != 0 {
		t.Errorf("RED: an empty diff reported %+v", report)
	}
}

func TestC1879_009_RuntimeReferenceDocumentsScopeDelta(t *testing.T) {
	// acs-predicate: config-check
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	if !acsassert.FileContains(t, doc, "evolve cycle scope-delta") {
		t.Errorf("RED: runtime-reference.md does not document evolve cycle scope-delta")
	}
}

func TestC1879_010_NewGoCodeHasNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	newFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle_scope_delta.go")
	if n := commentCount(t, newFile, nil); n != 0 {
		t.Errorf("RED: cmd_cycle_scope_delta.go carries %d comment(s); new code has none", n)
	}
	dispatch := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")
	headSrc, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "show", "HEAD:go/cmd/evolve/cmd_cycle.go")
	if code != 0 {
		t.Fatalf("git show HEAD:go/cmd/evolve/cmd_cycle.go exited %d", code)
	}
	before := commentCount(t, "HEAD:cmd_cycle.go", []byte(headSrc))
	if after := commentCount(t, dispatch, nil); after > before {
		t.Errorf("RED: cmd_cycle.go gained comments: %d at HEAD, %d now", before, after)
	}
}

func commentCount(t *testing.T, name string, src []byte) int {
	t.Helper()
	var body any
	if src != nil {
		body = src
	}
	f, err := parser.ParseFile(token.NewFileSet(), name, body, parser.ParseComments)
	if err != nil {
		t.Fatalf("RED: parse %s: %v", name, err)
	}
	n := 0
	for _, g := range f.Comments {
		for _, c := range g.List {
			if !strings.HasPrefix(c.Text, "//go:") {
				n++
			}
		}
	}
	return n
}
