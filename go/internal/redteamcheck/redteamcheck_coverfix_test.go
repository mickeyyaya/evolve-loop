package redteamcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLedger_SkipsBlankAndMalformedLines(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	content := `{"kind":"cycle_terminal","cycle":3}` + "\n\nnot json\n" + `{"kind":"agent_subprocess","cycle":3}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, present, err := readLedger(path)
	if err != nil || !present {
		t.Fatalf("readLedger = present %v, err %v; want present and no error", present, err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want the 2 well-formed lines", len(entries))
	}
}
