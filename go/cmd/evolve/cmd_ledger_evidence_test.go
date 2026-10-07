package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

type legacyCarryLedger struct {
	evolveDir string
}

func newLegacyCarryLedger(t *testing.T, claimPatchID func(pid string) string) legacyCarryLedger {
	t.Helper()
	r := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(r.Dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "base.txt")
	r.Git("commit", "-q", "-m", "base")
	base := r.Git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(r.Dir, "lane.txt"), []byte("the lane's audited change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "lane.txt")
	tree := r.Git("write-tree")
	diff := r.Git(treedelta.Args(base, tree)...) + "\n"
	r.Git("rm", "-q", "--cached", "lane.txt")
	evolveDir := filepath.Join(r.Dir, ".evolve")
	gone := filepath.Join(evolveDir, "worktrees", "cycle-cd3ae73e-1766", ".evolve", "composition-artifacts", "composition-1766-audited.diff")
	line, err := json.Marshal(map[string]any{
		"kind": "composition-verdict", "method": "identical-rebase", "cycle": 1766,
		"patch_id": claimPatchID(compPatchID(t, r.Dir, diff)), "audited_base": base, "audited_tree_sha": tree,
		"git_head": base, "tree_state_sha": tree, "audited_diff_path": gone, "composed_diff_path": gone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return legacyCarryLedger{evolveDir: evolveDir}
}

func (l legacyCarryLedger) run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runLedger(append(args, "--evolve-dir", l.evolveDir), nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestLedgerEvidenceRestore_PreviewsThenRestoresALegacyCarrysDiffsFromGit(t *testing.T) {
	l := newLegacyCarryLedger(t, func(pid string) string { return pid })
	if rc, _, stderr := l.run(t, "verify"); rc != 2 {
		t.Fatalf("verify before the restore = %d, want 2 (BROKEN): %s", rc, stderr)
	}

	rc, stdout, stderr := l.run(t, "evidence", "restore", "--dry-run")
	if rc != 0 || !strings.Contains(stdout, "line 0 cycle 1766 identical-rebase: would be rebuilt") {
		t.Fatalf("dry run = %d\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
	if rc, _, _ := l.run(t, "verify"); rc != 2 {
		t.Fatalf("verify after a dry run = %d, want still 2", rc)
	}

	rc, stdout, stderr = l.run(t, "evidence", "restore")
	if rc != 0 || !strings.Contains(stdout, "line 0 cycle 1766 identical-rebase: rebuilt") || !strings.Contains(stderr, "[ledger] OK") {
		t.Fatalf("restore = %d\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
	if rc, _, stderr := l.run(t, "verify"); rc != 0 {
		t.Fatalf("verify after the restore = %d: %s", rc, stderr)
	}
}

func TestLedgerEvidenceRestore_RefusesALineWhoseDiffDoesNotReDeriveItsPatchID(t *testing.T) {
	l := newLegacyCarryLedger(t, func(string) string { return strings.Repeat("ab", 20) })

	rc, stdout, stderr := l.run(t, "evidence", "restore")

	if rc != 1 || !strings.Contains(stdout, "line 0 cycle 1766 identical-rebase: unrestorable: ") {
		t.Fatalf("restore = %d, want 1 naming the refused line\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
}

func TestLedgerEvidence_RefusesAnUnknownVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runLedger([]string{"evidence", "rewrite"}, nil, &stdout, &stderr)

	if rc != 10 || !strings.Contains(stderr.String(), "usage: evolve ledger evidence restore") {
		t.Fatalf("rc = %d, stderr = %q; want exit 10 with the evidence verb's own usage", rc, stderr.String())
	}
}

func TestLedgerEvidenceRestore_ExitsTwoWhenTheChainStillDoesNotVerify(t *testing.T) {
	l := newLegacyCarryLedger(t, func(pid string) string { return pid })
	ledgerPath := filepath.Join(l.evolveDir, "ledger.jsonl")
	body, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	unrelatedBreak := `{"kind":"k","role":"r","entry_seq":5,"prev_hash":"` + strings.Repeat("ab", 32) + `"}` + "\n"
	if err := os.WriteFile(ledgerPath, append(body, unrelatedBreak...), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, stdout, stderr := l.run(t, "evidence", "restore")

	if rc != 2 || !strings.Contains(stdout, "line 0 cycle 1766 identical-rebase: rebuilt") || !strings.Contains(stderr, "still does NOT verify") {
		t.Fatalf("restore = %d, want 2: the line is restored but an unrelated break remains\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
}
