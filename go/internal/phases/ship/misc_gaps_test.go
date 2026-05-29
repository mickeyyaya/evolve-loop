// misc_gaps_test.go — covers the remaining uncovered branches across
// verifyClass (ClassRelease, ClassTrivial, invalid), postShip (non-cycle,
// non-dryrun cycle success), Run (BYPASS_SHIP_VERIFY), writeShipBinding
// (no cycle_id), currentBranch (runner error), buildDiffFooterAtDir
// (empty files → empty footer), pluginVersion (invalid JSON), and
// useNativeShip (os.Getenv=0 path).
package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// --- verifyClass -----------------------------------------------------------

// TestVerifyClass_Release_LogsAndReturnsNil: ClassRelease logs two skip
// lines and returns nil (pipeline-internal path — no audit check).
func TestVerifyClass_Release_LogsAndReturnsNil(t *testing.T) {
	repo := makeRepo(t)
	opts := &Options{
		Class:          ClassRelease,
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner:         execRunner,
	}
	res := &RunResult{}
	if err := verifyClass(context.Background(), opts, res); err != nil {
		t.Fatalf("ClassRelease must not error; got %v", err)
	}
	if !containsLog(*res, "class: release") {
		t.Errorf("missing release class log; got %v", res.Logs)
	}
	if !containsLog(*res, "audit verification skipped: version-bump.sh") {
		t.Errorf("missing skip-audit log; got %v", res.Logs)
	}
	if res.Provenance != "release (pipeline-generated)" {
		t.Errorf("provenance=%q, want release (pipeline-generated)", res.Provenance)
	}
}

// TestVerifyClass_Invalid_ReturnsError: an unrecognized class string returns
// a plain error (not an IntegrityError) referencing "invalid class".
func TestVerifyClass_Invalid_ReturnsError(t *testing.T) {
	opts := &Options{Class: Class("bogus"), ProjectRoot: t.TempDir()}
	err := verifyClass(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "invalid class") {
		t.Fatalf("invalid class must error with 'invalid class'; got %v", err)
	}
}

// TestVerifyClass_Trivial_ReadsCycleState: ClassTrivial reads cycle-state.json.
// A missing .evolve dir causes a read error, which should surface.
func TestVerifyClass_Trivial_MissingCycleState_Errors(t *testing.T) {
	// No .evolve dir → readStateMap returns empty map → stateString returns "".
	// That means cycle_size_estimate = "" ≠ "trivial" → IntegrityError.
	root := t.TempDir()
	opts := &Options{
		Class:       ClassTrivial,
		ProjectRoot: root,
		Runner:      (&scriptedRunner{}).runner(),
	}
	err := verifyClass(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("missing cycle-state.json must error")
	}
}

// --- postShip (non-cycle path) ---------------------------------------------

// TestPostShip_NonCycleClass_OnlyLogsDone: for ClassRelease, postShip
// skips advance/promote/repin entirely and just logs the DONE line.
func TestPostShip_NonCycleClass_OnlyLogsDone(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "fake binary content\n")
	opts := &Options{
		Class:          ClassRelease,
		ProjectRoot:    root,
		ShipBinaryPath: bin,
	}
	// Seed state.json so repinPostCycle doesn't fail on missing path.
	mustWrite(t, filepath.Join(root, ".evolve", "state.json"), `{}`)

	res := &RunResult{ClassUsed: ClassRelease, CommitSHA: "abc123"}
	if err := postShip(context.Background(), opts, res); err != nil {
		t.Fatalf("postShip non-cycle errored: %v", err)
	}
	if !containsLog(*res, "DONE: shipped release at abc123") {
		t.Errorf("missing DONE log; got %v", res.Logs)
	}
}

// TestPostShip_DryRun_NoopAndNoLog: postShip with DryRun=true returns nil
// immediately without writing any log (the commit is not on remote).
func TestPostShip_DryRun_NoopAndNoLog(t *testing.T) {
	opts := &Options{DryRun: true, Class: ClassCycle, ProjectRoot: t.TempDir()}
	res := &RunResult{}
	if err := postShip(context.Background(), opts, res); err != nil {
		t.Fatalf("postShip DryRun errored: %v", err)
	}
	if len(res.Logs) != 0 {
		t.Errorf("DryRun postShip must produce no logs; got %v", res.Logs)
	}
}

