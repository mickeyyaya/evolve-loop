package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
)

func TestTestCommand_UsageAndUnknownVerb(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runTest(nil, nil, &stdout, &stderr); code != cliExitOK || !strings.Contains(stdout.String(), "test audit") {
		t.Errorf("help: code=%d stdout=%q", code, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--help"}, nil, &stdout, &stderr); code != cliExitOK || !strings.Contains(stdout.String(), "usage: evolve test audit") {
		t.Errorf("audit --help: code=%d stdout=%q", code, stdout.String())
	}
	for _, args := range [][]string{{"bogus"}, {"audit", "--bogus"}, {"audit", "--rebaseline", "a", "--check"}, {"audit", "extra"}} {
		stdout.Reset()
		stderr.Reset()
		if code := runTest(args, nil, &stdout, &stderr); code != exitUsage {
			t.Errorf("%v: code=%d stderr=%q, want %d", args, code, stderr.String(), exitUsage)
		}
	}
}

func TestTestAudit_ListCheckAndJSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	weak := "package x\n\nimport \"testing\"\n\nfunc TestWeak(t *testing.T) {\n\t_ = 1\n}\n"
	contractUnitWrite(t, root, "pkg/x/x_test.go", weak)

	var stdout, stderr bytes.Buffer
	if code := runTest([]string{"audit", "--root", root}, nil, &stdout, &stderr); code != cliExitOK || !strings.Contains(stdout.String(), "pkg/x\tTestWeak\tno-assertion") {
		t.Fatalf("list text: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", root, "--json"}, nil, &stdout, &stderr); code != cliExitOK {
		t.Fatalf("list json: code=%d stderr=%q", code, stderr.String())
	}
	var listed struct {
		Rows []testaudit.Finding `json:"rows"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil || len(listed.Rows) != 1 || listed.Rows[0].Package != "pkg/x" {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", root, "--check"}, nil, &stdout, &stderr); code != cliExitRefused {
		t.Fatalf("check before rebaseline: code=%d, want %d", code, cliExitRefused)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", root, "--rebaseline", "pkg/x", "--json"}, nil, &stdout, &stderr); code != cliExitOK {
		t.Fatalf("rebaseline: code=%d stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", root, "--check", "--json"}, nil, &stdout, &stderr); code != cliExitOK || !strings.Contains(stdout.String(), `"ok":true`) {
		t.Fatalf("check json after rebaseline: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", root, "--check"}, nil, &stdout, &stderr); code != cliExitOK || !strings.Contains(stdout.String(), "test audit: clean") {
		t.Fatalf("check text after rebaseline: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestTestAudit_ErrorPathsAndCentralBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	weak1 := "package g\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {\n\t_ = 1\n}\n"
	weak2 := "package g\n\nimport \"testing\"\n\nfunc TestTwo(t *testing.T) {\n\t_ = 1\n}\n"
	clean1 := "package g\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {\n\tt.Fatal(\"ok\")\n}\n"
	contractUnitWrite(t, root, "internal/testaudit/unwritable-baselines.json", `{"packages":["pkg/central"],"rows":[]}`)
	contractUnitWrite(t, root, "pkg/central/a_test.go", weak1)

	var stdout, stderr bytes.Buffer
	if code := runTest([]string{"audit", "--root", root, "--rebaseline", "pkg/central"}, nil, &stdout, &stderr); code == cliExitOK || !strings.Contains(stderr.String(), "TestOne") {
		t.Fatalf("central 0->1 growth: code=%d stderr=%q, want refusal naming TestOne", code, stderr.String())
	}

	contractUnitWrite(t, root, "internal/testaudit/unwritable-baselines.json", `{"packages":["pkg/central"],"rows":[{"package":"pkg/central","test":"TestOne","rule":"no-assertion"}]}`)
	contractUnitWrite(t, root, "pkg/central/a_test.go", clean1)
	contractUnitWrite(t, root, "pkg/central/b_test.go", weak2)
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", root, "--rebaseline", "pkg/central"}, nil, &stdout, &stderr); code == cliExitOK || !strings.Contains(stderr.String(), "TestTwo") {
		t.Fatalf("central equal-count swap: code=%d stderr=%q, want refusal naming TestTwo", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", filepath.Join(t.TempDir(), "absent")}, nil, &stdout, &stderr); code != cliExitIO {
		t.Fatalf("absent root list: code=%d stderr=%q, want %d", code, stderr.String(), cliExitIO)
	}

	brokenConfig := func() (testaudit.Config, error) {
		return testaudit.Config{}, errors.New("broken config")
	}
	stdout.Reset()
	stderr.Reset()
	if code := runTestAuditWith([]string{"--root", root}, brokenConfig, &stdout, &stderr); code != cliExitIO || !strings.Contains(stderr.String(), "broken config") {
		t.Fatalf("config error: code=%d stderr=%q, want %d", code, stderr.String(), cliExitIO)
	}
}

func TestTestAudit_LocalEqualCountSwapRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	weak1 := "package x\n\nimport \"testing\"\n\nfunc TestWeakA(t *testing.T) {\n\t_ = 1\n}\n"
	clean1 := "package x\n\nimport \"testing\"\n\nfunc TestWeakA(t *testing.T) {\n\tt.Fatal(\"ok\")\n}\n"
	weak2 := "package x\n\nimport \"testing\"\n\nfunc TestWeakB(t *testing.T) {\n\t_ = 1\n}\n"
	contractUnitWrite(t, root, "pkg/x/a_test.go", weak1)

	var stdout, stderr bytes.Buffer
	if code := runTest([]string{"audit", "--root", root, "--rebaseline", "pkg/x"}, nil, &stdout, &stderr); code != cliExitOK {
		t.Fatalf("initial rebaseline: code=%d stderr=%q", code, stderr.String())
	}
	contractUnitWrite(t, root, "pkg/x/a_test.go", clean1)
	contractUnitWrite(t, root, "pkg/x/b_test.go", weak2)
	stdout.Reset()
	stderr.Reset()
	if code := runTest([]string{"audit", "--root", root, "--rebaseline", "pkg/x"}, nil, &stdout, &stderr); code == cliExitOK || !strings.Contains(stderr.String(), "TestWeakB") {
		t.Fatalf("local equal-count swap: code=%d stderr=%q, want refusal naming TestWeakB", code, stderr.String())
	}
}
