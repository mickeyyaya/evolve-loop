//go:build acs

package cycle1936

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/kbgraph"
	"github.com/mickeyyaya/evolve-loop/go/internal/treewalk"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var evolveBuild struct {
	once    sync.Once
	dir     string
	path    string
	failure string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		os.RemoveAll(evolveBuild.dir)
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	evolveBuild.once.Do(func() {
		dir, err := os.MkdirTemp("", "cycle1936-evolve-")
		if err != nil {
			evolveBuild.failure = err.Error()
			return
		}
		evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
		cmd.Dir = goDir
		if out, err := cmd.CombinedOutput(); err != nil {
			evolveBuild.failure = fmt.Sprintf("%v\n%s", err, out)
		}
	})
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func runEvolve(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(evolveBin(t), args...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run evolve %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestC1936_001_TreewalkDiffReportsNewFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg", "kept_test.go"), "package pkg")
	before, err := treewalk.Take(root)
	if err != nil {
		t.Fatalf("Take before: %v", err)
	}
	writeFile(t, filepath.Join(root, "pkg", "audit-report.md"), "leak")
	after, err := treewalk.Take(root)
	if err != nil {
		t.Fatalf("Take after: %v", err)
	}
	got := treewalk.Diff(before, after)
	want := []string{filepath.Join("pkg", "audit-report.md")}
	if !slices.Equal(got, want) {
		t.Errorf("Diff = %v, want %v", got, want)
	}
}

func TestC1936_002_TreewalkDiffReportsNewEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "subagent"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := treewalk.Take(root)
	if err != nil {
		t.Fatalf("Take before: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "subagent", ".evolve", "runs", "cycle-0"), 0o755); err != nil {
		t.Fatal(err)
	}
	after, err := treewalk.Take(root)
	if err != nil {
		t.Fatalf("Take after: %v", err)
	}
	got := treewalk.Diff(before, after)
	for _, want := range []string{
		filepath.Join("subagent", ".evolve"),
		filepath.Join("subagent", ".evolve", "runs"),
		filepath.Join("subagent", ".evolve", "runs", "cycle-0"),
	} {
		if !slices.Contains(got, want) {
			t.Errorf("Diff = %v, missing empty-dir leak %q", got, want)
		}
	}
}

func TestC1936_003_TreewalkDiffIsEmptyWhenTreeUnchanged(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "b.txt"), "x")
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := treewalk.Take(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := treewalk.Take(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := treewalk.Diff(before, after); len(got) != 0 {
		t.Errorf("Diff of identical trees = %v, want none", got)
	}
}

func TestC1936_004_TreewalkTakeRejectsMissingRoot(t *testing.T) {
	if _, err := treewalk.Take(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("Take of a missing root returned nil error")
	}
}

func TestC1936_005_TreewalkCLIFailsOnLeakedEmptyDirAndPassesWhenClean(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg", "x_test.go"), "package pkg")
	baseline := filepath.Join(t.TempDir(), "baseline.json")

	if _, stderr, code := runEvolve(t, "treewalk", "snapshot", "--root", root, "--out", baseline); code != 0 {
		t.Fatalf("treewalk snapshot exit=%d stderr=%s", code, stderr)
	}
	if stdout, stderr, code := runEvolve(t, "treewalk", "check", "--root", root, "--baseline", baseline); code != 0 {
		t.Fatalf("treewalk check on unchanged tree exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	leak := filepath.Join(root, "pkg", ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(leak, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runEvolve(t, "treewalk", "check", "--root", root, "--baseline", baseline)
	if code == 0 {
		t.Fatalf("treewalk check passed despite a leaked empty dir; stdout=%s", stdout)
	}
	if !strings.Contains(stdout+stderr, filepath.Join("pkg", ".evolve", "runs", "cycle-1")) {
		t.Errorf("check output does not name the leaked dir:\n%s%s", stdout, stderr)
	}
}

func TestC1936_006_TreewalkCLIRejectsMissingBaseline(t *testing.T) {
	_, _, code := runEvolve(t, "treewalk", "check", "--root", t.TempDir(), "--baseline", filepath.Join(t.TempDir(), "none.json"))
	if code == 0 {
		t.Error("treewalk check with an absent baseline exited 0")
	}
}

func TestC1936_007_MakeTestTargetRunsTreewalkGuard(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	cmd := exec.Command("make", "-n", "test")
	cmd.Dir = goDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n test: %v\n%s", err, out)
	}
	text := string(out)
	snap := strings.Index(text, "treewalk snapshot")
	test := strings.Index(text, "go test")
	check := strings.LastIndex(text, "treewalk check")
	if snap < 0 || check < 0 || test < 0 || !(snap < test && test < check) {
		t.Errorf("make test must run treewalk snapshot, then go test, then treewalk check; got:\n%s", text)
	}
}

type cycleFixture struct {
	cycle       int
	fingerprint string
	defects     int
	lessons     []string
}

func writeCycle(t *testing.T, root string, f cycleFixture) {
	t.Helper()
	d := dossier.Dossier{
		SchemaVersion: dossier.CurrentSchemaVersion,
		Cycle:         f.cycle,
		Goal:          "goal",
		FinalVerdict:  dossier.VerdictFail,
		Phases:        []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictFail}},
	}
	if f.fingerprint != "" {
		d.Failure = &dossier.FailureRecord{Fingerprint: f.fingerprint, PreClass: "contract-block"}
	}
	for i := 0; i < f.defects; i++ {
		d.Defects = append(d.Defects, dossier.Defect{ID: fmt.Sprintf("D%d-%d", f.cycle, i), Severity: "HIGH", Summary: "s"})
	}
	for _, id := range f.lessons {
		d.Lessons = append(d.Lessons, dossier.Lesson{ID: id, Pattern: "p"})
	}
	body, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dossier.CyclesDir(root), fmt.Sprintf("cycle-%d.json", f.cycle)), string(body))
}

