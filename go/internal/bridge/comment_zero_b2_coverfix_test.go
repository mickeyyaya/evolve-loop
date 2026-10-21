package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func TestLoadShadowObservers_SkipsRuleWhosePatternDoesNotCompile(t *testing.T) {
	root := t.TempDir()
	dir := interactionRulesDir(root)
	if _, err := interaction.PromoteRule(dir, "Do you want to overwrite the existing\\?", "n,Enter", "t", healthyCorpus); err != nil {
		t.Fatalf("PromoteRule: %v", err)
	}
	orig := compileShadowPattern
	compileShadowPattern = func(string) (*regexp.Regexp, error) { return nil, errors.New("boom") }
	defer func() { compileShadowPattern = orig }()

	if got := loadShadowObservers(root); len(got) != 0 {
		t.Fatalf("a shadow rule whose pattern fails to compile must not observe: %+v", got)
	}
}

func TestInjectEnvelope_SystemRuleIdle_PastesUnderRulesHeadingAndReturnsCorrID(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	deps := covDeps()
	deps.Tmux = &fakeTmux{paneSeq: []string{"❯"}}
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	corr := injectEnvelope(context.Background(), injectCfg(ws), deps, lp, inbox.Envelope{Kind: inbox.KindSystemRule, Body: "be brief", CorrID: "c-1"})

	body, err := os.ReadFile(scratchPath(ws))
	if err != nil {
		t.Fatalf("system rule was not pasted: %v", err)
	}
	if string(body) != "## Rules\nbe brief" {
		t.Errorf("pasted body = %q, want the rules heading before the rule", string(body))
	}
	if corr != "c-1" {
		t.Errorf("correlation id = %q, want c-1", corr)
	}
}

func TestInjectEnvelope_DeferredRequeueFailureWarnsAndReturnsNoCorrID(t *testing.T) {
	t.Parallel()
	notADir := filepath.Join(t.TempDir(), "workspace-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var errb bytes.Buffer
	deps := covDeps()
	deps.Stderr = &errb
	deps.Tmux = &fakeTmux{paneSeq: []string{"thinking..."}}
	lp := tmuxLaunch{name: "claude-tmux", session: "s", promptMarker: "❯"}

	corr := injectEnvelope(context.Background(), injectCfg(notADir), deps, lp, inbox.Envelope{Kind: inbox.KindCommand, Body: "later", CorrID: "c-2"})

	if corr != "" {
		t.Errorf("a deferred envelope returns no correlation id, got %q", corr)
	}
	if !strings.Contains(errb.String(), "WARN re-queue of") {
		t.Errorf("a failed re-queue must warn, stderr=%q", errb.String())
	}
}

func linkedWorktreeFixture(t *testing.T, head string, withCommondir bool) (worktree, common string) {
	t.Helper()
	base := t.TempDir()
	worktree = filepath.Join(base, "wt")
	common = filepath.Join(base, "main.git")
	gitDir := filepath.Join(common, "worktrees", "wt")
	for _, d := range []string{worktree, gitDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(worktree, ".git"): "gitdir: " + gitDir + "\n",
		filepath.Join(gitDir, "HEAD"):   head + "\n",
	}
	if withCommondir {
		files[filepath.Join(gitDir, "commondir")] = "../..\n"
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return worktree, common
}

func TestSandboxGitWritePaths_LinkedWorktreeWithoutCommondirIsAnError(t *testing.T) {
	t.Parallel()
	worktree, _ := linkedWorktreeFixture(t, "ref: refs/heads/feature", false)
	writes, denies, err := sandboxGitWritePaths(worktree, "darwin")
	if err == nil || len(writes) != 0 || len(denies) != 0 {
		t.Fatalf("missing commondir must fail closed: %v %v %v", writes, denies, err)
	}
}

func TestSandboxGitWritePaths_DetachedHeadGrantsNoBranchRefs(t *testing.T) {
	t.Parallel()
	worktree, common := linkedWorktreeFixture(t, "0123456789abcdef0123456789abcdef01234567", true)
	writes, denies, err := sandboxGitWritePaths(worktree, "darwin")
	if err != nil {
		t.Fatalf("detached HEAD is data, not an error: %v", err)
	}
	if len(denies) == 0 {
		t.Fatal("metadata denials must still apply")
	}
	if writes[0] != filepath.Join(common, "objects") {
		t.Errorf("first grant = %q, want the shared object store", writes[0])
	}
	for _, w := range writes {
		if strings.Contains(w, filepath.Join("refs", "heads")) {
			t.Errorf("a detached HEAD must not be granted branch refs: %q", w)
		}
	}
}

func TestSandboxGitWritePaths_RefusesBranchThatEscapesOrGlobs(t *testing.T) {
	t.Parallel()
	for _, branch := range []string{"../evil", "a*b"} {
		worktree, _ := linkedWorktreeFixture(t, "ref: refs/heads/"+branch, true)
		writes, denies, err := sandboxGitWritePaths(worktree, "darwin")
		if err == nil || len(writes) != 0 || len(denies) != 0 {
			t.Errorf("branch %q must be refused: %v %v %v", branch, writes, denies, err)
		}
	}
}

func TestSandboxGitWritePaths_NamedBranchGrantsItsRefAndLock(t *testing.T) {
	t.Parallel()
	worktree, common := linkedWorktreeFixture(t, "ref: refs/heads/feature", true)
	writes, _, err := sandboxGitWritePaths(worktree, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	ref := filepath.Join(common, "refs/heads", "feature")
	for _, want := range []string{ref, ref + ".lock"} {
		found := false
		for _, w := range writes {
			found = found || w == want
		}
		if !found {
			t.Errorf("grant %q missing from %v", want, writes)
		}
	}
}

func TestSandboxGitWritePaths_ALinkedWorktreePaneMayTakeTheHostVerificationLock(t *testing.T) {
	t.Parallel()
	worktree, common := linkedWorktreeFixture(t, "ref: refs/heads/feature", true)
	writes, _, err := sandboxGitWritePaths(worktree, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	lock, hub := verifylock.LockPath(worktree)
	if !hub || lock != filepath.Join(common, "evolve-verify.lock") || !slices.Contains(writes, lock) {
		t.Errorf("verifylock.LockPath = %q (hub %t), grants = %v; want <common>/evolve-verify.lock granted", lock, hub, writes)
	}
}
