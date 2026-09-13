package main

// cmd_signals_test.go — `evolve signals codes generate|check` (ADR-0101 S2):
// the checked-in docs/architecture/signal-codes.md is a projection of the code
// registry every module links into this binary. The repo-doc test is the CI
// gate: a code registered without regenerating the doc (or documented
// differently from its registration) fails here.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignalsCodes_RepoDocIsInSyncWithTheLinkedRegistry(t *testing.T) {
	docPath := filepath.Join("..", "..", "..", "docs", "architecture", "signal-codes.md")
	var out, errb bytes.Buffer
	if rc := signalCodesRun(docPath, false, &out, &errb); rc != 0 {
		t.Fatalf("signal-codes.md drifted from the registry (rc=%d): %s — run `evolve signals codes generate`", rc, errb.String())
	}
	doc, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"### orchestrator", "### ship", "### signalcenter", "`ORCHESTRATOR_PHASE_VERDICT_FAIL`", "`SHIP_GIT_FLEET_REBASE_NEEDED`", "`SIGNALCENTER_LISTENER_PANICKED`"} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("the generated doc lists every linked module's codes; missing %s", want)
		}
	}
}

func TestSignalsCodes_GenerateFillsTheRegionAndCheckSeesDrift(t *testing.T) {
	docPath := filepath.Join(t.TempDir(), "signal-codes.md")
	stale := "# Codes\n\nprose kept byte-for-byte\n\n" + signalCodesBegin + "\n\nstale\n" + signalCodesEnd + "\n\ntrailing prose\n"
	if err := os.WriteFile(docPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := signalCodesRun(docPath, false, &out, &errb); rc != 2 || !strings.Contains(errb.String(), "stale") {
		t.Errorf("check exits 2 on drift and names the remedy: rc=%d %s", rc, errb.String())
	}
	if rc := signalCodesRun(docPath, true, &out, &errb); rc != 0 || !strings.Contains(out.String(), "regenerated") {
		t.Fatalf("generate writes the region: rc=%d %s %s", rc, out.String(), errb.String())
	}
	doc, _ := os.ReadFile(docPath)
	if !strings.HasPrefix(string(doc), "# Codes\n\nprose kept byte-for-byte\n") || !strings.HasSuffix(string(doc), "\n\ntrailing prose\n") {
		t.Errorf("hand-written prose outside the markers is preserved:\n%s", doc)
	}
	if !strings.Contains(string(doc), "| `SHIP_GIT_IO` |") || strings.Contains(string(doc), "\nstale\n") {
		t.Errorf("the region holds the rendered registry:\n%s", doc)
	}
	out.Reset()
	if rc := signalCodesRun(docPath, false, &out, &errb); rc != 0 || !strings.Contains(out.String(), "in sync") {
		t.Errorf("check passes after generate: rc=%d %s", rc, out.String())
	}
	out.Reset()
	if rc := signalCodesRun(docPath, true, &out, &errb); rc != 0 || !strings.Contains(out.String(), "up to date") {
		t.Errorf("generate is idempotent: rc=%d %s", rc, out.String())
	}
}

func TestSignalsCodes_ErrorsAreLoud(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := signalCodesRun(filepath.Join(t.TempDir(), "missing.md"), false, &out, &errb); rc != 1 {
		t.Errorf("a missing doc is an error, not a pass: rc=%d", rc)
	}
	unmarked := filepath.Join(t.TempDir(), "signal-codes.md")
	if err := os.WriteFile(unmarked, []byte("# no markers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := signalCodesRun(unmarked, true, &out, &errb); rc != 0 {
		t.Errorf("a doc without the markers gets the region appended (the flags precedent): rc=%d %s", rc, errb.String())
	}
	if doc, _ := os.ReadFile(unmarked); !strings.HasPrefix(string(doc), "# no markers\n") || !strings.Contains(string(doc), signalCodesBegin) || !strings.HasSuffix(string(doc), signalCodesEnd+"\n") {
		t.Errorf("the region is appended after the existing prose:\n%s", doc)
	}
	halfMarked := filepath.Join(t.TempDir(), "signal-codes.md")
	if err := os.WriteFile(halfMarked, []byte(signalCodesBegin+"\nno end marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	if rc := signalCodesRun(halfMarked, false, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "splice") {
		t.Errorf("a BEGIN marker without its END is a splice error: rc=%d %s", rc, errb.String())
	}
	if rc := runSignals(nil, nil, &out, &errb); rc != 10 {
		t.Errorf("usage error exits 10, got %d", rc)
	}
	if rc := runSignals([]string{"codes", "frobnicate"}, nil, &out, &errb); rc != 10 {
		t.Errorf("unknown subcommand exits 10, got %d", rc)
	}
}

// The command resolves the doc from the SOURCE root (the worktree under the
// ACS suite, like `evolve flags`), and dispatches generate|check.
func TestRunSignals_DispatchesAgainstTheSourceRoot(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_WORKTREE_ROOT", root)
	var out, errb bytes.Buffer
	if rc := runSignals([]string{"codes", "check"}, nil, &out, &errb); rc != 0 || !strings.Contains(out.String(), "in sync") {
		t.Fatalf("check against the repo doc: rc=%d %s %s", rc, out.String(), errb.String())
	}
	out.Reset()
	if rc := runSignals([]string{"codes", "generate"}, nil, &out, &errb); rc != 0 || !strings.Contains(out.String(), "up to date") {
		t.Errorf("generate on an in-sync doc rewrites nothing: rc=%d %s %s", rc, out.String(), errb.String())
	}
}

func TestSignalsCodes_WriteFailureIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file-mode write checks")
	}
	docPath := filepath.Join(t.TempDir(), "signal-codes.md")
	if err := os.WriteFile(docPath, []byte(signalCodesBegin+"\nstale\n"+signalCodesEnd+"\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := signalCodesRun(docPath, true, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "write") {
		t.Errorf("an unwritable doc is a loud error: rc=%d %s", rc, errb.String())
	}
}
