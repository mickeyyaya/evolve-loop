package handoff_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

func twoAskRequest() handoff.Request {
	return handoff.Request{
		Pair: "triage-tdd",
		Asks: []handoff.Ask{
			{ID: "a1", Text: "go test of ./internal/core reports 100.0%", Source: "inbox:acceptance:1", EvidenceKind: "predicate", Required: true},
			{ID: "a2", Text: "./internal/core 100 is listed in go/.cover-strict", Source: "inbox:acceptance:2", EvidenceKind: "predicate", Required: true},
		},
	}
}

func metEntry(ask string) handoff.Entry {
	return handoff.Entry{
		Ask:      ask,
		Status:   handoff.StatusMet,
		Evidence: []handoff.Evidence{{Kind: "predicate", Ref: "go/acs/cycle1950/predicates_test.go:TestC1950_001"}},
	}
}

func ledgerFor(req handoff.Request, entries ...handoff.Entry) handoff.Ledger {
	return handoff.Ledger{
		SchemaVersion: handoff.SchemaVersion,
		Pair:          req.Pair,
		RequestDigest: req.Digest(),
		OutputDigest:  "sha256:tree",
		Author:        handoff.Author{CLI: "claude-tmux", Model: "claude-opus-5-5", Tier: "deep"},
		Round:         1,
		Entries:       entries,
	}
}

func TestValidate_RefusalTable(t *testing.T) {
	req := twoAskRequest()
	otherRequest := twoAskRequest()
	otherRequest.Asks[0].Text = "an older request"

	cases := []struct {
		name    string
		ledger  handoff.Ledger
		wantErr error
		wantIn  string
	}{
		{
			name:    "an entry for an ask id the request does not have",
			ledger:  ledgerFor(req, metEntry("a1"), metEntry("a2"), metEntry("a9")),
			wantErr: handoff.ErrUnknownAsk,
			wantIn:  "a9",
		},
		{
			name: "a request_digest of another request",
			ledger: func() handoff.Ledger {
				l := ledgerFor(req, metEntry("a1"), metEntry("a2"))
				l.RequestDigest = otherRequest.Digest()
				return l
			}(),
			wantErr: handoff.ErrStaleRequestDigest,
		},
		{
			name: "an empty request_digest",
			ledger: func() handoff.Ledger {
				l := ledgerFor(req, metEntry("a1"), metEntry("a2"))
				l.RequestDigest = ""
				return l
			}(),
			wantErr: handoff.ErrStaleRequestDigest,
		},
		{
			name: "a ledger of another pair",
			ledger: func() handoff.Ledger {
				l := ledgerFor(req, metEntry("a1"), metEntry("a2"))
				l.Pair = "tdd-build"
				return l
			}(),
			wantErr: handoff.ErrWrongPair,
			wantIn:  "tdd-build",
		},
		{
			name:    "an ask without an entry",
			ledger:  ledgerFor(req, metEntry("a1")),
			wantErr: handoff.ErrMissingEntry,
			wantIn:  "a2",
		},
		{
			name:    "a ledger with no entries",
			ledger:  ledgerFor(req),
			wantErr: handoff.ErrMissingEntry,
			wantIn:  "a1",
		},
		{
			name:    "an ask with two entries",
			ledger:  ledgerFor(req, metEntry("a1"), metEntry("a1"), metEntry("a2")),
			wantErr: handoff.ErrDuplicateEntry,
			wantIn:  "a1",
		},
		{
			name: "a not_applicable entry without a note",
			ledger: ledgerFor(req, metEntry("a1"),
				handoff.Entry{Ask: "a2", Status: handoff.StatusNotApplicable}),
			wantErr: handoff.ErrNoteRequired,
			wantIn:  "a2",
		},
		{
			name: "a not_applicable entry with a blank note",
			ledger: ledgerFor(req, metEntry("a1"),
				handoff.Entry{Ask: "a2", Status: handoff.StatusNotApplicable, Note: " \t\n"}),
			wantErr: handoff.ErrNoteRequired,
			wantIn:  "a2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := handoff.Validate(tc.ledger, req)
			if err == nil {
				t.Fatal("Validate accepted the ledger, want a refusal")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want errors.Is %v", err, tc.wantErr)
			}
			if tc.wantIn != "" && !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("err = %q does not name %q", err, tc.wantIn)
			}
		})
	}
}

