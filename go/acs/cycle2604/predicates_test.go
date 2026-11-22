//go:build acs

package cycle2604

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	noReading    = "no reading yet"
	usageExit    = 10
	runTimeout   = 90 * time.Second
	coverTimeout = 5 * time.Minute
	channelLock  = 5 * time.Second
)

var (
	binDir  string
	binOnce sync.Once
	binPath string
	binErr  error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cycle2604-bin-")
	if err != nil {
		os.Exit(1)
	}
	binDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		binPath = filepath.Join(binDir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), coverTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/evolve")
		cmd.Dir = goDir(t)
		cmd.Env = ipcenv.GoTestEnv()
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			binErr = err
			binPath = out.String()
		}
	})
	if binErr != nil {
		t.Fatalf("go build ./cmd/evolve: %v\n%s", binErr, acsassert.OutputTail(binPath, 20))
	}
	return binPath
}

func runStatus(t *testing.T, root string, extra ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	args := append([]string{"status", "--project-root", root}, extra...)
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir = root
	cmd.Env = ipcenv.GoTestEnv()
	cmd.WaitDelay = 5 * time.Second
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout.String(), ee.ExitCode()
	}
	t.Fatalf("evolve status did not run: %v", err)
	return "", -1
}

func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(paths.EvolveDirOf(root), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

type seed struct {
	index  string
	iv     qualitypanel.Interval
	method qualitypanel.Method
	n      int
	window string
	stage  qualitypanel.Stage
	cycle  int
	ts     string
}

func (s seed) event(t *testing.T) signalcenter.Event {
	t.Helper()
	r := qualitypanel.Reading{Index: s.index, Interval: s.iv, Method: s.method, N: s.n, Window: s.window, Source: "acs", Cycle: s.cycle}
	evs := qualitypanel.Events(r, s.stage)
	if len(evs) == 0 || evs[0].Kind != signalcenter.KindQualityReading {
		t.Fatalf("qualitypanel.Events gave no reading event for %s", s.index)
	}
	e := evs[0]
	e.SchemaVersion, e.TS, e.PID = signalcenter.SchemaVersion, s.ts, 1
	return e
}

func appendStream(t *testing.T, path string, events ...signalcenter.Event) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func cycleStream(root string, cycle int) string {
	return filepath.Join(paths.RunWorkspace(root, cycle), signalcenter.StreamFileName)
}

func rootStream(root string) string {
	return filepath.Join(paths.EvolveDirOf(root), signalcenter.StreamFileName)
}

func appendChannel(t *testing.T, root string, events ...signalcenter.Event) {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "quality.reading", channel.Config{LockDeadline: channelLock})
	if err != nil {
		t.Fatal(err)
	}
	records := make([]channel.Record, len(events))
	for i := range events {
		records[i] = channel.Record{Source: "emit", Signal: &events[i]}
	}
	if _, err := log.Append(records); err != nil {
		t.Fatal(err)
	}
}

var (
	cq2Seed = seed{index: "CQ2", iv: qualitypanel.Interval{Value: 2.5, Lo: 2.0, Hi: 3.0}, method: qualitypanel.MethodBootstrap95, n: 119, window: "waves:82-140", stage: qualitypanel.StageReport, cycle: 12, ts: "2026-11-20T10:00:00Z"}
	pq1Seed = seed{index: "PQ1", iv: qualitypanel.Interval{Value: 0.424, Lo: 0.312, Hi: 0.544}, method: qualitypanel.MethodWilson95, n: 66, window: "waves:131-140", stage: qualitypanel.StageReport, ts: "2026-11-20T10:01:00Z"}
	pq4Seed = seed{index: "PQ4", iv: qualitypanel.Interval{Value: 0.167, Lo: 0.12, Hi: 0.227}, method: qualitypanel.MethodWilson95, n: 186, window: "waves:82-140", stage: qualitypanel.StageReport, ts: "2026-11-20T10:02:00Z"}
	jq3Seed = seed{index: "JQ3", iv: qualitypanel.Interval{Value: 0.6, Lo: 0.55, Hi: 0.7}, method: qualitypanel.MethodWilson95, n: 98, window: "reviews:all", stage: qualitypanel.StageReport, ts: "2026-11-20T10:03:00Z"}
	oq2Seed = seed{index: "OQ2", iv: qualitypanel.Interval{Value: 0.09}, method: qualitypanel.MethodCensus, n: 274, window: "commits:14-30d", stage: qualitypanel.StageReport, ts: "2026-11-20T10:04:00Z"}
	cq3Seed = seed{index: "CQ3", iv: qualitypanel.Interval{Value: 7.5, Lo: 6.5, Hi: 8.5}, method: qualitypanel.MethodBootstrap95, n: 40, window: "tree:main", stage: qualitypanel.StageShadow, ts: "2026-11-20T10:05:00Z"}
)

