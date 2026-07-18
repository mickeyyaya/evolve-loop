package bridge

import "testing"

// The gate must fast-fail only after the wall PERSISTS for the threshold, and a
// single transient match (wall text passing through a working agent's pane) must
// never cross — that is the whole point (kill a working agent = cardinal sin).
func TestExhaustionGate_PersistenceSemantics(t *testing.T) {
	if exhaustionPersistObservations < 2 {
		t.Fatalf("threshold must be >= 2 to distinguish a transient frame from a persistent wall; got %d", exhaustionPersistObservations)
	}

	cases := []struct {
		name    string
		matches []bool // one entry per observation
		wantFF  []bool // expected fast-fail signal per observation
	}{
		{
			name:    "persistent wall crosses on the threshold-th consecutive match",
			matches: []bool{true, true, true},
			wantFF:  []bool{false, true, true},
		},
		{
			name:    "single transient frame never crosses (working agent quoting a wall)",
			matches: []bool{true, false, true, false},
			wantFF:  []bool{false, false, false, false},
		},
		{
			name:    "a reset before the threshold restarts the count",
			matches: []bool{true, false, true, true},
			wantFF:  []bool{false, false, false, true},
		},
		{
			name:    "no match never fires",
			matches: []bool{false, false, false},
			wantFF:  []bool{false, false, false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newExhaustionGate()
			for i, m := range tc.matches {
				got := g.observe(m)
				if got != tc.wantFF[i] {
					t.Errorf("observe #%d(matched=%v) = %v, want %v (streak semantics)", i, m, got, tc.wantFF[i])
				}
			}
		})
	}
}
