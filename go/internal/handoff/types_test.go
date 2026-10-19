package handoff_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal into map: %v", err)
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestSchemaVersion_IsOne(t *testing.T) {
	if handoff.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", handoff.SchemaVersion)
	}
}

func TestStatusAndExitValues_MatchTheSpecWords(t *testing.T) {
	statuses := map[handoff.Status]string{
		handoff.StatusMet:           "met",
		handoff.StatusUnmet:         "unmet",
		handoff.StatusBlocked:       "blocked",
		handoff.StatusNotApplicable: "not_applicable",
	}
	for got, want := range statuses {
		if string(got) != want {
			t.Errorf("status %q, want %q", got, want)
		}
	}
	exits := map[handoff.Exit]string{
		handoff.ExitPass:    "pass",
		handoff.ExitCap:     "cap",
		handoff.ExitBlocked: "blocked",
	}
	for got, want := range exits {
		if string(got) != want {
			t.Errorf("exit %q, want %q", got, want)
		}
	}
}

func TestJSONKeys_MatchTheSpecSchema(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want []string
	}{
		{"ask", handoff.Ask{}, []string{"evidence_kind", "id", "required", "source", "text"}},
		{"request", handoff.Request{}, []string{"asks", "pair"}},
		{"evidence", handoff.Evidence{}, []string{"kind", "ref"}},
		{"entry", handoff.Entry{}, []string{"ask", "evidence", "note", "status"}},
		{"author", handoff.Author{}, []string{"cli", "model", "tier"}},
		{"ledger", handoff.Ledger{}, []string{"author", "entries", "output_digest", "pair", "request_digest", "round", "schema_version"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonKeys(t, tc.v); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("keys = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequestDigest_IsSha256OfTheCanonicalJSON(t *testing.T) {
	req := twoAskRequest()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	sum := sha256.Sum256(raw)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if got := req.Digest(); got != want {
		t.Errorf("Digest() = %q, want %q", got, want)
	}
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(req.Digest()) {
		t.Errorf("Digest() = %q is not sha256:<64 hex>", req.Digest())
	}
}

func TestRequestDigest_TracksEveryField(t *testing.T) {
	base := twoAskRequest()
	mutations := map[string]func(*handoff.Request){
		"pair":          func(r *handoff.Request) { r.Pair = "tdd-build" },
		"ask id":        func(r *handoff.Request) { r.Asks[0].ID = "a9" },
		"ask text":      func(r *handoff.Request) { r.Asks[0].Text = "something else" },
		"ask source":    func(r *handoff.Request) { r.Asks[0].Source = "scout:slug" },
		"evidence kind": func(r *handoff.Request) { r.Asks[0].EvidenceKind = "path" },
		"required":      func(r *handoff.Request) { r.Asks[0].Required = false },
		"extra ask": func(r *handoff.Request) {
			r.Asks = append(r.Asks, handoff.Ask{ID: "a3", Text: "third", Required: true})
		},
		"dropped ask": func(r *handoff.Request) { r.Asks = r.Asks[:1] },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := twoAskRequest()
			mutate(&changed)
			if changed.Digest() == base.Digest() {
				t.Errorf("digest did not change when the %s changed", name)
			}
		})
	}
}

func TestRequestDigest_EqualRequestsShareADigest(t *testing.T) {
	if twoAskRequest().Digest() != twoAskRequest().Digest() {
		t.Error("two equal requests produced different digests")
	}
}
