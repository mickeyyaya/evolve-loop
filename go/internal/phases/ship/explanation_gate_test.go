package ship

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func TestPhaseRun_MissingRequiredExplanationBlocksBeforeNativeShip(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: t.TempDir(), Workspace: workspace,
		BaseSHA: strings.Repeat("a", 40), Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	nativeCalled := false
	p := New(Config{Runner: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		nativeCalled = true
		return 0, nil
	}})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 42, RunID: "run-42", ProjectRoot: root, Worktree: binding.Worktree, Workspace: workspace,
		WorktreeBaseSHA: binding.BaseSHA, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	})
	if err == nil || !strings.Contains(err.Error(), "ship explanation documentation gate") {
		t.Fatalf("missing explanation error=%v", err)
	}
	if resp.Verdict != core.VerdictFAIL || nativeCalled {
		t.Fatalf("gate response=%+v nativeCalled=%v", resp, nativeCalled)
	}
}

func TestPhaseRun_UnreadableActiveMarkerCannotBypassExplanationGate(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, ".evolve", "build-explanation-contracts", "cycle-42.json")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	nativeCalled := false
	p := New(Config{Runner: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		nativeCalled = true
		return 0, nil
	}})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 42, RunID: "run-42", ProjectRoot: root, Worktree: t.TempDir(), Workspace: workspace,
		WorktreeBaseSHA: strings.Repeat("a", 40), ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	})
	if err == nil || !strings.Contains(err.Error(), "activation marker") || nativeCalled || resp.Verdict != core.VerdictFAIL {
		t.Fatalf("marker bypass: response=%+v err=%v nativeCalled=%v", resp, err, nativeCalled)
	}
}

func TestNativeRun_EnforcesExplanationGateWithoutPhaseWrapper(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	worktree := t.TempDir()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: worktree, Workspace: workspace,
		BaseSHA: strings.Repeat("a", 40), Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	// The workspace mirror is Builder-writable and deliberately lies about
	// every activation field. Native Ship must use the explicit host identity
	// carried by Options, not this file.
	runState := `{"cycle_id":999,"run_id":"attacker","explanation_documentation_version":0}`
	if err := os.WriteFile(filepath.Join(workspace, core.RunStateFile), []byte(runState), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Run(context.Background(), Options{
		Class: ClassCycle, CommitMessage: "must not ship", ProjectRoot: root,
		WorkspacePath: workspace, Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard,
		CycleID: 42, ActiveWorktree: worktree, WorktreeBaseSHA: binding.BaseSHA,
		RunID: "run-42", ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	})
	if err == nil || !strings.Contains(err.Error(), "explanation documentation") {
		t.Fatalf("native Run bypassed explanation gate: result=%+v err=%v", res, err)
	}
	if shipErr, ok := core.AsShipError(err); !ok || shipErr.Code != core.CodeExplanationDocumentation || shipErr.Stage != core.StageVerifyExplanation {
		t.Fatalf("explanation failure lost structured ship identity: ok=%v err=%+v", ok, shipErr)
	}
}

func TestVerifyNativeExplanation_IncompleteTypedIdentityCannotHideActiveMarker(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Workspace: workspace, Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatal(err)
	}
	err := verifyNativeExplanation(context.Background(), &Options{
		Class: ClassCycle, ProjectRoot: root, WorkspacePath: workspace,
		// Deliberately omit CycleID, RunID, and contract version.
	})
	if err == nil || !strings.Contains(err.Error(), "host activation") {
		t.Fatalf("incomplete typed identity hid active marker: %v", err)
	}
}

func TestVerifyNativeExplanation_LegacyNoMarkerIgnoresSyntheticWorkspaceCycle(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	err := verifyNativeExplanation(context.Background(), &Options{
		Class: ClassCycle, ProjectRoot: root, WorkspacePath: workspace, CycleID: 42,
		// Version zero and no host marker are the explicit legacy contract.
	})
	if err != nil {
		t.Fatalf("legacy no-marker ship was rejected by synthetic workspace identity: %v", err)
	}
}

