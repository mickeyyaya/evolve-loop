package bridge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestWithProtectedWrites_EnforceDeniesEachManifestFragmentUnderEachBase(t *testing.T) {
	t.Parallel()
	wt, root := realTempDir(t), realTempDir(t)
	cfg := Deps{GuaranteeStage: "enforce"}.withProtectedWrites(sandbox.Config{}, wt, root)
	if !cfg.ProtectedWriteEnforce {
		t.Fatalf("enforce must set the deny: %+v", cfg)
	}
	for _, base := range []string{wt, root} {
		if !slices.Contains(cfg.ProtectedWriteSubpaths, filepath.Join(base, "go", "internal", "guards")) {
			t.Fatalf("the guards directory under %s must be a protected subpath: %q", base, cfg.ProtectedWriteSubpaths)
		}
		for _, file := range []string{"go/internal/phases/audit/probe_quarantine", ".claude/settings.json"} {
			if !slices.Contains(cfg.ProtectedWritePrefixes, filepath.Join(base, file)) {
				t.Fatalf("%s under %s must be a protected prefix: %q", file, base, cfg.ProtectedWritePrefixes)
			}
		}
	}
	if n := len(cfg.ProtectedWriteSubpaths) + len(cfg.ProtectedWritePrefixes); n != 2*len(guards.ProtectedSurfaceManifest) {
		t.Fatalf("each manifest fragment gives one rule for each base: got %d, want %d", n, 2*len(guards.ProtectedSurfaceManifest))
	}
}

func TestWithProtectedWrites_ShadowKeepsTheRulesAsReportsAndOffDropsThem(t *testing.T) {
	t.Parallel()
	wt := realTempDir(t)
	root := realTempDir(t)
	shadow := Deps{GuaranteeStage: "shadow"}.withProtectedWrites(sandbox.Config{}, wt, root)
	if shadow.ProtectedWriteEnforce || !slices.Contains(shadow.ProtectedWriteSubpaths, filepath.Join(wt, "go", "internal", "guards")) {
		t.Fatalf("shadow must report each protected write of the worktree and deny none: %+v", shadow)
	}
	if slices.ContainsFunc(append(shadow.ProtectedWriteSubpaths, shadow.ProtectedWritePrefixes...), func(p string) bool { return strings.HasPrefix(p, root) }) {
		t.Fatalf("shadow must name no path of the read-only root, its report allow would reopen the root deny: %+v", shadow)
	}
	off := Deps{GuaranteeStage: "off"}.withProtectedWrites(sandbox.Config{}, wt, root)
	if len(off.ProtectedWriteSubpaths)+len(off.ProtectedWritePrefixes) != 0 {
		t.Fatalf("off must emit no protected rule: %+v", off)
	}
}

func TestSandboxWrap_EnforceDeniesTheProtectedPathsOfTheWorktree(t *testing.T) {
	t.Parallel()
	wt := realTempDir(t)
	prefix, ok := defaultSandboxWrapWithProbe(Deps{GuaranteeStage: "enforce", ExecStage: "off", SandboxMode: "on"}, fakeProbe("darwin", true))(SandboxWrapRequest{
		Phase: "build", CLI: "agy-tmux", Worktree: wt, Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if !ok {
		t.Fatal("the wrap was refused")
	}
	sbpl, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatal(err)
	}
	if want := `(deny file-write* (subpath "` + filepath.Join(wt, "go", "internal", "guards") + `"))`; !strings.Contains(string(sbpl), want) {
		t.Fatalf("an agy cell must get the protected write deny %s:\n%s", want, sbpl)
	}
}

func TestSandboxWrap_TheProtectedRulesNameTheRealPathsOfASymlinkedWorktreeAndTheRoot(t *testing.T) {
	t.Parallel()
	realWT, root := realTempDir(t), realTempDir(t)
	link := filepath.Join(realTempDir(t), "wt")
	if err := os.Symlink(realWT, link); err != nil {
		t.Fatal(err)
	}
	deps := Deps{GuaranteeStage: "enforce", ExecStage: "off", SandboxMode: "on"}
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))(SandboxWrapRequest{
		Phase: "build", CLI: "agy-tmux", Worktree: link, Workspace: t.TempDir(), RepoRoot: root,
	})
	if !ok {
		t.Fatal("the wrap was refused")
	}
	sbpl, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{realWT, root} {
		if want := `(deny file-write* (subpath "` + filepath.Join(base, "go", "internal", "guards") + `"))`; !strings.Contains(string(sbpl), want) {
			t.Fatalf("the sandbox matches the real path, so the rule must name it: %s\n%s", want, sbpl)
		}
	}
}

func TestOwnWorktree_IsTheFirstRealWritePathOnlyWhenTheCellHasAWorktree(t *testing.T) {
	t.Parallel()
	req := SandboxWrapRequest{Worktree: "/wt", Workspace: "/ws"}
	if got := ownWorktree(req, sandboxWritePaths(req)); got != "/wt" {
		t.Fatalf("sandboxWritePaths must put the worktree first, ownWorktree reads it by index: got %q", got)
	}
	if got := ownWorktree(SandboxWrapRequest{}, []string{"/ws"}); got != "" {
		t.Fatalf("a cell with no worktree has no worktree base, got %q", got)
	}
	if got := ownWorktree(SandboxWrapRequest{Worktree: "/link"}, []string{"/real", "/ws"}); got != "/real" {
		t.Fatalf("ownWorktree = %q, want the real worktree /real", got)
	}
}

func TestGuaranteeStage_ReadsIsolationI7OfThePolicy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"isolation":{"i7":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := Deps{Env: map[string]string{"EVOLVE_PROJECT_ROOT": root}}
	if got, exec := deps.guaranteeStage(), deps.execStage(); got != "enforce" || exec != "shadow" {
		t.Fatalf("guaranteeStage = %q, execStage = %q; want enforce from isolation.i7 and shadow for the unset i3", got, exec)
	}
}
