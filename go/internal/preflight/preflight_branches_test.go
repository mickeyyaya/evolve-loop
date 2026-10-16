package preflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProbe_DefaultSeams(t *testing.T) {
	root := t.TempDir()
	p := Probe(Options{ProjectRoot: root})
	if p.SchemaVersion != 3 {
		t.Errorf("schema = %d, want 3", p.SchemaVersion)
	}
	if p.ProbedAt == "" {
		t.Error("default Now seam must stamp a probed_at timestamp")
	}
	switch p.Host.OS {
	case "darwin", "linux", "other":
	default:
		t.Errorf("default OSType mapping produced unexpected host OS %q", p.Host.OS)
	}
	if p.Host.Shell == "" {
		t.Error("shell should fall back to a basename, not empty")
	}
}

func TestProbe_DefaultProjectRootFromGetwd(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	p := Probe(Options{
		OSType:   "darwin",
		Env:      stubEnv(map[string]string{"HOME": dir}),
		LookPath: stubLookPath(nil),
		Now:      fixedNow(),
		IsNested: func() bool { return false },
	})
	wantPrefix, _ := filepath.EvalSymlinks(dir)
	gotDir, _ := filepath.EvalSymlinks(p.Filesystem.StateDir)
	if !strings.HasPrefix(gotDir, wantPrefix) {
		t.Errorf("state dir %q should be anchored under cwd %q", gotDir, wantPrefix)
	}
}

func TestProbe_DefaultIsNested_FromClaudecodeEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": root, "CLAUDECODE": "1"}),
		LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
		Now:         fixedNow(),
	})
	if !p.ClaudeCode.Nested {
		t.Error("CLAUDECODE=1 (non-host) should be detected as nested via the default heuristic")
	}
	if p.ClaudeCode.ClaudecodeEnv == nil || *p.ClaudeCode.ClaudecodeEnv != "1" {
		t.Errorf("claudecode_env pointer should be set to \"1\", got %#v", p.ClaudeCode.ClaudecodeEnv)
	}
	if p.AutoConfig.SandboxFallbackOnEPERM != "1" {
		t.Errorf("nested should set EPERM fallback to 1, got %s", p.AutoConfig.SandboxFallbackOnEPERM)
	}
}

func TestProbe_DefaultIsNested_HostTypeNotNested(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env: stubEnv(map[string]string{
			"HOME": root, "CLAUDECODE": "1", "CLAUDECODE_TYPE": "host-session",
		}),
		LookPath: stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
		Now:      fixedNow(),
	})
	if p.ClaudeCode.Nested {
		t.Error("CLAUDECODE_TYPE=host-session must NOT be treated as nested")
	}
}

func TestSelectWorktreeBase_OverrideNotWritableFallsThrough(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	p := Probe(Options{
		ProjectRoot:  root,
		OSType:       "darwin",
		WorktreeBase: blocker,
		Env:          stubEnv(map[string]string{"HOME": root}),
		LookPath:     stubLookPath(nil),
		Now:          fixedNow(),
		IsNested:     func() bool { return false },
	})
	if strings.Contains(p.AutoConfig.WorktreeBaseReason, "operator-provided") {
		t.Errorf("unwritable override must NOT win; reason was %q", p.AutoConfig.WorktreeBaseReason)
	}
	if !strings.Contains(p.AutoConfig.WorktreeBaseReason, "in-project") {
		t.Errorf("should fall through to in-project, got reason %q", p.AutoConfig.WorktreeBaseReason)
	}
}

func TestSelectWorktreeBase_CacheDirWhenNestedNoTmpdir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": home}),
		LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
		Now:         fixedNow(),
		IsNested:    func() bool { return true },
	})
	if !strings.Contains(p.AutoConfig.WorktreeBaseReason, "cache dir") {
		t.Errorf("nested + no TMPDIR + writable cache should pick the cache dir, got %q",
			p.AutoConfig.WorktreeBaseReason)
	}
	if !strings.HasPrefix(p.AutoConfig.WorktreeBase, home) {
		t.Errorf("cache-dir base should live under HOME, got %q", p.AutoConfig.WorktreeBase)
	}
}

func TestSelectWorktreeBase_LastResortInProjectWhenNested(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{}),
		LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
		Now:         fixedNow(),
		IsNested:    func() bool { return true },
	})
	if !strings.Contains(p.AutoConfig.WorktreeBaseReason, "isolation degraded") {
		t.Errorf("nested with no TMPDIR/cache should fall to degraded in-project, got %q",
			p.AutoConfig.WorktreeBaseReason)
	}
	if p.AutoConfig.WorktreeBase != filepath.Join(root, ".evolve", "worktrees") {
		t.Errorf("last-resort base should be the in-project worktrees dir, got %q", p.AutoConfig.WorktreeBase)
	}
}

func TestProfile_PrettyJSON(t *testing.T) {
	t.Parallel()
	p := Probe(Options{
		ProjectRoot: t.TempDir(),
		OSType:      "linux",
		Env:         stubEnv(map[string]string{"HOME": "/home/test"}),
		LookPath:    stubLookPath(nil),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	out := p.PrettyJSON()
	if !strings.Contains(out, "\n  \"schema_version\": 3") {
		t.Errorf("PrettyJSON must be 2-space indented with schema_version, got:\n%s", out)
	}
}

func TestProfile_Summary_NoWorktreeBaseShowsNone(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp("", "preflight-none-*")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()
	p := Probe(Options{
		ProjectRoot: f.Name(),
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": f.Name(), "TMPDIR": f.Name()}),
		LookPath:    stubLookPath(nil),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	if p.AutoConfig.WorktreeBase != "" {
		t.Fatalf("precondition: expected empty worktree base, got %q", p.AutoConfig.WorktreeBase)
	}
	if !strings.Contains(p.Summary(), "worktree_base=<NONE>") {
		t.Errorf("empty worktree base must render as <NONE> in the summary:\n%s", p.Summary())
	}
}

func TestProfile_WriteToFile_MkdirError(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp("", "preflight-write-*")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()
	p := Profile{SchemaVersion: 3}
	if err := p.WriteToFile(f.Name()); err == nil {
		t.Fatal("expected a mkdir error when the project root is a regular file")
	}
}

func TestProfile_WriteToFile_RenameError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "environment.json"), 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	p := Profile{SchemaVersion: 3}
	if err := p.WriteToFile(root); err == nil {
		t.Fatal("expected a rename error when the destination is a directory")
	}
}
