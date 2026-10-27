//go:build acs

package cycle2062

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	historyFileName   = "usage-history.ndjson"
	snapshotFileName  = "usage-windows.json"
	probedFamily      = "claude"
	headerOnlyOutput  = "CLI  SCOPE  KIND  USED  RESETS  TARGET  EXHAUSTED  BENCHES FAMILY\n"
	recordFailureWarn = "recording its usage windows failed"
)

type historyLine struct {
	CLI         string  `json:"cli"`
	Kind        string  `json:"kind"`
	PercentUsed float64 `json:"percent_used"`
}

func recordLikeTheCLI(evolveDir string) func(string, []quotastate.UsageWindow) error {
	return func(cli string, windows []quotastate.UsageWindow) error {
		return usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: cli, ObservedAt: time.Now().UTC(), Windows: windows})
	}
}

func proberReading(t *testing.T, evolveDir string, log *bytes.Buffer, windows func() []quotastate.UsageWindow) *usageprobe.Prober {
	t.Helper()
	return &usageprobe.Prober{
		Families: []string{probedFamily},
		Probe:    func(context.Context, string) (string, error) { return "the /usage screen", nil },
		Classify: func(string, string) bool { return false },
		Windows:  func(string, string) []quotastate.UsageWindow { return windows() },
		Record:   recordLikeTheCLI(evolveDir),
		Store:    clihealth.NewStore(t.TempDir(), nil),
		Log:      log,
	}
}

func sessionWindow(percentUsed float64) quotastate.UsageWindow {
	return quotastate.UsageWindow{Scope: "session", Kind: quotastate.KindSession, PercentUsed: percentUsed, Family: probedFamily}
}

func readHistory(t *testing.T, evolveDir string) []historyLine {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(evolveDir, historyFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read %s: %v", historyFileName, err)
	}
	var lines []historyLine
	for _, raw := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if raw == "" {
			continue
		}
		var l historyLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("history line is not a JSON object: %q: %v", raw, err)
		}
		lines = append(lines, l)
	}
	return lines
}

func TestC2062_001_TwoProbesGiveTwoHistoryLinesAndTheSnapshotKeepsTheLatest(t *testing.T) {
	evolveDir := t.TempDir()
	reading := 10.0
	prober := proberReading(t, evolveDir, &bytes.Buffer{}, func() []quotastate.UsageWindow {
		defer func() { reading += 25 }()
		return []quotastate.UsageWindow{sessionWindow(reading)}
	})

	prober.Run(context.Background())
	prober.Run(context.Background())

	lines := readHistory(t, evolveDir)
	if len(lines) != 2 {
		t.Fatalf("two probes of one window each must leave 2 history lines in %s, got %d", historyFileName, len(lines))
	}
	if lines[0].PercentUsed != 10 || lines[1].PercentUsed != 35 {
		t.Errorf("history must keep both readings in probe order, got %v then %v", lines[0].PercentUsed, lines[1].PercentUsed)
	}
	for i, l := range lines {
		if l.CLI != probedFamily || l.Kind != quotastate.KindSession {
			t.Errorf("history line %d must name its cli and window kind, got cli=%q kind=%q", i, l.CLI, l.Kind)
		}
	}
	latest, err := usageprobe.LoadObservations(evolveDir)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(latest) != 1 || len(latest[probedFamily].Windows) != 1 || latest[probedFamily].Windows[0].PercentUsed != 35 {
		t.Errorf("%s must hold only the latest observation (35%%), got %+v", snapshotFileName, latest)
	}
}

func TestC2062_002_EachWindowGetsItsOwnLineAndAnEmptyObservationAddsNone(t *testing.T) {
	evolveDir := t.TempDir()
	week := quotastate.UsageWindow{Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: 64, Family: probedFamily}
	observed := time.Date(2026, 10, 27, 7, 0, 0, 0, time.UTC)

	if err := usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: probedFamily, ObservedAt: observed, Windows: []quotastate.UsageWindow{sessionWindow(12), week}}); err != nil {
		t.Fatalf("record two windows: %v", err)
	}
	afterTwoWindows := readHistory(t, evolveDir)
	if len(afterTwoWindows) != 2 {
		t.Fatalf("an observation of 2 windows must append 2 lines, got %d", len(afterTwoWindows))
	}
	byKind := map[string]float64{}
	for _, l := range afterTwoWindows {
		byKind[l.Kind] = l.PercentUsed
	}
	if byKind[quotastate.KindSession] != 12 || byKind[quotastate.KindWeek] != 64 {
		t.Errorf("each window line must carry its own kind and percent, got %v", byKind)
	}

	if err := usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: probedFamily, ObservedAt: observed.Add(time.Minute), Error: "usage query failed"}); err != nil {
		t.Fatalf("record an observation with no windows: %v", err)
	}
	if got := readHistory(t, evolveDir); len(got) != 2 {
		t.Errorf("an observation with no windows must append no history line, got %d lines", len(got))
	}
}

func TestC2062_003_AppendKeepsLinesAlreadyInTheHistory(t *testing.T) {
	evolveDir := t.TempDir()
	earlier := `{"cli":"claude","kind":"week","percent_used":7}` + "\n"
	if err := os.WriteFile(filepath.Join(evolveDir, historyFileName), []byte(earlier), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: probedFamily, ObservedAt: time.Now().UTC(), Windows: []quotastate.UsageWindow{sessionWindow(50)}}); err != nil {
		t.Fatalf("record: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(evolveDir, historyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), earlier) {
		t.Errorf("history must be append-only; the earlier line was lost: %q", data)
	}
	if lines := readHistory(t, evolveDir); len(lines) != 2 {
		t.Errorf("want the earlier line plus 1 appended, got %d lines", len(lines))
	}
}

