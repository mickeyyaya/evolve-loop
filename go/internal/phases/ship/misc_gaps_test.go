//go:build integration

package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestVerifyClass_Release_LogsAndReturnsNil(t *testing.T) {
	t.Parallel()
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

func TestVerifyClass_Invalid_ReturnsError(t *testing.T) {
	t.Parallel()
	opts := &Options{Class: Class("bogus"), ProjectRoot: t.TempDir()}
	err := verifyClass(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "invalid class") {
		t.Fatalf("invalid class must error with 'invalid class'; got %v", err)
	}
}

func TestVerifyClass_Trivial_MissingCycleState_Errors(t *testing.T) {
	t.Parallel()
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

func TestPostShip_NonCycleClass_OnlyLogsDone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "fake binary content\n")
	opts := &Options{
		Class:          ClassRelease,
		ProjectRoot:    root,
		ShipBinaryPath: bin,
	}
	mustWrite(t, filepath.Join(root, ".evolve", "state.json"), `{}`)

	res := &RunResult{ClassUsed: ClassRelease, CommitSHA: "abc123"}
	if err := postShip(context.Background(), opts, res); err != nil {
		t.Fatalf("postShip non-cycle errored: %v", err)
	}
	if !containsLog(*res, "DONE: shipped release at abc123") {
		t.Errorf("missing DONE log; got %v", res.Logs)
	}
}

func TestPostShip_DryRun_NoopAndNoLog(t *testing.T) {
	t.Parallel()
	opts := &Options{DryRun: true, Class: ClassCycle, ProjectRoot: t.TempDir()}
	res := &RunResult{}
	if err := postShip(context.Background(), opts, res); err != nil {
		t.Fatalf("postShip DryRun errored: %v", err)
	}
	if len(res.Logs) != 0 {
		t.Errorf("DryRun postShip must produce no logs; got %v", res.Logs)
	}
}

func TestPostShip_ClassCycle_AdvancesAndLogs(t *testing.T) {
	t.Parallel()
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

func TestRun_BypassShipVerify_FlagSilentlyIgnored_NoBridge(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "bypass.txt"), "bypass test\n")

	res, _ := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "bypass: flag silently ignored",
		AutoConfirm:   "test",
		Env: map[string]string{
			"EVOLVE_BYPASS_SHIP_VERIFY": "1",
			"EVOLVE_BYPASS_ROLE_GATE":   "1",
			"EVOLVE_BYPASS_SHIP_GATE":   "1",
			"EVOLVE_BYPASS_PREFIX_GATE": "1",
		},
	})
	if res.ClassUsed == ClassManual {
		t.Errorf("ClassUsed=%q, flag must not bridge to ClassManual anymore", res.ClassUsed)
	}
	if containsLog(res, "DEPRECATION: EVOLVE_BYPASS_SHIP_VERIFY=1") {
		t.Errorf("deprecation log emitted — bridge should be removed: %v", res.Logs)
	}
}

func TestWriteShipBinding_NoCycleID_Errors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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

func TestWriteShipBinding_MissingCycleState_Errors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	opts := &Options{ProjectRoot: root}
	err := writeShipBinding(opts, "tree", "commit")
	if err == nil {
		t.Fatal("missing cycle-state.json must return error")
	}
}

func TestCurrentBranch_RunnerError_Propagates(t *testing.T) {
	t.Parallel()
	errRunner := func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
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

func TestBuildDiffFooterAtDir_EmptyDiff_ReturnsEmptyString(t *testing.T) {
	t.Parallel()
	r := &scriptedRunner{}
	r.runner()
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

func TestPluginVersion_InvalidJSON_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{not valid json`)
	if got := pluginVersion(root); got != "" {
		t.Errorf("invalid JSON must yield empty version; got %q", got)
	}
}

func TestVerifyManualConfirm_NonTTY_IntegrityError(t *testing.T) {
	t.Parallel()
	r := &scriptedRunner{}
	r.runner()
	r.scripts["git diff"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "", exit: 1}

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

func TestVerifyAuditBinding_TreeMismatch_IntegrityError(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture modified post-audit\n")

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "uncommitted changes")
}
