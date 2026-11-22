package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func qualityEvent(r qualitypanel.Reading, stage qualitypanel.Stage, ts string) signalcenter.Event {
	evs := qualitypanel.Events(r, stage)
	e := evs[0]
	e.SchemaVersion = signalcenter.SchemaVersion
	e.TS = ts
	e.PID = 1
	return e
}

func appendQualityStream(t *testing.T, path string, events ...signalcenter.Event) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, line...)
		data = append(data, '\n')
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendQualityChannel(t *testing.T, root string, events ...signalcenter.Event) {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "quality.reading", channel.Config{LockDeadline: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	recs := make([]channel.Record, len(events))
	for i := range events {
		recs[i] = channel.Record{Source: "test", Signal: &events[i]}
	}
	if _, err := log.Append(recs); err != nil {
		t.Fatal(err)
	}
}

func findQualityEntry(t *testing.T, list []QualityReading, id string) QualityReading {
	t.Helper()
	for _, q := range list {
		if q.Index == id {
			return q
		}
	}
	t.Fatalf("missing quality entry %s", id)
	return QualityReading{}
}

func TestReadQuality_CollectsLatestAcrossStreamsAndChannel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldCQ2 := qualityEvent(qualitypanel.Reading{Index: "CQ2", Interval: qualitypanel.Interval{Value: 1.0, Lo: 1.0, Hi: 1.0}, Method: qualitypanel.MethodBootstrap95, N: 10, Window: "waves:1-2", Cycle: 5}, qualitypanel.StageReport, "2026-11-10T10:00:00Z")
	newCQ2 := qualityEvent(qualitypanel.Reading{Index: "CQ2", Interval: qualitypanel.Interval{Value: 2.5, Lo: 2.0, Hi: 3.0}, Method: qualitypanel.MethodBootstrap95, N: 119, Window: "waves:82-140", Cycle: 12}, qualitypanel.StageReport, "2026-11-20T10:00:00Z")
	oldPQ4 := qualityEvent(qualitypanel.Reading{Index: "PQ4", Interval: qualitypanel.Interval{Value: 0.5, Lo: 0.4, Hi: 0.6}, Method: qualitypanel.MethodWilson95, N: 10, Window: "waves:1-2"}, qualitypanel.StageReport, "2026-11-01T10:00:00Z")
	pq4 := qualityEvent(qualitypanel.Reading{Index: "PQ4", Interval: qualitypanel.Interval{Value: 0.167, Lo: 0.12, Hi: 0.227}, Method: qualitypanel.MethodWilson95, N: 186, Window: "waves:82-140"}, qualitypanel.StageReport, "2026-11-20T10:02:00Z")
	oq2 := qualityEvent(qualitypanel.Reading{Index: "OQ2", Interval: qualitypanel.Interval{Value: 0.09}, Method: qualitypanel.MethodCensus, N: 274, Window: "commits:14-30d"}, qualitypanel.StageReport, "2026-11-20T10:04:00Z")
	oq3 := qualityEvent(qualitypanel.Reading{Index: "OQ3", Interval: qualitypanel.Interval{Value: 0.0, Lo: 0.0, Hi: 0.55}, Method: qualitypanel.MethodWilson95, N: 695, Window: "days:30"}, qualitypanel.StageReport, "2026-11-20T10:05:00Z")
	unknown := oldCQ2
	unknown.Fields["index"] = "ZZ9"

	appendQualityStream(t, filepath.Join(runsDir(root), "cycle-5", signalcenter.StreamFileName), oldCQ2, oldPQ4, unknown)
	appendQualityStream(t, filepath.Join(runsDir(root), "cycle-12", signalcenter.StreamFileName), newCQ2)
	appendQualityStream(t, filepath.Join(evolveDir, signalcenter.StreamFileName), pq4)
	appendQualityChannel(t, root, oq2, oq3)

	snap := Collect(root, time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC))
	if len(snap.Quality) != len(qualitypanel.Indexes()) {
		t.Fatalf("len(Quality) = %d, want %d", len(snap.Quality), len(qualitypanel.Indexes()))
	}
	gotCQ2 := findQualityEntry(t, snap.Quality, "CQ2")
	if !gotCQ2.HasReading || gotCQ2.Value == nil || *gotCQ2.Value != 2.5 || gotCQ2.Lo == nil || *gotCQ2.Lo != 2.0 || gotCQ2.Hi == nil || *gotCQ2.Hi != 3.0 || gotCQ2.N != 119 || gotCQ2.Window != "waves:82-140" || gotCQ2.Stage != "report" || gotCQ2.Status != "ok" {
		t.Fatalf("CQ2 = %+v", gotCQ2)
	}
	gotOQ2 := findQualityEntry(t, snap.Quality, "OQ2")
	if !gotOQ2.HasReading || gotOQ2.Lo != nil || gotOQ2.Hi != nil || gotOQ2.Value == nil || *gotOQ2.Value != 0.09 {
		t.Fatalf("OQ2 = %+v", gotOQ2)
	}
	gotOQ3 := findQualityEntry(t, snap.Quality, "OQ3")
	if !gotOQ3.HasReading || gotOQ3.Value == nil || *gotOQ3.Value != 0.0 || gotOQ3.Lo == nil || *gotOQ3.Lo != 0.0 {
		t.Fatalf("OQ3 zero values must survive: %+v", gotOQ3)
	}
	gotPQ4 := findQualityEntry(t, snap.Quality, "PQ4")
	if !gotPQ4.HasReading || gotPQ4.N != 186 {
		t.Fatalf("PQ4 = %+v", gotPQ4)
	}
}

