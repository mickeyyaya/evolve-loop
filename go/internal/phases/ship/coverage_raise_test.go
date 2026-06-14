// coverage_raise_test.go — deterministic error/branch coverage for the
// ship-package helpers the parity matrix and existing gap suites still miss.
// Every test pins a behavior (a specific refusal/error or a specific
// log/state side-effect), not a line. No real network, no sleeps; git is
// only used through the shared real-repo helpers (makeRepo etc.).
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

// TestWriteStateMap_MarshalError pins that a value JSON cannot encode
// (a func) surfaces as a "marshal" error rather than a partial/atomic
// write. Exercises the json.MarshalIndent error branch.
func TestWriteStateMap_MarshalError(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "state.json")
	bad := map[string]any{"fn": func() {}} // funcs are not JSON-encodable

	// Act
	err := writeStateMap(path, bad)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("want marshal error for unencodable value, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("marshal failure must not create %s", path)
	}
}

// TestWriteStateMap_RenameError pins that when the destination path is an
// existing directory, the final atomic rename fails (a file cannot replace
// a directory) and the tmp file is cleaned up rather than left behind.
func TestWriteStateMap_RenameError(t *testing.T) {
	// Arrange: dest is a directory; its parent (TempDir) is writable so the
	// tmp create/write/sync/close all succeed and only os.Rename fails.
	root := t.TempDir()
	dest := filepath.Join(root, "state-as-dir")
	mustMkdir(t, dest)

	// Act
	err := writeStateMap(dest, map[string]any{"k": "v"})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "rename") {
		t.Fatalf("want rename error when dest is a directory, got %v", err)
	}
	// No leftover *.tmp siblings in the parent dir.
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("tmp file %q leaked after rename failure", e.Name())
		}
	}
}

// --- verify.go: IntegrityError.Unwrap nil branch ------------------------

// TestIntegrityError_Unwrap_NilWrapped pins that an IntegrityError with no
// wrapped ShipError unwraps to nil (so errors.As stops cleanly rather than
// dereferencing a nil *core.ShipError).
func TestIntegrityError_Unwrap_NilWrapped(t *testing.T) {
	ie := &IntegrityError{Msg: "legacy direct-construction"}
	if got := ie.Unwrap(); got != nil {
		t.Errorf("Unwrap with nil wrapped = %v, want nil", got)
	}
	// Error() falls back to Msg when se is nil.
	if got := ie.Error(); got != "legacy direct-construction" {
		t.Errorf("Error()=%q, want the bare Msg", got)
	}
}

// --- verify.go: isTerminal Stat-error branch ----------------------------

// TestIsTerminal_ClosedFile_ReturnsFalse pins that an *os.File whose Stat
// fails (because it is closed) is treated as non-terminal — the
// conservative default that forces --class manual to refuse rather than
// hang waiting on a phantom tty.
func TestIsTerminal_ClosedFile_ReturnsFalse(t *testing.T) {
	// Arrange
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	_ = f.Close() // now Stat() on f fails

	// Act / Assert
	if isTerminal(f) {
		t.Error("a closed *os.File must not be reported as a terminal")
	}
}

// TestIsTerminal_RegularFile_ReturnsFalse is the non-error negative case: a
// regular (non-char-device) file is not a terminal.
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

// commitGateOpts builds Options for a direct verifyCommitGateAttestation
// unit call: manual class, not dry-run, no bypass.
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

// TestVerifyCommitGate_MalformedJSON_Refuses pins that an attestation file
// that is not valid JSON is a config refusal (CodeCommitGateMalformed),
// directing the operator to re-run /commit.
func TestVerifyCommitGate_MalformedJSON_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"), "{ not json")

	err := verifyCommitGateAttestation(context.Background(), commitGateOpts(t, repo), &RunResult{})
	wantShipErr(t, err, core.CodeCommitGateMalformed, core.ShipClassConfig, "malformed JSON")
}

// TestVerifyCommitGate_EmptyTreeSHA_Refuses pins that a well-formed
// attestation missing tree_state_sha is rejected as malformed.
func TestVerifyCommitGate_EmptyTreeSHA_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"),
		`{"ts":"2026-05-27T00:00:00Z"}`)

	err := verifyCommitGateAttestation(context.Background(), commitGateOpts(t, repo), &RunResult{})
	wantShipErr(t, err, core.CodeCommitGateMalformed, core.ShipClassConfig, "no tree_state_sha")
}