func seedMixedProject(t *testing.T) string {
	t.Helper()
	root := newProject(t)
	appendStream(t, cycleStream(root, 12), cq2Seed.event(t))
	appendStream(t, rootStream(root), pq4Seed.event(t))
	appendChannel(t, root, pq1Seed.event(t), jq3Seed.event(t), oq2Seed.event(t))
	appendChannel(t, root, cq3Seed.event(t))
	return root
}

func idsByStage(wantShadow bool) []string {
	var ids []string
	for _, ix := range qualitypanel.Indexes() {
		if (ix.DefaultStage() == qualitypanel.StageShadow) == wantShadow {
			ids = append(ids, ix.ID)
		}
	}
	return ids
}

func qualityBlock(out string) []string {
	var block []string
	in := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case !in && strings.HasPrefix(line, "Quality"):
			in = true
		case in && strings.TrimSpace(line) != "" && (line[0] == ' ' || line[0] == '\t'):
			block = append(block, line)
		case in:
			return block
		}
	}
	return block
}

func lineOf(block []string, id string) (string, int) {
	var found string
	count := 0
	for _, line := range block {
		if f := strings.Fields(line); len(f) > 0 && f[0] == id {
			found = line
			count++
		}
	}
	return found, count
}

func requireStatus(t *testing.T, root string, extra ...string) []string {
	t.Helper()
	out, code := runStatus(t, root, extra...)
	if code != 0 {
		t.Fatalf("evolve status %v exit=%d\n%s", extra, code, acsassert.OutputTail(out, 30))
	}
	block := qualityBlock(out)
	if len(block) == 0 {
		t.Fatalf("evolve status %v prints no Quality block\n%s", extra, acsassert.OutputTail(out, 30))
	}
	return block
}

func requireContainsAll(t *testing.T, id, line string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(line, w) {
			t.Errorf("%s line %q lacks %q", id, line, w)
		}
	}
}