func writeInboxItem(t *testing.T, root, id string, connectsTo, deps []string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"id": id, "title": id, "connects_to": connectsTo, "deps": deps})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "2026-10-01T00-00-00Z-"+id+".json"), string(body))
}

func nodeIDs(s kbgraph.Subgraph) []string {
	var ids []string
	for _, n := range s.Nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

func TestC1936_101_NeighborsLinksCyclesSharingAFingerprint(t *testing.T) {
	root := t.TempDir()
	writeCycle(t, root, cycleFixture{cycle: 10, fingerprint: "fp-abc"})
	writeCycle(t, root, cycleFixture{cycle: 11, fingerprint: "fp-abc"})
	writeCycle(t, root, cycleFixture{cycle: 12, fingerprint: "fp-other"})
	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	one, err := g.Neighbors("cycle:10", 1)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if !slices.Contains(nodeIDs(one), "fingerprint:fp-abc") || slices.Contains(nodeIDs(one), "cycle:11") {
		t.Errorf("1 hop nodes = %v, want the fingerprint but not the sibling cycle", nodeIDs(one))
	}
	two, err := g.Neighbors("cycle:10", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(nodeIDs(two), "cycle:11") {
		t.Errorf("2 hop nodes = %v, want cycle:11 via the shared fingerprint", nodeIDs(two))
	}
	if slices.Contains(nodeIDs(two), "cycle:12") {
		t.Errorf("2 hop nodes = %v, unrelated cycle:12 must not appear", nodeIDs(two))
	}
	if !slices.ContainsFunc(two.Edges, func(e kbgraph.Edge) bool {
		return e.From == "cycle:10" && e.To == "fingerprint:fp-abc" && e.Kind == "recurs-as"
	}) {
		t.Errorf("edges = %v, want cycle:10 -recurs-as-> fingerprint:fp-abc", two.Edges)
	}
}

func TestC1936_102_NeighborsCapsHopsAtTwo(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a", []string{"b"}, nil)
	writeInboxItem(t, root, "b", []string{"c"}, nil)
	writeInboxItem(t, root, "c", []string{"d"}, nil)
	writeInboxItem(t, root, "d", nil, nil)
	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.Neighbors("item:a", 9)
	if err != nil {
		t.Fatal(err)
	}
	ids := nodeIDs(got)
	if !slices.Contains(ids, "item:b") || !slices.Contains(ids, "item:c") {
		t.Errorf("nodes = %v, want items b and c within two hops", ids)
	}
	if slices.Contains(ids, "item:d") {
		t.Errorf("nodes = %v, item:d is three hops away and must be cut by the cap", ids)
	}
}

func TestC1936_103_NeighborsCapsNodesAtForty(t *testing.T) {
	root := t.TempDir()
	writeCycle(t, root, cycleFixture{cycle: 20, fingerprint: "fp-big", defects: 80})
	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.Neighbors("cycle:20", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) > 40 {
		t.Errorf("returned %d nodes, cap is 40", len(got.Nodes))
	}
	if !got.Truncated {
		t.Error("Truncated = false although 80 defects were cut")
	}
}

func TestC1936_104_AbsentEdgesAreReportedNeverSynthesized(t *testing.T) {
	root := t.TempDir()
	writeCycle(t, root, cycleFixture{cycle: 30})
	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.Neighbors("cycle:30", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got.Absent, "recurs-as") {
		t.Errorf("Absent = %v, want recurs-as for a dossier without a failure block", got.Absent)
	}
	for _, id := range nodeIDs(got) {
		if strings.HasPrefix(id, "fingerprint:") {
			t.Errorf("synthesized node %q for a dossier with no fingerprint", id)
		}
	}
}

func TestC1936_105_NeighborsRejectsUnknownNode(t *testing.T) {
	g, err := kbgraph.Build(t.TempDir())
	if err != nil {
		t.Fatalf("Build of an empty corpus: %v", err)
	}
	if _, err := g.Neighbors("cycle:999", 1); err == nil {
		t.Error("Neighbors of an unknown node returned nil error")
	}
}

func TestC1936_106_RecurrencesCountsCorpusWideInstances(t *testing.T) {
	root := t.TempDir()
	writeCycle(t, root, cycleFixture{cycle: 40, fingerprint: "fp-rep"})
	writeCycle(t, root, cycleFixture{cycle: 41, fingerprint: "fp-rep"})
	writeCycle(t, root, cycleFixture{cycle: 42, fingerprint: "fp-solo"})
	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	rep := g.Recurrences("fp-rep")
	if rep.Count != 2 || !slices.Equal(rep.Cycles, []int{40, 41}) {
		t.Errorf("Recurrences(fp-rep) = %+v, want count 2 cycles [40 41]", rep)
	}
	if solo := g.Recurrences("fp-solo"); solo.Count != 1 {
		t.Errorf("Recurrences(fp-solo).Count = %d, want 1", solo.Count)
	}
	if none := g.Recurrences("fp-never"); none.Count != 0 || len(none.Cycles) != 0 {
		t.Errorf("Recurrences(fp-never) = %+v, want empty", none)
	}
}

func TestC1936_107_BuildIsDeterministicAndReadOnly(t *testing.T) {
	root := t.TempDir()
	writeCycle(t, root, cycleFixture{cycle: 50, fingerprint: "fp-d", lessons: []string{"L1"}})
	writeInboxItem(t, root, "x", []string{"y"}, []string{"z"})
	before, err := treewalk.Take(root)
	if err != nil {
		t.Fatal(err)
	}
	var renders []string
	for i := 0; i < 2; i++ {
		g, err := kbgraph.Build(root)
		if err != nil {
			t.Fatal(err)
		}
		sub, err := g.Neighbors("cycle:50", 2)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(sub)
		renders = append(renders, string(body))
	}
	if renders[0] != renders[1] {
		t.Errorf("two builds rendered differently:\n%s\n%s", renders[0], renders[1])
	}
	after, err := treewalk.Take(root)
	if err != nil {
		t.Fatal(err)
	}
	if delta := treewalk.Diff(before, after); len(delta) != 0 {
		t.Errorf("the projector wrote to the corpus: %v", delta)
	}
}

func TestC1936_108_KBGraphCLIPrintsNeighborsAndAbsentEdges(t *testing.T) {
	root := t.TempDir()
	writeCycle(t, root, cycleFixture{cycle: 60, fingerprint: "fp-cli"})
	writeCycle(t, root, cycleFixture{cycle: 61})
	stdout, stderr, code := runEvolve(t, "kb", "graph", "neighbors", "cycle:60", "--hops", "2", "--project-root", root)
	if code != 0 || !strings.Contains(stdout, "fingerprint:fp-cli") {
		t.Fatalf("neighbors exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	stdout, stderr, code = runEvolve(t, "kb", "graph", "neighbors", "cycle:61", "--project-root", root)
	if code != 0 || !strings.Contains(stdout, "ABSENT") {
		t.Errorf("neighbors of a failure-less cycle exit=%d, want ABSENT in stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestC1936_109_KBGraphCLIRecurrencesSurfacesSecondInstance(t *testing.T) {
	root := t.TempDir()
	writeCycle(t, root, cycleFixture{cycle: 70, fingerprint: "fp-two"})
	writeCycle(t, root, cycleFixture{cycle: 71, fingerprint: "fp-two"})
	stdout, stderr, code := runEvolve(t, "kb", "graph", "recurrences", "fp-two", "--project-root", root)
	if code != 0 {
		t.Fatalf("recurrences exit=%d stderr=%s", code, stderr)
	}
	for _, want := range []string{"70", "71", "2"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("recurrences output missing %q:\n%s", want, stdout)
		}
	}
}

func TestC1936_110_KBGraphCLIRejectsBadInvocations(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"kb", "graph", "neighbors", "cycle:404", "--project-root", root},
		{"kb", "graph", "nonsense", "--project-root", root},
		{"kb", "graph", "neighbors", "--project-root", root},
	} {
		if _, _, code := runEvolve(t, args...); code == 0 {
			t.Errorf("evolve %v exited 0, want failure", args)
		}
	}
}

func TestC1936_111_NewPackagesAreEnrolledInApicover(t *testing.T) {
	// acs-predicate: config-check
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, pkg := range []string{"treewalk", "kbgraph"} {
		acsassert.FileContains(t, filepath.Join(goDir, ".apicover-enforce"), "./internal/"+pkg)
		acsassert.FileExists(t, filepath.Join(goDir, "internal", pkg, "apicover_named_test.go"))
	}
}