func TestReadQuality_AbsentAndMalformedStreamsDefaultCleanly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	warnEvent := qualityEvent(qualitypanel.Reading{Index: "CQ2", Interval: qualitypanel.Interval{Value: 2.0, Lo: 2.0, Hi: 2.0}, Method: qualitypanel.MethodBootstrap95, N: 50, Window: "w"}, qualitypanel.StageReport, "2026-11-20T10:00:00Z")
	warnEvent.Kind = signalcenter.KindQualityWarning
	badValue := qualityEvent(qualitypanel.Reading{Index: "CQ2", Interval: qualitypanel.Interval{Value: 2.0, Lo: 2.0, Hi: 2.0}, Method: qualitypanel.MethodBootstrap95, N: 50, Window: "w"}, qualitypanel.StageReport, "2026-11-20T10:00:00Z")
	badValue.Fields["value"] = "nan_nope"
	badTS := qualityEvent(qualitypanel.Reading{Index: "PQ1", Interval: qualitypanel.Interval{Value: 0.5, Lo: 0.4, Hi: 0.6}, Method: qualitypanel.MethodWilson95, N: 50, Window: "w"}, qualitypanel.StageReport, "not-a-timestamp")

	appendQualityStream(t, filepath.Join(runsDir(root), "cycle-3", signalcenter.StreamFileName), warnEvent, badValue)
	if err := os.WriteFile(filepath.Join(evolveDir, signalcenter.StreamFileName), []byte("corrupt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appendQualityChannel(t, root, badTS)

	list := readQuality(root)
	if len(list) != len(qualitypanel.Indexes()) {
		t.Fatalf("len = %d, want %d", len(list), len(qualitypanel.Indexes()))
	}
	for _, q := range list {
		if q.HasReading {
			t.Errorf("expected no reading for %s, got %+v", q.Index, q)
		}
		ix, _ := qualitypanel.Lookup(q.Index)
		if q.Stage != string(ix.DefaultStage()) {
			t.Errorf("%s stage = %q, want %q", q.Index, q.Stage, ix.DefaultStage())
		}
	}
}

func TestReadQuality_EmptyStageFallsBackAndBadChannelIsIgnored(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	chRoot := paths.EventChannelsRoot(paths.EvolveDirOf(root))
	if err := os.MkdirAll(chRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chRoot, "quality.reading"), []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev := qualityEvent(qualitypanel.Reading{Index: "CQ2", Interval: qualitypanel.Interval{Value: 2.0, Lo: 1.5, Hi: 2.5}, Method: qualitypanel.MethodBootstrap95, N: 20, Window: "w"}, qualitypanel.StageReport, "2026-11-20T10:00:00Z")
	ev.Fields["stage"] = ""
	ev.Fields["lo"] = "bad"
	appendQualityStream(t, filepath.Join(paths.EvolveDirOf(root), signalcenter.StreamFileName), ev)

	list := readQuality(root)
	got := findQualityEntry(t, list, "CQ2")
	if !got.HasReading || got.Stage != "report" || got.Lo != nil || got.Hi != nil {
		t.Fatalf("CQ2 = %+v", got)
	}
}
