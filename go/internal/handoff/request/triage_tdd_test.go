package request_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	cycle1946Item    = "core-package-coverage-to-100"
	cycle1946Ask1    = "go test -count=1 -tags integration -coverprofile of ./internal/core reports 100.0% of statements"
	cycle1946Ask2    = "./internal/core 100 is listed in go/.cover-strict"
	cycle1946Fixture = "testdata/cycle1946"
)

func fixtureResolver(t *testing.T) request.PathResolver {
	t.Helper()
	return func(id string) string {
		path := filepath.Join(cycle1946Fixture, "inbox", id+".json")
		if _, err := os.Stat(path); err != nil {
			return ""
		}
		return path
	}
}

func writeWorkspace(t *testing.T, decision string) string {
	t.Helper()
	ws := t.TempDir()
	if decision != "" {
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(decision), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func TestTriageTDD_Cycle1946FixtureListsItsTwoAcceptanceEntries(t *testing.T) {
	got, err := request.TriageTDD(filepath.Join(cycle1946Fixture, "workspace"), fixtureResolver(t))
	if err != nil {
		t.Fatalf("TriageTDD: %v", err)
	}
	want := handoff.Request{
		Pair: "triage-tdd",
		Asks: []handoff.Ask{
			{ID: cycle1946Item + "#1", Text: cycle1946Ask1, Source: "inbox:" + cycle1946Item + "#acceptance/1", EvidenceKind: "predicate", Required: true},
			{ID: cycle1946Item + "#2", Text: cycle1946Ask2, Source: "inbox:" + cycle1946Item + "#acceptance/2", EvidenceKind: "predicate", Required: true},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestTriageTDD_TextIsTheInboxAcceptanceVerbatim(t *testing.T) {
	item, _, err := inboxbatch.LoadFile(filepath.Join(cycle1946Fixture, "inbox", cycle1946Item+".json"))
	if err != nil {
		t.Fatal(err)
	}
	got := request.TriageTDDFromItems([]inboxbatch.Item{item})
	if len(got.Asks) != len(item.Acceptance) {
		t.Fatalf("asks = %d, acceptance entries = %d", len(got.Asks), len(item.Acceptance))
	}
	for i, ask := range got.Asks {
		if ask.Text != item.Acceptance[i] {
			t.Errorf("ask %d text = %q, want the inbox entry %q", i, ask.Text, item.Acceptance[i])
		}
	}
}

func TestTriageTDDFromItems_Table(t *testing.T) {
	cases := []struct {
		name  string
		items []inboxbatch.Item
		want  []handoff.Ask
	}{
		{name: "no items give no asks", items: nil, want: nil},
		{
			name:  "an item without acceptance gives no asks",
			items: []inboxbatch.Item{{ID: "bare"}},
			want:  nil,
		},
		{
			name:  "padding around an entry is trimmed like the task contract",
			items: []inboxbatch.Item{{ID: "padded", Acceptance: []string{"  keep the text  \n"}}},
			want:  []handoff.Ask{{ID: "padded#1", Text: "keep the text", Source: "inbox:padded#acceptance/1", EvidenceKind: "predicate", Required: true}},
		},
		{
			name:  "a blank entry is skipped and the numbering keeps the position",
			items: []inboxbatch.Item{{ID: "gap", Acceptance: []string{"first", "   ", "third"}}},
			want: []handoff.Ask{
				{ID: "gap#1", Text: "first", Source: "inbox:gap#acceptance/1", EvidenceKind: "predicate", Required: true},
				{ID: "gap#3", Text: "third", Source: "inbox:gap#acceptance/3", EvidenceKind: "predicate", Required: true},
			},
		},
		{
			name: "asks follow the item order and ids never collide across items",
			items: []inboxbatch.Item{
				{ID: "b-item", Acceptance: []string{"b one"}},
				{ID: "a-item", Acceptance: []string{"a one"}},
			},
			want: []handoff.Ask{
				{ID: "b-item#1", Text: "b one", Source: "inbox:b-item#acceptance/1", EvidenceKind: "predicate", Required: true},
				{ID: "a-item#1", Text: "a one", Source: "inbox:a-item#acceptance/1", EvidenceKind: "predicate", Required: true},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := request.TriageTDDFromItems(tc.items)
			if got.Pair != "triage-tdd" {
				t.Errorf("pair = %q, want triage-tdd", got.Pair)
			}
			if !reflect.DeepEqual(got.Asks, tc.want) {
				t.Errorf("asks mismatch\n got: %+v\nwant: %+v", got.Asks, tc.want)
			}
		})
	}
}

func TestTriageTDD_RefusesARequestItCannotBuildWhole(t *testing.T) {
	resolver := fixtureResolver(t)
	cases := []struct {
		name     string
		decision string
		resolve  request.PathResolver
		wantErr  error
		wantText string
	}{
		{
			name:     "no triage decision means no committed set",
			decision: "",
			resolve:  resolver,
			wantErr:  request.ErrNoCommittedSet,
		},
		{
			name:     "an empty top_n commits nothing",
			decision: `{"top_n":[]}`,
			resolve:  resolver,
			wantErr:  request.ErrNoCommittedSet,
		},
		{
			name:     "a committed id with no inbox record is named in the error",
			decision: `{"top_n":[{"id":"ghost-item"}]}`,
			resolve:  resolver,
			wantErr:  request.ErrUnresolvedItem,
			wantText: "ghost-item",
		},
		{
			name:     "an unreadable inbox record is named in the error",
			decision: `{"top_n":[{"id":"core-package-coverage-to-100"}]}`,
			resolve:  func(string) string { return filepath.Join(t.TempDir(), "missing.json") },
			wantErr:  request.ErrUnresolvedItem,
			wantText: "core-package-coverage-to-100",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := request.TriageTDD(writeWorkspace(t, tc.decision), tc.resolve)
			if err == nil {
				t.Fatalf("want an error, got request %+v", got)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantErr)
			}
			if tc.wantText != "" && !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error %q does not name %q", err, tc.wantText)
			}
			if len(got.Asks) != 0 {
				t.Errorf("a refused build returned asks: %+v", got.Asks)
			}
		})
	}
}

func TestTriageTDD_AnItemDeferredByTriageGetsNoAsk(t *testing.T) {
	decision := `{"top_n":[{"id":"core-package-coverage-to-100"},{"id":"other"}],"deferred":[{"id":"other"}]}`
	got, err := request.TriageTDD(writeWorkspace(t, decision), fixtureResolver(t))
	if err != nil {
		t.Fatalf("TriageTDD: %v", err)
	}
	if len(got.Asks) != 2 {
		t.Fatalf("asks = %d, want the two asks of the committed item only", len(got.Asks))
	}
}

func TestTriageTDD_LaneScopePinWinsOverTheDecision(t *testing.T) {
	ws := writeWorkspace(t, `{"top_n":[{"id":"other"}]}`)
	if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), []byte(`{"todo_ids":["core-package-coverage-to-100"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := request.TriageTDD(ws, fixtureResolver(t))
	if err != nil {
		t.Fatalf("TriageTDD: %v", err)
	}
	if len(got.Asks) != 2 || got.Asks[0].ID != cycle1946Item+"#1" {
		t.Fatalf("the pinned item was not the source: %+v", got.Asks)
	}
}

func TestTriageTDD_RequestBindsALedgerThatValidates(t *testing.T) {
	req, err := request.TriageTDD(filepath.Join(cycle1946Fixture, "workspace"), fixtureResolver(t))
	if err != nil {
		t.Fatal(err)
	}
	requireMetLedgerValidates(t, req)
	again, err := request.TriageTDD(filepath.Join(cycle1946Fixture, "workspace"), fixtureResolver(t))
	if err != nil {
		t.Fatal(err)
	}
	if again.Digest() != req.Digest() {
		t.Errorf("digest is not stable across two builds: %s vs %s", again.Digest(), req.Digest())
	}
}

func requireMetLedgerValidates(t *testing.T, req handoff.Request) {
	t.Helper()
	ledger := handoff.Ledger{SchemaVersion: handoff.SchemaVersion, Pair: req.Pair, RequestDigest: req.Digest()}
	for _, ask := range req.Asks {
		ledger.Entries = append(ledger.Entries, handoff.Entry{Ask: ask.ID, Status: handoff.StatusMet})
	}
	if err := handoff.Validate(ledger, req); err != nil {
		t.Fatalf("a ledger for the built request must validate: %v", err)
	}
}
