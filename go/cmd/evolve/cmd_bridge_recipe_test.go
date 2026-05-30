package main

import (
	"bytes"
	"strings"
	"testing"
)

func runRecipeCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := runBridge(append([]string{"recipe"}, args...), nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRecipeCLI_List(t *testing.T) {
	code, out, _ := runRecipeCLI("list")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "plugin-install") || !strings.Contains(out, "list-capabilities") {
		t.Errorf("list output missing recipes: %q", out)
	}
}

func TestRecipeCLI_Show(t *testing.T) {
	code, out, _ := runRecipeCLI("show", "plugin-install")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "\"name\": \"plugin-install\"") || !strings.Contains(out, "per_cli") {
		t.Errorf("show output unexpected: %q", out)
	}
}

func TestRecipeCLI_ShowErrors(t *testing.T) {
	if code, _, _ := runRecipeCLI("show"); code != 10 {
		t.Errorf("show no-name code=%d want 10", code)
	}
	if code, _, _ := runRecipeCLI("show", "no-such-recipe"); code != 10 {
		t.Errorf("show bad-name code=%d want 10", code)
	}
}

func TestRecipeCLI_Dispatch(t *testing.T) {
	if code, _, _ := runRecipeCLI(); code != 10 {
		t.Errorf("no-action code=%d want 10", code)
	}
	if code, _, _ := runRecipeCLI("frobnicate"); code != 10 {
		t.Errorf("unknown-action code=%d want 10", code)
	}
	if code, out, _ := runRecipeCLI("--help"); code != 0 || !strings.Contains(out, "recipe run") {
		t.Errorf("help code=%d out=%q", code, out)
	}
}

func TestRecipeCLI_RunFlagValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"missing all", []string{"run"}},
		{"missing workspace", []string{"run", "plugin-install", "--cli=claude-tmux"}},
		{"missing name", []string{"run", "--cli=claude-tmux", "--workspace=/tmp/x"}},
		{"bad param", []string{"run", "plugin-install", "--cli=claude-tmux", "--workspace=/tmp/x", "--param=bogus"}},
		{"unknown flag", []string{"run", "plugin-install", "--cli=claude-tmux", "--workspace=/tmp/x", "--zzz"}},
		{"extra positional", []string{"run", "plugin-install", "extra", "--cli=claude-tmux", "--workspace=/tmp/x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, _ := runRecipeCLI(tc.args...); code != 10 {
				t.Errorf("code=%d want 10", code)
			}
		})
	}
}

func TestRecipeCLI_RunHelp(t *testing.T) {
	if code, out, _ := runRecipeCLI("run", "--help"); code != 0 || !strings.Contains(out, "recipe run") {
		t.Errorf("run --help code=%d out=%q", code, out)
	}
}

func TestRecipeCLI_RunUnknownRecipe(t *testing.T) {
	// LoadRecipe fails before any tmux interaction → exit 1, no spawn.
	tmp := t.TempDir()
	code, _, errs := runRecipeCLI("run", "no-such-recipe", "--cli=claude-tmux", "--workspace="+tmp)
	if code != 1 {
		t.Fatalf("code=%d want 1; stderr=%q", code, errs)
	}
}

func TestRecipeCLI_RunUnsupportedCLI(t *testing.T) {
	// stepsFor rejects ollama before EnsureSession → exit 1, no tmux spawn.
	tmp := t.TempDir()
	code, _, _ := runRecipeCLI("run", "plugin-install", "--cli=ollama-tmux", "--workspace="+tmp,
		"--param=marketplace=x", "--param=plugin=y")
	if code != 1 {
		t.Fatalf("code=%d want 1", code)
	}
}

func TestRecipeCLI_RunAllFlagsParsed(t *testing.T) {
	// Exercises every optional flag branch; fails at the unsupported-CLI step
	// (before any tmux spawn) so the parse is fully covered deterministically.
	tmp := t.TempDir()
	code, _, _ := runRecipeCLI("run", "plugin-install", "--cli=ollama-tmux", "--workspace="+tmp,
		"--agent=installer", "--session-name=sess", "--worktree="+tmp,
		"--permission-mode=plan", "--allow-bypass",
		"--param=marketplace=x", "--param=plugin=y")
	if code != 1 {
		t.Fatalf("code=%d want 1", code)
	}
}
