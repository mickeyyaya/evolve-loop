package preflight

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stubEnv(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func stubLookPath(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
}

func fixedNow() func() time.Time {
	return func() time.Time { return time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC) }
}

func TestProbe_DarwinStandalone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": root, "SHELL": "/bin/zsh"}),
		LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec", "claude": "/usr/local/bin/claude"}),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	if p.SchemaVersion != 3 {
		t.Errorf("schema = %d, want 3", p.SchemaVersion)
	}
	if p.Host.OS != "darwin" || p.Host.Shell != "zsh" {
		t.Errorf("host wrong: %+v", p.Host)
	}
	if p.ClaudeCode.Nested {
		t.Errorf("standalone should not be nested")
	}
	if !p.Sandbox.ExpectedToWork {
		t.Errorf("darwin+sandbox-exec+standalone should work; got %+v", p.Sandbox)
	}
	if p.AutoConfig.SandboxFallbackOnEPERM != "0" {
		t.Errorf("standalone fallback should be 0, got %s", p.AutoConfig.SandboxFallbackOnEPERM)
	}
	if !p.AutoConfig.InnerSandbox {
		t.Errorf("standalone+working sandbox should enable inner_sandbox")
	}
	if p.CLIBinaries.Claude == nil || *p.CLIBinaries.Claude != "/usr/local/bin/claude" {
		t.Errorf("claude path wrong: %+v", p.CLIBinaries.Claude)
	}
}

func TestProbe_DarwinNested(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": root}),
		LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
		Now:         fixedNow(),
		IsNested:    func() bool { return true },
	})
	if p.Sandbox.ExpectedToWork {
		t.Errorf("nested-claude darwin should not expect sandbox to work")
	}
	if !strings.Contains(p.Sandbox.Reason, "EPERM") {
		t.Errorf("missing EPERM reason: %s", p.Sandbox.Reason)
	}
	if p.AutoConfig.SandboxFallbackOnEPERM != "1" {
		t.Errorf("nested should set fallback=1, got %s", p.AutoConfig.SandboxFallbackOnEPERM)
	}
	if p.AutoConfig.InnerSandbox {
		t.Errorf("nested should disable inner_sandbox")
	}
	if !strings.Contains(p.AutoConfig.Reasoning, "nested-Claude detected") {
		t.Errorf("missing nested reasoning: %s", p.AutoConfig.Reasoning)
	}
}

func TestProbe_LinuxWithBwrap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "linux",
		Env:         stubEnv(map[string]string{"HOME": root, "XDG_CACHE_HOME": filepath.Join(root, "xdg-cache")}),
		LookPath:    stubLookPath(map[string]string{"bwrap": "/usr/bin/bwrap"}),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	if !p.Sandbox.ExpectedToWork {
		t.Errorf("linux+bwrap should work; got %+v", p.Sandbox)
	}
	if !p.Sandbox.BwrapAvailable {
		t.Error("bwrap should be available")
	}
	if !p.AutoConfig.InnerSandbox {
		t.Error("linux+bwrap+standalone should enable inner_sandbox")
	}
}

func TestProbe_UnsupportedOS(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "freebsd",
		Env:         stubEnv(map[string]string{"HOME": root}),
		LookPath:    stubLookPath(nil),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	if p.Sandbox.ExpectedToWork {
		t.Errorf("freebsd should not have sandbox support")
	}
	if !strings.Contains(p.Sandbox.Reason, "Unsupported OS") {
		t.Errorf("missing unsupported reason: %s", p.Sandbox.Reason)
	}
	if p.AutoConfig.InnerSandbox {
		t.Error("unsupported OS should disable inner_sandbox")
	}
}

func TestProbe_NoWritableWorktreeBase(t *testing.T) {
	t.Parallel()
	// Use a path that doesn't exist and the user can't create.
	// We can't easily simulate this on a normal filesystem; instead use a
	// path that resolves to a regular file (mkdir fails).
	tmpfile, _ := os.CreateTemp("", "evolve-block-*")
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Pass the file's path as project root — mkdir for .evolve will fail.
	root := tmpfile.Name() // a regular file, not a directory
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		// HOME is also a file → cache dir mkdir fails too
		Env:      stubEnv(map[string]string{"HOME": root, "TMPDIR": tmpfile.Name()}),
		LookPath: stubLookPath(nil),
		Now:      fixedNow(),
		IsNested: func() bool { return false },
	})
	if p.AutoConfig.WorktreeBase != "" {
		t.Errorf("with no writable target, worktree_base should be empty, got %q", p.AutoConfig.WorktreeBase)
	}
	if !strings.Contains(p.AutoConfig.Reasoning, "ERROR: no writable worktree base") {
		t.Errorf("missing ERROR reasoning: %s", p.AutoConfig.Reasoning)
	}
}

func TestProbe_OperatorWorktreeBaseOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	override := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env: stubEnv(map[string]string{
			"HOME": root, "EVOLVE_WORKTREE_BASE": override,
		}),
		LookPath: stubLookPath(nil),
		Now:      fixedNow(),
		IsNested: func() bool { return false },
	})
	if p.AutoConfig.WorktreeBase != override {
		t.Errorf("operator override should win; got %q", p.AutoConfig.WorktreeBase)
	}
	if !strings.Contains(p.AutoConfig.WorktreeBaseReason, "operator-provided") {
		t.Errorf("missing operator-provided reason: %s", p.AutoConfig.WorktreeBaseReason)
	}
}

func TestProbe_NestedPrefersTmpdir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tmp := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": root, "TMPDIR": tmp}),
		LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
		Now:         fixedNow(),
		IsNested:    func() bool { return true },
	})
	if !strings.HasPrefix(p.AutoConfig.WorktreeBase, tmp) {
		t.Errorf("nested should prefer TMPDIR; got %s", p.AutoConfig.WorktreeBase)
	}
	if !strings.Contains(p.AutoConfig.WorktreeBaseReason, "TMPDIR") {
		t.Errorf("missing TMPDIR reason: %s", p.AutoConfig.WorktreeBaseReason)
	}
}

func TestProjectHash8(t *testing.T) {
	t.Parallel()
	h := projectHash8("/Users/x/ai/claude/evolve-loop")
	if len(h) != 8 {
		t.Errorf("hash length = %d, want 8", len(h))
	}
	h2 := projectHash8("/Users/x/ai/claude/evolve-loop")
	if h != h2 {
		t.Errorf("hash not deterministic")
	}
	hDiff := projectHash8("/different/path")
	if h == hDiff {
		t.Errorf("different paths should hash differently")
	}
}

func TestProbeWritable(t *testing.T) {
	t.Parallel()
	tmpdir := t.TempDir()
	if !probeWritable(tmpdir) {
		t.Error("created tmpdir should be writable")
	}
	if probeWritable("") {
		t.Error("empty path should be unwritable")
	}
	// A path that's a regular file (not a dir) should fail
	file, _ := os.CreateTemp("", "probe-block-*")
	defer os.Remove(file.Name())
	file.Close()
	if probeWritable(file.Name()) {
		t.Error("regular file path should fail mkdir-and-touch")
	}
}

func TestProfile_JSONMarshaling(t *testing.T) {
	t.Parallel()
	p := Probe(Options{
		ProjectRoot: t.TempDir(),
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": "/Users/test"}),
		LookPath:    stubLookPath(nil),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"schema_version":3`) {
		t.Errorf("missing schema_version: %s", b)
	}
	// nil pointer fields should serialize as null
	if !strings.Contains(string(b), `"claude":null`) {
		t.Errorf("expected claude:null when not on PATH: %s", b)
	}
}

func TestProfile_Summary(t *testing.T) {
	t.Parallel()
	p := Probe(Options{
		ProjectRoot: t.TempDir(),
		OSType:      "linux",
		Env:         stubEnv(map[string]string{"HOME": "/home/test"}),
		LookPath:    stubLookPath(nil),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	s := p.Summary()
	for _, want := range []string{
		"Environment Profile",
		"Host:",
		"Nested-Claude:    false",
		"Sandbox works:",
		"worktree_base",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in summary:\n%s", want, s)
		}
	}
}

func TestProfile_WriteToFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := Probe(Options{
		ProjectRoot: root,
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": root}),
		LookPath:    stubLookPath(nil),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	if err := p.WriteToFile(root); err != nil {
		t.Fatalf("write: %v", err)
	}
	target := filepath.Join(root, ".evolve", "environment.json")
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), `"schema_version": 3`) {
		t.Errorf("missing schema in written file: %s", body)
	}
}

func TestCacheDirPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		osType, home, xdg, want string
	}{
		{"darwin", "/Users/x", "", "/Users/x/Library/Caches/evolve-loop/hash"},
		{"linux", "/home/x", "", "/home/x/.cache/evolve-loop/hash"},
		{"linux", "/home/x", "/custom/cache", "/custom/cache/evolve-loop/hash"},
		{"other", "/home/x", "", "/home/x/.cache/evolve-loop/hash"},
		{"darwin", "", "", ""},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.osType, func(t *testing.T) {
			t.Parallel()
			env := stubEnv(map[string]string{"HOME": tc.home, "XDG_CACHE_HOME": tc.xdg})
			got := cacheDirPath(tc.osType, env, "hash")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
