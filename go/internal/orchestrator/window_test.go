package orchestrator

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var _ interface {
	CleanLanding()
	Conflict()
} = (*Window)(nil)

func newWindow(start, floor, sized int) (*Window, *eventLog) {
	events := &eventLog{}
	return NewWindow(WindowConfig{Start: start, Min: floor, Sized: sized}, events.publish), events
}

func landings(w *Window, clean int) {
	for range clean {
		w.CleanLanding()
	}
}

func TestWindow_GrowsOnACleanLandingAndHalvesOnAConflict(t *testing.T) {
	w, _ := newWindow(4, 2, 12)
	if got := w.Size(); got != 4 {
		t.Fatalf("Size at the start = %d, want window.start 4 (a width above 4 does not open full)", got)
	}
	landings(w, 4)
	if got := w.Size(); got != 8 {
		t.Fatalf("Size after 4 clean landings = %d, want 8 (one more for each landing)", got)
	}
	w.Conflict()
	if got := w.Size(); got != 4 {
		t.Fatalf("Size after a conflict at 8 = %d, want 4 (halved)", got)
	}
	landings(w, 3)
	w.Conflict()
	if got := w.Size(); got != 3 {
		t.Fatalf("Size after a conflict at 7 = %d, want 3 (halving rounds down)", got)
	}
	w.Conflict()
	if got := w.Size(); got != 2 {
		t.Fatalf("Size after a conflict at 3 = %d, want window.min 2 (3 halves to 1, under the floor)", got)
	}
}

func TestWindow_AConflictNeverHalvesBelowWindowMin(t *testing.T) {
	w, events := newWindow(6, 4, 8)
	for range 3 {
		w.Conflict()
	}
	if got := w.Size(); got != 4 {
		t.Errorf("Size after 3 conflicts = %d, want window.min 4", got)
	}
	if got := len(events.withCode(CodeWindow)); got != 1 {
		t.Errorf("window events = %d, want 1: only the first conflict changed the window", got)
	}
}

func TestWindow_NeverGrowsPastTheSizedCount(t *testing.T) {
	w, events := newWindow(4, 4, 6)
	landings(w, 10)
	if got := w.Size(); got != 6 {
		t.Errorf("Size after 10 clean landings = %d, want the sized count 6", got)
	}
	if got := len(events.withCode(CodeWindow)); got != 2 {
		t.Errorf("window events = %d, want 2 (4 to 5, 5 to 6): a landing at the ceiling changes nothing", got)
	}
}

func TestWindow_AtOrBelowWidthFourItIsTheSizedCount(t *testing.T) {
	for sized := 1; sized <= 4; sized++ {
		t.Run("width "+strconv.Itoa(sized), func(t *testing.T) {
			w, events := newWindow(4, 4, sized)
			if got := w.Size(); got != sized {
				t.Fatalf("Size at the start = %d, want the sized count %d", got, sized)
			}
			w.Conflict()
			landings(w, 3)
			w.Conflict()
			if got := w.Size(); got != sized {
				t.Errorf("Size after landings and conflicts = %d, want the sized count %d: the window has no effect here", got, sized)
			}
			if got := len(events.withCode(CodeWindow)); got != 0 {
				t.Errorf("window events = %d, want none: the window never moved", got)
			}
		})
	}
}

func TestWindow_ResizeDownBelowTheWindowClampsItAndResizeUpDoesNotJumpIt(t *testing.T) {
	w, events := newWindow(6, 4, 8)
	w.Resize(5)
	if got := w.Size(); got != 5 {
		t.Fatalf("Size after Resize(5) = %d, want 5: the window never exceeds the sized count", got)
	}
	w.Resize(8)
	w.Resize(16)
	if got := w.Size(); got != 5 {
		t.Errorf("Size after Resize up = %d, want 5: a larger sized count does not open the window", got)
	}
	got := events.withCode(CodeWindow)
	if len(got) != 1 || got[0].Fields["cause"] != "resize" || got[0].Fields["old"] != "6" || got[0].Fields["new"] != "5" {
		t.Errorf("window events = %+v, want one resize event from 6 to 5", got)
	}
}

