package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedResetProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"),
		[]byte(`{"expected_ship_sha":"OLDPIN","lastCycleNumber":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunResetSHA_OperatorRepinsToRunningBinary(t *testing.T) {
	root := seedResetProject(t)
	var out, errb bytes.Buffer
	code := runResetSHA([]string{"--operator", "--project-root", root}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	b, _ := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("torn state.json: %v\n%s", err, b)
	}
	if sha, _ := s["expected_ship_sha"].(string); sha == "OLDPIN" || len(sha) != 64 {
		t.Errorf("expected_ship_sha not re-pinned to a real sha: %q", sha)
	}
	if s["lastCycleNumber"] != float64(3) {
		t.Errorf("unrelated state key lost: %+v", s)
	}
	if !strings.Contains(out.String(), "re-pinned") {
		t.Errorf("missing success output: %s", out.String())
	}
}

func TestRunResetSHA_RefusesWithoutProvenanceOrOperator(t *testing.T) {
	// The test binary has no verifiable provenance against the (non-git) temp
	// project, and --operator is absent → RepinShipSHA refuses → non-zero exit,
	// pin unchanged.
	root := seedResetProject(t)
	var out, errb bytes.Buffer
	code := runResetSHA([]string{"--project-root", root}, nil, &out, &errb)
	if code == 0 {
		t.Fatal("expected refusal without provenance or --operator")
	}
	b, _ := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	if !strings.Contains(string(b), `"OLDPIN"`) {
		t.Errorf("pin must be UNCHANGED on refusal: %s", b)
	}
}

func resetSHAChdir(t *testing.T, dir string) string {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}

func resetSHAPin(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("torn state.json: %v\n%s", err, b)
	}
	sha, _ := s["expected_ship_sha"].(string)
	return sha
}

func runResetSHAOperator(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runResetSHA(append([]string{"--operator"}, args...), nil, &out, &errb)
	return code, errb.String()
}

func TestRunResetSHA_RootResolutionPrecedence(t *testing.T) {
	cases := []struct {
		name      string
		useFlag   bool
		useEnv    bool
		useCwd    bool
		wantPinAt string
	}{
		{name: "flag wins over env and cwd", useFlag: true, useEnv: true, useCwd: true, wantPinAt: "flag"},
		{name: "env used when flag empty", useEnv: true, useCwd: true, wantPinAt: "env"},
		{name: "cwd used when flag and env empty", useCwd: true, wantPinAt: "cwd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roots := map[string]string{"flag": seedResetProject(t), "env": seedResetProject(t), "cwd": seedResetProject(t)}
			t.Setenv("EVOLVE_PROJECT_ROOT", "")
			if tc.useEnv {
				t.Setenv("EVOLVE_PROJECT_ROOT", roots["env"])
			}
			if tc.useCwd {
				resetSHAChdir(t, roots["cwd"])
			}
			var args []string
			if tc.useFlag {
				args = []string{"--project-root", roots["flag"]}
			}
			code, stderr := runResetSHAOperator(t, args...)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			for which, root := range roots {
				pin := resetSHAPin(t, root)
				if which == tc.wantPinAt && (pin == "OLDPIN" || len(pin) != 64) {
					t.Errorf("%s root must be re-pinned, got %q", which, pin)
				}
				if which != tc.wantPinAt && pin != "OLDPIN" {
					t.Errorf("%s root must stay untouched when %s wins, got %q", which, tc.wantPinAt, pin)
				}
			}
		})
	}
}

func TestRunResetSHA_RelativeRootResolvedAgainstCwd(t *testing.T) {
	root := seedResetProject(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	resetSHAChdir(t, filepath.Dir(root))
	code, stderr := runResetSHAOperator(t, "--project-root", filepath.Base(root))
	if code != 0 {
		t.Fatalf("a relative --project-root must resolve to an absolute root before the re-pin: exit=%d stderr=%s", code, stderr)
	}
	if pin := resetSHAPin(t, root); pin == "OLDPIN" || len(pin) != 64 {
		t.Errorf("relative root not re-pinned: %q", pin)
	}
}

func TestRunResetSHA_RelativeEnvRootResolvedAgainstCwd(t *testing.T) {
	root := seedResetProject(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", filepath.Base(root))
	resetSHAChdir(t, filepath.Dir(root))
	code, stderr := runResetSHAOperator(t)
	if code != 0 {
		t.Fatalf("a relative EVOLVE_PROJECT_ROOT must resolve to an absolute root before the re-pin: exit=%d stderr=%s", code, stderr)
	}
	if pin := resetSHAPin(t, root); pin == "OLDPIN" || len(pin) != 64 {
		t.Errorf("relative env root not re-pinned: %q", pin)
	}
}

func TestRunResetSHA_MissingRelativeRootFailsOnAbsoluteStatePath(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	cwd := resetSHAChdir(t, t.TempDir())
	code, stderr := runResetSHAOperator(t, "--project-root", "no-such-project")
	if code != 1 {
		t.Fatalf("a missing project root must fail with exit 1, got %d stderr=%s", code, stderr)
	}
	wantPath := filepath.Join(cwd, "no-such-project", ".evolve", "state.json")
	if !strings.Contains(stderr, wantPath) {
		t.Errorf("the refusal must name the absolute state path %s (root resolved before RepinShipSHA), got: %s", wantPath, stderr)
	}
	if strings.Contains(stderr, "statePath must be absolute") {
		t.Errorf("a relative root reached RepinShipSHA unresolved: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(cwd, "no-such-project")); !os.IsNotExist(err) {
		t.Errorf("a failed re-pin must not create the project root: %v", err)
	}
}
