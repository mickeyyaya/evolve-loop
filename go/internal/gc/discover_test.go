package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover_EvidenceNotNames(t *testing.T) {
	dir := t.TempDir()
	runs := filepath.Join(dir, "runs")

	writeFile(t, filepath.Join(runs, "oddly-named-run", "run.json"), `{"cycle_id":7}`)
	writeFile(t, filepath.Join(runs, "legacy-run", "scout-report.md"), "r")
	if err := os.MkdirAll(filepath.Join(runs, "cycle-999"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(runs, "phase-d-validation.log"), "log")
	reffed := filepath.Join(runs, "manual-release-v10.16.0")
	if err := os.MkdirAll(reffed, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(dir, DiscoverOptions{LedgerRefs: []string{reffed}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	byPath := map[string]RunDir{}
	for _, r := range got {
		byPath[r.Path] = r
	}
	for _, want := range []string{
		filepath.Join(runs, "oddly-named-run"),
		filepath.Join(runs, "legacy-run"),
		reffed,
	} {
		if _, ok := byPath[want]; !ok {
			t.Errorf("evidenced run dir missing from discovery: %s", want)
		}
	}
	if _, ok := byPath[filepath.Join(runs, "cycle-999")]; ok {
		t.Error("markerless dir must be invisible even with a cycle-N name (no name-parsing)")
	}
	if len(got) != 3 {
		t.Errorf("want exactly 3 evidenced dirs, got %d: %+v", len(got), got)
	}
}

func TestDiscover_LivenessFromRunStateAndLease(t *testing.T) {
	dir := t.TempDir()
	runs := filepath.Join(dir, "runs")

	current := filepath.Join(runs, "cycle-7")
	writeFile(t, filepath.Join(current, "run.json"), `{"cycle_id":7}`)
	leased := filepath.Join(runs, "cycle-6")
	writeFile(t, filepath.Join(leased, "run.json"), `{"cycle_id":6}`)
	staleLease := filepath.Join(runs, "cycle-5")
	writeFile(t, filepath.Join(staleLease, "run.json"), `{"cycle_id":5}`)
	dead := filepath.Join(runs, "cycle-4")
	writeFile(t, filepath.Join(dead, "run.json"), `{"cycle_id":4}`)

	writeFile(t, filepath.Join(dir, "cycle-state.json"),
		`{"cycle_id":7,"phase":"build","workspace_path":"`+current+`"}`)
	if err := runlease.Write(leased, runlease.Lease{RunID: "r6"}, t0.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(staleLease, runlease.Lease{RunID: "r5"}, t0.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(dir, DiscoverOptions{Now: nowT0})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	live := map[string]bool{}
	for _, r := range got {
		live[r.Path] = r.Live
	}
	if !live[current] {
		t.Error("the run named by cycle-state.json must be LIVE")
	}
	if !live[leased] {
		t.Error("a fresh .lease must mark the run LIVE")
	}
	if live[staleLease] {
		t.Error("a stale .lease must not prove liveness")
	}
	if live[dead] {
		t.Error("no signal → dead")
	}
}

func TestDiscover_FailsClosedOnUnreadableRunState(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "runs", "cycle-1", "run.json"), `{"cycle_id":1}`)
	writeFile(t, filepath.Join(dir, "cycle-state.json"), `{torn`)
	if _, err := Discover(dir, DiscoverOptions{Now: nowT0}); err == nil {
		t.Fatal("unparsable cycle-state.json must fail discovery closed (a live run without a lease would be misclassified dead)")
	}
}

func TestDiscover_ReadRunsDirError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "runs"), "not a dir")
	if _, err := Discover(dir, DiscoverOptions{Now: nowT0}); err == nil {
		t.Fatal("runs path that exists but is not a directory must report an error")
	}
}

func TestDiscover_MissingRunsDirYieldsEmpty(t *testing.T) {
	got, err := Discover(t.TempDir(), DiscoverOptions{Now: nowT0})
	if err != nil {
		t.Fatalf("Discover with missing runs dir: %v", err)
	}
	if got != nil {
		t.Fatalf("missing runs dir must yield nil discovery, got %+v", got)
	}
}

func TestDiscover_FailsClosedOnUnreadableRunStateFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "runs", "cycle-1", "run.json"), `{"cycle_id":1}`)
	if err := os.Mkdir(filepath.Join(dir, "cycle-state.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(dir, DiscoverOptions{Now: nowT0}); err == nil {
		t.Fatal("unreadable cycle-state.json path must fail discovery closed")
	}
}

func TestDiscover_TerminalRunStateIsIdle(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-1")
	writeFile(t, filepath.Join(run, "run.json"), `{"cycle_id":1}`)
	writeFile(t, filepath.Join(dir, "cycle-state.json"), `{"cycle_id":0,"workspace_path":"`+run+`"}`)
	got, err := Discover(dir, DiscoverOptions{Now: nowT0})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].Live {
		t.Fatalf("cycle_id=0 is idle state; run should be discovered dead, got %+v", got)
	}
}

func TestDiscover_SymlinkedRunDirIsVisible(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "relocated", "cycle-9")
	writeFile(t, filepath.Join(real, "run.json"), `{"cycle_id":9}`)
	link := filepath.Join(dir, "runs", "cycle-9")
	if err := os.MkdirAll(filepath.Join(dir, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(dir, DiscoverOptions{Now: nowT0})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].Path != link {
		t.Fatalf("symlinked run dir must be discovered (else it bypasses every protection): %+v", got)
	}
}

func TestDiscover_FailsClosedOnNonAbsoluteWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "runs", "cycle-1", "run.json"), `{"cycle_id":1}`)
	writeFile(t, filepath.Join(dir, "cycle-state.json"), `{"cycle_id":5,"workspace_path":""}`)
	if _, err := Discover(dir, DiscoverOptions{Now: nowT0}); err == nil {
		t.Fatal("in-flight cycle with a non-absolute workspace_path must fail discovery closed")
	}
}

func TestApply_RefusesRunLeasedAfterPlan(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-3")
	writeFile(t, filepath.Join(run, "run.json"), `{"cycle_id":3}`)
	eph := filepath.Join(run, ".ephemeral")
	if err := os.MkdirAll(eph, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(run, runlease.Lease{RunID: "r3"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	err := Apply(dir, Manifest{Items: []Item{
		{Path: run, Action: ActionDelete, Rule: "runs.delete_after_days"},
		{Path: eph, Action: ActionDelete, Rule: "tracker_ttl_days"},
	}})
	if err == nil {
		t.Fatal("Apply must refuse a run (and its subtrees) leased after Plan")
	}
	if _, statErr := os.Stat(run); statErr != nil {
		t.Errorf("now-live run must survive: %v", statErr)
	}
	if _, statErr := os.Stat(eph); statErr != nil {
		t.Errorf("now-live run's .ephemeral must survive: %v", statErr)
	}
}

func TestDiscover_GarbageLeaseIsNotLive(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-2")
	writeFile(t, filepath.Join(run, "run.json"), `{"cycle_id":2}`)
	writeFile(t, filepath.Join(run, runlease.FileName), `{torn`)
	got, err := Discover(dir, DiscoverOptions{Now: nowT0})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].Live {
		t.Errorf("garbage lease must read as no-lease (dead, still discovered): %+v", got)
	}
}
