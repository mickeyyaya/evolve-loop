package llmcalls

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }

func testRecord(id string) Record {
	return Record{
		SchemaVersion:     SchemaVersion,
		CallID:            id,
		TS:                "2026-09-11T08:00:02Z",
		StartedAt:         "2026-09-11T08:00:00Z",
		EndedAt:           "2026-09-11T08:00:02Z",
		TimingScope:       TimingBridgeDispatch,
		Agent:             "build",
		Phase:             "build",
		CLI:               "codex",
		Model:             "deep",
		RequestedModel:    "deep",
		DispatchedModel:   "gpt-5.6-sol",
		DispatchSource:    DispatchArgv,
		Attempt:           1,
		Tokens:            cyclestate.TokenUsage{Input: 100, Output: 20, CacheRead: 50},
		Source:            "events_result",
		UsageStatus:       UsageMeasured,
		DurationMS:        int64Ptr(2000),
		ExitCode:          intPtr(0),
		CauseCode:         "",
		FirstOutputMS:     nil,
		FirstOutputSource: "",
	}
}

func TestAppendWorkspace_RepairsPartialTailAndReadSkipsMalformed(t *testing.T) {
	ws := t.TempDir()
	path := Path(ws)
	if err := os.WriteFile(path, []byte(`{"phase":`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := testRecord("call-1")
	if err := AppendWorkspace(ws, want); err != nil {
		t.Fatalf("AppendWorkspace: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\n{\"schema_version\":") {
		t.Fatalf("append did not separate a torn trailing record: %q", raw)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Skipped != 1 || len(got.Records) != 1 {
		t.Fatalf("Read result = %+v, want one valid and one skipped record", got)
	}
	rec := got.Records[0]
	if rec.CallID != want.CallID || rec.DispatchedModel != want.DispatchedModel || rec.UsageStatus != UsageMeasured {
		t.Fatalf("record lost canonical fields: %+v", rec)
	}
	if rec.DurationMS == nil || *rec.DurationMS != 2000 || rec.ExitCode == nil || *rec.ExitCode != 0 {
		t.Fatalf("record lost measured outcome: %+v", rec)
	}
}

func TestRead_LegacyUnknownFieldsRemainCompatible(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	line := `{"ts":"2026-09-11T08:00:00Z","phase":"audit","cli":"claude-tmux","model":"deep","duration_ms":75,"exit_code":0,"future":{"x":1}}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 0 || len(got.Records) != 1 {
		t.Fatalf("Read = %+v", got)
	}
	rec := got.Records[0]
	if rec.Phase != "audit" || rec.Model != "deep" || rec.DurationMS == nil || *rec.DurationMS != 75 {
		t.Fatalf("legacy record did not decode: %+v", rec)
	}
	if model, source := rec.ModelIdentity(); model != UnknownModel || source != DispatchLegacyUnverified {
		t.Fatalf("legacy requested selector was treated as dispatched: model=%q source=%q", model, source)
	}
}

func TestAppendWorkspace_ConcurrentCallsProduceCompleteLines(t *testing.T) {
	ws := t.TempDir()
	const count = 48
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := testRecord(fmt.Sprintf("call-%02d", i))
			if err := AppendWorkspace(ws, rec); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("AppendWorkspace: %v", err)
	}

	got, err := ReadWorkspace(ws)
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 0 || len(got.Records) != count {
		t.Fatalf("concurrent ledger: records=%d skipped=%d, want %d/0", len(got.Records), got.Skipped, count)
	}
	ids := make(map[string]struct{}, count)
	for _, rec := range got.Records {
		ids[rec.CallID] = struct{}{}
	}
	if len(ids) != count {
		t.Fatalf("unique call IDs = %d, want %d", len(ids), count)
	}
}

func TestImport_DeduplicatesCallIdentityAndLegacyBytes(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "durable.ndjson")
	src := filepath.Join(dir, "scratch.ndjson")
	prior := `{"call_id":"call-1","phase":"model-probe","exit_code":0}`
	legacy := `{"phase":"legacy","call":2}`
	if err := os.WriteFile(dst, []byte(prior+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte(prior+"\n"+legacy+"\n"+`{"phase":`), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Import(dst, src)
	if err != nil {
		t.Fatalf("first Import: %v", err)
	}
	if first.Imported != 1 || first.Duplicates != 1 || first.Skipped != 1 {
		t.Fatalf("first Import = %+v", first)
	}
	second, err := Import(dst, src)
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if second.Imported != 0 || second.Duplicates != 2 || second.Skipped != 1 {
		t.Fatalf("second Import = %+v", second)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != prior+"\n"+legacy+"\n" {
		t.Fatalf("Import changed or duplicated ledger bytes: %q", raw)
	}
}

func TestAppend_ExactRecordLimitRemainsReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	rec := testRecord("limit")
	rec.Agent = ""
	base, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	rec.Agent = strings.Repeat("a", maxRecordBytes-len(base))
	encoded, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != maxRecordBytes {
		t.Fatalf("boundary fixture = %d bytes, want %d", len(encoded), maxRecordBytes)
	}
	if err := Append(path, rec); err != nil {
		t.Fatalf("Append exact-limit record: %v", err)
	}
	if err := Append(path, testRecord("after-limit")); err != nil {
		t.Fatalf("Append trailing record: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read accepted exact-limit record: %v", err)
	}
	if got.Skipped != 0 || len(got.Records) != 2 || got.Records[1].CallID != "after-limit" {
		t.Fatalf("exact-limit round trip = records %d skipped %d", len(got.Records), got.Skipped)
	}
	importedPath := filepath.Join(dir, "imported.ndjson")
	imported, err := Import(importedPath, path)
	if err != nil {
		t.Fatalf("Import exact-limit record: %v", err)
	}
	if imported.Imported != 2 || imported.Skipped != 0 {
		t.Fatalf("exact-limit import = %+v", imported)
	}
	importRead, err := Read(importedPath)
	if err != nil || len(importRead.Records) != 2 {
		t.Fatalf("read exact-limit import = %+v, %v", importRead, err)
	}
}

func TestRead_RejectsNonRecordJSONValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	raw := "null\n{}\n{\"future_envelope\":true}\n{\"phase\":\"audit\",\"exit_code\":0}\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 3 || len(got.Records) != 1 || got.Records[0].Phase != "audit" {
		t.Fatalf("non-record validation = %+v", got)
	}
}

func TestNewCallID_IsNonEmptyAndUniqueAtSameTimestamp(t *testing.T) {
	now := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	a, b := NewCallID(now), NewCallID(now)
	if a == "" || b == "" || a == b {
		t.Fatalf("NewCallID(%s) = %q, %q", now, a, b)
	}
}
