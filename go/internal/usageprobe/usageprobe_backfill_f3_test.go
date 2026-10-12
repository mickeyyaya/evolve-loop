package usageprobe

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func blockedStoreF3(t *testing.T) *clihealth.Store {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("a file where the state directory belongs"), 0o644); err != nil {
		t.Fatal(err)
	}
	return clihealth.NewStore(root, func() time.Time { return probeNow })
}

func blockedEvolveDirF3(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(blocker, "evolve")
}

func TestLoadObservations_EveryShapeOfTheFileOnDisk(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		wantLen int
		wantErr string
	}{
		{name: "no file yet", setup: func(*testing.T, string) {}},
		{name: "json null", setup: func(t *testing.T, dir string) { writeWindowsF3(t, dir, "null") }},
		{name: "empty object", setup: func(t *testing.T, dir string) { writeWindowsF3(t, dir, "{}") }},
		{name: "one cli", setup: func(t *testing.T, dir string) {
			writeWindowsF3(t, dir, `{"agy":{"cli":"agy","observed_at":"2026-10-06T12:00:00Z","windows":null}}`)
		}, wantLen: 1},
		{name: "torn json", setup: func(t *testing.T, dir string) { writeWindowsF3(t, dir, `{"agy":`) }, wantErr: WindowsFile + ": unexpected end of JSON input"},
		{name: "a directory where the file belongs", setup: func(t *testing.T, dir string) {
			if err := os.MkdirAll(WindowsPath(dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}, wantErr: "usage windows:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			tc.setup(t, dir)

			got, err := LoadObservations(dir)

			if tc.wantErr != "" {
				if got != nil || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got=%v err=%v", got, err)
				}
				return
			}
			if err != nil || got == nil || len(got) != tc.wantLen {
				t.Fatalf("a readable file always yields a usable map: got=%v err=%v", got, err)
			}
		})
	}
}

