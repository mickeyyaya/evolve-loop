package interaction

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLedgerF4(t *testing.T, ws, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteRollup_AnEncodeFailureIsReturnedAndNoSummaryFileAppears(t *testing.T) {
	orig := rollupMarshalIndent
	t.Cleanup(func() { rollupMarshalIndent = orig })
	boom := errors.New("encoder down")
	rollupMarshalIndent = func(any, string, string) ([]byte, error) { return nil, boom }
	ws := t.TempDir()
	writeLedgerF4(t, ws, "scout-interactions.ndjson", `{"kind":"ask","result":"answered","decision_id":"d1","cost_usd":0.5}`+"\n")

	err := WriteRollup(ws)

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "interaction-summary.json")); !os.IsNotExist(statErr) {
		t.Errorf("no summary may be written: %v", statErr)
	}
}

func TestWriteRollup_SummarizesLedgersAndSkipsCorruptLinesAndEmptyWorkspaces(t *testing.T) {
	ws := t.TempDir()
	writeLedgerF4(t, ws, "a-interactions.ndjson", `{"kind":"ask","result":"answered","decision_id":"d1","cost_usd":0.25}`+"\nnot json\n\n"+`{"kind":"ask","result":"timeout","rung":"2","decision_id":"d1"}`+"\n")
	writeLedgerF4(t, ws, "b-interactions.ndjson", `{"kind":"correct","result":"answered","decision_id":"d2","cost_usd":0.5}`+"\n")

	if err := WriteRollup(ws); err != nil {
		t.Fatal(err)
	}
	s, ok := Rollup(ws)

	if !ok || s.Total != 3 || s.Decisions != 2 || s.CostUSD != 0.75 || s.ByKind["ask"] != 2 || s.ByRung["none"] != 2 || s.ByRung["2"] != 1 {
		t.Fatalf("summary = %+v ok=%v", s, ok)
	}
	body, err := os.ReadFile(filepath.Join(ws, "interaction-summary.json"))
	if err != nil || !strings.HasSuffix(string(body), "}\n") {
		t.Fatalf("summary file = %q err=%v", body, err)
	}

	empty := t.TempDir()
	if err := WriteRollup(empty); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(empty, "interaction-summary.json")); !os.IsNotExist(statErr) {
		t.Errorf("a workspace with nothing to summarize gets no file: %v", statErr)
	}
	if _, ok := Rollup(""); ok {
		t.Error("an empty workspace path summarizes nothing")
	}
}
