package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type llmBinaries struct {
	lookPath          func(string) (string, error)
	claudeReal, agyAt string
}

func installLLMBinaries(t *testing.T) llmBinaries {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := llmBinaries{claudeReal: filepath.Join(dir, "versions", "claude-2.1"), agyAt: filepath.Join(dir, "bin", "agy")}
	link := filepath.Join(dir, "bin", "claude")
	for _, p := range []string{b.claudeReal, b.agyAt} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(b.claudeReal, link); err != nil {
		t.Fatal(err)
	}
	found := map[string]string{"claude": link, "agy": b.agyAt, "ollama": filepath.Join(dir, "bin", "gone")}
	b.lookPath = func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", errors.New("not on PATH")
	}
	return b
}

func wrapForExecDeny(t *testing.T, deps Deps, goos, cli string) string {
	t.Helper()
	deps.Env = map[string]string{"EVOLVE_SANDBOX": "on"}
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe(goos, true))(SandboxWrapRequest{
		Phase: "build", CLI: cli, Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if !ok {
		t.Fatalf("%s: the wrap was refused", goos)
	}
	if goos == "linux" {
		return strings.Join(prefix, " ")
	}
	sbpl, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatal(err)
	}
	return string(sbpl)
}

func TestSandboxWrap_EnforceDeniesTheRealPathOfEachManifestBinaryOnMacOS(t *testing.T) {
	t.Parallel()
	b := installLLMBinaries(t)
	sbpl := wrapForExecDeny(t, Deps{ExecStage: "enforce", LookPath: b.lookPath}, "darwin", "claude-tmux")
	for _, p := range []string{b.claudeReal, b.agyAt} {
		if !strings.Contains(sbpl, `(deny process-exec (require-all (literal "`+p+`")`) {
			t.Errorf("the profile must deny the exec of %s; a literal matches the real path only:\n%s", p, sbpl)
		}
	}
	if strings.Contains(sbpl, "gone") || strings.Contains(sbpl, `literal "`+filepath.Dir(b.agyAt)+`/claude"`) {
		t.Fatalf("a binary that does not resolve, and the symlink path, get no rule:\n%s", sbpl)
	}
}

func TestSandboxWrap_EnforceMasksEachOtherFamilyBinaryOnLinux(t *testing.T) {
	t.Parallel()
	b := installLLMBinaries(t)
	argv := wrapForExecDeny(t, Deps{ExecStage: "enforce", LookPath: b.lookPath}, "linux", "claude-tmux")
	if !strings.Contains(argv, "--ro-bind /dev/null "+b.agyAt) {
		t.Errorf("bwrap must mask the agy binary for a claude cell: %s", argv)
	}
	if strings.Contains(argv, b.claudeReal) {
		t.Errorf("bwrap cannot mask the binary that it launches: %s", argv)
	}
}

func TestSandboxWrap_ShadowReportsTheExecAndTheDefaultStageIsShadow(t *testing.T) {
	b := installLLMBinaries(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	sbpl := wrapForExecDeny(t, Deps{LookPath: b.lookPath}, "darwin", "codex")
	if !strings.Contains(sbpl, `(allow process-exec (require-all (literal "`+b.claudeReal+`")`) || strings.Contains(sbpl, "(deny process-exec") {
		t.Fatalf("with no stage the rule must report the exec and deny nothing:\n%s", sbpl)
	}
	if argv := wrapForExecDeny(t, Deps{LookPath: b.lookPath}, "linux", "codex"); strings.Contains(argv, "/dev/null "+b.agyAt) {
		t.Fatalf("shadow masks nothing on Linux: %s", argv)
	}
}

func TestSandboxWrap_OffStageGivesNoExecRule(t *testing.T) {
	t.Parallel()
	b := installLLMBinaries(t)
	if sbpl := wrapForExecDeny(t, Deps{ExecStage: "off", LookPath: b.lookPath}, "darwin", "codex"); strings.Contains(sbpl, b.claudeReal) {
		t.Fatalf("off must emit no exec rule:\n%s", sbpl)
	}
}

func TestExecStage_ReadsIsolationI3OfThePolicy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"isolation":{"i3":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := Deps{Env: map[string]string{"EVOLVE_PROJECT_ROOT": root}}
	if got, env := deps.execStage(), deps.envStage(); got != "enforce" || env != "shadow" {
		t.Fatalf("execStage = %q, envStage = %q; want enforce from isolation.i3 and shadow for the unset i2", got, env)
	}
}

func TestLLMBinaryPaths_UsesTheProcessPathWhenNoLookPathIsInjected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	real, err := filepath.EvalSymlinks(filepath.Join(dir, "codex"))
	if err != nil {
		t.Fatal(err)
	}
	if got := (Deps{}).llmBinaryPaths(""); !slices.Equal(got, []string{real}) {
		t.Fatalf("llmBinaryPaths = %v, want [%s]", got, real)
	}
}
