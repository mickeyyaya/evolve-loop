package recipe

import (
	"context"
	"testing"
	"time"
)

type recordingClock struct{ sleeps []time.Duration }

func (c *recordingClock) Sleep(d time.Duration) { c.sleeps = append(c.sleeps, d) }

func TestEngineRun_AwaitWithoutIntervalPollsAtDefaultCadence(t *testing.T) {
	t.Parallel()
	clock := &recordingClock{}
	d := &fakeDriver{paneSeq: []string{"still working"}}
	e := &Engine{Driver: d, Clock: clock, PromptMarker: "❯"}
	r := Recipe{Name: "x", Steps: []Step{cmdStep("go", "/go", Await{Kind: AwaitPromptMarker, TimeoutS: 6})}}

	_, err := e.Run(context.Background(), r, "claude-tmux", nil)

	if err == nil {
		t.Fatal("a step that never satisfies must time out")
	}
	want := time.Duration(defaultAwaitIntervalS) * time.Second
	if len(clock.sleeps) != 6/defaultAwaitIntervalS {
		t.Fatalf("sleeps=%v want %d polls of %v", clock.sleeps, 6/defaultAwaitIntervalS, want)
	}
	for _, got := range clock.sleeps {
		if got != want {
			t.Errorf("poll interval=%v want %v", got, want)
		}
	}
}
