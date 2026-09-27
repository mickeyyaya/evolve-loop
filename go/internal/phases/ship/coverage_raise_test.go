//go:build integration

package ship

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// --- statefile.go: writeStateMap error branches --------------------------

func TestWriteStateMap_MarshalError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	bad := map[string]any{"fn": func() {}} // funcs are not JSON-encodable

	err := writeStateMap(path, bad)

	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("want marshal error for unencodable value, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("marshal failure must not create %s", path)
	}
}

func TestWriteStateMap_RenameError(t *testing.T) {
	// dest is a directory: create/write/sync/close succeed, only os.Rename fails.
	root := t.TempDir()
	dest := filepath.Join(root, "state-as-dir")
	mustMkdir(t, dest)

	err := writeStateMap(dest, map[string]any{"k": "v"})

	if err == nil || !strings.Contains(err.Error(), "rename") {
		t.Fatalf("want rename error when dest is a directory, got %v", err)
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("tmp file %q leaked after rename failure", e.Name())
		}
	}
}

// --- verify.go: IntegrityError.Unwrap nil branch ------------------------

func TestIntegrityError_Unwrap_NilWrapped(t *testing.T) {
	ie := &IntegrityError{Msg: "legacy direct-construction"}
	if got := ie.Unwrap(); got != nil {
		t.Errorf("Unwrap with nil wrapped = %v, want nil", got)
	}
	if got := ie.Error(); got != "legacy direct-construction" {
		t.Errorf("Error()=%q, want the bare Msg", got)
	}
}

// --- verify.go: isTerminal Stat-error branch ----------------------------

func TestIsTerminal_ClosedFile_ReturnsFalse(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	_ = f.Close() // now Stat() on f fails

	if isTerminal(f) {
		t.Error("a closed *os.File must not be reported as a terminal")
	}
}

func TestIsTerminal_RegularFile_ReturnsFalse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "plain.txt")
	mustWrite(t, p, "data\n")
	f, err := os.Open(p)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("a regular file must not be reported as a terminal")
	}
}

// --- commitgate.go: malformed / empty-SHA / read-error branches ---------

func commitGateOpts(t *testing.T, repo string) *Options {
	t.Helper()
	return &Options{
		Class:       ClassManual,
		ProjectRoot: repo,
		PluginRoot:  repo,
		Runner:      execRunner,
		NowFn:       defaultNow,
	}
}

func TestVerifyCommitGate_MalformedJSON_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"), "{ not json")

	err := verifyCommitGateAttestation(context.Background(), commitGateOpts(t, repo), &RunResult{})
	wantShipErr(t, err, core.CodeCommitGateMalformed, core.ShipClassConfig, "malformed JSON")
}

func TestVerifyCommitGate_EmptyTreeSHA_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"),
		`{"ts":"2026-05-27T00:00:00Z"}`)

	err := verifyCommitGateAttestation(context.Background(), commitGateOpts(t, repo), &RunResult{})
	wantShipErr(t, err, core.CodeCommitGateMalformed, core.ShipClassConfig, "no tree_state_sha")
}

func TestVerifyCommitGate_ReadError_NonNotExist_Transient(t *testing.T) {
	repo := makeRepo(t)
	// Make attestation.json a directory so os.ReadFile returns a non-NotExist error.
	mustMkdir(t, filepath.Join(repo, ".commit-gate", "attestation.json"))

	err := verifyCommitGateAttestation(context.Background(), commitGateOpts(t, repo), &RunResult{})
	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "read commit-gate attestation")
}

// --- audit.go: WorktreeTreeSHA priority + alien-line skip ----------------

func TestVerifyAuditBinding_WorktreeTreeSHA_TakesPriority(t *testing.T) {
	repo := makeRepo(t)
	wantWT := strings.TrimSpace(runGitOut(t, repo, "write-tree"))
	seedCustomAudit(t, repo, "Verdict: PASS\n<!-- audit_bound_tree_sha: deadbeef -->\n", 0)

	opts := auditOpts(t, repo)
	if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err != nil {
		t.Fatalf("verifyAuditBinding: %v", err)
	}
	if opts.internalAuditBoundTreeSHA != wantWT {
		t.Errorf("audit-bound tree SHA = %q, want worktree value %q",
			opts.internalAuditBoundTreeSHA, wantWT)
	}
}

