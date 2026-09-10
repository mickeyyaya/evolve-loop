package committedset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, ws, name string, v any) {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func ids(in ...string) []map[string]string {
	out := make([]map[string]string, 0, len(in))
	for _, s := range in {
		out = append(out, map[string]string{"id": s})
	}
	return out
}

// TestCommitted_Precedence pins the one answer every consumer projects from.
// The shapes are the ones real cycles leave on disk: runtime cycle-1621 had a
// lane pin naming two members while triage's top_n named one, and 17 of the
// last 20 runtime cycles carried a pin — so a top_n-only reader under-reports
// on the dominant shape.
func TestCommitted_Precedence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pin      []string
		topN     []string
		deferred []string
		want     string
		wantOK   bool
	}{
		{name: "nothing recorded ⇒ unknown", wantOK: false},
		{name: "decision only", topN: []string{"alpha", "beta"}, want: "alpha,beta", wantOK: true},
		{name: "lane pin wins over the decision (cycle-1621 shape)", pin: []string{"alpha", "beta"}, topN: []string{"beta"}, want: "alpha,beta", wantOK: true},
		{name: "deferral removes a member from either source", pin: []string{"alpha", "beta"}, deferred: []string{"beta"}, want: "alpha", wantOK: true},
		{name: "deferral applies to the decision path too", topN: []string{"alpha", "beta"}, deferred: []string{"alpha"}, want: "beta", wantOK: true},
		{name: "explicit empty commitment is KNOWN, not unknown", topN: []string{}, want: "", wantOK: true},
		{name: "empty pin falls through to the decision", pin: []string{}, topN: []string{"alpha"}, want: "alpha", wantOK: true},
		{name: "pin present with no decision at all", pin: []string{"alpha"}, want: "alpha", wantOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			if tc.pin != nil {
				write(t, ws, LanePinFile, map[string]any{"todo_ids": tc.pin})
			}
			if tc.topN != nil || tc.deferred != nil {
				write(t, ws, DecisionFile, map[string]any{
					"top_n": ids(tc.topN...), "deferred": ids(tc.deferred...),
				})
			}
			got, ok := Committed(ws)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got %v)", ok, tc.wantOK, got)
			}
			if !ok {
				if got != nil {
					t.Errorf("unknown must return nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("a known commitment must be non-nil, so empty is distinguishable from unknown")
			}
			if strings.Join(got, ",") != tc.want {
				t.Errorf("Committed = %v, want %q", got, tc.want)
			}
		})
	}
}

// TestCommitted_MalformedIsUnknownNotEmpty: a corrupt artifact must never be
// read as "committed to nothing" — that would let a parse failure masquerade
// as a finding.
func TestCommitted_MalformedIsUnknownNotEmpty(t *testing.T) {
	ws := t.TempDir()
	for _, name := range []string{LanePinFile, DecisionFile} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, ok := Committed(ws); ok || got != nil {
		t.Fatalf("malformed artifacts = (%v,%v), want unknown", got, ok)
	}
}

// TestLanePinAndDecision_BlankIDsDropped keeps a malformed entry from becoming
// a phantom member.
func TestLanePinAndDecision_BlankIDsDropped(t *testing.T) {
	ws := t.TempDir()
	write(t, ws, LanePinFile, map[string]any{"todo_ids": []string{"alpha", "  ", ""}})
	if got := LanePin(ws); strings.Join(got, ",") != "alpha" {
		t.Errorf("LanePin = %v, want the non-blank ids", got)
	}
	ws2 := t.TempDir()
	write(t, ws2, DecisionFile, map[string]any{"top_n": ids("alpha", ""), "deferred": ids("beta", " ")})
	got, ok := DecisionTopN(ws2)
	if !ok || strings.Join(got, ",") != "alpha" {
		t.Errorf("DecisionTopN = (%v,%v), want [alpha]", got, ok)
	}
	if d := Deferred(ws2); strings.Join(d, ",") != "beta" {
		t.Errorf("Deferred = %v, want [beta]", d)
	}
	if d := Deferred(t.TempDir()); d != nil {
		t.Errorf("absent decision defers nothing, got %v", d)
	}
}

// TestLanePinDoc_IsTheOneWireShape names the pin's declaration directly. It is
// the single home for those tags — core aliases this type rather than
// redeclaring it, which is what cycleoutcome's
// TestLaneScopeProjection_SingleWireShapeDeclaration enforces — so the fields
// must round-trip exactly as the lane provisioner writes them.
func TestLanePinDoc_IsTheOneWireShape(t *testing.T) {
	ws := t.TempDir()
	want := LanePinDoc{TodoIDs: []string{"alpha", "beta"}, GoalHash: "abc123"}
	write(t, ws, LanePinFile, want)

	var got LanePinDoc
	body, err := os.ReadFile(filepath.Join(ws, LanePinFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.TodoIDs, ",") != "alpha,beta" || got.GoalHash != "abc123" {
		t.Fatalf("round-trip = %+v, want %+v", got, want)
	}
	// And the projection reads that same shape.
	if ids := LanePin(ws); strings.Join(ids, ",") != "alpha,beta" {
		t.Errorf("LanePin = %v, want the pinned ids", ids)
	}
}