func TestWindow_AResizeBelowWindowMinPinsTheWindowToTheSizedCountAndTheFloorLiftsWithIt(t *testing.T) {
	w, _ := newWindow(6, 4, 8)
	w.Resize(3)
	if got := w.Size(); got != 3 {
		t.Fatalf("Size after Resize(3) = %d, want 3: a sized count under window.min pins the window to it", got)
	}
	w.Resize(8)
	if got := w.Size(); got != 4 {
		t.Errorf("Size after Resize(8) = %d, want window.min 4: the floor returns with the width, and nothing more", got)
	}
}

func TestWindow_PublishesOnceForEachChangeWithTheOldSizeTheNewSizeAndTheCause(t *testing.T) {
	w, events := newWindow(4, 4, 8)
	w.CleanLanding()
	w.Conflict()
	w.Conflict()
	w.Resize(6)
	w.CleanLanding()
	w.Resize(4)

	want := []map[string]string{
		{"old": "4", "new": "5", "cause": "clean_landing", "sized": "8"},
		{"old": "5", "new": "4", "cause": "conflict", "sized": "8"},
		{"old": "4", "new": "5", "cause": "clean_landing", "sized": "6"},
		{"old": "5", "new": "4", "cause": "resize", "sized": "4"},
	}
	got := events.withCode(CodeWindow)
	if len(got) != len(want) {
		t.Fatalf("window events = %d, want %d: one for each change, none for the second conflict at the floor or the resize that left the size", len(got), len(want))
	}
	for i, e := range got {
		if e.Module != signalcenter.ModuleLoop || e.Kind != signalcenter.KindLoopWave || e.Severity != signalcenter.SeverityInfo {
			t.Errorf("event %d = %+v, want an INFO loop.wave event on the loop module", i, e)
		}
		for key, val := range want[i] {
			if e.Fields[key] != val {
				t.Errorf("event %d field %s = %q, want %q", i, key, e.Fields[key], val)
			}
		}
	}
}

func TestWindow_APublisherCanReadTheWindowBecauseTheEventGoesOutAfterTheUnlock(t *testing.T) {
	var w *Window
	seen := make(chan int, 1)
	w = NewWindow(WindowConfig{Start: 4, Min: 4, Sized: 8}, func(signalcenter.Event) { seen <- w.Size() })
	returned := make(chan struct{})
	go func() {
		w.CleanLanding()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("CleanLanding did not return: the publisher waits for the mutex, so the event goes out while the window is locked")
	}
	if got := <-seen; got != 5 {
		t.Errorf("the publisher read size %d, want 5: the new size is visible inside the event", got)
	}
}

func TestNewWindow_ClampsAConfigThatPolicyWouldRefuse(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg                         WindowConfig
		want, afterConflict, landed int
	}{
		"no values":         {WindowConfig{}, 1, 1, 1},
		"start above sized": {WindowConfig{Start: 9, Min: 2, Sized: 5}, 5, 2, 3},
		"start below min":   {WindowConfig{Start: 1, Min: 3, Sized: 6}, 3, 3, 4},
		"min above sized":   {WindowConfig{Start: 4, Min: 4, Sized: 3}, 3, 3, 3},
	} {
		t.Run(name, func(t *testing.T) {
			w := NewWindow(tc.cfg, nil)
			if got := w.Size(); got != tc.want {
				t.Fatalf("Size = %d, want %d", got, tc.want)
			}
			w.Conflict()
			if got := w.Size(); got != tc.afterConflict {
				t.Errorf("Size after a conflict = %d, want %d", got, tc.afterConflict)
			}
			w.CleanLanding()
			if got := w.Size(); got != tc.landed {
				t.Errorf("Size after a clean landing = %d, want %d", got, tc.landed)
			}
		})
	}
}

func TestWindow_ConcurrentLandingsResizesAndReadsKeepItInsideItsBounds(t *testing.T) {
	w, _ := newWindow(4, 2, 12)
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				switch worker % 4 {
				case 0:
					w.CleanLanding()
				case 1:
					w.Conflict()
				case 2:
					w.Resize(6 + worker)
				default:
					if size := w.Size(); size < 2 || size > 12 {
						t.Errorf("Size = %d, want window.min 2 to the largest sized count 12", size)
					}
				}
			}
		}()
	}
	wg.Wait()
	if size := w.Size(); size < 2 || size > 12 {
		t.Errorf("Size after the run = %d, want window.min 2 to the largest sized count 12", size)
	}
}
