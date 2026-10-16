package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBridgePolicy(t *testing.T, bridge map[string]string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"bridge": bridge})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
}

func TestRunBridge_Help(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runBridge([]string{"help"}, nil, &out, &errb); code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "evolve bridge") {
		t.Fatalf("help output missing banner; got %q", out.String())
	}
}

func TestRunBridge_NoArgs(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runBridge(nil, nil, &out, &errb); code != 10 {
		t.Fatalf("no-args exit = %d, want 10", code)
	}
}

func TestRunBridge_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := runBridge([]string{"frobnicate"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("unknown-sub exit = %d, want 10", code)
	}
	if !strings.Contains(errb.String(), "unknown subcommand") {
		t.Fatalf("stderr should report unknown subcommand; got %q", errb.String())
	}
}

func TestRunBridge_Probe(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runBridge([]string{"probe"}, nil, &out, &errb); code != 0 {
		t.Fatalf("probe exit = %d, want 0", code)
	}
	s := out.String()
	if !strings.Contains(s, `"os"`) || !strings.Contains(s, `"results"`) {
		t.Fatalf("probe JSON should carry os + results; got %q", s)
	}
	for _, cli := range []string{"claude-p", "claude-tmux", "codex", "agy"} {
		if !strings.Contains(s, `"`+cli+`"`) {
			t.Fatalf("probe output missing cli %q; got %q", cli, s)
		}
	}
}

