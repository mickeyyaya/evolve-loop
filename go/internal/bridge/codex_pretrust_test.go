package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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
			// Fully idempotent only when BOTH the trust entries AND the cycle-142
			// [notice] rate-limit-nudge suppression are already present; pretrust
			// adds the notice alongside the trust entries, so a fixture missing it
			// would (correctly) be rewritten.
			name: "IdempotentWhenAlreadyTrusted",
			existing: `model = "gpt-5.5"

[projects."/Users/op/proj/.evolve/worktrees/cycle-123"]
trust_level = "trusted"

[projects."/Users/op/proj/.evolve/runs/cycle-123"]
trust_level = "trusted"

[notice]
hide_rate_limit_model_nudge = true
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
			cfg := &Config{Worktree: tt.worktree, Workspace: tt.workspace, codexConfigPath: path}
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

// TestPretrustCodexProjects_ConcurrentCalls_AllEntriesSurvive is the N10
// (ADR-0049) regression. Under `evolve fleet` the whole-cycle project lock is
// skipped, so multiple cycles' codex Preflights pre-trust DISTINCT paths into
// the SAME host-global ~/.codex/config.toml concurrently. The unique CreateTemp
// already prevents temp-file clobbering, but the read-merge-write-RENAME was
// last-writer-wins: cycles that each read a config WITHOUT the others' entries
// each rename their own snapshot, so the final file keeps only the last writer's
// trust entries. A dropped trust entry re-arms codex's "Press enter to confirm"
// runtime modal that hung cycle-122 for the cycle whose entry lost the race.
//
// The fix serializes the whole RMW under flock.WithPathLock(configPath), so the
// append-only merges compose losslessly: EVERY path stays trusted. A start
// barrier forces all readers to observe the same (empty) initial state, so the
// pre-fix lost-update trips reliably across the iterations. (This REPLACES the
// old test, which asserted only "at least the LAST writer's entries" — it
// enshrined the very lost-update this fix removes.)
func TestPretrustCodexProjects_ConcurrentCalls_AllEntriesSurvive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfgs := []*Config{
		{Worktree: "/tmp/wt-A", Workspace: "/tmp/ws-A", codexConfigPath: path},
		{Worktree: "/tmp/wt-B", Workspace: "/tmp/ws-B", codexConfigPath: path},
		{Worktree: "/tmp/wt-C", Workspace: "/tmp/ws-C", codexConfigPath: path},
		{Worktree: "/tmp/wt-D", Workspace: "/tmp/ws-D", codexConfigPath: path},
	}
	var wantHeaders []string
	for _, c := range cfgs {
		wantHeaders = append(wantHeaders, codexProjectHeader(c.Worktree), codexProjectHeader(c.Workspace))
	}

	const iters = 100
	for i := 0; i < iters; i++ {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatalf("iter %d reset: %v", i, err)
		}
		start := make(chan struct{})
		errs := make([]error, len(cfgs))
		var wg sync.WaitGroup
		wg.Add(len(cfgs))
		for j, c := range cfgs {
			go func(idx int, cfg *Config) {
				defer wg.Done()
				<-start
				errs[idx] = pretrustCodexProjects(cfg)
			}(j, c)
		}
		close(start)
		wg.Wait()

		for j, err := range errs {
			if err != nil {
				t.Fatalf("iter %d cfg %d: pretrustCodexProjects: %v", i, j, err)
			}
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("iter %d read: %v", i, err)
		}
		for _, h := range wantHeaders {
			if !strings.Contains(string(got), h) {
				t.Fatalf("iter %d lost trust entry under concurrent pretrust: %q absent\n%s", i, h, got)
			}
		}
	}
}

// TestPretrustCodexProjects_CreatesParentDir guards that a fresh host
// without ~/.codex/ gets the directory created (0700) before the merge.
func TestPretrustCodexProjects_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "codex-home", ".codex", "config.toml")

	cfg := &Config{Worktree: "/tmp/wt", codexConfigPath: nested}
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
	if err := pretrustCodexProjects(nil); err != nil {
		t.Fatalf("nil cfg should be no-op: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("nil cfg should not create file (err=%v)", err)
	}
}

// TestPretrustCodexProjects_WritesHideRateLimitNudge — cycle-142: the pretrust
// pass must also suppress codex's "Approaching rate limits / Switch to mini?"
// model-switch modal, which otherwise hangs the phase until the artifact-wait
// deadline. The [notice] key is written alongside the trust entries and is
// idempotent.
func TestPretrustCodexProjects_WritesHideRateLimitNudge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Worktree: filepath.Join(dir, "wt"), Workspace: filepath.Join(dir, "ws"), codexConfigPath: path}
	if err := pretrustCodexProjects(cfg); err != nil {
		t.Fatalf("pretrust: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "[notice]") || !strings.Contains(string(got), "hide_rate_limit_model_nudge = true") {
		t.Errorf("config must contain a [notice] table suppressing the rate-limit nudge; got:\n%s", got)
	}
	// Trust entries must still be present (notice did not clobber them).
	if !strings.Contains(string(got), "trust_level") {
		t.Errorf("trust entries must coexist with the notice; got:\n%s", got)
	}
	// Idempotent: a second pass must not duplicate the key.
	if err := pretrustCodexProjects(cfg); err != nil {
		t.Fatalf("pretrust 2: %v", err)
	}
	got2, _ := os.ReadFile(path)
	if n := strings.Count(string(got2), "hide_rate_limit_model_nudge"); n != 1 {
		t.Errorf("notice must be idempotent; got %d occurrences:\n%s", n, got2)
	}
}

// TestAppendCodexNotice is a pure-string unit test of the notice merge.
func TestAppendCodexNotice(t *testing.T) {
	out := appendCodexNotice("")
	if !strings.Contains(out, "[notice]") || !strings.Contains(out, "hide_rate_limit_model_nudge = true") {
		t.Errorf("empty → must add notice; got %q", out)
	}
	if out2 := appendCodexNotice(out); strings.Count(out2, "hide_rate_limit_model_nudge") != 1 {
		t.Errorf("idempotent: already-present must be a no-op; got %q", out2)
	}
	existing := "[projects.\"/x\"]\ntrust_level = \"trusted\"\n"
	out3 := appendCodexNotice(existing)
	if !strings.Contains(out3, "trust_level") || !strings.Contains(out3, "hide_rate_limit_model_nudge") {
		t.Errorf("must preserve existing content + append notice; got %q", out3)
	}
	// Cover the `out += "\n"` branch: existing non-empty without trailing newline.
	out4 := appendCodexNotice("no-newline-at-end")
	if !strings.HasSuffix(out4, "\n") || !strings.Contains(out4, "hide_rate_limit_model_nudge") {
		t.Errorf("non-newline-terminated input must have newline added + notice appended; got %q", out4)
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
