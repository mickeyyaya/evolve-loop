package cycleblocker

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
)

func llm(minutes float64) cyclespend.Verdict {
	return cyclespend.Verdict{LLMMinutesToVerdict: minutes}
}

func TestAttribute_ACheckTagBeatsWaiting(t *testing.T) {
	t.Parallel()
	in := Input{CheckTags: []string{"ste-lint", "other"}, Class: "build|gate-block", Verdict: llm(13.5), WallMinutes: 200}
	got := Attribute(in, 50)
	if got.Blocker != "check:ste-lint" || got.Minutes != 13.5 {
		t.Fatalf("Attribute = %+v; want check:ste-lint with 13.5 minutes", got)
	}
}

func TestAttribute_AnUntaggedFailNamesItsClassNeverWaiting(t *testing.T) {
	t.Parallel()
	in := Input{Class: "audit|verdict-fail", Verdict: llm(10), WallMinutes: 500}
	got := Attribute(in, 20)
	if got.Blocker != "class:audit|verdict-fail" || got.Minutes != 10 {
		t.Fatalf("Attribute = %+v; want class:audit|verdict-fail with 10 minutes", got)
	}
}

func TestAttribute_AnUntaggedFailWithNoClassIsNone(t *testing.T) {
	t.Parallel()
	got := Attribute(Input{Verdict: llm(10), WallMinutes: 500}, 20)
	if got != (Attribution{Blocker: None}) {
		t.Fatalf("Attribute = %+v; want none with 0 minutes", got)
	}
}

func TestAttribute_TheFingerprintHoldsNoCycleNumber(t *testing.T) {
	t.Parallel()
	a := Attribute(Input{CheckTags: []string{"ste-lint"}, Verdict: llm(13.5)}, 50)
	b := Attribute(Input{CheckTags: []string{"ste-lint"}, Verdict: llm(8.7)}, 50)
	if a.Blocker != b.Blocker || a.Blocker != "check:ste-lint" {
		t.Fatalf("blockers %q and %q; want one equal check:ste-lint", a.Blocker, b.Blocker)
	}
	if strings.ContainsAny(a.Blocker, "0123456789") {
		t.Fatalf("blocker %q holds a digit", a.Blocker)
	}
}

func shipped(llmMin, hostMin, wall float64) Input {
	v := llm(llmMin)
	v.HostMinutesToVerdict = hostMin
	return Input{Shipped: true, Verdict: v, WallMinutes: wall}
}

func TestAttribute_WallFarAboveLLMNamesWaiting(t *testing.T) {
	t.Parallel()
	got := Attribute(shipped(20, 5, 90), 50)
	if got.Blocker != Waiting || got.Minutes != 65 {
		t.Fatalf("Attribute = %+v; want waiting with 65 minutes", got)
	}
}

func TestAttribute_GateMinutesAreNotWaiting(t *testing.T) {
	t.Parallel()
	got := Attribute(shipped(20, 40, 60), 50)
	if got != (Attribution{Blocker: None}) {
		t.Fatalf("Attribute = %+v; want none", got)
	}
}

func TestAttribute_APausedCycleNeverNamesWaiting(t *testing.T) {
	t.Parallel()
	in := shipped(20, 5, 90)
	in.Paused = true
	if got := Attribute(in, 50); got.Blocker != None {
		t.Fatalf("Attribute = %+v; want none", got)
	}
}

func TestAttribute_WaitingNeedsTheShareOfTheThreshold(t *testing.T) {
	t.Parallel()
	in := shipped(20, 5, 100)
	if got := Attribute(in, 75); got.Blocker != Waiting {
		t.Errorf("share 75 at threshold 75: %+v; want waiting", got)
	}
	if got := Attribute(in, 76); got.Blocker != None {
		t.Errorf("share 75 at threshold 76: %+v; want none", got)
	}
}

func TestAttribute_ANoWallShippedCycleIsNone(t *testing.T) {
	t.Parallel()
	if got := Attribute(shipped(20, 5, 0), 50); got.Blocker != None {
		t.Fatalf("Attribute = %+v; want none", got)
	}
}

func TestDecodeRecord_RefusesAnotherSchemaAndReadsV1(t *testing.T) {
	t.Parallel()
	ok, err := DecodeRecord([]byte(`{"schema":"evolve.blocker/1","cycle":2049,"blocker":"check:ste-lint","minutes":13.5}`))
	if err != nil || ok != (Record{Schema: SchemaV1, Cycle: 2049, Blocker: "check:ste-lint", Minutes: 13.5}) {
		t.Fatalf("DecodeRecord = %+v, %v", ok, err)
	}
	if _, err := DecodeRecord([]byte(`{"schema":"evolve.blocker/2","cycle":1}`)); !errors.Is(err, ErrSchema) {
		t.Errorf("another schema: err = %v; want ErrSchema", err)
	}
	if _, err := DecodeRecord([]byte(`{`)); err == nil {
		t.Error("malformed JSON: want an error")
	}
}