// TestPostShip_ClassCycle_AdvancesAndLogs: ClassCycle + cycle_id present →
// advances lastCycleNumber and appends the DONE log.
func TestPostShip_ClassCycle_AdvancesAndLogs(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "fake binary\n")
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":10}`)
	mustWrite(t, filepath.Join(root, ".evolve", "state.json"), `{"lastCycleNumber":9}`)
	opts := &Options{
		Class:          ClassCycle,
		ProjectRoot:    root,
		ShipBinaryPath: bin,
	}
	res := &RunResult{ClassUsed: ClassCycle, CommitSHA: "deadbeef"}
	if err := postShip(context.Background(), opts, res); err != nil {
		t.Fatalf("postShip cycle errored: %v", err)
	}
	if !containsLog(*res, "DONE: shipped cycle at deadbeef") {
		t.Errorf("missing DONE log; got %v", res.Logs)
	}
	m, _ := readStateMap(filepath.Join(root, ".evolve", "state.json"))
	if n, _ := stateInt(m, "lastCycleNumber"); n != 10 {
		t.Errorf("lastCycleNumber not advanced; got %d want 10", n)
	}
}

// --- Run: BYPASS_SHIP_VERIFY bridge ----------------------------------------

// TestRun_BypassShipVerify_TranslatesToManualAutoConfirm: when
// EVOLVE_BYPASS_SHIP_VERIFY=1 is set with ClassCycle, the bridge must:
//  1. log the deprecation warning
//  2. re-classify as ClassManual + EVOLVE_SHIP_AUTO_CONFIRM=1
//  3. ship successfully (using a real repo with staged changes)
func TestRun_BypassShipVerify_TranslatesToManualAutoConfirm(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "bypass.txt"), "bypass test\n")
	// No seedAudit — bypass skips audit entirely.

	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "bypass: deprecation bridge",
		Env: map[string]string{
			"EVOLVE_BYPASS_SHIP_VERIFY": "1",
			"EVOLVE_SHIP_AUTO_CONFIRM":  "1",
			"EVOLVE_BYPASS_ROLE_GATE":   "1",
			"EVOLVE_BYPASS_SHIP_GATE":   "1",
			"EVOLVE_BYPASS_PREFIX_GATE": "1",
		},
	})
	if err != nil {
		t.Fatalf("bypass ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("bypass ship want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "DEPRECATION: EVOLVE_BYPASS_SHIP_VERIFY=1") {
		t.Errorf("missing deprecation log; got %v", res.Logs)
	}
	if res.ClassUsed != ClassManual {
		t.Errorf("ClassUsed=%q, want ClassManual after bypass translate", res.ClassUsed)
	}
}

// --- writeShipBinding: no cycle_id -----------------------------------------

