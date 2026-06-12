package triagecap

import (
	"os"
	"path/filepath"
	"testing"
)

// knownPkgsFixture mirrors the package basenames that exist in the repo —
// the subset relevant to the replay fixtures plus common English-word
// packages to prove word-boundary matching does not overcount. It includes
// the names that collide with the triage bullet contract's own vocabulary
// (`evidence`, `scout` — every bullet must carry evidence=/source=scout)
// and with coverage prose (`paths` — "error paths"): cycle 301 failed on
// exactly these phantoms, so the production vocabulary must be represented
// here or the replay pins prove nothing.
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

// TestCountCommittedFloors_Cycle301Replay pins the soak-#2 incident
// (2026-06-12): a correctly-sized 2-bullet coverage commitment was counted
// as 6 floors because the contract-mandated evidence=/source=scout fields
// and the prose word "paths" matched real package basenames. The phantom
// floors made the correction directive unsatisfiable — triage could not
// remove tokens its own bullet contract requires — so the cycle burned both
// corrections and failed. True count: one package per bullet.
func TestCountCommittedFloors_Cycle301Replay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle301.md")
	got := CountCommittedFloors(artifact, knownPkgsFixture)
	if got != 2 {
		t.Errorf("cycle-301 committed floors = %d, want 2 (clihealth + ledger; evidence/scout/paths are phantoms)", got)
	}
}

// TestCountCommittedFloors_Cycle298Bullet pins the window-poisoning shape:
// cycle 298's single floor-bearing bullet was recorded as 4 floors
// (gc + evidence + scout + "safety-critical paths"), fabricating K=4 for
// the throughput window. True count: 1.
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
