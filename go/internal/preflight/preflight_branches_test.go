package preflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProbe_DefaultSeams pins the nil-option fallbacks in Probe: with only a
// ProjectRoot supplied, Env→os.Getenv, Now→time.Now, LookPath→exec.LookPath,
// OSType→runtime.GOOS, and IsNested→the $CLAUDECODE heuristic all engage. The
// profile must still be well-formed (schema 3, a probed timestamp, a real
// host OS). Not parallel: it reads ambient process env via the default seams.
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
		// one of the runtime.GOOS-mapped values
	default:
		t.Errorf("default OSType mapping produced unexpected host OS %q", p.Host.OS)
	}
	if p.Host.Shell == "" {
		t.Error("shell should fall back to a basename, not empty")
	}
}

// TestProbe_DefaultProjectRootFromGetwd pins the ProjectRoot=="" branch:
// Probe falls back to os.Getwd, so the state dir is anchored under the cwd.
// Not parallel (mutates cwd via Chdir, restored on cleanup).
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
	// macOS resolves /var → /private/var etc; compare resolved paths.
	wantPrefix, _ := filepath.EvalSymlinks(dir)
	gotDir, _ := filepath.EvalSymlinks(p.Filesystem.StateDir)
	if !strings.HasPrefix(gotDir, wantPrefix) {
		t.Errorf("state dir %q should be anchored under cwd %q", gotDir, wantPrefix)
	}
}

// TestProbe_DefaultIsNested_FromClaudecodeEnv pins the default IsNested
// heuristic (no IsNested override): CLAUDECODE set without a "host" type marks
// the run nested, and the CLAUDECODE pointer is populated on the profile.
func TestProbe_DefaultIsNested_FromClaudecodeEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": root, "CLAUDECODE": "1"}),
		LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
		Now:         fixedNow(),
		// IsNested omitted → exercises the $CLAUDECODE heuristic.
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

// TestProbe_DefaultIsNested_HostTypeNotNested pins the "host" carve-out of the
// default heuristic: CLAUDECODE set but CLAUDECODE_TYPE contains "host" is NOT
// nested (a host-side Claude Code session, not an inner nested one).
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

// TestSelectWorktreeBase_OverrideNotWritableFallsThrough pins the branch where
// EVOLVE_WORKTREE_BASE is set but unwritable: the override is ignored and the
// standalone in-project base is chosen instead.
func TestSelectWorktreeBase_OverrideNotWritableFallsThrough(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Point the override at a regular file → probeWritable fails for it.
	blocker := filepath.Join(root, "blocker-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": root, "EVOLVE_WORKTREE_BASE": blocker}),
		LookPath:    stubLookPath(nil),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	if strings.Contains(p.AutoConfig.WorktreeBaseReason, "operator-provided") {
		t.Errorf("unwritable override must NOT win; reason was %q", p.AutoConfig.WorktreeBaseReason)
	}
	if !strings.Contains(p.AutoConfig.WorktreeBaseReason, "in-project") {
		t.Errorf("should fall through to in-project, got reason %q", p.AutoConfig.WorktreeBaseReason)
	}
}

// TestSelectWorktreeBase_CacheDirWhenNestedNoTmpdir pins worktree-base step 4:
// nested (so the in-project preference is skipped) with no TMPDIR but a
// writable cache dir selects the cache dir.
func TestSelectWorktreeBase_CacheDirWhenNestedNoTmpdir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := t.TempDir() // writable HOME → Library/Caches under it is creatable
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": home}), // no TMPDIR
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

// TestSelectWorktreeBase_LastResortInProjectWhenNested pins step 5: nested with
// neither TMPDIR nor a writable cache dir (HOME unset → cacheDirPath returns "")
// falls back to the in-project worktrees dir with the degraded-isolation reason.
func TestSelectWorktreeBase_LastResortInProjectWhenNested(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{}), // no HOME, no TMPDIR
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

// TestProfile_PrettyJSON pins PrettyJSON: it produces 2-space-indented JSON
// that round-trips and carries the schema version.
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

// TestProfile_Summary_NoWorktreeBaseShowsNone pins the Summary <NONE> branch:
// when no writable worktree base was found, the summary renders worktree_base
// as <NONE> rather than an empty string.
func TestProfile_Summary_NoWorktreeBaseShowsNone(t *testing.T) {
	t.Parallel()
	// Force WorktreeBase="" by making the project root a regular file (every
	// writability probe fails), mirroring TestProbe_NoWritableWorktreeBase.
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

// TestProfile_WriteToFile_MkdirError pins the WriteToFile MkdirAll-error leg:
// when the parent path is a regular file, the .evolve dir can't be created and
// the error surfaces.
func TestProfile_WriteToFile_MkdirError(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp("", "preflight-write-*")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()
	p := Profile{SchemaVersion: 3}
	// projectRoot is a regular file → filepath.Dir(target)=<file>/.evolve →
	// MkdirAll fails with ENOTDIR.
	if err := p.WriteToFile(f.Name()); err == nil {
		t.Fatal("expected a mkdir error when the project root is a regular file")
	}
}

// TestProfile_WriteToFile_RenameError pins the WriteToFile rename-error leg:
// when the destination environment.json already exists as a directory, the
// atomic rename of the temp over it fails.
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
