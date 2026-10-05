package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

func TestParseCIClassifyArgs_OneTargetAndKnownFlags(t *testing.T) {
	a, err := parseCIClassifyArgs([]string{"--json", "pr:12", "--rerun", "--project-root", "/p"})
	if err != nil || a.target != (ciwatch.Target{Kind: ciwatch.TargetPR, Value: "12"}) || !a.asJSON || !a.rerun || a.projectRoot != "/p" {
		t.Fatalf("parseCIClassifyArgs = %+v, %v", a, err)
	}
	for _, bad := range [][]string{{}, {"501", "502"}, {"#12"}, {""}, {"501", "--bogus"}, {"501", "--project-root"}} {
		if _, err := parseCIClassifyArgs(bad); err == nil {
			t.Errorf("parseCIClassifyArgs(%q) must be a usage error", bad)
		}
	}
}

func TestRunCI_UsageExitsTen(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"classify"}, {"classify", "abcde1"}} {
		var stdout, stderr bytes.Buffer
		if code := runCI(args, nil, &stdout, &stderr); code != exitUsage || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage: evolve ci classify") {
			t.Errorf("runCI(%q) = %d, stdout %q, stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestGoModulePath_ReadsTheModuleLine(t *testing.T) {
	dir := t.TempDir()
	goMod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goMod, []byte("// header\nmodule example.com/fix\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := goModulePath(goMod); err != nil || got != "example.com/fix" {
		t.Errorf("goModulePath = %q, %v", got, err)
	}
	if _, err := goModulePath(filepath.Join(dir, "absent.mod")); err == nil {
		t.Error("an unreadable go.mod must be reported")
	}
}

func TestWriteClassifyReport_TextTableAndJSON(t *testing.T) {
	rep := ciwatch.ClassifyReport{
		RunID: 501, RunURL: "https://x/runs/501", Conclusion: "failure",
		HeadSHA: "0123456789abcdef0123", BaseSHA: "fedcba9876543210fedc",
		Failures: []ciwatch.ClassifiedFailure{{
			FailingTest: ciwatch.FailingTest{Jobs: []string{"lint"}},
			Evidence:    ciwatch.Evidence{Touched: ciwatch.FactUnknown, BaseRed: ciwatch.FactNo, RecurredOnMain: ciwatch.FactNo, RerunGreen: ciwatch.FactUnknown},
			Label:       ciwatch.LabelUnknown, Rule: "default",
		}},
	}
	var text bytes.Buffer
	if err := writeClassifyReport(&text, rep, false); err != nil {
		t.Fatal(err)
	}
	want := "run 501 https://x/runs/501 head=0123456789ab base=fedcba987654 conclusion=failure\n" + ciClassifyHeader +
		"\nunknown\t-\t-\tdefault\tunknown\tno\tno\tunknown\nretry-safe: no\n"
	if text.String() != want {
		t.Errorf("text report:\n%q\nwant\n%q", text.String(), want)
	}
	var js bytes.Buffer
	if err := writeClassifyReport(&js, rep, true); err != nil || !strings.Contains(js.String(), `"retry_safe": false`) || !strings.Contains(js.String(), `"rule": "default"`) {
		t.Errorf("json report: %v\n%s", err, js.String())
	}
}

func TestRunComments_IsCommentauditMain(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}} {
		var evolveOut, evolveErr, toolOut, toolErr bytes.Buffer
		got := runComments(args, nil, &evolveOut, &evolveErr)
		want := commentaudit.Main(args, &toolOut, &toolErr, commentaudit.ExecGit{})
		if got != 2 || got != want || evolveOut.String() != toolOut.String() || evolveErr.String() != toolErr.String() {
			t.Errorf("runComments(%q) = %d %q %q; commentaudit.Main = %d %q %q", args, got, evolveOut.String(), evolveErr.String(), want, toolOut.String(), toolErr.String())
		}
	}
}
