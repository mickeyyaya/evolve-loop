//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func acsFakeGoTestJSON(t *testing.T, events string, exit int) {
	t.Helper()
	dir := t.TempDir()
	stream := filepath.Join(dir, "events.json")
	if err := os.WriteFile(stream, []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat '" + stream + "'\nexit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunACSRun_ARedPredicateFromARealGoTestStreamExits2(t *testing.T) {
	acsFakeGoTestJSON(t, `{"Action":"run","Test":"TestC7_001_Red"}
{"Action":"fail","Test":"TestC7_001_Red"}
{"Action":"run","Test":"TestC7_002_Green"}
{"Action":"pass","Test":"TestC7_002_Green"}
`, 1)
	evolveDir := t.TempDir()
	var out, errb bytes.Buffer

	code := runACSRun([]string{"--cycle", "7", "--evolve-dir", evolveDir, "./acs/cycle7"}, &out, &errb)

	if code != 2 {
		t.Fatalf("exit = %d, want 2 on a red predicate (stderr=%q)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "verdict written to "+suiteVerdictPath(evolveDir, "7")+" (red=1/2)") {
		t.Errorf("stderr = %q, want the written note with red=1/2", errb.String())
	}
	if !strings.Contains(out.String(), `"red_count": 1`) || !strings.Contains(out.String(), `"verdict": "FAIL"`) {
		t.Errorf("stdout = %q, want a FAIL verdict with one red", out.String())
	}
}

func TestRunACSRun_AllGreenFromARealGoTestStreamExits0WithoutWritingUnderJSONFalse(t *testing.T) {
	acsFakeGoTestJSON(t, `{"Action":"run","Test":"TestC7_002_Green"}
{"Action":"pass","Test":"TestC7_002_Green"}
`, 0)
	evolveDir := t.TempDir()
	var out, errb bytes.Buffer

	code := runACSRun([]string{"--cycle", "7", "--evolve-dir", evolveDir, "--json=false", "./acs/cycle7"}, &out, &errb)

	if code != 0 {
		t.Fatalf("exit = %d, want 0 on an all-green run (stderr=%q)", code, errb.String())
	}
	if !strings.Contains(out.String(), `"verdict": "PASS"`) || !strings.Contains(out.String(), `"ship_eligible": true`) {
		t.Errorf("stdout = %q, want a shippable PASS verdict", out.String())
	}
	assertNoSuiteVerdict(t, suiteVerdictPath(evolveDir, "7"))
}

func TestRunACSSuite_AnAllGreenPredicateTreeExits0(t *testing.T) {
	root, evolveDir := t.TempDir(), t.TempDir()
	pkgDir := filepath.Join(root, "go", "acs", "cycle7")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(root, "go", "go.mod"):         "module example.com/acsfixture\n\ngo 1.21\n",
		filepath.Join(pkgDir, "predicates_test.go"): "//go:build acs\n\npackage cycle7\n\nimport \"testing\"\n\nfunc TestC7_001_Green(t *testing.T) {}\n",
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", root, "--evolve-dir", evolveDir)

	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "[acs suite] cycle=7 verdict=PASS green=1 red=0") || strings.Contains(stdout, "RED") {
		t.Errorf("stdout = %q, want a PASS summary with no RED line", stdout)
	}
	if v := readSuiteVerdict(t, suiteVerdictPath(evolveDir, "7")); v.Verdict != "PASS" || !v.ShipEligible {
		t.Errorf("verdict = %q ship_eligible=%v, want PASS true", v.Verdict, v.ShipEligible)
	}
}
