package bridge

import "testing"

func TestSetOnStopReviewStoresAndClearsCallback(t *testing.T) {
	a := New()

	var observed struct {
		cycle  int
		phase  string
		action string
		reason string
	}
	a.SetOnStopReview(func(cycle int, phase, action, reason string) {
		observed.cycle = cycle
		observed.phase = phase
		observed.action = action
		observed.reason = reason
	})
	if a.onStopReview == nil {
		t.Fatal("SetOnStopReview did not store callback")
	}

	a.onStopReview(282, "build", "pause", "update menu detected")
	if observed.cycle != 282 || observed.phase != "build" || observed.action != "pause" || observed.reason != "update menu detected" {
		t.Fatalf("callback observed %+v, want cycle=282 phase=build action=pause reason=update menu detected", observed)
	}

	a.SetOnStopReview(nil)
	if a.onStopReview != nil {
		t.Fatal("SetOnStopReview(nil) did not clear callback")
	}
}
