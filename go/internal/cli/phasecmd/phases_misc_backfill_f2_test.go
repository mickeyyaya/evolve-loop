package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasewatchdog"
)

const provenanceHeaderF2 = "<!-- evolve:provenance phase=build cycle=5 -->\n# Build Report\n"

func TestCollectProvenance_SkipsAbsentAndNonMarkdownArtifactsAndCountsTheCheckedOnesF2(t *testing.T) {
	provenanceFixture(t, provenanceHeaderF2)
	project := os.Getenv("EVOLVE_PROJECT_ROOT")
	var errb bytes.Buffer

	report, err := collectProvenance(project, 5, &errb)

	if err != nil || report.Cycle != 5 || report.ArtifactsChecked != 1 || len(report.Violations) != 0 || errb.Len() != 0 {
		t.Errorf("report=%+v err=%v stderr=%q, want one clean artifact checked (the other phases' artifacts are absent)", report, err, errb.String())
	}
	empty, err := collectProvenance(project, 99, &errb)
	if err != nil || empty.ArtifactsChecked != 0 || empty.Violations == nil {
		t.Errorf("a cycle with no artifacts: report=%+v err=%v, want zero checked and a non-nil violations list", empty, err)
	}
}

func TestCollectProvenance_AnArtifactThatCannotBeReadIsAnErrorNamingItF2(t *testing.T) {
	provenanceFixture(t, "unused")
	project := os.Getenv("EVOLVE_PROJECT_ROOT")
	artifact := filepath.Join(project, ".evolve", "runs", "cycle-5", "build-report.md")
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(artifact, 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := collectProvenance(project, 5, &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), "read artifact "+artifact) || report.ArtifactsChecked != 0 {
		t.Errorf("report=%+v err=%v, want the read error naming the artifact", report, err)
	}
}

func TestCollectProvenance_ALedgerThatCannotBeScannedFailsTheCheckF2(t *testing.T) {
	provenanceFixture(t, provenanceHeaderF2)
	project := os.Getenv("EVOLVE_PROJECT_ROOT")
	t.Setenv("EVOLVE_LEDGER_OVERRIDE", t.TempDir())

	report, err := collectProvenance(project, 5, &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), "scan ledger") || report.ArtifactsChecked != 0 {
		t.Errorf("report=%+v err=%v, want the ledger scan error and nothing counted", report, err)
	}
	var stdout, stderr bytes.Buffer
	if rc := RunPhases([]string{"check-provenance", "--cycle", "5"}, nil, &stdout, &stderr); rc != provenanceExitUnreadable || !strings.Contains(stderr.String(), "check-provenance: scan ledger") {
		t.Errorf("RunPhases rc=%d err=%q, want the unreadable exit", rc, stderr.String())
	}
}

func TestCollectProvenance_APhaseRootWithABrokenSpecWarnsAndStillChecksTheRestF2(t *testing.T) {
	provenanceFixture(t, provenanceHeaderF2)
	project := os.Getenv("EVOLVE_PROJECT_ROOT")
	brokenDir := filepath.Join(project, ".evolve", "phases", "broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "phase.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errb bytes.Buffer

	report, err := collectProvenance(project, 5, &errb)

	if err != nil || report.ArtifactsChecked != 1 || !strings.Contains(errb.String(), "WARN: ") {
		t.Errorf("report=%+v err=%v stderr=%q, want a discovery warning on stderr and the build artifact still checked", report, err, errb.String())
	}
}

func TestRunPhaseWatchdog_ValidatesItsArgumentsBeforeWatchingF2(t *testing.T) {
	ws := t.TempDir()
	cases := []struct {
		name   string
		args   []string
		wantRC int
		want   string
	}{
		{"three arguments", []string{ws, "1", "2"}, phasewatchdog.ExitInvalidArg, "requires 4 arguments"},
		{"no arguments", nil, phasewatchdog.ExitInvalidArg, "requires 4 arguments"},
		{"pgid not a number", []string{ws, "abc", "2", "state.json"}, phasewatchdog.ExitInvalidArg, "target_pgid must be a positive integer, got: abc"},
		{"empty pgid", []string{ws, "", "2", "state.json"}, phasewatchdog.ExitInvalidArg, "target_pgid must be a positive integer, got: "},
		{"cycle not a number", []string{ws, "1", "two", "state.json"}, phasewatchdog.ExitInvalidArg, "cycle must be a positive integer, got: two"},
		{"zero pgid reaches the watchdog's own check", []string{ws, "0", "2", "state.json"}, phasewatchdog.ExitInvalidArg, "target_pgid must be a positive integer, got: 0"},
		{"negative cycle reaches the watchdog's own check", []string{ws, "1", "-2", "state.json"}, phasewatchdog.ExitInvalidArg, "cycle must be a positive integer, got: -2"},
		{"workspace that does not exist", []string{filepath.Join(ws, "gone"), "1", "2", "state.json"}, phasewatchdog.ExitInvalidArg, "workspace directory does not exist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVOLVE_PROJECT_ROOT", writeObserverPolicy(t, `{"observer":{"watchdog_disabled":true}}`))
			var out, errb bytes.Buffer

			rc := RunPhaseWatchdog(tc.args, nil, &out, &errb)

			if rc != tc.wantRC || out.Len() != 0 || !strings.Contains(errb.String(), tc.want) {
				t.Errorf("rc=%d out=%q err=%q, want %d containing %q", rc, out.String(), errb.String(), tc.wantRC, tc.want)
			}
		})
	}
}