// TestWriteShipBinding_NoCycleID_Errors: when cycle-state.json has no
// cycle_id field, writeShipBinding must return a plain "no cycle_id" error
// (not panic, not silently skip the file write).
func TestWriteShipBinding_NoCycleID_Errors(t *testing.T) {
	root := t.TempDir()
	// cycle-state.json with no cycle_id key.
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"phase":"ship"}`)
	opts := &Options{ProjectRoot: root}
	err := writeShipBinding(opts, "abc123tree", "abc123commit")
	if err == nil {
		t.Fatal("missing cycle_id must return error")
	}
	if !strings.Contains(err.Error(), "no cycle_id") {
		t.Errorf("error should mention no cycle_id; got %q", err.Error())
	}
}

// TestWriteShipBinding_MissingCycleState_Errors: cycle-state.json doesn't
// exist → readStateMap returns empty map → no cycle_id → error.
func TestWriteShipBinding_MissingCycleState_Errors(t *testing.T) {
	root := t.TempDir()
	// No cycle-state.json created.
	opts := &Options{ProjectRoot: root}
	err := writeShipBinding(opts, "tree", "commit")
	if err == nil {
		t.Fatal("missing cycle-state.json must return error")
	}
}

// --- currentBranch: runner error -------------------------------------------

// TestCurrentBranch_RunnerError_Propagates: when the Runner itself returns
// an error (not just a non-zero exit code), currentBranch must propagate it.
func TestCurrentBranch_RunnerError_Propagates(t *testing.T) {
	errRunner := func(ctx context.Context, name string, args, env []string, cwd string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		// simulate a hard runner failure (not an exit-code failure)
		if name == "git" {
			return -1, &IntegrityError{Msg: "runner died"}
		}
		return 0, nil
	}
	opts := &Options{ProjectRoot: t.TempDir(), Runner: errRunner}
	_, err := currentBranch(context.Background(), opts)
	if err == nil {
		t.Fatal("runner error in symbolic-ref must propagate")
	}
}

// --- buildDiffFooterAtDir: empty staged diff → empty footer ----------------

// TestBuildDiffFooterAtDir_EmptyDiff_ReturnsEmptyString: when the staged
// area has no files changed, buildDiffFooterAtDir returns an empty footer
// string (no footer appended to the commit message).
func TestBuildDiffFooterAtDir_EmptyDiff_ReturnsEmptyString(t *testing.T) {
	r := &scriptedRunner{}
	r.runner()
	// Both git diff queries return empty stdout (nothing staged).
	r.scripts["git diff"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "", exit: 0}
	opts := &Options{ProjectRoot: t.TempDir(), Runner: r.runner()}
	footer, err := buildDiffFooterAtDir(context.Background(), opts, opts.ProjectRoot)
	if err != nil {
		t.Fatalf("empty diff must not error: %v", err)
	}
	if footer != "" {
		t.Errorf("empty staged area must yield empty footer; got %q", footer)
	}
}

// --- pluginVersion: invalid JSON path --------------------------------------

// TestPluginVersion_InvalidJSON_ReturnsEmpty: a plugin.json that contains
// invalid JSON must return "" (not panic, not error-out).
func TestPluginVersion_InvalidJSON_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{not valid json`)
	if got := pluginVersion(root); got != "" {
		t.Errorf("invalid JSON must yield empty version; got %q", got)
	}
}

// --- useNativeShip: os.Getenv="0" fallback ---------------------------------

// TestUseNativeShip_OSGetenvZero_ReturnsFalse: when EVOLVE_NATIVE_SHIP is
// not in the map but is "0" in the process env, useNativeShip should return
// false (legacy bash path). This exercises the os.Getenv branch.
func TestUseNativeShip_OSGetenvZero_ReturnsFalse(t *testing.T) {
	t.Setenv("EVOLVE_NATIVE_SHIP", "0")
	// Pass an empty map so the env-map branch is skipped.
	if useNativeShip(map[string]string{}) {
		t.Error("EVOLVE_NATIVE_SHIP=0 in os env with empty map must return false")
	}
}

// --- verifyManualConfirm: non-tty stdin blocks (IntegrityError) -------------

// TestVerifyManualConfirm_NonTTY_IntegrityError: when git diff --cached has
// staged changes and EVOLVE_SHIP_AUTO_CONFIRM is not set, but stdin is not a
// TTY (bytes.Buffer), the function must return IntegrityError requiring
// interactive stdin.
func TestVerifyManualConfirm_NonTTY_IntegrityError(t *testing.T) {
	r := &scriptedRunner{}
	r.runner()
	// git diff --cached --quiet → exit 1 = there are staged changes.
	r.scripts["git diff"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "", exit: 1}
	// All "git diff …" calls match this single key: the scriptedRunner
	// dispatches on the first non-flag arg, so --cached/--stat/--quiet all
	// resolve to "git diff". exit 1 = staged changes present; the --stat and
	// full-diff calls ignore the exit code (impl checks only err), so this one
	// entry drives the whole verifyManualConfirm flow up to the isTerminal check.

	// Use a bytes.Buffer (not os.Stdin) so isTerminal returns false.
	var stdinBuf strings.Builder
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner:      r.runner(),
		Stderr:      io.Discard,
		Stdin:       strings.NewReader(stdinBuf.String()),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeManualNotTTY, core.ShipClassConfig, "not a tty")
}

// --- verifyAuditBinding: TreeStateSHA mismatch (uncommitted changes) --------

// TestVerifyAuditBinding_TreeMismatch_IntegrityError: HEAD matches but the
// working tree has uncommitted changes since audit (tree-state SHA mismatch).
func TestVerifyAuditBinding_TreeMismatch_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	// Modify a tracked file WITHOUT staging — the tree-state SHA changes.
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture modified post-audit\n")

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "uncommitted changes")
}
