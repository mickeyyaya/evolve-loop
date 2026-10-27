package llmcalls

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rowBeforeAttribution = `{"schema_version":2,"call_id":"c-1","ts":"2026-10-01T00:00:00Z","agent":"build","phase":"build","cli":"claude-tmux","model":"opus","attempt":1,"tokens":{"input":1,"output":2,"cache_read":3,"cache_write":4},"source":"transcript","tripwire":false,"fill_pct":0.1}`

func TestRecord_AttributionFieldsRoundTripUnderTheirJSONKeys(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(Record{Phase: "build", Cycle: 2040, RunID: "r01", Profile: "builder", Purpose: PurposeRetry})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"cycle":2040`, `"run_id":"r01"`, `"profile":"builder"`, `"purpose":"retry"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("row %s lacks %s", raw, want)
		}
	}
	var back Record
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Cycle != 2040 || back.RunID != "r01" || back.Profile != "builder" || back.Purpose != PurposeRetry {
		t.Errorf("round trip = %+v", back)
	}
}

func TestRecord_UnattributedRowWritesNoAttributionKeys(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(Record{Phase: "build"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"cycle"`, `"run_id"`, `"profile"`, `"purpose"`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("an unattributed row must omit %s, got %s", key, raw)
		}
	}
}

func TestPurposeVocabulary_IsTheThreeBridgeDerivedValues(t *testing.T) {
	t.Parallel()
	got := []string{PurposeFirst, PurposeRetry, PurposeContractCorrection}
	want := []string{"first", "retry", "contract_correction"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("purpose %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRead_OldRowWithoutAttributionDecodesUnchanged(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), Filename)
	if err := os.WriteFile(path, []byte(rowBeforeAttribution+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || res.Skipped != 0 {
		t.Fatalf("records=%d skipped=%d, want 1 and 0", len(res.Records), res.Skipped)
	}
	got := res.Records[0]
	if got.Cycle != 0 || got.RunID != "" || got.Profile != "" || got.Purpose != "" {
		t.Errorf("an old row must decode with empty attribution, got %+v", got)
	}
	if got.CallID != "c-1" || got.Agent != "build" || got.Tokens.CacheRead != 3 || got.Attempt != 1 {
		t.Errorf("old row fields changed: %+v", got)
	}
}

func TestImport_OldRowCopiesByteForByteAndAttributedRowKeepsItsFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.ndjson")
	destination := filepath.Join(dir, "destination.ndjson")
	attributed := `{"schema_version":2,"call_id":"c-2","ts":"2026-10-01T00:00:01Z","agent":"build","phase":"build","cli":"claude-tmux","model":"opus","attempt":2,"cycle":2040,"run_id":"r01","profile":"builder","purpose":"retry","tokens":{},"source":"transcript","tripwire":false,"fill_pct":0}`
	if err := os.WriteFile(source, []byte(rowBeforeAttribution+"\n"+attributed+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Import(destination, source)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 2 || res.Skipped != 0 {
		t.Fatalf("import = %+v, want 2 imported", res)
	}
	raw, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if lines[0] != rowBeforeAttribution || lines[1] != attributed {
		t.Errorf("import must copy each row unchanged, got %q", lines)
	}
	read, err := Read(destination)
	if err != nil {
		t.Fatal(err)
	}
	if got := read.Records[1]; got.Cycle != 2040 || got.Profile != "builder" || got.Purpose != PurposeRetry {
		t.Errorf("imported attributed row = %+v", got)
	}
}