func TestRunBridge_LaunchMissingRequiredFlags(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := runBridge([]string{"launch", "--cli=claude-p"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("launch-missing-flags exit = %d, want 10 (ExitBadFlags)", code)
	}
	if !strings.Contains(errb.String(), "missing required") {
		t.Fatalf("stderr should report missing required flags; got %q", errb.String())
	}
}

func TestRunBridge_AddRule(t *testing.T) {
	writeBridgePolicy(t, map[string]string{"manifest_dir": t.TempDir()})
	var out, errb bytes.Buffer
	if code := runBridge([]string{"add-rule", "--cli=claude-p", "--regex=FOO", "--response=y,Enter"}, nil, &out, &errb); code != 0 {
		t.Fatalf("add-rule exit = %d, want 0; stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "appended rule") {
		t.Fatalf("add-rule should confirm; got %q", out.String())
	}
	var out2, errb2 bytes.Buffer
	if code := runBridge([]string{"add-rule", "--cli=claude-p"}, nil, &out2, &errb2); code != 10 {
		t.Fatalf("add-rule missing regex exit = %d, want 10", code)
	}
}

func TestRunBridge_Doctor(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := runBridge([]string{"doctor", "--cli=claude-p"}, nil, &out, &errb)
	if code < 0 || code > 2 {
		t.Fatalf("doctor exit = %d, want a verdict code 0/1/2", code)
	}
	if !strings.Contains(out.String(), "summary:") {
		t.Fatalf("doctor table should print a summary; got %q", out.String())
	}
	var out2 bytes.Buffer
	runBridge([]string{"doctor", "--json", "--cli=claude-p"}, nil, &out2, &errb)
	if !strings.Contains(out2.String(), `"summary"`) {
		t.Fatalf("doctor --json should emit JSON; got %q", out2.String())
	}
	var out3 bytes.Buffer
	if c := runBridge([]string{"doctor", "--help"}, nil, &out3, &errb); c != 0 {
		t.Fatalf("doctor --help exit = %d, want 0", c)
	}
}

func TestRunBridge_Billing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out, errb bytes.Buffer
	if code := runBridge([]string{"billing", "snapshot", dir, "pre"}, nil, &out, &errb); code != 0 {
		t.Fatalf("billing snapshot exit = %d; stderr=%q", code, errb.String())
	}
	snap := strings.TrimSpace(out.String())
	if !strings.Contains(snap, "snap-pre-") {
		t.Fatalf("snapshot path = %q", snap)
	}
	var out2 bytes.Buffer
	if c := runBridge([]string{"billing", "compare", snap, snap}, nil, &out2, &errb); c < 0 || c > 2 {
		t.Fatalf("billing compare exit = %d, want 0/1/2", c)
	}
	for _, args := range [][]string{{"billing"}, {"billing", "snapshot"}, {"billing", "compare"}, {"billing", "bogus"}} {
		var o bytes.Buffer
		if c := runBridge(args, nil, &o, &errb); c != 10 {
			t.Fatalf("billing %v exit = %d, want 10", args, c)
		}
	}
}

func TestRunBridgePromoteRule(t *testing.T) {
	t.Parallel()
	const regex = `Press any key to continue\.\.\.`
	corpusFile := func(t *testing.T, lines ...string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "corpus.txt")
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name     string
		args     func(corpus string) []string
		corpus   []string
		wantCode int
		wantRule bool
		wantErr  string
	}{
		{"clean corpus writes a shadow rule", func(c string) []string {
			return []string{"--regex=" + regex, "--response-keys=Enter", "--corpus=" + c}
		}, []string{"Reading main.go"}, 0, true, ""},
		{"commented corpus line does not refuse", func(c string) []string {
			return []string{"--regex=" + regex, "--response-keys=Enter", "--corpus=" + c}
		}, []string{"# Press any key to continue...", "", "Reading main.go"}, 0, true, ""},
		{"matching corpus line refuses", func(c string) []string {
			return []string{"--regex=" + regex, "--response-keys=Enter", "--corpus=" + c}
		}, []string{"Press any key to continue..."}, 1, false, "Press any key to continue..."},
		{"empty response keys", func(c string) []string {
			return []string{"--regex=" + regex, "--response-keys=,", "--corpus=" + c}
		}, []string{"x"}, 10, false, "no response keys"},
		{"uncompilable regex", func(c string) []string {
			return []string{"--regex=(unbalanced paren", "--response-keys=Enter", "--corpus=" + c}
		}, []string{"x"}, 10, false, "does not compile"},
		{"missing corpus file", func(string) []string {
			return []string{"--regex=" + regex, "--response-keys=Enter", "--corpus=/nonexistent/corpus.txt"}
		}, nil, 10, false, "read corpus"},
		{"positional argument", func(c string) []string {
			return []string{"--regex=" + regex, "--response-keys=Enter", "--corpus=" + c, "stray"}
		}, []string{"x"}, 10, false, "no positional argument"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{"promote-rule", "--dir=" + dir}, tc.args(corpusFile(t, tc.corpus...))...)
			var out, errb bytes.Buffer
			if code := runBridge(args, nil, &out, &errb); code != tc.wantCode {
				t.Fatalf("exit=%d, want %d; stdout=%q stderr=%q", code, tc.wantCode, out.String(), errb.String())
			}
			files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if got := len(files) == 1; got != tc.wantRule || len(files) > 1 {
				t.Fatalf("rule files=%v, want one written=%v", files, tc.wantRule)
			}
			if tc.wantRule && !strings.Contains(out.String(), files[0]) {
				t.Errorf("stdout %q does not name %s", out.String(), files[0])
			}
			if !strings.Contains(errb.String(), tc.wantErr) {
				t.Errorf("stderr %q does not contain %q", errb.String(), tc.wantErr)
			}
		})
	}
}

func TestRunBridgePromoteRuleHelpAndDefaultDir(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runBridge([]string{"promote-rule", "--help"}, nil, &out, &errb); code != 0 || !strings.Contains(out.String(), bridgePromoteRuleUsage) {
		t.Fatalf("--help exit=%d stdout=%q", code, out.String())
	}
	root := t.TempDir()
	corpus := filepath.Join(t.TempDir(), "corpus.txt")
	if err := os.WriteFile(corpus, []byte("Reading main.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	args := []string{"promote-rule", "--regex=Press any key to continue", "--response-keys=Enter", "--corpus=" + corpus, "--project-root=" + root}
	if code := runBridge(args, nil, &out, &errb); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb.String())
	}
	files, err := filepath.Glob(filepath.Join(root, ".evolve", "instincts", "interaction-rules", "*.yaml"))
	if err != nil || len(files) != 1 {
		t.Fatalf("default registry files=%v, want 1", files)
	}
	if got := corpusLines("  # note\n\n a \nb"); strings.Join(got, "|") != "a|b" {
		t.Errorf("corpusLines=%q, want [a b]", got)
	}
}
