package agentcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func keepFixture(t *testing.T, ledgerLines string) standaloneRun {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "ephemeral", "agent-runs", "run-1")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(llmcalls.Path(workspace), []byte(ledgerLines), 0o644); err != nil {
		t.Fatal(err)
	}
	return standaloneRun{spec: phasespec.PhaseSpec{Name: "prof"}, root: root, label: "run-1", workspace: workspace}
}

func TestKeepLLMCalls_MalformedRowsAreReportedAndValidRowsAreKept(t *testing.T) {
	r := keepFixture(t, "garbage\n"+`{"call_id":"ok","phase":"build"}`+"\n")
	var stderr bytes.Buffer

	keepLLMCalls(r, &stderr)

	if !strings.Contains(stderr.String(), "skipped 1 malformed") {
		t.Errorf("stderr = %q, want the skipped-row warning", stderr.String())
	}
	res, err := llmcalls.Read(filepath.Join(r.root, ".evolve", "agent-runs", llmcalls.Filename))
	if err != nil || len(res.Records) != 1 {
		t.Fatalf("durable ledger = %+v, %v; want one row", res, err)
	}
	if got := res.Records[0]; got.RunID != "run-1" || got.Profile != "prof" {
		t.Errorf("tags = %q/%q, want run-1/prof", got.RunID, got.Profile)
	}
}

func TestKeepLLMCalls_ABlockedLedgerDirectoryWarnsAndNamesTheLedger(t *testing.T) {
	r := keepFixture(t, `{"call_id":"ok","phase":"build"}`+"\n")
	blocker := filepath.Join(r.root, ".evolve", "agent-runs")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	keepLLMCalls(r, &stderr)

	if !strings.Contains(stderr.String(), llmcalls.Filename) {
		t.Errorf("stderr = %q, want a warning naming %s", stderr.String(), llmcalls.Filename)
	}
}
