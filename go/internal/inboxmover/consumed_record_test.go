package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func consumedCorpusDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".evolve", "inbox", "consumed")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("consumed corpus %s unreadable or empty (err=%v): the round-trip proves nothing without it", dir, err)
	}
	return dir
}

func consumedRecordJSONKeys() map[string]bool {
	keys := map[string]bool{}
	rt := reflect.TypeOf(ConsumedRecord{})
	for i := 0; i < rt.NumField(); i++ {
		keys[strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	return keys
}

func TestConsumedRecord_RoundTripAgainstCorpus(t *testing.T) {
	t.Parallel()
	dir := consumedCorpusDir(t)
	known := consumedRecordJSONKeys()
	entries, _ := os.ReadDir(dir)
	objects := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		rec, ok, err := ParseConsumedRecord(body)
		if err != nil {
			t.Errorf("%s: corpus file does not parse: %v", e.Name(), err)
			continue
		}
		var doc struct {
			Consumed json.RawMessage `json:"consumed"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		var raw map[string]any
		if json.Unmarshal(doc.Consumed, &raw) != nil || raw == nil {
			continue
		}
		objects++
		if !ok {
			t.Errorf("%s: a consumed object parsed as absent", e.Name())
			continue
		}
		for k, v := range raw {
			if !known[k] {
				t.Errorf("%s: corpus key %q has no field on ConsumedRecord — the type is not the single source", e.Name(), k)
			}
			if s, isStr := v.(string); isStr {
				if got := consumedFieldByTag(rec, k); got != s {
					t.Errorf("%s: field %q = %q, want %q", e.Name(), k, got, s)
				}
			}
		}
		again, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		back, ok2, err := ParseConsumedRecord([]byte(`{"consumed":` + string(again) + `}`))
		if err != nil || !ok2 || back != rec {
			t.Errorf("%s: round trip changed the record: %#v -> %#v (ok=%v err=%v)", e.Name(), rec, back, ok2, err)
		}
	}
	if objects < 100 {
		t.Fatalf("only %d consumed objects found in the corpus, expected the tracked history", objects)
	}
}

func consumedFieldByTag(rec ConsumedRecord, tag string) string {
	rv := reflect.ValueOf(rec)
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		if strings.Split(rt.Field(i).Tag.Get("json"), ",")[0] == tag {
			return rv.Field(i).String()
		}
	}
	return "<no field>"
}

func TestParseConsumedRecord_Shapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		item    string
		want    ConsumedRecord
		wantOK  bool
		wantErr bool
	}{
		{name: "object with string cycle", item: `{"id":"a","consumed":{"at":"2026-09-01T00:00:00Z","via":"evolve inbox consume","cycle":"console","resolution":"done"}}`,
			want: ConsumedRecord{At: "2026-09-01T00:00:00Z", Via: "evolve inbox consume", Cycle: "console", Resolution: "done"}, wantOK: true},
		{name: "numeric cycle is kept as its decimal text", item: `{"id":"a","consumed":{"cycle":5}}`,
			want: ConsumedRecord{Cycle: "5"}, wantOK: true},
		{name: "non-numeric cycle label survives verbatim", item: `{"consumed":{"cycle":"1513+1514"}}`,
			want: ConsumedRecord{Cycle: "1513+1514"}, wantOK: true},
		{name: "merge-style keys land on their fields", item: `{"consumed":{"at":"t","via":"merge","merged_into":"other-item","note":"dup"}}`,
			want: ConsumedRecord{At: "t", Via: "merge", MergedInto: "other-item", Note: "dup"}, wantOK: true},
		{name: "legacy bare timestamp string is the consume time", item: `{"id":"a","consumed":"2026-09-01T12:30:00Z"}`,
			want: ConsumedRecord{At: "2026-09-01T12:30:00Z"}, wantOK: true},
		{name: "null consumed is not consumed", item: `{"id":"a","consumed":null}`},
		{name: "absent consumed is not consumed", item: `{"id":"a"}`},
		{name: "unknown extra key is tolerated", item: `{"consumed":{"at":"t","future_field":"x"}}`,
			want: ConsumedRecord{At: "t"}, wantOK: true},
		{name: "array consumed is a schema error", item: `{"consumed":["x"]}`, wantErr: true},
		{name: "boolean consumed is a schema error", item: `{"consumed":true}`, wantErr: true},
		{name: "fractional cycle is a schema error", item: `{"consumed":{"cycle":1.5}}`, wantErr: true},
		{name: "malformed item is an error", item: `{"consumed":`, wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok, err := ParseConsumedRecord([]byte(tc.item))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("ParseConsumedRecord(%s) = %#v, %v; want %#v, %v", tc.item, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
