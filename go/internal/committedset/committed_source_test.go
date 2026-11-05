package committedset

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCommittedWithSource_NamesTheFileTheSetCameFrom(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pin        []string
		topN       []string
		deferred   []string
		malformed  bool
		wantIDs    []string
		wantSource Source
	}{
		{name: "no file", wantSource: SourceUnknown},
		{name: "malformed decision", malformed: true, wantSource: SourceUnknown},
		{name: "pin wins over the decision", pin: []string{"a", "b"}, topN: []string{"c"}, wantIDs: []string{"a", "b"}, wantSource: SourcePin},
		{name: "decision only", topN: []string{"c", "d"}, wantIDs: []string{"c", "d"}, wantSource: SourceDecision},
		{name: "decision with every item deferred is empty", topN: []string{"c"}, deferred: []string{"c"}, wantIDs: []string{}, wantSource: SourceDecision},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			if len(tc.pin) > 0 {
				write(t, ws, LanePinFile, LanePinDoc{TodoIDs: tc.pin})
			}
			if tc.topN != nil {
				doc := map[string]any{"top_n": ids(tc.topN...)}
				if tc.deferred != nil {
					doc["deferred"] = ids(tc.deferred...)
				}
				write(t, ws, DecisionFile, doc)
			}
			if tc.malformed {
				if err := os.WriteFile(filepath.Join(ws, DecisionFile), []byte("{not json"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, source := CommittedWithSource(ws)
			if source != tc.wantSource || !slices.Equal(got, tc.wantIDs) {
				t.Fatalf("CommittedWithSource = %v, %q; want %v, %q", got, source, tc.wantIDs, tc.wantSource)
			}
		})
	}
}

func TestCommitted_AgreesWithCommittedWithSource(t *testing.T) {
	ws := t.TempDir()
	write(t, ws, DecisionFile, map[string]any{"top_n": ids("x")})
	withSource, source := CommittedWithSource(ws)
	got, ok := Committed(ws)
	if !ok || source == SourceUnknown || !slices.Equal(got, withSource) {
		t.Fatalf("Committed = %v, %v; CommittedWithSource = %v, %q", got, ok, withSource, source)
	}
	if _, ok := Committed(t.TempDir()); ok {
		t.Fatal("Committed reports a set for an empty workspace")
	}
}