func writeWindowsF3(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(WindowsPath(dir), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecordObservation_FailuresLeaveTheRecordUntouched(t *testing.T) {
	t.Parallel()
	t.Run("the state directory cannot be created", func(t *testing.T) {
		t.Parallel()

		err := RecordObservation(blockedEvolveDirF3(t), Observation{CLI: "agy"})

		if err == nil || !strings.Contains(err.Error(), "usage windows:") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("an unreadable record is not overwritten", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeWindowsF3(t, dir, `{torn`)

		err := RecordObservation(dir, Observation{CLI: "agy"})

		if err == nil || !strings.Contains(err.Error(), "usage windows:") {
			t.Fatalf("err = %v", err)
		}
		if body, _ := os.ReadFile(WindowsPath(dir)); string(body) != `{torn` {
			t.Fatalf("the corrupt record must be left for a person to inspect, got %q", body)
		}
	})
}

func TestRecordObservation_ConcurrentWritersLoseNoCLI(t *testing.T) {
	dir := t.TempDir()
	clis := []string{"agy", "claude", "codex", "ollama", "gemini", "aider", "x1", "x2"}
	var wg sync.WaitGroup
	for _, cli := range clis {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := RecordObservation(dir, Observation{CLI: cli, ObservedAt: probeNow}); err != nil {
				t.Errorf("%s: %v", cli, err)
			}
		}()
	}
	wg.Wait()

	got, err := LoadObservations(dir)

	if err != nil || len(got) != len(clis) {
		t.Fatalf("got %d of %d records (err=%v): %v", len(got), len(clis), err, got)
	}
	for _, cli := range clis {
		if got[cli].CLI != cli {
			t.Errorf("record for %s = %+v", cli, got[cli])
		}
	}
}

func TestProberBenchFamily_AFailingStoreIsAWarningNotAPanic(t *testing.T) {
	t.Parallel()
	log := &bytes.Buffer{}
	p := &Prober{Store: blockedStoreF3(t), Log: log}

	p.benchFamily("claude", "the pane")

	if !strings.Contains(log.String(), "WARN bench claude failed:") || strings.Contains(log.String(), "capped") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestProberBenchWall_AFailingStoreIsAWarningNamingTheFamily(t *testing.T) {
	t.Parallel()
	log := &bytes.Buffer{}
	p := &Prober{Store: blockedStoreF3(t), Log: log}

	p.benchWall("agy", familyWall{family: "agy-claude", evidence: []string{"a", "b"}})

	if !strings.Contains(log.String(), "WARN bench agy-claude failed:") || strings.Contains(log.String(), "benched") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestProberRecord_ARecorderErrorIsLoggedAndNothingElseHappens(t *testing.T) {
	t.Parallel()
	log := &bytes.Buffer{}
	boom := errors.New("disk full")
	p := &Prober{Log: log, Record: func(string, []quotastate.UsageWindow) error { return boom }}
	w := []quotastate.UsageWindow{{Scope: "s"}}

	p.record("agy", reading{windows: w})

	if want := "[usage-probe] WARN agy: recording its usage windows failed: disk full\n"; log.String() != want {
		t.Fatalf("log = %q, want %q", log.String(), want)
	}
	log.Reset()
	p.record("agy", reading{})
	(&Prober{Log: log}).record("agy", reading{windows: w})
	if log.Len() != 0 {
		t.Fatalf("no windows or no recorder means no write and no log: %q", log.String())
	}
}

func TestWithWall_MergesWindowsOfOneFamilyKeepingTheLatestResetAndEveryEvidence(t *testing.T) {
	t.Parallel()
	early, late := probeNow.Add(time.Hour), probeNow.Add(5*time.Hour)
	win := func(family, scope string, reset *time.Time) quotastate.UsageWindow {
		return quotastate.UsageWindow{Family: family, Scope: scope, Kind: quotastate.KindFiveHour, PercentUsed: 100, ResetsAt: reset, Exhausted: true}
	}
	cases := []struct {
		name      string
		windows   []quotastate.UsageWindow
		wantWalls int
		wantReset time.Time
		wantEvid  int
	}{
		{name: "one window", windows: []quotastate.UsageWindow{win("a", "s1", &early)}, wantWalls: 1, wantReset: early, wantEvid: 1},
		{name: "a later reset replaces an earlier one", windows: []quotastate.UsageWindow{win("a", "s1", &early), win("a", "s2", &late)}, wantWalls: 1, wantReset: late, wantEvid: 2},
		{name: "an earlier reset never replaces a later one", windows: []quotastate.UsageWindow{win("a", "s1", &late), win("a", "s2", &early)}, wantWalls: 1, wantReset: late, wantEvid: 2},
		{name: "a window with no reset keeps the known one", windows: []quotastate.UsageWindow{win("a", "s1", &early), win("a", "s2", nil)}, wantWalls: 1, wantReset: early, wantEvid: 2},
		{name: "families stay apart", windows: []quotastate.UsageWindow{win("a", "s1", &early), win("b", "s2", &late), win("a", "s3", &late)}, wantWalls: 2, wantReset: late, wantEvid: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var walls []familyWall

			for _, w := range tc.windows {
				walls = withWall(walls, w)
			}

			if len(walls) != tc.wantWalls || !walls[0].reset.Equal(tc.wantReset) || len(walls[0].evidence) != tc.wantEvid || walls[0].family != "a" {
				t.Fatalf("walls = %+v", walls)
			}
		})
	}
}

type flipCtxF3 struct {
	context.Context
	calls int
}

func (c *flipCtxF3) Err() error {
	c.calls++
	if c.calls > 1 {
		return context.Canceled
	}
	return nil
}

func TestProbeOne_ACancelAfterTheProbeAnswersBenchesAndRecordsNothing(t *testing.T) {
	t.Parallel()
	log := &bytes.Buffer{}
	touched := 0
	p := &Prober{
		Probe:    func(context.Context, string) (string, error) { return "a pane read just before the interrupt", nil },
		Classify: func(string, string) bool { touched++; return true },
		Windows:  func(string, string) []quotastate.UsageWindow { touched++; return agyWindows(100) },
		Record:   func(string, []quotastate.UsageWindow) error { touched++; return nil },
		Store:    clihealth.NewStore(t.TempDir(), func() time.Time { return probeNow }),
		Log:      log,
	}
	ctx := &flipCtxF3{Context: context.Background()}

	p.probeOne(ctx, "agy")

	if touched != 0 || log.Len() != 0 || len(p.Store.Active()) != 0 || ctx.calls != 2 {
		t.Fatalf("touched=%d log=%q benches=%v err-calls=%d", touched, log.String(), p.Store.Active(), ctx.calls)
	}
}

func TestEvidenceSourceNow_FallsBackToTheWallClock(t *testing.T) {
	t.Parallel()
	before := time.Now()

	got := (&EvidenceSource{}).now()

	if got.Before(before) || time.Since(got) > time.Minute {
		t.Fatalf("now() = %v, want the current time (began %v)", got, before)
	}
	fixed := &EvidenceSource{Now: func() time.Time { return probeNow }}
	if !fixed.now().Equal(probeNow) {
		t.Fatalf("an injected clock wins: %v", fixed.now())
	}
}

func TestEvidenceSourceFresh_WithoutAnEvolveDirOnlyTheInMemoryCacheCounts(t *testing.T) {
	t.Parallel()
	s := &EvidenceSource{TTL: time.Hour, Now: func() time.Time { return probeNow }}
	q := Query{CLI: "agy", Family: "agy"}

	if obs, ok := s.fresh(q); ok || obs.CLI != "" {
		t.Fatalf("an empty cache and no directory yields nothing: %+v %v", obs, ok)
	}
	s.cache = map[string]Observation{"agy": {CLI: "agy", ObservedAt: probeNow.Add(-time.Minute)}}
	if obs, ok := s.fresh(q); !ok || obs.CLI != "agy" {
		t.Fatalf("a young cached observation is reused: %+v %v", obs, ok)
	}
	s.cache["agy"] = Observation{CLI: "agy", ObservedAt: probeNow.Add(-2 * time.Hour)}
	if _, ok := s.fresh(q); ok {
		t.Fatal("an observation older than the TTL is not reused")
	}
}

func TestEvidenceSourceRemember_AFailedRecordIsLoggedWhenALogIsWiredAndSilentOtherwise(t *testing.T) {
	t.Parallel()
	obs := Observation{CLI: "agy", ObservedAt: probeNow}
	t.Run("logged", func(t *testing.T) {
		t.Parallel()
		log := &bytes.Buffer{}
		s := &EvidenceSource{Act: true, EvolveDir: blockedEvolveDirF3(t), Prober: &Prober{Log: log}}

		s.remember(obs)

		if got := log.String(); !strings.HasPrefix(got, "[usage-evidence] WARN agy: recording the usage query failed: usage windows:") {
			t.Fatalf("log = %q", got)
		}
	})
	t.Run("no log wired", func(t *testing.T) {
		t.Parallel()
		s := &EvidenceSource{Act: true, EvolveDir: blockedEvolveDirF3(t), Prober: &Prober{}}

		s.remember(obs)
	})
	t.Run("not acting writes nothing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		(&EvidenceSource{Act: false, EvolveDir: dir, Prober: &Prober{}}).remember(obs)
		(&EvidenceSource{Act: true, EvolveDir: "", Prober: &Prober{}}).remember(obs)

		if _, err := os.Stat(WindowsPath(dir)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a passive source must not write: %v", err)
		}
	})
}

func TestEvidenceSourceFresh_WithoutAnEvolveDirNeverReadsTheWorkingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := RecordObservation(dir, Observation{CLI: "agy", ObservedAt: probeNow.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("restore the working directory: %v", err)
		}
	})
	s := &EvidenceSource{TTL: time.Hour, Now: func() time.Time { return probeNow }}

	if obs, ok := s.fresh(Query{CLI: "agy", Family: "agy"}); ok {
		t.Fatalf("a source with no EvolveDir must not borrow a record from the working directory: %+v", obs)
	}
}
