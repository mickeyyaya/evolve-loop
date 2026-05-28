package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPretrustCodexProjects covers Fix 1 of the cycle-122 remediation
// (see docs/incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md):
// codex-tmux must pre-trust the worktree + workspace paths in
// ~/.codex/config.toml so codex's own permission layer does not prompt
// "Press enter to confirm" at runtime when the agent shells out a
// command that writes outside the worktree boundary.
//
// Test seam: EVOLVE_CODEX_CONFIG_PATH redirects the merge target to a
// per-test tempdir so the real ~/.codex is never touched.
func TestPretrustCodexProjects(t *testing.T) {
	tests := []struct {
		name           string
		existing       string
		worktree       string
		workspace      string
		wantSubstrings []string
		wantAbsent     []string
		wantUnchanged  bool
	}{
		{
			name:      "AppendsBothPaths_OnEmptyConfig",
			existing:  "",
			worktree:  "/Users/op/proj/.evolve/worktrees/cycle-123",
			workspace: "/Users/op/proj/.evolve/runs/cycle-123",
			wantSubstrings: []string{
				`[projects."/Users/op/proj/.evolve/worktrees/cycle-123"]`,
				`[projects."/Users/op/proj/.evolve/runs/cycle-123"]`,
				`trust_level = "trusted"`,
			},
		},
		{
			name: "IdempotentWhenAlreadyTrusted",
			existing: `model = "gpt-5.5"

[projects."/Users/op/proj/.evolve/worktrees/cycle-123"]
trust_level = "trusted"

[projects."/Users/op/proj/.evolve/runs/cycle-123"]
trust_level = "trusted"
`,
			worktree:      "/Users/op/proj/.evolve/worktrees/cycle-123",
			workspace:     "/Users/op/proj/.evolve/runs/cycle-123",
			wantUnchanged: true,
		},
		{
			name: "PreservesExistingContent_AppendsNewOnly",
			existing: `# Operator-authored comment that must survive
model = "gpt-5.5"
model_reasoning_effort = "medium"

[projects."/Users/op/proj"]
trust_level = "trusted"
`,
			worktree:  "/Users/op/proj/.evolve/worktrees/cycle-99",
			workspace: "/Users/op/proj/.evolve/runs/cycle-99",
			wantSubstrings: []string{
				`# Operator-authored comment that must survive`,
				`model_reasoning_effort = "medium"`,
				`[projects."/Users/op/proj"]`,
				`[projects."/Users/op/proj/.evolve/worktrees/cycle-99"]`,
				`[projects."/Users/op/proj/.evolve/runs/cycle-99"]`,
			},
		},
		{
			name:      "WorktreeOnly_NoWorkspace",
			existing:  "",
			worktree:  "/tmp/wt",
			workspace: "",
			wantSubstrings: []string{
				`[projects."/tmp/wt"]`,
			},
			wantAbsent: []string{
				`[projects.""]`,
			},
		},
		{
			name:      "WorkspaceOnly_NoWorktree",
			existing:  "",
			worktree:  "",
			workspace: "/tmp/ws",
			wantSubstrings: []string{
				`[projects."/tmp/ws"]`,
			},
		},
		{
			name:          "BothEmpty_NoOp",
			existing:      "",
			worktree:      "",
			workspace:     "",
			wantUnchanged: true,
		},
		{
			name:      "DedupWhenWorktreeEqualsWorkspace",
			existing:  "",
			worktree:  "/tmp/same",
			workspace: "/tmp/same",
			wantSubstrings: []string{
				`[projects."/tmp/same"]`,
			},
		},
		{
			name:      "MissingNewlineAtEnd_NormalizesBeforeAppend",
			existing:  `model = "gpt-5.5"`, // no trailing newline
			worktree:  "/tmp/wt",
			workspace: "",
			wantSubstrings: []string{
				`model = "gpt-5.5"`,
				`[projects."/tmp/wt"]`,
			},
		},
		{
			name:      "PathWithQuote_TOMLEscaped",
			existing:  "",
			worktree:  `/tmp/with"quote`,
			workspace: "",
			wantSubstrings: []string{
				`[projects."/tmp/with\"quote"]`,
			},
		},
		{
			// HIGH-2 from cycle-122 review: control characters in a path
			// would corrupt config.toml and prevent codex from starting.
			// All TOML §2.4 prohibited chars must be escaped.
			name:      "PathWithControlChars_AllEscaped",
			existing:  "",
			worktree:  "/tmp/with\nnewline\tand\rcr",
			workspace: "",
			wantSubstrings: []string{
				`[projects."/tmp/with\nnewline\tand\rcr"]`,
			},
			wantAbsent: []string{
				"\n[projects",  // newline must NOT split the header line
				"newline\nand", // raw newline must not leak through
			},
		},
		{
			// HIGH-2 companion: backslash escape happens BEFORE other
			// escapes (Replacer applies all simultaneously, not
			// sequentially); a path containing both \ and " must
			// produce the right byte sequence.
			name:      "PathWithBackslashAndQuote_EscapedOnce",
			existing:  "",
			worktree:  `/tmp/a\b"c`,
			workspace: "",
			wantSubstrings: []string{
				`[projects."/tmp/a\\b\"c"]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			if tt.existing != "" {
				if err := os.WriteFile(path, []byte(tt.existing), 0o600); err != nil {
					t.Fatalf("seed existing: %v", err)
				}
			}
			t.Setenv("EVOLVE_CODEX_CONFIG_PATH", path)

			cfg := &Config{Worktree: tt.worktree, Workspace: tt.workspace}
			if err := pretrustCodexProjects(cfg); err != nil {
				t.Fatalf("pretrustCodexProjects: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) && tt.wantUnchanged && tt.existing == "" {
					return // no-op + no seed = no file is correct
				}
				t.Fatalf("read merged: %v", err)
			}
			gotStr := string(got)

			if tt.wantUnchanged {
				if gotStr != tt.existing {
					t.Fatalf("expected file unchanged\n--- expected ---\n%s\n--- got ---\n%s", tt.existing, gotStr)
				}
				return
			}

			for _, sub := range tt.wantSubstrings {
				if !strings.Contains(gotStr, sub) {
					t.Errorf("missing substring %q in:\n%s", sub, gotStr)
				}
			}
			for _, abs := range tt.wantAbsent {
				if strings.Contains(gotStr, abs) {
					t.Errorf("unexpected substring %q present in:\n%s", abs, gotStr)
				}
			}
			// Defensive: every appended section must be paired with a trust_level line.
			headerCount := strings.Count(gotStr, "[projects.")
			trustCount := strings.Count(gotStr, `trust_level = "trusted"`)
			if trustCount < headerCount {
				t.Errorf("trust_level lines (%d) < project headers (%d)\n%s",
					trustCount, headerCount, gotStr)
			}
			// MEDIUM-2 from cycle-122 review: when worktree==workspace,
			// the section must appear EXACTLY once, not just at-least-once.
			if tt.worktree != "" && tt.worktree == tt.workspace {
				h := codexProjectHeader(tt.worktree)
				if got := strings.Count(gotStr, h); got != 1 {
					t.Errorf("dedup: header %q count = %d, want 1\n%s", h, got, gotStr)
				}
			}
		})
	}
}

// TestPretrustCodexProjects_ConcurrentCallsInSameProcess covers
// MEDIUM-1 from the cycle-122 review: -race must show no data race
// across two goroutines pre-trusting different paths into the same
// config file. (Cross-process race is mitigated separately by the
// PID-suffixed tmp filename — see HIGH-1 fix.)
func TestPretrustCodexProjects_ConcurrentCallsInSameProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("EVOLVE_CODEX_CONFIG_PATH", path)

	cfgs := []*Config{
		{Worktree: "/tmp/wt-A", Workspace: "/tmp/ws-A"},
		{Worktree: "/tmp/wt-B", Workspace: "/tmp/ws-B"},
		{Worktree: "/tmp/wt-C", Workspace: "/tmp/ws-C"},
	}
	done := make(chan error, len(cfgs))
	for _, c := range cfgs {
		c := c
		go func() { done <- pretrustCodexProjects(c) }()
	}
	for range cfgs {
		if err := <-done; err != nil {
			t.Errorf("concurrent pretrustCodexProjects: %v", err)
		}
	}
	// Best-effort: in-process last-writer-wins means SOME entries may
	// be absent from the final file (the docstring documents this).
	// The invariant under test is "no data race + no panic + no
	// half-written file" — proven by -race + the read below succeeding.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after concurrent calls: %v", err)
	}
	// At least the LAST writer's entries must be present (last rename wins).
	if !strings.Contains(string(got), `trust_level = "trusted"`) {
		t.Errorf("expected at least one trust entry; got:\n%s", got)
	}
}

// TestPretrustCodexProjects_CreatesParentDir guards that a fresh host
// without ~/.codex/ gets the directory created (0700) before the merge.
func TestPretrustCodexProjects_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "codex-home", ".codex", "config.toml")
	t.Setenv("EVOLVE_CODEX_CONFIG_PATH", nested)

	cfg := &Config{Worktree: "/tmp/wt"}
	if err := pretrustCodexProjects(cfg); err != nil {
		t.Fatalf("pretrustCodexProjects: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(nested)); err != nil {
		t.Fatalf("expected parent dir created: %v", err)
	}
	got, err := os.ReadFile(nested)
	if err != nil {
		t.Fatalf("read created config: %v", err)
	}
	if !strings.Contains(string(got), `[projects."/tmp/wt"]`) {
		t.Errorf("missing pretrust block in fresh config:\n%s", got)
	}
}

// TestPretrustCodexProjects_NilCfg guards that a nil Config is a no-op
// rather than a nil-deref panic — the helper must be safe to call from
// any code path that has not yet populated the cfg.
func TestPretrustCodexProjects_NilCfg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("EVOLVE_CODEX_CONFIG_PATH", path)
	if err := pretrustCodexProjects(nil); err != nil {
		t.Fatalf("nil cfg should be no-op: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("nil cfg should not create file (err=%v)", err)
	}
}

// TestAppendCodexTrustEntries is a pure-string unit test of the merge
// math, exercising edge cases that don't need a tempdir.
func TestAppendCodexTrustEntries(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		paths    []string
		want     string
	}{
		{
			name:     "EmptyToOnePath",
			existing: "",
			paths:    []string{"/a"},
			want:     "[projects.\"/a\"]\ntrust_level = \"trusted\"\n",
		},
		{
			name:     "NoPaths_PreservesExistingExactly",
			existing: "model = \"x\"\n",
			paths:    nil,
			want:     "model = \"x\"\n",
		},
		{
			name:     "AlreadyPresent_NoChange",
			existing: "[projects.\"/a\"]\ntrust_level = \"trusted\"\n",
			paths:    []string{"/a"},
			want:     "[projects.\"/a\"]\ntrust_level = \"trusted\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendCodexTrustEntries(tt.existing, tt.paths)
			if got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