// TestVerifyCommitGate_ReadError_NonNotExist_Transient pins that a read
// error that is NOT os.IsNotExist (here: the attestation path is a
// directory) maps to a transient state-IO error, distinct from the
// "missing attestation" config refusal.
func TestVerifyCommitGate_ReadError_NonNotExist_Transient(t *testing.T) {
	repo := makeRepo(t)
	// Make attestation.json a directory so os.ReadFile returns a non-NotExist error.
	mustMkdir(t, filepath.Join(repo, ".commit-gate", "attestation.json"))

	err := verifyCommitGateAttestation(context.Background(), commitGateOpts(t, repo), &RunResult{})
	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "read commit-gate attestation")
}

// --- audit.go: WorktreeTreeSHA priority + alien-line skip ----------------

// TestVerifyAuditBinding_WorktreeTreeSHA_TakesPriority pins that when the
// ledger auditor entry carries worktree_tree_sha, it is the bound tree SHA
// (the changes-commit tree), overriding any report-comment value. This is
// the cycle-152 fix that prevents INTEGRITY_TREE_DRIFT on every worktree cycle.
func TestVerifyAuditBinding_WorktreeTreeSHA_TakesPriority(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS", map[string]string{}) // matching HEAD/tree
	// Append worktree_tree_sha to the auditor ledger entry.
	wantWT := "feedface00000000000000000000000000000000000000000000000000000000"
	ledger := filepath.Join(repo, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	patched := strings.Replace(string(raw), `"tree_state_sha":`,
		`"worktree_tree_sha":"`+wantWT+`","tree_state_sha":`, 1)
	mustWrite(t, ledger, patched)

	opts := auditOpts(t, repo)
	if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err != nil {
		t.Fatalf("verifyAuditBinding: %v", err)
	}
	if opts.internalAuditBoundTreeSHA != wantWT {
		t.Errorf("audit-bound tree SHA = %q, want worktree value %q",
			opts.internalAuditBoundTreeSHA, wantWT)
	}
}

// TestFindLatestAudit_SkipsUnparseableLine pins that a non-JSON ("alien")
// line in the ledger is skipped (forward-compat) rather than crashing
// ship-gate, and the real auditor entry below it is still found.
func TestFindLatestAudit_SkipsUnparseableLine(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	ledger := filepath.Join(repo, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	// Append an alien line AFTER the auditor entry. findLatestAudit walks
	// backwards, so it hits the alien line FIRST — it must `continue` past
	// the unmarshal error and still find the auditor entry below it.
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

// TestRepinPostCycle_MissingBinary_BestEffortNoop pins that a ship binary
// that cannot be hashed (path does not exist) is swallowed as best-effort
// (returns nil, no repin), because the post-cycle self-update must never
// fail an already-pushed ship.
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

// TestWriteDryRunJournal_MkdirFails_BestEffortNoPath pins that an
// unwritable journal directory (its parent .evolve is a regular file) is
// swallowed: no DryRunPath is set and the run is not failed.
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

// TestVerifyTrivial_UnreadableCycleState_Transient pins that an unreadable
// cycle-state.json (here: a directory) is a transient state-IO error, not a
// silent "not trivial" pass — the trivial fast-path must not bypass audit on
// a corrupt cycle-state.
func TestVerifyTrivial_UnreadableCycleState_Transient(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".evolve", "cycle-state.json"))
	opts := &Options{ProjectRoot: root, Runner: execRunner}

	err := verifyTrivial(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "read cycle-state.json")
}

// --- gitops.go: maybeCreateRelease missing-plugin WITH notes set --------

// TestMaybeCreateRelease_NotesSetButNoPluginJSON_WarnsAndContinues pins
// that with EVOLVE_SHIP_RELEASE_NOTES set but no plugin.json on disk, the
// release is skipped with a WARN (best-effort) rather than failing the
// ship. (The existing missing-plugin test leaves notes empty, so it
// short-circuits before this branch.)
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

// TestPhaseRun_NilRunner_Errors pins that a zero-value Phase (nil runner)
// fails fast with a "runner required" error rather than panicking.
func TestPhaseRun_NilRunner_Errors(t *testing.T) {
	p := &Phase{nowFn: time.Now}
	_, err := p.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "runner required") {
		t.Fatalf("want 'runner required' error, got %v", err)
	}
}

// TestPhaseRun_DefaultCommitMessage_WhenContextMissing pins that an absent
// Context["commit_message"] is backfilled with the synthesized cycle
// message (cycle-150 fix) so the ship still proceeds end-to-end. We assert
// the synthesized message reached the commit by reading HEAD's subject.
func TestPhaseRun_DefaultCommitMessage_WhenContextMissing(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ndefault-msg path\n")
	seedAudit(t, repo, "PASS")
	addRemote(t, repo)

	p := New(Config{Runner: execRunner})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle:       42,
		ProjectRoot: repo,
		Workspace:   filepath.Join(repo, ".evolve", "runs", "cycle-1"),
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