func TestVerifyNativeExplanation_FreezesStandaloneHostIdentity(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	worktree := t.TempDir()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", worktree}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", ".gitignore"}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", append([]string{"-C", worktree}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cmd := exec.Command("git", "-C", worktree, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(out))
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: worktree, Workspace: workspace, BaseSHA: base,
		Cycle: 42, RunID: "run-42", ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "build-report.md"), []byte("## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: no material implementation diff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, ".evolve", "cycle-state.json")
	state := fmt.Sprintf(`{"cycle_id":42,"run_id":"run-42","active_worktree":%q,"workspace_path":%q,"worktree_base_sha":"%s","explanation_documentation_version":%d}`,
		worktree, workspace, base, explanationdocs.CurrentContractVersion)
	if err := os.WriteFile(statePath, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := &Options{Class: ClassCycle, ProjectRoot: root}
	if err := verifyNativeExplanation(context.Background(), opts); err != nil {
		t.Fatalf("verifyNativeExplanation: %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`{"active_worktree":"/attacker/tree"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, typed, err := activeWorktreeForShip(opts)
	if err != nil || !typed || got != worktree {
		t.Fatalf("standalone identity was re-read after verification: worktree=%q typed=%v err=%v opts=%+v", got, typed, err, opts)
	}
}

func TestNativeRun_PostPushExplanationRetryUsesLandedTreeAfterWorktreeCleanup(t *testing.T) {
	repo := makeRepo(t)
	runGit(t, repo, "branch", "-M", "main")
	worktree := makeWorktree(t, repo, "cycle-42-branch")
	workspace := filepath.Join(repo, ".evolve", "runs", "cycle-42")
	mustMkdir(t, workspace)
	base := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	binding := explanationdocs.CycleBinding{
		ProjectRoot: repo, Worktree: worktree, Workspace: workspace, BaseSHA: base,
		Cycle: 42, RunID: "run-42", ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}

	document, err := explanationdocs.DocumentPath(42, "run-42")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(worktree, "fixture.txt"), "fixture line 1\nlanded explanation change\n")
	docBody := "# Build Explanation — Cycle 42\n\n" +
		"## Build Binding\n- Cycle: 42\n- Base SHA: " + base + "\n\n" +
		"## Summary\nThe fixture now records the landed explanation retry behavior.\n\n" +
		"## Rationale\nThe existing tracked fixture gives the retry test a minimal material change with no extra production surface.\n\n" +
		"## Changed Areas\n- `fixture.txt` — adds material content used to verify the landed cycle diff.\n\n" +
		"## Design Decisions\nThe test uses the real linked-worktree lifecycle and one host-owned snapshot.\n\n" +
		"## Verification\nThe native Ship retry must return the existing commit without another mutation.\n\n" +
		"## Compatibility\nNo public interface changes.\n\n" +
		"## Limitations\nCovers the post-push cleanup path only.\n"
	mustWrite(t, filepath.Join(worktree, filepath.FromSlash(document)), docBody)
	mustWrite(t, filepath.Join(workspace, "build-report.md"), "## Explanation Documentation\n- Status: REQUIRED\n- Document: "+document+"\n")
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	view, err := explanationdocs.Load(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}

	runGit(t, worktree, "add", "-A")
	runGit(t, worktree, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle 42 landed build")
	runGit(t, repo, "merge", "--ff-only", "cycle-42-branch")
	commit := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD^{tree}"))
	mustWrite(t, filepath.Join(workspace, "ship-binding.json"), fmt.Sprintf(
		`{"audit_bound_tree_sha":%q,"tree_sha_committed":%q,"commit_sha":%q,"cycle":42}`,
		tree, tree, commit,
	))
	runGit(t, repo, "worktree", "remove", "--force", worktree)
	mustWrite(t, filepath.Join(repo, "local-operator-notes.txt"), "unrelated untracked main-side state\n")

	result, err := runShip(t, repo, Options{
		Class: ClassCycle, CommitMessage: "cycle 42 retry", CycleID: 42,
		ActiveWorktree: worktree, WorktreeBaseSHA: base, WorkspacePath: workspace,
		RunID: "run-42", ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanation: view, RequireBuildExplanationHandoff: true,
	})
	if err != nil || result.ExitCode != ExitOK || result.CommitSHA != commit || !containsLog(result, "succeeding report-only") {
		t.Fatalf("post-push explanation retry result=%+v err=%v, want report-only commit %s", result, err, commit)
	}
}

func TestPostPushBinding_UsesTypedCycleAndRequiresExactTreeWitness(t *testing.T) {
	repo := makeRepo(t)
	runGit(t, repo, "branch", "-M", "main")
	workspace := filepath.Join(repo, ".evolve", "runs", "cycle-42")
	mustMkdir(t, workspace)
	mustWrite(t, filepath.Join(workspace, core.RunStateFile), `{"cycle_id":999}`)
	head := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD^{tree}"))
	opts := &Options{
		Class: ClassCycle, ProjectRoot: repo, WorkspacePath: workspace, CycleID: 42,
		Runner: execRunner, Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard,
	}
	bindingPath := filepath.Join(workspace, "ship-binding.json")

	writeBinding := func(cycle int, auditTree, committedTree, commit string) {
		t.Helper()
		mustWrite(t, bindingPath, fmt.Sprintf(
			`{"audit_bound_tree_sha":%q,"tree_sha_committed":%q,"commit_sha":%q,"cycle":%d}`,
			auditTree, committedTree, commit, cycle,
		))
	}
	writeBinding(42, tree, tree, head)
	if _, ok, err := checkPostPushIdempotency(context.Background(), opts); err != nil || !ok {
		t.Fatalf("exact typed binding not recognized: ok=%v err=%v", ok, err)
	}
	for name, mutate := range map[string]func(){
		"wrong cycle":   func() { writeBinding(41, tree, tree, head) },
		"missing audit": func() { writeBinding(42, "", tree, head) },
		"wrong audit":   func() { writeBinding(42, strings.Repeat("a", 40), tree, head) },
		"wrong tree":    func() { writeBinding(42, tree, strings.Repeat("b", 40), head) },
		"wrong commit":  func() { writeBinding(42, tree, tree, strings.Repeat("c", 40)) },
	} {
		t.Run(name, func(t *testing.T) {
			mutate()
			if _, ok, err := checkPostPushIdempotency(context.Background(), opts); err != nil || ok {
				t.Fatalf("forged binding accepted: ok=%v err=%v", ok, err)
			}
		})
	}
}

func TestWriteShipBinding_UsesTypedCycleInsteadOfRunMirror(t *testing.T) {
	repo := makeRepo(t)
	workspace := filepath.Join(repo, ".evolve", "runs", "cycle-42")
	mustMkdir(t, workspace)
	mustWrite(t, filepath.Join(workspace, core.RunStateFile), `{"cycle_id":999}`)
	opts := &Options{ProjectRoot: repo, WorkspacePath: workspace, CycleID: 42, internalAuditBoundTreeSHA: "tree"}
	if err := writeShipBinding(opts, "tree", "commit"); err != nil {
		t.Fatalf("writeShipBinding: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "ship-binding.json")); err != nil {
		t.Fatalf("typed cycle binding missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "runs", "cycle-999", "ship-binding.json")); !os.IsNotExist(err) {
		t.Fatalf("run mirror selected binding target: %v", err)
	}
}