func TestRunPhaseWatchdog_HelpPrintsUsageAnywhereInTheArgumentsF2(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"a", "b", "c", "-h"}} {
		var out, errb bytes.Buffer

		rc := RunPhaseWatchdog(args, nil, &out, &errb)

		if rc != 0 || errb.Len() != 0 || !strings.Contains(out.String(), "Usage: evolve phase-watchdog <workspace> <target_pgid> <cycle> <cycle_state_path>") ||
			!strings.Contains(out.String(), "observer.{stall_s,watchdog_poll_s,watchdog_warn_pct,watchdog_grace_s,watchdog_disabled}") {
			t.Errorf("args %v: rc=%d out=%q err=%q, want exit 0 and the two usage lines", args, rc, out.String(), errb.String())
		}
	}
}

func TestRunPhaseWatchdog_APolicyThatDisablesTheWatchdogExitsCleanlyWithoutWatchingF2(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", writeObserverPolicy(t, `{"observer":{"watchdog_disabled":true}}`))
	var out, errb bytes.Buffer

	rc := RunPhaseWatchdog([]string{ws, strconv.Itoa(os.Getpid()), "7", filepath.Join(ws, "cycle-state.json")}, nil, &out, &errb)

	if rc != phasewatchdog.ExitOK || out.Len() != 0 || !strings.Contains(errb.String(), "observer policy disables watchdog; exiting.") || strings.Contains(errb.String(), "started:") {
		t.Errorf("rc=%d out=%q err=%q, want exit 0 and the disabled notice", rc, out.String(), errb.String())
	}
}

func TestPhaseVerify_AFlagItDoesNotKnowIsAUsageErrorF2(t *testing.T) {
	var out, errb bytes.Buffer

	rc := phaseCommand{}.runPhaseVerify([]string{"scout", "--bogus"}, &out, &errb)

	if rc != 10 || out.Len() != 0 || !strings.Contains(errb.String(), "flag provided but not defined: -bogus") {
		t.Errorf("rc=%d out=%q err=%q, want exit 10", rc, out.String(), errb.String())
	}
}

func TestBuildSelfCheck_VerifyReturnsTheContractFailureAndSkipsTheFloorF2(t *testing.T) {
	root, ws := floorWorkspace(t)
	artifact := filepath.Join(ws, "build-report.md")
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	floor := &recordedFloor{failures: []string{"must not run"}}
	var errb bytes.Buffer

	got, err := BuildSelfCheck{Floor: floor.floorFor, Probe: core.BuildHandoffProbe{Workspace: ws, EvolveDir: filepath.Join(root, ".evolve"), ProjectRoot: root}}.Verify(&errb)

	if err == nil || got.Unbound == nil || got.OK || len(floor.roots) != 0 || !strings.Contains(errb.String(), "WARN no cycle binding") {
		t.Errorf("result=%+v err=%v floor ran=%v stderr=%q, want the contract error, the unbound note and no floor run", got, err, len(floor.roots) != 0, errb.String())
	}
}
