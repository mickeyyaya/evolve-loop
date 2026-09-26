package triagecap

import (
	"os"
	"path/filepath"
	"testing"
)

// knownPkgsFixture must keep evidence, scout and paths: they collide with contract metadata and coverage
// prose, and without them the replay pins pass against a vocabulary production does not have.
var knownPkgsFixture = []string{
	"swarmrunner", "swarmplan", "swarm",
	"bridge", "phasecoherence", "looppreflight", "modelcatalog",
	"ship", "recovery", "interaction", "evalgate", "faillearn",
	"core", "config", "router", "registry",
	"evidence", "scout", "paths", "clihealth", "ledger", "gc",
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestCountCommittedFloors_Cycle283Replay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle283.md")
	got := CountCommittedFloors(artifact, knownPkgsFixture)
	if got != 12 {
		t.Errorf("cycle-283 committed floors = %d, want 12 (3+4+5 distinct packages across the three ≥98%% items)", got)
	}
}

func TestCountCommittedFloors_Cycle281Replay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle281.md")
	got := CountCommittedFloors(artifact, knownPkgsFixture)
	if got != 1 {
		t.Errorf("cycle-281 committed floors = %d, want 1 (single aggregate coverage push)", got)
	}
}

func TestCountCommittedFloors_Cycle301Replay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle301.md")
	got := CountCommittedFloors(artifact, knownPkgsFixture)
	if got != 2 {
		t.Errorf("cycle-301 committed floors = %d, want 2 (clihealth + ledger; evidence/scout/paths are phantoms)", got)
	}
}

func TestCountCommittedFloors_Cycle298Bullet(t *testing.T) {
	artifact := "## top_n\n" +
		"- gc-coverage-boost: Boost internal/gc coverage from 88.8% to ≥95% by covering Apply/nowLive/protected/dirEntriesOlderThan safety-critical paths — priority=M, evidence=scout-report.md#task-2, source=scout\n"
	got := CountCommittedFloors(artifact, knownPkgsFixture)
	if got != 1 {
		t.Errorf("cycle-298 bullet floors = %d, want 1 (gc only)", got)
	}
}

func TestCountCommittedFloors_Table(t *testing.T) {
	tests := []struct {
		name     string
		artifact string
		want     int
	}{
		{
			name:     "empty artifact",
			artifact: "",
			want:     0,
		},
		{
			name:     "no top_n section",
			artifact: "# Triage\n\n## deferred\n- coverage-x: push core to 98% coverage\n",
			want:     0,
		},
		{
			name: "non-coverage items count zero floors",
			artifact: "## top_n\n" +
				"- fix-bug: Fix the dispatch worktree bug — priority=H\n" +
				"- add-suite: Build fault-injection test suite\n",
			want: 0,
		},
		{
			name: "coverage item without resolvable packages counts one floor",
			artifact: "## top_n\n" +
				"- coverage-push: Push internal coverage toward 93%\n",
			want: 1,
		},
		{
			name: "coverage item with three packages counts three floors",
			artifact: "## top_n\n" +
				"- coverage-multi: Tests for swarmrunner, swarmplan, swarm coverage ≥98%\n",
			want: 3,
		},
		{
			name: "deferred section floors are NOT committed",
			artifact: "## top_n\n" +
				"- coverage-one: Push bridge coverage to ≥98%\n" +
				"\n## deferred\n" +
				"- coverage-rest: Push recovery, interaction, evalgate to ≥98% coverage\n",
			want: 1,
		},
		{
			name: "percent without coverage context is not a floor",
			artifact: "## top_n\n" +
				"- perf-task: Reduce latency by 30% in router hot path\n",
			want: 0,
		},
		{
			name: "word-boundary: swarm does not double-count inside swarmrunner",
			artifact: "## top_n\n" +
				"- coverage-sw: swarmrunner package floor ≥98% coverage\n",
			want: 1,
		},
		{
			name: "contract metadata fields never count as packages",
			artifact: "## top_n\n" +
				"- coverage-one: Push bridge coverage to ≥98% — priority=H, evidence=scout-report.md#task-1, source=scout\n",
			want: 1,
		},
		{
			name: "evidence path value still counts its real packages",
			artifact: "## top_n\n" +
				"- coverage-seal: add unit tests for writeSegment resume (50% covered) — priority=H, evidence=go/internal/adapters/ledger/seal.go:161, source=scout\n",
			want: 1,
		},
		{
			name: "prose 'error paths' is not a mention of package paths",
			artifact: "## top_n\n" +
				"- coverage-gc: cover gc Apply error paths to ≥95%\n",
			want: 1,
		},
		{
			name: "slash-qualified internal/paths does count package paths",
			artifact: "## top_n\n" +
				"- coverage-paths: raise internal/paths coverage to ≥95%\n",
			want: 1,
		},
		{
			name: "later slash-qualified mention counts even after a non-boundary one",
			artifact: "## top_n\n" +
				"- coverage-paths2: fix scripts/pathsXgen then raise internal/paths coverage to ≥95%\n",
			want: 1,
		},
		{
			name: "bare scout outside source= is a legitimate package reference",
			artifact: "## top_n\n" +
				"- coverage-scout: raise scout package coverage to 90% — priority=M, source=scout\n",
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountCommittedFloors(tt.artifact, knownPkgsFixture); got != tt.want {
				t.Errorf("CountCommittedFloors = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestKnownPackages_RealTree(t *testing.T) {
	root := repoRoot(t)
	pkgs := KnownPackages(root)
	want := []string{"swarmrunner", "bridge", "phasecoherence", "evalgate", "triagecap"}
	set := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		set[p] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("KnownPackages missing %q (got %d packages)", w, len(pkgs))
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(wd)))
}