func TestC2062_004_EvidenceQueryPathWritesHistoryToo(t *testing.T) {
	evolveDir := t.TempDir()
	source := &usageprobe.EvidenceSource{
		Prober: &usageprobe.Prober{
			Probe:    func(context.Context, string) (string, error) { return "the /usage screen", nil },
			Classify: func(string, string) bool { return false },
			Windows: func(string, string) []quotastate.UsageWindow {
				return []quotastate.UsageWindow{sessionWindow(41)}
			},
			Store: clihealth.NewStore(t.TempDir(), nil),
			Log:   &bytes.Buffer{},
		},
		EvolveDir: evolveDir,
		Timeout:   10 * time.Second,
		TTL:       time.Minute,
		Act:       true,
	}

	source.Explain(context.Background(), usageprobe.Query{CLI: probedFamily, Family: probedFamily})

	lines := readHistory(t, evolveDir)
	if len(lines) != 1 || lines[0].PercentUsed != 41 || lines[0].CLI != probedFamily {
		t.Errorf("the evidence query path must append the observed window to the history, got %+v", lines)
	}
}

func TestC2062_005_HistoryWriteFailureWarnsAndTheProbeStillActs(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(evolveDir, historyFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	exhausted := quotastate.UsageWindow{Scope: "session", Kind: quotastate.KindSession, PercentUsed: 100, Family: probedFamily, Exhausted: true}
	log := &bytes.Buffer{}
	prober := proberReading(t, evolveDir, log, func() []quotastate.UsageWindow { return []quotastate.UsageWindow{exhausted} })

	recordErr := recordLikeTheCLI(evolveDir)(probedFamily, []quotastate.UsageWindow{sessionWindow(5)})
	if recordErr == nil {
		t.Errorf("RecordObservation must return the history write failure so the callers can warn")
	}

	prober.Run(context.Background())

	if !strings.Contains(log.String(), "WARN "+probedFamily+": "+recordFailureWarn) {
		t.Errorf("the probe must warn that recording failed, log was:\n%s", log.String())
	}
	if _, benched := prober.Store.Active()[probedFamily]; !benched {
		t.Errorf("a history write failure must not stop the probe from benching the exhausted family; log:\n%s", log.String())
	}
	latest, err := usageprobe.LoadObservations(evolveDir)
	if err != nil {
		t.Fatalf("the snapshot must stay valid after a history failure: %v", err)
	}
	if len(latest[probedFamily].Windows) != 1 || !latest[probedFamily].Windows[0].Exhausted {
		t.Errorf("%s must still hold the latest observation after a history failure, got %+v", snapshotFileName, latest)
	}
}

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "acs-cycle2062-bin-")
		if err != nil {
			buildErr = err
			return
		}
		wd, err := os.Getwd()
		if err != nil {
			buildErr = err
			return
		}
		module := filepath.Join(wd, "..", "..")
		bin := filepath.Join(dir, "evolve-under-test")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
		cmd.Dir = module
		if out, berr := cmd.CombinedOutput(); berr != nil {
			buildErr = errors.New("go build ./cmd/evolve: " + berr.Error() + "\n" + string(out))
			return
		}
		builtBin = bin
	})
	if buildErr != nil {
		t.Fatalf("evolve CLI did not build: %v", buildErr)
	}
	return builtBin
}

func TestC2062_006_ClihealthUsageOutputIsUnchangedAndWritesNoHistory(t *testing.T) {
	bin := evolveBinary(t)
	projectRoot := t.TempDir()

	cmd := exec.Command(bin, "clihealth", "usage", "--project-root", projectRoot, "no-such-family")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Errorf("an unknown family must still exit 1, got %v", err)
	}
	if stdout.String() != headerOnlyOutput {
		t.Errorf("evolve clihealth usage stdout changed:\nwant %q\ngot  %q", headerOnlyOutput, stdout.String())
	}
	if !strings.Contains(stderr.String(), `no-such-family: clicontrol: bridge: no interactive driver for family "no-such-family"`) {
		t.Errorf("evolve clihealth usage error line changed, stderr:\n%s", stderr.String())
	}
	for _, name := range []string{historyFileName, snapshotFileName} {
		if _, statErr := os.Stat(filepath.Join(projectRoot, ".evolve", name)); statErr == nil {
			t.Errorf("evolve clihealth usage is a read-only view and must not create %s", name)
		}
	}
}

func functionCoverageOfUsageprobe(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	module, err := acsassert.ModuleDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "usageprobe.cover")
	run := exec.Command("go", "test", "-count=1", "-coverprofile="+profile, "./internal/usageprobe")
	run.Dir = module
	if out, runErr := run.CombinedOutput(); runErr != nil {
		t.Fatalf("go test ./internal/usageprobe failed: %v\n%s", runErr, acsassert.OutputTail(string(out), 20))
	}
	report := exec.Command("go", "tool", "cover", "-func="+profile)
	report.Dir = module
	out, err := report.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, out)
	}
	return string(out)
}

func TestC2062_007_AppendHistoryAndRecordObservationAreFullyCovered(t *testing.T) {
	report := functionCoverageOfUsageprobe(t)
	for _, name := range []string{"appendHistory", "RecordObservation"} {
		pct, err := acsassert.FuncCoverage(report, name)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if pct != 100 {
			t.Errorf("%s statement coverage is %.1f%%, want 100%%: every error branch (marshal, open, write, close, history-failure return) needs a test", name, pct)
		}
	}
}
