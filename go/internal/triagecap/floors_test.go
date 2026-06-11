package triagecap

import (
	"os"
	"path/filepath"
	"testing"
)

// knownPkgsFixture mirrors the package basenames that exist in the repo —
// the subset relevant to the cycle-281/283 replay fixtures plus common
// English-word packages to prove word-boundary matching does not overcount.
var knownPkgsFixture = []string{
	"swarmrunner", "swarmplan", "swarm",
	"bridge", "phasecoherence", "looppreflight", "modelcatalog",
	"ship", "recovery", "interaction", "evalgate", "faillearn",
	"core", "config", "router", "registry",
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// TestCountCommittedFloors_Cycle283Replay pins the overpacked shape that
// failed three consecutive coverage cycles (inbox coverage-floor-overpacking):
// 3 tasks × ~12 package floors @98% must count as 12 committed floors.
func TestCountCommittedFloors_Cycle283Replay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle283.md")
	got := CountCommittedFloors(artifact, knownPkgsFixture)
	if got != 12 {
		t.Errorf("cycle-283 committed floors = %d, want 12 (3+4+5 distinct packages across the three ≥98%% items)", got)
	}
}

// TestCountCommittedFloors_Cycle281Replay pins the PASS baseline: one
// aggregate coverage item ("toward 93%") = 1 floor; the two non-coverage
// tasks contribute zero.
func TestCountCommittedFloors_Cycle281Replay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle281.md")
	got := CountCommittedFloors(artifact, knownPkgsFixture)
	if got != 1 {
		t.Errorf("cycle-281 committed floors = %d, want 1 (single aggregate coverage push)", got)
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountCommittedFloors(tt.artifact, knownPkgsFixture); got != tt.want {
				t.Errorf("CountCommittedFloors = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestKnownPackages_RealTree proves the enumerator finds the actual repo
// packages the replay fixtures mention (run against this repository's tree).
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
	// triagecap lives at <root>/go/internal/triagecap.
	return filepath.Dir(filepath.Dir(filepath.Dir(wd)))
}
