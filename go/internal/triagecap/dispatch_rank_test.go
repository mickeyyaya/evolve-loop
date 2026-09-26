package triagecap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func ids(cs []FleetCandidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

func TestRankForDispatch_WeightFirstThenVerifiedAdmissible(t *testing.T) {
	in := []FleetCandidate{
		{ID: "unknown-085", Weight: 0.85},
		{ID: "declared-084", Weight: 0.84, Declared: true},
		{ID: "declared-085", Weight: 0.85, Declared: true},
		{ID: "unknown-086", Weight: 0.86},
		{ID: "unknown-084", Weight: 0.84},
	}
	got := ids(rankForDispatch(in))
	want := []string{"unknown-086", "declared-085", "unknown-085", "declared-084", "unknown-084"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rankForDispatch = %v, want %v", got, want)
	}
	if in[0].ID != "unknown-085" {
		t.Error("rankForDispatch must not reorder its input in place")
	}
}

func TestSelectFleetWidthTopN_EqualWeightsSeedVerifiedFirst(t *testing.T) {
	cands := []FleetCandidate{
		{ID: "resume-stale-base", Weight: 0.86},
		{ID: "phantom-binding", Weight: 0.85},
		{ID: "landing-witness", Weight: 0.85, Files: []string{"go/internal/landingwitness/a.go"}, Declared: true},
	}
	if got := ids(SelectFleetWidthTopN(cands, 2)); !reflect.DeepEqual(got, []string{"resume-stale-base", "landing-witness"}) {
		t.Errorf("count=2 seeded %v, want [resume-stale-base landing-witness]", got)
	}
}

func TestWidenTopNToFleetWidth_BackfillsByWeightThenVerified(t *testing.T) {
	committed := []FleetCandidate{{ID: "kept", Weight: 0.5, Files: []string{"go/internal/k/k.go"}, Declared: true}}
	tied := []FleetCandidate{
		{ID: "unknown", Weight: 0.6},
		{ID: "declared", Weight: 0.6, Files: []string{"go/internal/d/d.go"}, Declared: true},
	}
	if got := ids(WidenTopNToFleetWidth(committed, tied, 2)); !reflect.DeepEqual(got, []string{"kept", "declared"}) {
		t.Errorf("tied backfill = %v, want [kept declared]", got)
	}
	heavier := []FleetCandidate{
		{ID: "unknown", Weight: 0.9},
		{ID: "declared", Weight: 0.6, Files: []string{"go/internal/d/d.go"}, Declared: true},
	}
	if got := ids(WidenTopNToFleetWidth(committed, heavier, 2)); !reflect.DeepEqual(got, []string{"kept", "unknown"}) {
		t.Errorf("weighted backfill = %v, want [kept unknown] (the weight is the priority)", got)
	}
}

func TestReadInboxBacklog_CarriesTheDeclaredSurfaceBelief(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"a.json": `{"id":"declared","weight":0.5,"files":["go/internal/inboxbatch/item.go"]}`,
		"b.json": `{"id":"placeholder","weight":0.5,"files":["TBD"]}`,
	} {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]bool{}
	for _, c := range ReadInboxBacklog(dir, nil) {
		got[c.ID] = c.Declared
	}
	if !got["declared"] || got["placeholder"] {
		t.Errorf("Declared = %v, want declared=true placeholder=false", got)
	}
}