func snapshotQuality(t *testing.T, root string) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(dashboard.Collect(root, time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Quality []map[string]any `json:"quality"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Quality) == 0 {
		t.Fatalf("Snapshot JSON has no quality array: %s", acsassert.OutputTail(string(raw), 5))
	}
	return doc.Quality
}

func entryFor(t *testing.T, quality []map[string]any, id string) map[string]any {
	t.Helper()
	for _, q := range quality {
		if q["index"] == id {
			return q
		}
	}
	t.Fatalf("Snapshot.Quality has no entry for %s", id)
	return nil
}

func TestC2604_001_StatusPrintsOneLinePerReportIndexWithValueIntervalNWindowStage(t *testing.T) {
	root := seedMixedProject(t)
	block := requireStatus(t, root)
	line, count := lineOf(block, "CQ2")
	if count != 1 {
		t.Fatalf("CQ2 has %d lines, want 1\n%s", count, strings.Join(block, "\n"))
	}
	requireContainsAll(t, "CQ2", line, "2.5 [2.0, 3.0]", "n=119", "waves:82-140", "report")
	line, count = lineOf(block, "PQ1")
	if count != 1 {
		t.Fatalf("PQ1 has %d lines, want 1", count)
	}
	requireContainsAll(t, "PQ1", line, "42.4% [31.2, 54.4]", "n=66", "waves:131-140", "report")
	reportIDs := idsByStage(false)
	if len(block) != len(reportIDs) {
		t.Errorf("block has %d lines, want one per report or gate index (%d)\n%s", len(block), len(reportIDs), strings.Join(block, "\n"))
	}
	for _, id := range reportIDs {
		if _, c := lineOf(block, id); c != 1 {
			t.Errorf("index %s has %d lines, want 1", id, c)
		}
	}
	for _, id := range idsByStage(true) {
		if _, c := lineOf(block, id); c != 0 {
			t.Errorf("shadow index %s is printed without --quality all", id)
		}
	}
}

func TestC2604_002_SnapshotQualityCarriesTheReadingFields(t *testing.T) {
	root := seedMixedProject(t)
	q := entryFor(t, snapshotQuality(t, root), "CQ2")
	want := map[string]any{"has_reading": true, "value": 2.5, "lo": 2.0, "hi": 3.0, "n": 119.0, "window": "waves:82-140", "stage": "report", "status": "ok"}
	for k, v := range want {
		if !reflect.DeepEqual(q[k], v) {
			t.Errorf("CQ2 entry %s = %#v, want %#v", k, q[k], v)
		}
	}
}

func TestC2604_003_LatestReadingWinsAcrossRootAndCycleStreams(t *testing.T) {
	root := newProject(t)
	older := cq2Seed
	older.iv, older.cycle, older.ts = qualitypanel.Interval{Value: 1.0, Lo: 1.0, Hi: 1.0}, 9, "2026-11-10T10:00:00Z"
	appendStream(t, cycleStream(root, 9), older.event(t))
	appendStream(t, cycleStream(root, 12), cq2Seed.event(t))
	appendStream(t, rootStream(root), pq4Seed.event(t))
	quality := snapshotQuality(t, root)
	if got := entryFor(t, quality, "CQ2")["value"]; got != 2.5 {
		t.Errorf("CQ2 value = %#v, want the newer reading 2.5", got)
	}
	if got := entryFor(t, quality, "PQ4")["has_reading"]; got != true {
		t.Errorf("PQ4 has_reading = %#v, want true: the root stream is not read", got)
	}
	block := requireStatus(t, root)
	line, _ := lineOf(block, "CQ2")
	if strings.Contains(line, "1.0 [1.0, 1.0]") || !strings.Contains(line, "2.5 [2.0, 3.0]") {
		t.Errorf("CQ2 line %q does not show the latest reading only", line)
	}
}

func TestC2604_004_ReadingFromTheEventChannelReachesThePanel(t *testing.T) {
	root := newProject(t)
	appendChannel(t, root, pq1Seed.event(t))
	block := requireStatus(t, root)
	line, _ := lineOf(block, "PQ1")
	requireContainsAll(t, "PQ1", line, "42.4% [31.2, 54.4]", "n=66")
	if got := entryFor(t, snapshotQuality(t, root), "PQ1")["has_reading"]; got != true {
		t.Errorf("Snapshot PQ1 has_reading = %#v, want true", got)
	}
}

func TestC2604_005_CensusReadingPrintsNoInterval(t *testing.T) {
	root := newProject(t)
	appendChannel(t, root, oq2Seed.event(t))
	line, _ := lineOf(requireStatus(t, root), "OQ2")
	requireContainsAll(t, "OQ2", line, "9.0%", "n=274", "commits:14-30d")
	if strings.ContainsAny(line, "[]") {
		t.Errorf("a census reading has no interval, line is %q", line)
	}
}

func TestC2604_006_NoLineOrFieldSumsOrAveragesIndexes(t *testing.T) {
	root := seedMixedProject(t)
	block := requireStatus(t, root, "--quality", "all")
	if want := len(qualitypanel.Indexes()); len(block) != want {
		t.Errorf("block has %d lines, want exactly one per index (%d): an extra line is an aggregate", len(block), want)
	}
	for _, line := range block {
		id := strings.Fields(line)[0]
		if _, ok := qualitypanel.Lookup(id); !ok {
			t.Errorf("block line %q does not start with a registered index id", line)
		}
		lower := strings.ToLower(line)
		for _, word := range []string{"total", "overall", "average", "mean ", "composite", "score", "sum "} {
			if strings.Contains(lower, word) {
				t.Errorf("line %q carries the aggregate word %q", line, word)
			}
		}
	}
	seen := map[string]bool{}
	for _, q := range snapshotQuality(t, root) {
		id, _ := q["index"].(string)
		if _, ok := qualitypanel.Lookup(id); !ok || seen[id] {
			t.Errorf("Snapshot.Quality entry %#v is not one registered index", q)
		}
		seen[id] = true
		for k := range q {
			if strings.Contains(strings.ToLower(k), "total") || strings.Contains(strings.ToLower(k), "average") || strings.Contains(strings.ToLower(k), "composite") {
				t.Errorf("Snapshot.Quality field %q aggregates indexes", k)
			}
		}
	}
}

func TestC2604_007_OnlyABreachReadingIsMarkedWithBang(t *testing.T) {
	root := seedMixedProject(t)
	block := requireStatus(t, root)
	marked := 0
	for _, line := range block {
		if strings.Contains(line, "!") {
			marked++
			if !strings.HasSuffix(strings.TrimSpace(line), "!") {
				t.Errorf("the mark is not at the end of %q", line)
			}
		}
	}
	line, _ := lineOf(block, "JQ3")
	if !strings.HasSuffix(strings.TrimSpace(line), "!") {
		t.Errorf("JQ3 breach line %q is not marked with !", line)
	}
	if marked != 1 {
		t.Errorf("%d lines carry !, want only the JQ3 breach\n%s", marked, strings.Join(block, "\n"))
	}
	if got := entryFor(t, snapshotQuality(t, root), "JQ3")["status"]; got != "breach" {
		t.Errorf("Snapshot JQ3 status = %#v, want breach", got)
	}
}

func TestC2604_008_AbsentStreamsPrintNoReadingYetForEachIndexAndExitZero(t *testing.T) {
	root := newProject(t)
	block := requireStatus(t, root)
	reportIDs := idsByStage(false)
	if len(block) != len(reportIDs) {
		t.Fatalf("block has %d lines, want %d\n%s", len(block), len(reportIDs), strings.Join(block, "\n"))
	}
	for _, id := range reportIDs {
		line, count := lineOf(block, id)
		if count != 1 || !strings.Contains(line, noReading) {
			t.Errorf("index %s line %q (count %d) lacks %q", id, line, count, noReading)
		}
	}
	for _, line := range block {
		if strings.Contains(strings.ToLower(line), "error") || strings.Contains(line, "!") {
			t.Errorf("an absent stream reads as a fault: %q", line)
		}
	}
	for _, q := range snapshotQuality(t, root) {
		if q["has_reading"] != false {
			t.Errorf("Snapshot entry %#v claims a reading with no stream", q)
		}
	}
}

func TestC2604_009_StreamsWithOnlyOtherKindsStillReadAsNoReadingYet(t *testing.T) {
	root := newProject(t)
	other := cq2Seed.event(t)
	other.Kind, other.Module = signalcenter.KindQualityWarning, signalcenter.ModuleQuality
	appendStream(t, cycleStream(root, 12), other)
	if err := os.WriteFile(rootStream(root), []byte("not json at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	line, _ := lineOf(requireStatus(t, root), "CQ2")
	if !strings.Contains(line, noReading) {
		t.Errorf("CQ2 line %q should read %q when no quality.reading exists", line, noReading)
	}
}

type coverTarget struct{ pkg, runFilter, fn string }

var coverTargets = []coverTarget{
	{"./internal/dashboard", ".", "readQuality"},
	{"./cmd/evolve", "Quality|Status", "writeStatusQuality"},
}

func coverFunc(t *testing.T, tg coverTarget) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), coverTimeout)
	defer cancel()
	run := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", tg.runFilter, "-coverprofile="+profile, tg.pkg)
	show := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+profile)
	var shown string
	for _, cmd := range []*exec.Cmd{run, show} {
		cmd.Dir, cmd.Env, cmd.WaitDelay = goDir(t), ipcenv.GoTestEnv(), 5*time.Second
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s: %v\n%s%s", strings.Join(cmd.Args, " "), err, stdout.String(), stderr.String())
		}
		shown = stdout.String()
	}
	return shown
}

func TestC2604_010_ChangedFunctionsAreAtFullCoverage(t *testing.T) {
	for _, tg := range coverTargets {
		pct, err := acsassert.FuncCoverage(coverFunc(t, tg), tg.fn)
		if err != nil {
			t.Errorf("%s: %v", tg.pkg, err)
			continue
		}
		if pct < 100.0 {
			t.Errorf("%s coverage %.1f%%, want 100.0%%", tg.fn, pct)
		}
	}
}

func TestC2604_011_QualityAllAlsoPrintsTheShadowIndexes(t *testing.T) {
	root := seedMixedProject(t)
	block := requireStatus(t, root, "--quality", "all")
	for _, ix := range qualitypanel.Indexes() {
		if _, c := lineOf(block, ix.ID); c != 1 {
			t.Errorf("--quality all prints %d lines for %s, want 1", c, ix.ID)
		}
	}
	line, _ := lineOf(block, "CQ3")
	requireContainsAll(t, "CQ3", line, "7.5 [6.5, 8.5]", "n=40", "tree:main", "shadow")
	shadowAbsent, _ := lineOf(block, idsByStage(true)[len(idsByStage(true))-1])
	if !strings.Contains(shadowAbsent, noReading) {
		t.Errorf("an absent shadow index line %q lacks %q", shadowAbsent, noReading)
	}
	if _, c := lineOf(requireStatus(t, root), "CQ3"); c != 0 {
		t.Errorf("the default view prints the shadow index CQ3")
	}
}

func TestC2604_012_UnknownQualityValueIsAUsageError(t *testing.T) {
	root := newProject(t)
	if _, code := runStatus(t, root, "--quality", "report"); code != 0 {
		t.Fatalf("--quality report exit=%d, want 0: the flag must exist before a bad value can be refused", code)
	}
	out, code := runStatus(t, root, "--quality", "bogus")
	if code != usageExit {
		t.Errorf("--quality bogus exit=%d, want %d", code, usageExit)
	}
	if len(qualityBlock(out)) != 0 {
		t.Errorf("a bad --quality value still prints a Quality block")
	}
}

func TestC2604_013_SnapshotQualityIsATypedSliceOfQualityReading(t *testing.T) {
	field, ok := reflect.TypeOf(dashboard.Snapshot{}).FieldByName("Quality")
	if !ok {
		t.Fatal("dashboard.Snapshot has no Quality field")
	}
	if field.Type.Kind() != reflect.Slice || field.Type.Elem().Name() != "QualityReading" {
		t.Errorf("Snapshot.Quality is %s, want []QualityReading", field.Type)
	}
	if tag := field.Tag.Get("json"); tag != "quality" {
		t.Errorf("Snapshot.Quality json tag = %q, want quality", tag)
	}
}

// acs-predicate: config-check
func TestC2604_014_APICoverNamesQualityReading(t *testing.T) {
	file := filepath.Join(goDir(t), "internal", "dashboard", "apicover_named_test.go")
	if !acsassert.FileContains(t, file, "QualityReading") {
		t.Errorf("internal/dashboard is enrolled in .apicover-enforce; %s must name QualityReading", file)
	}
}