func TestFindLatestAudit_SkipsUnparseableLine(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	ledger := filepath.Join(repo, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	// Appended after the real entry: findLatestAudit walks backward, so this
	// alien line is hit first.
	mustWrite(t, ledger, strings.TrimRight(string(raw), "\n")+"\nthis-is-not-json\n")

	entry, err := findLatestAudit(ledger, "")
	if err != nil {
		t.Fatalf("findLatestAudit must skip the alien line, got %v", err)
	}
	if entry.Role != "auditor" {
		t.Errorf("found entry role=%q, want auditor", entry.Role)
	}
}

// --- postship.go: repinPostCycle error branches --------------------------

func TestRepinPostCycle_MissingBinary_BestEffortNoop(t *testing.T) {
	repo := makeRepo(t)
	opts := &Options{
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: filepath.Join(repo, "does-not-exist"),
	}
	res := &RunResult{}
	if err := repinPostCycle(opts, res); err != nil {
		t.Fatalf("repinPostCycle should swallow a missing binary, got %v", err)
	}
	for _, l := range res.Logs {
		if strings.Contains(l, "post-cycle self-update") {
			t.Errorf("must not log a repin when the binary cannot be hashed: %q", l)
		}
	}
}

// --- dryrun.go: writeDryRunJournal mkdir-failure best-effort -------------

func TestWriteDryRunJournal_MkdirFails_BestEffortNoPath(t *testing.T) {
	root := t.TempDir()
	// Make .evolve a FILE so MkdirAll(.evolve/release-journal) fails.
	mustWrite(t, filepath.Join(root, ".evolve"), "i am a file, not a dir\n")
	opts := &Options{ProjectRoot: root, DryRun: true, Class: ClassCycle, Runner: execRunner}
	res := &RunResult{}

	writeDryRunJournal(context.Background(), opts, res, "test")

	if res.DryRunPath != "" {
		t.Errorf("DryRunPath must stay empty when the journal dir is unwritable; got %q", res.DryRunPath)
	}
}

// --- verify.go: verifyTrivial cycle-state read error --------------------

func TestVerifyTrivial_UnreadableCycleState_Transient(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".evolve", "cycle-state.json"))
	opts := &Options{ProjectRoot: root, Runner: execRunner}

	err := verifyTrivial(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "read cycle-state.json")
}

// --- gitops.go: maybeCreateRelease missing-plugin WITH notes set --------

func TestMaybeCreateRelease_NotesSetButNoPluginJSON_WarnsAndContinues(t *testing.T) {
	root := t.TempDir() // no .claude-plugin/plugin.json
	opts := &Options{
		Class:       ClassRelease,
		ProjectRoot: root,
		PluginRoot:  root,
		Env:         map[string]string{"EVOLVE_SHIP_RELEASE_NOTES": "notes body"},
		Runner:      execRunner,
	}
	res := &RunResult{}
	if err := maybeCreateRelease(context.Background(), opts, res); err != nil {
		t.Fatalf("missing plugin.json must be best-effort, got %v", err)
	}
	if !containsLog(*res, "no .claude-plugin/plugin.json — skipping release") {
		t.Errorf("missing skip-release WARN log; got %v", res.Logs)
	}
}

// --- ship.go: Phase.Run guards ------------------------------------------

func TestPhaseRun_NilRunner_Errors(t *testing.T) {
	p := &Phase{nowFn: time.Now}
	_, err := p.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "runner required") {
		t.Fatalf("want 'runner required' error, got %v", err)
	}
}

func TestPhaseRun_DefaultCommitMessage_WhenContextMissing(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ndefault-msg path\n")
	seedAudit(t, repo, "PASS", map[string]string{"cycle": "42"})
	addRemote(t, repo)

	p := New(Config{Runner: execRunner})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 42,
		RunID: "test-run", AuditRound: 1,
		ProjectRoot: repo,
		Workspace:   filepath.Join(repo, ".evolve", "runs", "cycle-42"),
		// No Context commit_message → defaultCommitMessage("evolve-cycle 42").
		Env: map[string]string{"EVOLVE_PLUGIN_ROOT": repo},
	})
	if err != nil {
		t.Fatalf("Run with default message errored: %v (diags=%v)", err, resp.Diagnostics)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("want VerdictPASS, got %q (diags=%v)", resp.Verdict, resp.Diagnostics)
	}
	subject := strings.TrimSpace(runGitOut(t, repo, "log", "-1", "--format=%s"))
	if !strings.Contains(subject, "evolve-cycle 42") {
		t.Errorf("commit subject = %q, want synthesized 'evolve-cycle 42'", subject)
	}
}