func TestValidate_AcceptsALedgerThatAnswersEachAskOnce(t *testing.T) {
	req := twoAskRequest()
	cases := []struct {
		name   string
		ledger handoff.Ledger
	}{
		{"every ask met", ledgerFor(req, metEntry("a1"), metEntry("a2"))},
		{"entries in reverse order", ledgerFor(req, metEntry("a2"), metEntry("a1"))},
		{"unmet and blocked need no note", ledgerFor(req,
			handoff.Entry{Ask: "a1", Status: handoff.StatusUnmet},
			handoff.Entry{Ask: "a2", Status: handoff.StatusBlocked})},
		{"not_applicable with a note", ledgerFor(req, metEntry("a1"),
			handoff.Entry{Ask: "a2", Status: handoff.StatusNotApplicable, Note: "the package has no exported symbol"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := handoff.Validate(tc.ledger, req); err != nil {
				t.Errorf("Validate refused a sound ledger: %v", err)
			}
		})
	}
}

func TestValidate_AnEmptyRequestNeedsNoEntries(t *testing.T) {
	req := handoff.Request{Pair: "scout-triage"}
	if err := handoff.Validate(ledgerFor(req), req); err != nil {
		t.Errorf("Validate refused an empty ledger for an empty request: %v", err)
	}
}

func TestValidate_ReportsEveryViolationAtOnce(t *testing.T) {
	req := twoAskRequest()
	l := ledgerFor(req, metEntry("a1"), metEntry("a9"))
	l.Pair = "tdd-build"
	l.RequestDigest = "sha256:stale"

	err := handoff.Validate(l, req)
	for _, want := range []error{handoff.ErrWrongPair, handoff.ErrStaleRequestDigest, handoff.ErrUnknownAsk, handoff.ErrMissingEntry} {
		if !errors.Is(err, want) {
			t.Errorf("err = %v, want it to include %v", err, want)
		}
	}
}

func marshalLedger(t *testing.T, l handoff.Ledger) []byte {
	t.Helper()
	raw, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestParse_RoundTripsAMarshaledLedger(t *testing.T) {
	req := twoAskRequest()
	want := ledgerFor(req, metEntry("a1"),
		handoff.Entry{Ask: "a2", Status: handoff.StatusBlocked, Note: "needs a protected path",
			Evidence: []handoff.Evidence{{Kind: "path", Ref: "go/internal/core/blocker_breaker.go:76"}}})

	got, err := handoff.Parse(marshalLedger(t, want))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse = %+v, want %+v", got, want)
	}
	if err := handoff.Validate(got, req); err != nil {
		t.Errorf("Validate of the parsed ledger: %v", err)
	}
}

func TestParse_ReadsTheSpecExample(t *testing.T) {
	req := twoAskRequest()
	raw := `{
  "schema_version": 1,
  "pair": "triage-tdd",
  "request_digest": "` + req.Digest() + `",
  "output_digest": "sha256:abc",
  "author": {"cli": "claude-tmux", "model": "claude-opus-5-5", "tier": "deep"},
  "round": 1,
  "entries": [
    {"ask": "a1", "status": "met",
     "evidence": [{"kind": "predicate", "ref": "go/acs/cycle1950/predicates_test.go:TestC1950_001"}],
     "note": ""},
    {"ask": "a2", "status": "blocked",
     "evidence": [{"kind": "path", "ref": "go/internal/core/blocker_breaker.go:76"}],
     "note": "the error return after json.MarshalIndent is unreachable for []ResolvedFingerprint"}
  ]
}`
	l, err := handoff.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if l.Pair != "triage-tdd" || l.Round != 1 || l.Author.Tier != "deep" || len(l.Entries) != 2 {
		t.Errorf("parsed ledger = %+v", l)
	}
	if l.Entries[1].Status != handoff.StatusBlocked || l.Entries[1].Evidence[0].Ref != "go/internal/core/blocker_breaker.go:76" {
		t.Errorf("parsed entry = %+v", l.Entries[1])
	}
	if err := handoff.Validate(l, req); err != nil {
		t.Errorf("Validate of the spec example: %v", err)
	}
}

func TestParse_RefusalTable(t *testing.T) {
	good := ledgerFor(twoAskRequest(), metEntry("a1"), metEntry("a2"))
	withVersion := func(v int) []byte {
		l := good
		l.SchemaVersion = v
		return marshalLedger(t, l)
	}
	withStatus := func(s handoff.Status) []byte {
		l := good
		l.Entries = []handoff.Entry{{Ask: "a1", Status: s}}
		return marshalLedger(t, l)
	}

	cases := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{"schema version 0", withVersion(0), handoff.ErrSchemaVersion},
		{"schema version 2", withVersion(2), handoff.ErrSchemaVersion},
		{"an unknown status", withStatus("done"), handoff.ErrUnknownStatus},
		{"an empty status", withStatus(""), handoff.ErrUnknownStatus},
		{"malformed JSON", []byte(`{"schema_version": 1,`), nil},
		{"empty input", nil, nil},
		{"a JSON array", []byte(`[]`), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handoff.Parse(tc.data)
			if err == nil {
				t.Fatal("Parse accepted the input, want a refusal")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}

func TestParse_AcceptsEveryStatus(t *testing.T) {
	for _, s := range []handoff.Status{handoff.StatusMet, handoff.StatusUnmet, handoff.StatusBlocked, handoff.StatusNotApplicable} {
		l := ledgerFor(twoAskRequest(), handoff.Entry{Ask: "a1", Status: s, Note: "n"})
		if _, err := handoff.Parse(marshalLedger(t, l)); err != nil {
			t.Errorf("Parse refused status %q: %v", s, err)
		}
	}
}
