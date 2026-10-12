package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLLMRole(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		args       []string
		wantRole   string
		wantCode   int
		wantOK     bool
		wantStdout string
		wantStderr string
	}{
		{"one role", []string{"scout"}, "scout", 0, true, "", ""},
		{"unicode role", []string{"rôle"}, "rôle", 0, true, "", ""},
		{"single dash role is a role", []string{"-"}, "-", 0, true, "", ""},
		{"no args", nil, "", 2, false, "", "[resolve-llm] usage: evolve resolve-llm <role>\n"},
		{"long help", []string{"--help"}, "", 0, false, "Usage: evolve resolve-llm <role>\n", ""},
		{"short help after a role", []string{"scout", "-h"}, "", 0, false, "Usage: evolve resolve-llm <role>\n", ""},
		{"unknown flag", []string{"--json"}, "", 2, false, "", "[resolve-llm] unknown flag: --json\n"},
		{"bare double dash", []string{"--"}, "", 2, false, "", "[resolve-llm] unknown flag: --\n"},
		{"two roles", []string{"scout", "build"}, "", 2, false, "", "[resolve-llm] too many arguments\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer

			role, code, ok := resolveLLMRole(tc.args, &out, &errb)

			if role != tc.wantRole || code != tc.wantCode || ok != tc.wantOK {
				t.Fatalf("resolveLLMRole(%q) = %q,%d,%v want %q,%d,%v", tc.args, role, code, ok, tc.wantRole, tc.wantCode, tc.wantOK)
			}
			if !strings.HasPrefix(out.String(), tc.wantStdout) || (tc.wantStdout == "" && out.Len() != 0) {
				t.Errorf("stdout = %q, want prefix %q", out.String(), tc.wantStdout)
			}
			if errb.String() != tc.wantStderr {
				t.Errorf("stderr = %q, want %q", errb.String(), tc.wantStderr)
			}
		})
	}
}

func e3ResolveRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		writeFileForPublishTest(t, filepath.Join(root, rel), body)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("EVOLVE_PLUGIN_ROOT", "")
	return root
}

func TestRunResolveLLM(t *testing.T) {
	cases := []struct {
		name       string
		files      map[string]string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"profile tier wins", map[string]string{".evolve/profiles/e3-role.json": `{"cli":"agy","model_tier_default":"deep"}`},
			[]string{"e3-role"}, 0, `{"cli":"agy","model_tier":"deep","source":"profile"}` + "\n", ""},
		{"missing tier defaults to balanced", map[string]string{".evolve/profiles/e3-role.json": `{"cli":"claude"}`},
			[]string{"e3-role"}, 0, `{"cli":"claude","model_tier":"balanced","source":"profile"}` + "\n", ""},
		{"no profile", nil, []string{"e3-no-such-role"}, 1, "", "[resolve-llm] ERROR: profile not found for role 'e3-no-such-role'\n"},
		{"profile without a cli", map[string]string{".evolve/profiles/e3-role.json": `{"model_tier_default":"deep"}`},
			[]string{"e3-role"}, 1, "", "missing .cli field\n"},
		{"usage error", nil, []string{"a", "b"}, 2, "", "[resolve-llm] too many arguments\n"},
		{"refused routing table", map[string]string{".evolve/policy.json": `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`},
			[]string{"e3-role"}, exitRoutingRefused, "", "[resolve-llm] ERROR: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e3ResolveRoot(t, tc.files)
			var out, errb bytes.Buffer

			code := runResolveLLM(tc.args, nil, &out, &errb)

			if code != tc.wantCode || out.String() != tc.wantStdout {
				t.Fatalf("code %d stdout %q stderr %q, want %d %q", code, out.String(), errb.String(), tc.wantCode, tc.wantStdout)
			}
			if !strings.Contains(errb.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want %q", errb.String(), tc.wantStderr)
			}
		})
	}
}

func TestRunResolveLLM_ProfileReadErrorIsNotReportedAsNotFound(t *testing.T) {
	root := e3ResolveRoot(t, nil)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles", "e3-role.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	code := runResolveLLM([]string{"e3-role"}, nil, &out, &errb)

	if code != 1 || strings.Contains(errb.String(), "profile not found") || !strings.Contains(errb.String(), "[resolve-llm] ERROR: resolvellm: read profile: ") {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}
