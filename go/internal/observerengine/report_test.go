package observerengine

// report_test.go — §6 test 30 (with critic G4's three fault fixtures): G2
// replayed through the leaf; a failed write is BOTH returned and reported as
// OBSERVER_REPORT_WRITE_FAILED, with mkdir / write / rename each reachable.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReport_GoldenAndFailureReturnsAndReports(t *testing.T) {
	t.Parallel()
	t.Run("golden", func(t *testing.T) {
		t.Parallel()
		e := goldenEngine(t)
		e.emit("Engine.Tick", "stuck_no_output", "INCIDENT", goldenIncidentData())
		e.emit("Engine.Tick", "stuck_no_progress", "INCIDENT", map[string]any{"no_progress_s": 700, "threshold_s": 600, "tool_calls": 1, "tool_results": 1})
		if err := e.WriteReport(); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(e.s.Paths.Report)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(raw), readGolden(t, "report.golden.json"); got != want {
			t.Fatalf("report:\n got: %s\nwant: %s", got, want)
		}
		if _, err := os.Stat(e.s.Paths.Report + ".tmp"); !os.IsNotExist(err) {
			t.Errorf(".tmp renamed away: %v", err)
		}
	})
	faults := []struct {
		name string
		seed func(t *testing.T, s *Settings)
	}{
		{"mkdir", func(t *testing.T, s *Settings) { // a FILE where the workspace directory belongs
			file := filepath.Join(t.TempDir(), "ws-is-a-file")
			if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			s.Paths = PathsFor(filepath.Join(file, "nested"), "builder")
		}},
		{"write", func(t *testing.T, s *Settings) { // a DIRECTORY at <path>.tmp
			if err := os.Mkdir(s.Paths.Report+".tmp", 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"rename", func(t *testing.T, s *Settings) { // a DIRECTORY at the target
			if err := os.Mkdir(s.Paths.Report, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, f := range faults {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			e, rc, _ := newEngine(t, func(s *Settings, _ *Deps) { f.seed(t, s) })
			err := e.WriteReport()
			if err == nil {
				t.Fatal("the write error is returned")
			}
			got := rc.byCode(CodeReportWriteFailed)
			if len(got) != 1 || got[0].Origin != "Engine.WriteReport" || got[0].Reason != err.Error() || got[0].Fields["step"] != "report" || got[0].Fields["path"] != e.s.Paths.Report {
				t.Errorf("one report fault carrying the error: %+v", got)
			}
		})
	}
}
