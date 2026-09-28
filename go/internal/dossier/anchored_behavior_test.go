package dossier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestBuild_RejectsInvalidInputsWithExactErrors(t *testing.T) {
	cases := []struct {
		name      string
		cycle     int
		workspace string
		want      string
	}{
		{name: "negative cycle", cycle: -3, workspace: t.TempDir(), want: "dossier: Build: cycle must be >= 1, got -3"},
		{name: "whitespace-only workspace", cycle: 1, workspace: "   ", want: "dossier: Build: WorkspacePath must not be blank"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := Build(tc.cycle, BuildOpts{WorkspacePath: tc.workspace, Goal: "g"})

			if d != nil || err == nil || err.Error() != tc.want {
				t.Fatalf("Build = (%v, %v), want error %q", d, err, tc.want)
			}
		})
	}
}

func TestBuild_CarriesRunIDSkippedPhasesAndVerdictsNotAdopted(t *testing.T) {
	skipped := []cyclestate.SkippedPhase{{Phase: "tdd", Reason: "trivial"}}
	notAdopted := []cyclestate.VerdictNotAdopted{{Phase: "audit", Verdict: "WARN"}}

	d, err := Build(5, BuildOpts{WorkspacePath: t.TempDir(), Goal: "g", RunID: "run-5", SkippedPhases: skipped, VerdictsNotAdopted: notAdopted})

	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.RunID != "run-5" {
		t.Errorf("RunID = %q, want run-5", d.RunID)
	}
	if !reflect.DeepEqual(d.SkippedPhases, skipped) {
		t.Errorf("SkippedPhases = %+v, want %+v", d.SkippedPhases, skipped)
	}
	if !reflect.DeepEqual(d.PhasesRunVerdictNotAdopted, notAdopted) {
		t.Errorf("PhasesRunVerdictNotAdopted = %+v, want %+v", d.PhasesRunVerdictNotAdopted, notAdopted)
	}
}

func TestBuild_WarnVerdictGetsNoAuditFailDefect(t *testing.T) {
	d, err := Build(5, BuildOpts{WorkspacePath: t.TempDir(), Goal: "g", FinalVerdict: VerdictWarn})

	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Defects != nil || d.Carryover != nil {
		t.Errorf("WARN dossier Defects = %+v, Carryover = %+v, want none", d.Defects, d.Carryover)
	}
}

func TestBuild_FailVerdictSynthesizesAuditDefectAndCarryover(t *testing.T) {
	d, err := Build(42, BuildOpts{WorkspacePath: t.TempDir(), Goal: "g", FinalVerdict: VerdictFail})

	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	wantDefects := []Defect{{
		ID:       "audit-fail",
		Severity: "HIGH",
		Summary:  fmt.Sprintf("cycle did not pass audit; see %s + acs-verdict.json", auditArtifactName()),
		Fix:      "address the audit findings recorded for this cycle",
	}}
	wantCarryover := []Carryover{{ID: "address-audit-findings", Action: "resolve the audit findings that failed cycle 42", Priority: "high"}}
	if !reflect.DeepEqual(d.Defects, wantDefects) {
		t.Errorf("Defects = %+v, want %+v", d.Defects, wantDefects)
	}
	if !reflect.DeepEqual(d.Carryover, wantCarryover) {
		t.Errorf("Carryover = %+v, want %+v", d.Carryover, wantCarryover)
	}
}

func TestSweepOrphans_WrapsEnumerateFailure(t *testing.T) {
	boom := errors.New("exec boom")
	failing := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, boom
	}

	_, err := SweepOrphans(gitexec.Git{Dir: t.TempDir(), Exec: failing}, io.Discard)

	if !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), "dossier: sweep: enumerate tree: ") {
		t.Fatalf("SweepOrphans error = %v, want it to wrap %v under the enumerate-tree prefix", err, boom)
	}
}

func TestSweepOrphans_SweepsAscendingAndIgnoresUnparsableCycles(t *testing.T) {
	dir := t.TempDir()
	initSweepRepo(t, dir)
	for _, n := range []int{3, 1, 2} {
		writePairFile(t, dir, fmt.Sprintf("cycle-%d.json", n), "{}")
		writePairFile(t, dir, fmt.Sprintf("cycle-%d.md", n), "# md")
	}
	writePairFile(t, dir, "cycle-6.md", "# md")
	writePairFile(t, dir, "cycle-4.json", "{}")
	writePairFile(t, dir, "cycle-99999999999999999999.json", "{}")

	res, err := SweepOrphans(gitexec.Default(dir), io.Discard)

	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if !reflect.DeepEqual(res.Recommitted, []int{1, 2, 3}) {
		t.Errorf("Recommitted = %v, want [1 2 3]", res.Recommitted)
	}
	if !reflect.DeepEqual(res.Skipped, []int{4, 6}) {
		t.Errorf("Skipped = %v, want [4 6] (the unparsable cycle is ignored)", res.Skipped)
	}
}

func TestSweepOrphans_LogsFailedPairByDirectory(t *testing.T) {
	dir := t.TempDir()
	initSweepRepo(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "cycles"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePairFile(t, dir, "cycles/cycle-40.json", "{}")
	writePairFile(t, dir, "cycles/cycle-40.md", "# md")
	var log bytes.Buffer

	res, err := SweepOrphans(gitexec.Git{Dir: dir, Exec: (&interceptExec{blockBase: "cycle-40"}).run}, &log)

	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	want := fmt.Sprintf("[dossier-sweep] ERROR cycle 40 (cycles): recommit failed: %v\n", res.Failed[40])
	if log.String() != want {
		t.Errorf("log = %q, want %q", log.String(), want)
	}
}
