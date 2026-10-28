//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_BypassShipVerify_NilEnvMap_FlagIgnored(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "bypass2.txt"), "change\n")
	t.Setenv("EVOLVE_BYPASS_SHIP_VERIFY", "1")
	t.Setenv("EVOLVE_SHIP_AUTO_CONFIRM", "1")
	t.Setenv("EVOLVE_BYPASS_ROLE_GATE", "1")
	t.Setenv("EVOLVE_BYPASS_SHIP_GATE", "1")
	t.Setenv("EVOLVE_BYPASS_PREFIX_GATE", "1")

	ctx := context.Background()
	opts := Options{
		Class:          ClassCycle,
		CommitMessage:  "bypass: nil env map",
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner:         execRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
		Env:            nil,
	}
	res, _ := Run(ctx, opts)
	if res.ClassUsed == ClassManual {
		t.Errorf("ClassUsed=%q, flag must not bridge to ClassManual anymore", res.ClassUsed)
	}
	if containsLog(res, "DEPRECATION: EVOLVE_BYPASS_SHIP_VERIFY=1") {
		t.Errorf("deprecation log must not be emitted: %v", res.Logs)
	}
}

func TestShipFromWorktree_PostPushTreeSHAMismatch_BreachLog(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "postpush-test")
	mustWrite(t, filepath.Join(wt, "postpush.txt"), "content\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":88,"active_worktree":"`+wt+`"}`)

	seedAuditWithBoundTree(t, repo, "PASS", strings.Repeat("f", 40))

	t.Skip("post-push tree-SHA mismatch path (gitops.go:247) is structurally " +
		"unreachable: ff-merge preserves tree SHA, so pre-merge binding == " +
		"post-push committed tree by construction.")
}

func TestPostShip_CycleClass_RepinError_PropagatesFromPostShip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "bin-content\n")
	evolveDir := filepath.Join(root, ".evolve")
	mustWrite(t, filepath.Join(evolveDir, "cycle-state.json"), `{"cycle_id":33}`)
	mustWrite(t, filepath.Join(evolveDir, "state.json"), `{"lastCycleNumber":32}`)

	sha, _ := sha256File(bin)
	_ = sha

	t.Skip("postShip repinPostCycle error propagation (postship.go:33) requires " +
		"state.json to be writable for advance but fail for repin — not achievable " +
		"without a transactional seam. Covered by TestRepinPostCycle_StateReadError " +
		"unit test; postShip:33 documents the wiring.")
}

func argsContain(args []string, flags ...string) bool {
	for _, f := range flags {
		found := false
		for _, a := range args {
			if a == f {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func manualConfirmRunner(
	statFn func(stdout, stderr io.Writer) (int, error),
	diffFn func(stdout, stderr io.Writer) (int, error),
) CmdRunner {
	return func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name != "git" {
			return 0, nil
		}
		switch {
		case argsContain(args, "--quiet"):
			return 1, nil
		case argsContain(args, "--stat"):
			return statFn(stdout, stderr)
		case argsContain(args, "--cached"):
			return diffFn(stdout, stderr)
		default:
			return 0, nil
		}
	}
}

func TestVerifyManualConfirm_DiffStatRunnerError_Propagates(t *testing.T) {
	t.Parallel()
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner: manualConfirmRunner(
			func(_, _ io.Writer) (int, error) { return -1, errors.New("diff stat exploded") },
			func(_, _ io.Writer) (int, error) { return 0, nil },
		),
		Stderr: io.Discard,
		Stdin:  strings.NewReader(""),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("diff stat error must propagate")
	}
	if !strings.Contains(err.Error(), "diff stat") {
		t.Errorf("error should mention diff stat; got %q", err.Error())
	}
}

func TestVerifyManualConfirm_DiffRunnerError_Propagates(t *testing.T) {
	t.Parallel()
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner: manualConfirmRunner(
			func(_, _ io.Writer) (int, error) { return 0, nil },
			func(_, _ io.Writer) (int, error) { return -1, errors.New("full diff exploded") },
		),
		Stderr: io.Discard,
		Stdin:  strings.NewReader(""),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("diff runner error must propagate")
	}
	if !strings.Contains(err.Error(), "diff:") {
		t.Errorf("error should mention diff:; got %q", err.Error())
	}
}

func TestVerifyManualConfirm_LongDiff_Truncated(t *testing.T) {
	t.Parallel()
	var diffLines []string
	for i := 0; i < 90; i++ {
		diffLines = append(diffLines, "+line "+string(rune('a'+i%26)))
	}
	longDiff := strings.Join(diffLines, "\n")

	var stderrBuf strings.Builder
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner: manualConfirmRunner(
			func(_, _ io.Writer) (int, error) { return 0, nil },
			func(out, _ io.Writer) (int, error) {
				_, _ = out.Write([]byte(longDiff))
				return 0, nil
			},
		),
		Stderr: &stderrBuf,
		Stdin:  strings.NewReader(""),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeManualNotTTY, core.ShipClassConfig, "")
	if !strings.Contains(stderrBuf.String(), "diff truncated") {
		t.Errorf("truncation notice missing from stderr; got %q", stderrBuf.String())
	}
}

func TestVerifyAuditBinding_EGPSGateReadError_PropagatesFromBinding(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	acsPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "acs-verdict.json")
	if err := os.Remove(acsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(acsPath, 0o755); err != nil {
		t.Fatalf("mkdir acs-verdict: %v", err)
	}

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("EGPS gate read error must propagate from verifyAuditBinding")
	}
	if _, ok := err.(*IntegrityError); ok {
		t.Errorf("EGPS read error should be plain error, not IntegrityError; got %v", err)
	}
}
