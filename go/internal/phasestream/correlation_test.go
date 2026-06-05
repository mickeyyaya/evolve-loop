package phasestream

import "testing"

func TestCorrelation_BracketsAnswerSpan(t *testing.T) {
	c := newCorrelator()
	reqs := c.observe([]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`), 10)
	if len(reqs) != 1 || reqs[0].sub != "request" || reqs[0].atSeq != 10 {
		t.Fatalf("request = %+v", reqs)
	}
	done := c.observe([]byte(`{"evolve_channel":"idle_reached","corr_id":"c1"}`), 17)
	if len(done) != 1 || done[0].sub != "response_complete" || done[0].startSeq != 11 || done[0].endSeq != 17 {
		t.Fatalf("done = %+v", done)
	}
}

func TestCorrelation_IgnoresNonBreadcrumb(t *testing.T) {
	c := newCorrelator()
	if got := c.observe([]byte("normal stderr line"), 3); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestCorrelation_DuplicateIdleIgnored(t *testing.T) {
	c := newCorrelator()
	c.observe([]byte(`{"evolve_channel":"inject_applied","corr_id":"c1"}`), 1)
	c.observe([]byte(`{"evolve_channel":"idle_reached","corr_id":"c1"}`), 5)
	if got := c.observe([]byte(`{"evolve_channel":"idle_reached","corr_id":"c1"}`), 9); got != nil {
		t.Fatalf("second idle should be a no-op, got %+v", got)
	}
}

// TestCorrelation_UnknownChannelIgnored exercises the default branch of the
// switch — a valid JSON breadcrumb with an unrecognised evolve_channel value.
func TestCorrelation_UnknownChannelIgnored(t *testing.T) {
	c := newCorrelator()
	if got := c.observe([]byte(`{"evolve_channel":"bogus","corr_id":"c1"}`), 7); got != nil {
		t.Fatalf("unknown channel: got %+v, want nil", got)
	}
}

// TestCorrelation_MissingCorrIDIgnored covers the branch where JSON is valid
// but corr_id is absent (empty string after unmarshal).
func TestCorrelation_MissingCorrIDIgnored(t *testing.T) {
	c := newCorrelator()
	if got := c.observe([]byte(`{"evolve_channel":"inject_applied"}`), 2); got != nil {
		t.Fatalf("missing corr_id: got %+v, want nil", got)
	}
}

// TestCorrelation_OrphanIdleIgnored covers idle_reached with no preceding
// inject_applied for that corr_id (out-of-order / orphaned).
func TestCorrelation_OrphanIdleIgnored(t *testing.T) {
	c := newCorrelator()
	if got := c.observe([]byte(`{"evolve_channel":"idle_reached","corr_id":"never-opened"}`), 4); got != nil {
		t.Fatalf("orphan idle: got %+v, want nil", got)
	}
}
