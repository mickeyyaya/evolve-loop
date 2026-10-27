package selfheal

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendSignals(t *testing.T, root, name string, events ...signalcenter.Event) {
	t.Helper()
	log, err := channel.New(root, name, channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var records []channel.Record
	for i := range events {
		e := events[i]
		e.SchemaVersion, e.Module, e.Origin, e.Reason = signalcenter.SchemaVersion, signalcenter.ModuleQuality, "test.Fixture", "fixture"
		records = append(records, channel.Record{Source: "test", Signal: &e})
	}
	records = append(records, channel.Record{Source: "test", Gap: &channel.Gap{Reason: channel.ReasonQueueFull, From: 1, To: 2}})
	if _, err := log.Append(records); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSignals_ReadsEachNamedChannelFromTheWindowStartAndSkipsAnAbsentOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	appendSignals(t, root, "errors", pq1Breach(1, t0.Add(-time.Hour)), pq1Breach(2, t0.Add(time.Hour)))
	appendSignals(t, root, "loop", pq1Breach(3, t0))
	got, err := LoadSignals(root, []string{"errors", "loop", "absent"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	var seqs []uint64
	for _, e := range got {
		seqs = append(seqs, e.Seq)
	}
	if !slices.Equal(seqs, []uint64{2, 3}) {
		t.Fatalf("seqs %v, want the two signals at or after the window start", seqs)
	}
}

func TestLoadSignals_ABadChannelNameOrAnUnreadableChannelFails(t *testing.T) {
	t.Parallel()
	if _, err := LoadSignals(t.TempDir(), []string{"Bad Name"}, t0); err == nil {
		t.Fatal("a bad channel name must fail")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "errors"), "not a directory")
	if _, err := LoadSignals(root, []string{"errors"}, t0); err == nil {
		t.Fatal("a channel that cannot be listed must fail")
	}
}

func TestLoadShipped_ReadsTheCycleOfEachJournalLine(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ship-journal.jsonl")
	writeFile(t, path, `{"sha":"a","class":"cycle","ts":"2026-08-11T02:08:33Z"}`+"\n\n"+`{"sha":"b","class":"cycle","cycle":2036}`+"\n")
	got, err := LoadShipped(path)
	if err != nil || len(got) != 1 || !got[2036] {
		t.Fatalf("shipped %v %v", got, err)
	}
	if got, err := LoadShipped(filepath.Join(t.TempDir(), "absent")); err != nil || len(got) != 0 {
		t.Fatalf("absent journal: %v %v", got, err)
	}
}

func TestLoadShipped_AMalformedLineOrAnUnreadableJournalFails(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ship-journal.jsonl")
	writeFile(t, path, "{\n")
	if _, err := LoadShipped(path); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want the line number", err)
	}
	if _, err := LoadShipped(t.TempDir()); err == nil {
		t.Fatal("a directory in place of the journal must fail")
	}
}

func writeCycle(t *testing.T, runs string, id int, run, triage, defects string) {
	t.Helper()
	dir := filepath.Join(runs, "cycle-"+strconv.Itoa(id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"run.json": run, "triage-decision.json": triage, "defect-ledger.json": defects} {
		if body != "" {
			writeFile(t, filepath.Join(dir, name), body)
		}
	}
}

func TestLoadCycles_ReadsTheNewestCyclesWithTheirItemRepairsShipsAndProtectedFindings(t *testing.T) {
	t.Parallel()
	runs := t.TempDir()
	writeCycle(t, runs, 2034, `{"started_at":"2026-10-26T10:00:00Z"}`, `{"top_n":[{"id":"core-one"}]}`, "")
	writeCycle(t, runs, 2035, `{"started_at":"2026-10-26T18:29:01Z","audit_repair_attempts":2}`, `{"top_n":[{"id":"retro-minting"}]}`,
		`{"entries":[{"text":"M1: go/internal/core/failurelearning floor tests fail; protected surface","status":"OPEN"},{"text":"H1: go/internal/core/x_test.go fails; unprotected, lane-fixable","status":"OPEN"},{"text":"go/internal/guards/role.go changed","status":"OPEN"},{"text":"go/internal/guards/ship.go changed","status":"CLOSED"}]}`)
	writeCycle(t, runs, 2036, `{"started_at":"2026-10-26T19:00:00Z"}`, "", "")
	writeFile(t, filepath.Join(runs, "cycle-2036", "failure-decision.json"), `{"level":"system","action":"halt-and-diagnose"}`)
	writeCycle(t, runs, 2000, `{"started_at":"2026-10-20T00:00:00Z"}`, "", "")
	if err := os.MkdirAll(filepath.Join(runs, "cycle-2037"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(runs, "cycle-9999"), "a file, not a run")
	writeFile(t, filepath.Join(runs, "notes.md"), "x")
	got, warnings, err := LoadCycles(runs, 4, map[int]bool{2036: true})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("%v %v", err, warnings)
	}
	if len(got) != 3 || got[0].ID != 2034 || got[2].ID != 2036 || !got[2].Shipped || got[2].Item != "" || got[2].FailureLevel != "system" || got[1].FailureLevel != "" {
		t.Fatalf("the four newest cycle directories hold three runs: %+v", got)
	}
	c := got[1]
	if c.Item != "retro-minting" || c.RepairAttempts != 2 || c.Shipped || !c.Started.Equal(time.Date(2026, 10, 26, 18, 29, 1, 0, time.UTC)) {
		t.Fatalf("cycle 2035: %+v", c)
	}
	if !slices.Equal(c.ProtectedFindings, []string{"go/internal/core/failurelearning", "go/internal/guards/role.go"}) {
		t.Fatalf("protected findings %v: the word protected names the first path, a manifest path names itself, unprotected and closed entries do not count", c.ProtectedFindings)
	}
}

func TestLoadCycles_ACorruptFileIsAWarningAndAnAbsentRunsDirectoryIsNoCycle(t *testing.T) {
	t.Parallel()
	runs := t.TempDir()
	writeCycle(t, runs, 1, "{", "", "")
	writeCycle(t, runs, 2, `{"started_at":"2026-10-26T10:00:00Z"}`, "{", "")
	got, warnings, err := LoadCycles(runs, 10, nil)
	if err != nil || len(got) != 1 || got[0].ID != 2 || len(warnings) != 2 {
		t.Fatalf("a corrupt run.json drops its cycle, a corrupt triage file keeps it; both warn: %+v %v %v", got, warnings, err)
	}
	if got, _, err := LoadCycles(filepath.Join(runs, "absent"), 10, nil); err != nil || got != nil {
		t.Fatalf("absent: %v %v", got, err)
	}
	file := filepath.Join(t.TempDir(), "plain")
	writeFile(t, file, "")
	if _, _, err := LoadCycles(file, 10, nil); err == nil {
		t.Fatal("a file in place of the runs directory must fail")
	}
}

func TestLoadUsage_ReadsEachObservationWithItsDeclaration(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	for _, obs := range []usageprobe.Observation{{CLI: "claude", ObservedAt: t0}, {CLI: "agy", ObservedAt: t0, Error: "boom"}} {
		if err := usageprobe.RecordObservation(evolveDir, obs); err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadUsage(evolveDir, func(f string) bool { return f == "claude" })
	if err != nil || len(got) != 2 || got[0].Family != "agy" || got[0].Declared || got[0].Error != "boom" || !got[1].Declared || got[1].Windows != 0 {
		t.Fatalf("usage %+v %v", got, err)
	}
	writeFile(t, usageprobe.WindowsPath(evolveDir), "{")
	if _, err := LoadUsage(evolveDir, func(string) bool { return true }); err == nil {
		t.Fatal("a corrupt usage file must fail")
	}
}

func TestMainGreen_IsTheConclusionOfTheLatestCompletedMainRun(t *testing.T) {
	t.Parallel()
	green := CIRun{ID: 2, Branch: "main", Status: "completed", Conclusion: "success", Created: t0.Add(time.Hour)}
	if !MainGreen([]CIRun{redMain(1, t0), green}) || MainGreen([]CIRun{green, redMain(3, t0.Add(2*time.Hour))}) || MainGreen(nil) {
		t.Fatal("MainGreen follows the newest completed main run")
	}
}

func TestTreeVerifier_ReadsTheReproTestTheWavesAndTheRetrospectives(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go/internal/x/x_test.go"), "package x\n\nfunc TestRepro(t *testing.T) {}\n")
	writeFile(t, filepath.Join(root, "docs/retrospectives/2026-10-27-inc.md"), "Incident inc-2026-10-27-0a1b2c3d closed.\n")
	writeFile(t, filepath.Join(root, "docs/retrospectives/other.md"), "nothing\n")
	ended := t0.Add(3 * time.Hour)
	v := TreeVerifier{Root: root, MainGreen: true, Waves: []wave.Record{{StartedAt: t0.Add(time.Hour)}, {StartedAt: t0.Add(2 * time.Hour), EndedAt: &ended}}}
	if !v.ReproGreen(Incident{ReproTest: "go/internal/x/x_test.go:TestRepro"}) {
		t.Fatal("a repro test that exists on a green main is green")
	}
	for _, repro := range []string{"", "go/internal/x/x_test.go", "go/internal/x/x_test.go:", "go/internal/x/x_test.go:TestOther", "absent.go:TestRepro"} {
		if v.ReproGreen(Incident{ReproTest: repro}) {
			t.Errorf("repro %q is not proven green", repro)
		}
	}
	if (TreeVerifier{Root: root}).ReproGreen(Incident{ReproTest: "go/internal/x/x_test.go:TestRepro"}) {
		t.Fatal("a red main proves no repro test green")
	}
	if !v.WaveEndedAfter(t0) || v.WaveEndedAfter(t0.Add(2*time.Hour)) {
		t.Fatal("only a wave that started after the time and ended counts")
	}
	if !v.RetrospectiveExists("inc-2026-10-27-0a1b2c3d") || v.RetrospectiveExists("inc-2026-10-27-ffffffff") || (TreeVerifier{Root: t.TempDir()}).RetrospectiveExists("inc-x") {
		t.Fatal("a retrospective exists when a file under docs/retrospectives names the id")
	}
}

func TestLoadCycles_ANegativeLimitReadsNoCycle(t *testing.T) {
	t.Parallel()
	runs := t.TempDir()
	writeCycle(t, runs, 1, `{"started_at":"2026-10-26T10:00:00Z"}`, "", "")
	if got, _, err := LoadCycles(runs, -1, nil); err != nil || len(got) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
}
