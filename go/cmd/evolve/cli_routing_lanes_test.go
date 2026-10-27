package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

const headroomBuilderTable = `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"],"agents":{"builder":{"cli":["agy","claude"],"model":"balanced","order":"headroom"}}}}`

func laneFixtureEvolveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "..", "internal", "lanetime", "testdata", "evolve"))); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRecordLaneTimes_WritesTheMeasureThatTheRoutingHostReads(t *testing.T) {
	t.Parallel()
	dir := laneFixtureEvolveDir(t)
	var log bytes.Buffer
	recordLaneTimes(dir, &log)
	got, err := loadLaneTimes(dir)
	if err != nil {
		t.Fatalf("loadLaneTimes: %v", err)
	}
	cycles, _, err := lanetime.ReadCycles(dir, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := lanetime.LaneTimeToPass(cycles, lanetime.DefaultBuilds); !reflect.DeepEqual(got, want) || len(got) != 2 {
		t.Fatalf("read back %+v, want %+v", got, want)
	}
	if !strings.Contains(log.String(), "2 families") {
		t.Fatalf("the boundary must log the measure, got %q", log.String())
	}
}

func TestRecordLaneTimes_AMissingRunsDirWarnsAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var log bytes.Buffer
	recordLaneTimes(dir, &log)
	if !strings.Contains(log.String(), "WARN") {
		t.Fatalf("a failed read must warn, got %q", log.String())
	}
	if lanes, err := loadLaneTimes(dir); err != nil || lanes != nil {
		t.Fatalf("no file means no measure and no error, got %+v %v", lanes, err)
	}
}

func TestRecordLaneTimes_AFailedWriteWarns(t *testing.T) {
	t.Parallel()
	dir := laneFixtureEvolveDir(t)
	if err := os.Mkdir(filepath.Join(dir, laneTimesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	recordLaneTimes(dir, &log)
	if !strings.Contains(log.String(), "WARN") {
		t.Fatalf("a failed write must warn, got %q", log.String())
	}
}

func TestLoadLaneTimes_AMalformedFileIsAnErrorThatNamesIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, laneTimesFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadLaneTimes(dir); err == nil || !strings.Contains(err.Error(), laneTimesFile) {
		t.Fatalf("want an error naming %s, got %v", laneTimesFile, err)
	}
	if err := os.Chmod(filepath.Join(dir, laneTimesFile), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := loadLaneTimes(dir); err == nil {
		t.Fatal("an unreadable file must be an error")
	}
}

func headroomExplainRoot(t *testing.T, claudeWeekPct float64, lanes string) string {
	t.Helper()
	root := routingFixture(t, headroomBuilderTable)
	evolveDir := filepath.Join(root, ".evolve")
	week := []quotastate.UsageWindow{{Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: claudeWeekPct, Family: "claude"}}
	if err := usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: "claude", ObservedAt: time.Now(), Windows: week}); err != nil {
		t.Fatal(err)
	}
	if lanes != "" {
		if err := os.WriteFile(filepath.Join(evolveDir, laneTimesFile), []byte(lanes), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCLIRoutingExplain_TheBuildSeatShowsTheHeadroomFilterAndTheMeasuredOrder(t *testing.T) {
	rc, out, _ := runRoutingVerb(t, "explain", "builder", "--project-root", headroomExplainRoot(t, 14, ""))
	if rc != 0 || !strings.Contains(out, "chain:   claude-tmux → agy-tmux") {
		t.Fatalf("week 14%% and no measure: claude leads: rc=%d\n%s", rc, out)
	}
	for _, want := range []string{"headroom 60%: claude week 14% · agy week unknown", "no family has a measure"} {
		if !strings.Contains(out, want) {
			t.Fatalf("explain must show %q:\n%s", want, out)
		}
	}
	measured := `[{"family":"agy","builds":20,"passes":10,"lane":18000000000000,"per_pass":1800000000000},{"family":"claude","builds":20,"passes":16,"lane":57600000000000,"per_pass":3600000000000}]`
	rc, out, _ = runRoutingVerb(t, "explain", "builder", "--project-root", headroomExplainRoot(t, 14, measured))
	if rc != 0 || !strings.Contains(out, "chain:   agy-tmux → claude-tmux") || !strings.Contains(out, "agy 30m0s per PASS (20 builds, 10 passes)") {
		t.Fatalf("a shorter agy lane time to a PASS leads:\n%s", out)
	}
	rc, out, _ = runRoutingVerb(t, "explain", "builder", "--project-root", headroomExplainRoot(t, 75, ""))
	if rc != 0 || !strings.Contains(out, "chain:   agy-tmux → claude-tmux") || !strings.Contains(out, "leaves the head") {
		t.Fatalf("claude at 75%% leaves the head:\n%s", out)
	}
}

func TestCLIRoutingExplain_TheStaticViewHasNoUsageReadSoThePolicyOrderStays(t *testing.T) {
	rc, out, _ := runRoutingVerb(t, "explain", "builder", "--static", "--project-root", headroomExplainRoot(t, 14, ""))
	if rc != 0 || !strings.Contains(out, "chain:   agy-tmux → claude-tmux") || !strings.Contains(out, "the policy order stays") {
		t.Fatalf("static explain keeps the policy order and says why:\n%s", out)
	}
}

func TestRunPreWaveProbes_TheWaveBoundaryRecordsTheLaneTimeOnce(t *testing.T) {
	t.Parallel()
	dir := laneFixtureEvolveDir(t)
	var log bytes.Buffer
	if err := runPreWaveProbes(context.Background(), t.TempDir(), dir, &log); err != nil {
		t.Fatalf("runPreWaveProbes: %v", err)
	}
	lanes, err := loadLaneTimes(dir)
	if err != nil || len(lanes) != 2 {
		t.Fatalf("the boundary must leave the measure for the lanes, got %+v %v\n%s", lanes, err, log.String())
	}
}
