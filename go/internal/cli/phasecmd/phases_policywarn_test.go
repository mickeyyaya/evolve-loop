package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMalformedPolicyWarnsOnValidateAndLint(t *testing.T) {
	const code = "PHASE_ROOTS_POLICY_UNREADABLE"
	cases := []struct {
		name string
		run  func(stdout, stderr *bytes.Buffer) int
	}{
		{name: "phases validate", run: func(stdout, stderr *bytes.Buffer) int {
			return RunPhases([]string{"validate"}, strings.NewReader(""), stdout, stderr)
		}},
		{name: "phase lint", run: func(stdout, stderr *bytes.Buffer) int {
			return runPhaseLint([]string{"widget-check"}, stdout, stderr)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeUserPhase(t, root, "widget-check", `{"name":"widget-check","kind":"llm","optional":true}`)
			t.Setenv("EVOLVE_PROJECT_ROOT", root)
			var stdout, stderr bytes.Buffer

			cleanCode := tc.run(&stdout, &stderr)
			if strings.Contains(stdout.String()+stderr.String(), code) {
				t.Fatalf("missing policy.json must not warn; got stdout=%q stderr=%q", stdout.String(), stderr.String())
			}

			if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{`), 0o644); err != nil {
				t.Fatal(err)
			}
			stdout.Reset()
			stderr.Reset()
			if got := tc.run(&stdout, &stderr); got != cleanCode {
				t.Errorf("exit = %d with a malformed policy.json, want %d (the warning must not change the verdict)", got, cleanCode)
			}
			if out := stdout.String() + stderr.String(); !strings.Contains(out, "WARN: "+code+": ") {
				t.Errorf("malformed policy.json produced no %s warning; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}
