package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExistingEvolveDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "plain-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{"an existing directory resolves to itself", dir, dir, ""},
		{"a trailing separator is cleaned", dir + string(filepath.Separator), dir, ""},
		{"a regular file is refused", file, "", "--evolve-dir " + file + " is not a directory; pass --evolve-dir <project>/.evolve"},
		{"an absent path is refused with the hint", filepath.Join(dir, "ünïcode", ".evolve"), "", "run from the project root or pass --evolve-dir <project>/.evolve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := existingEvolveDir(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				if got != "" {
					t.Errorf("got %q beside an error, want empty", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("existingEvolveDir(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
			}
		})
	}
}

func TestExistingEvolveDir_AnUnresolvablePathNamesTheFlag(t *testing.T) {
	prev := evolveDirAbs
	t.Cleanup(func() { evolveDirAbs = prev })
	evolveDirAbs = func(string) (string, error) { return "", errors.New("getwd: no such file or directory") }

	got, err := existingEvolveDir("rel/.evolve")

	if got != "" || err == nil || err.Error() != `--evolve-dir "rel/.evolve": getwd: no such file or directory` {
		t.Errorf("existingEvolveDir = (%q, %v), want the quoted flag value and the cause", got, err)
	}
}

func TestRunACSRun_WithoutACycleIsAUsageError(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"./pkg"}, {"--cycle", "0", "./pkg"}, {"--cycle", "-3"}} {
		var out, errb bytes.Buffer
		code := runACSRun(args, &out, &errb)
		if code != 10 {
			t.Errorf("runACSRun(%v) = %d, want 10", args, code)
		}
		if out.Len() != 0 || !strings.Contains(errb.String(), "evolve acs run: --cycle is required (must be >0)") {
			t.Errorf("runACSRun(%v): stdout=%q stderr=%q, want only the --cycle refusal", args, out.String(), errb.String())
		}
	}
}

func TestRunACSRun_AVerdictThatCannotBeMarshalledExits1(t *testing.T) {
	prev := acsRunMarshalIndent
	t.Cleanup(func() { acsRunMarshalIndent = prev })
	acsRunMarshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("boom") }
	evolveDir := t.TempDir()
	var out, errb bytes.Buffer

	code := runACSRun([]string{"--cycle", "7", "--evolve-dir", evolveDir, "./no-such-predicate-package"}, &out, &errb)

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if out.Len() != 0 || !strings.Contains(errb.String(), "evolve acs run: marshal: boom") {
		t.Errorf("stdout=%q stderr=%q, want the marshal error and no output", out.String(), errb.String())
	}
	assertNoSuiteVerdict(t, suiteVerdictPath(evolveDir, "7"))
}

func TestRunACSRun_AVerdictWriteFailureExits1AfterPrintingIt(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "runs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	code := runACSRun([]string{"--cycle", "7", "--evolve-dir", evolveDir, "./no-such-predicate-package"}, &out, &errb)

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), `"verdict": "FAIL"`) {
		t.Errorf("stdout = %q, want the verdict JSON printed before the write", out.String())
	}
	if !strings.Contains(errb.String(), "evolve acs run: write verdict: ") || strings.Contains(errb.String(), "verdict written") {
		t.Errorf("stderr = %q, want the write error and no written claim", errb.String())
	}
}

func TestRunACSRun_AnUnknownFlagIsAUsageError(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer

	code := runACSRun([]string{"--cycle", "7", "--no-such-flag"}, &out, &errb)

	if code != 10 || out.Len() != 0 || !strings.Contains(errb.String(), "flag provided but not defined: -no-such-flag") {
		t.Errorf("code=%d stdout=%q stderr=%q, want 10 and the flag error", code, out.String(), errb.String())
	}
	if strings.Contains(errb.String(), "evolve acs run: ") {
		t.Errorf("stderr = %q, want the parse refusal alone, before any argument check", errb.String())
	}
}
