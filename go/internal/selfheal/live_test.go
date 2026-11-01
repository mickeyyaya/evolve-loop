package selfheal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var liveNow = time.Date(2026, 10, 27, 7, 55, 7, 0, time.UTC)

func TestFromFailureClass_OnlyAPipelineCodeClassAtTheConsoleFloorIsObserved(t *testing.T) {
	t.Parallel()
	class := "build|gate-block|class-7098bbb545f3"
	cases := []struct {
		name string
		fc   FailureClass
		want bool
	}{
		{"third pipeline-code", FailureClass{Cycle: 2055, Fingerprint: class, Layer: "pipeline-code", Recurrence: 3}, true},
		{"second pipeline-code", FailureClass{Cycle: 2046, Fingerprint: class, Layer: "pipeline-code", Recurrence: 2}, false},
		{"harness", FailureClass{Cycle: 9, Fingerprint: class, Layer: "harness", Recurrence: 5}, false},
		{"task-code", FailureClass{Cycle: 2048}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, ok := FromFailureClass(tc.fc, liveNow)
			if ok != tc.want {
				t.Fatalf("FromFailureClass ok = %v; want %v", ok, tc.want)
			}
			if ok && (o.Class != ClassPipelineDefect || o.Subject != class || o.Severity != signalcenter.SeverityIncident || o.Evidence[0].ID != "cycle:2055" || !strings.Contains(o.Summary, "cycle 2055")) {
				t.Errorf("observation = %+v", o)
			}
		})
	}
}

func TestDetectLive_OpensOnceAndAbsorbsTheNextSeal(t *testing.T) {
	t.Parallel()
	o, _ := FromFailureClass(FailureClass{Cycle: 2055, Fingerprint: "c", Layer: "pipeline-code", Recurrence: 3}, liveNow)
	other := Incident{ID: "inc-other", Fingerprint: "stall:x", State: StateOpen}

	first, opened := DetectLive([]Incident{other}, o, liveNow)
	next, _ := FromFailureClass(FailureClass{Cycle: 2056, Fingerprint: "c", Layer: "pipeline-code", Recurrence: 4}, liveNow.Add(time.Hour))
	second, absorbed := DetectLive(first, next, liveNow.Add(time.Hour))

	if len(first) != 2 || !reflect.DeepEqual(first[0], other) || len(opened) != 1 || opened[0].Kind != TransitionOpened || first[1].Tier != TierPipeline || first[1].State != StateOpen {
		t.Fatalf("first seal: %+v %+v; want the other record kept and one opened T1 record", first, opened)
	}
	if len(second) != 2 || len(absorbed) != 0 || second[1].ID != first[1].ID || second[1].Recurrence != 2 {
		t.Fatalf("second seal: %+v %+v; want the same record with recurrence 2 and no transition", second, absorbed)
	}
	closed := second[1]
	closed.State = StateClosed
	third, reopened := DetectLive([]Incident{closed}, next, liveNow.Add(2*time.Hour))
	if len(reopened) != 1 || reopened[0].Kind != TransitionReopened || third[0].State != StateOpen {
		t.Errorf("a seal after the close: %+v %+v; want a reopen", third, reopened)
	}
}

func TestLoadFailureClass_ReadsTheHarnessRecord(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	dir := filepath.Join(evolveDir, "failure-digests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-2055.json"), []byte(`{"cycle":2055,"class_fingerprint":"c","class_layer":"pipeline-code","class_recurrence":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-1.json"), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}

	fc, found, err := LoadFailureClass(evolveDir, 2055)
	if err != nil || !found || fc != (FailureClass{Cycle: 2055, Fingerprint: "c", Layer: "pipeline-code", Recurrence: 3}) {
		t.Fatalf("LoadFailureClass = %+v %v %v", fc, found, err)
	}
	if _, found, err := LoadFailureClass(evolveDir, 7); found || err != nil {
		t.Errorf("a missing record = found %v, err %v; want neither", found, err)
	}
	if _, found, err := LoadFailureClass(evolveDir, 1); found || err == nil {
		t.Errorf("a corrupt record = found %v, err %v; want an error", found, err)
	}
}
