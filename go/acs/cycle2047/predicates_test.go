//go:build acs

package cycle2047

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditorSpec = `fix-spec: 1
expected: a negative span is refused
edit: go/internal/retryladder/audit_fix.go:100-104
> func spanLines(from, to string) int {
> 	return 0
> }
edit: go/internal/retryladder/decide.go:20-20
> const x = 1
`

func citedSpec(t *testing.T) retryladder.FixSpec {
	t.Helper()
	spec, err := retryladder.ParseFixSpec(auditorSpec)
	if err != nil {
		t.Fatalf("ParseFixSpec(auditor spec) error = %v", err)
	}
	return spec
}

func diffFor(file, hunk string) string {
	return "diff --git a/" + file + " b/" + file + "\n--- a/" + file + "\n+++ b/" + file + "\n" + hunk + "\n"
}

func TestC2047_001_ParserReadsLocationBehaviorAndReplacementLines(t *testing.T) {
	spec := citedSpec(t)
	if spec.Expected != "a negative span is refused" {
		t.Errorf("expected behavior = %q, want %q", spec.Expected, "a negative span is refused")
	}
	if len(spec.Edits) != 2 {
		t.Fatalf("edits = %d, want 2", len(spec.Edits))
	}
	first := spec.Edits[0]
	if first.File != "go/internal/retryladder/audit_fix.go" || first.FromLine != 100 || first.ToLine != 104 {
		t.Errorf("location = %s:%d-%d, want go/internal/retryladder/audit_fix.go:100-104", first.File, first.FromLine, first.ToLine)
	}
	wantLines := []string{"func spanLines(from, to string) int {", "\treturn 0", "}"}
	if !reflect.DeepEqual(first.Replacement, wantLines) {
		t.Errorf("replacement = %q, want %q", first.Replacement, wantLines)
	}
}

func TestC2047_002_ParserRefusesMalformedSpecs(t *testing.T) {
	malformed := map[string]string{
		"no expected behavior": "fix-spec: 1\nedit: a.go:1-1\n> x\n",
		"no replacement lines": "fix-spec: 1\nexpected: ok\nedit: a.go:1-1\n",
		"reversed line range":  "fix-spec: 1\nexpected: ok\nedit: a.go:9-3\n> x\n",
		"no location":          "fix-spec: 1\nexpected: ok\n> x\n",
		"path leaves the repo": "fix-spec: 1\nexpected: ok\nedit: ../outside.go:1-1\n> x\n",
		"empty text":           "",
	}
	for name, text := range malformed {
		spec, err := retryladder.ParseFixSpec(text)
		if !errors.Is(err, retryladder.ErrMalformedFixSpec) {
			t.Errorf("%s: error = %v, want ErrMalformedFixSpec", name, err)
		}
		if !reflect.DeepEqual(spec, retryladder.FixSpec{}) {
			t.Errorf("%s: returned %+v with an error, want the zero spec", name, spec)
		}
	}
}

func TestC2047_003_FixTouchingAFileOutsideTheSpecFailsTheScopeGuard(t *testing.T) {
	spec := citedSpec(t)
	diff := diffFor("go/internal/retryladder/audit_fix.go", "@@ -100,5 +100,3 @@\n-a\n+b") +
		diffFor("go/internal/core/other.go", "@@ -1 +1 @@\n-a\n+b")
	got, err := retryladder.FixScopeViolations(spec, diff)
	if err != nil {
		t.Fatalf("FixScopeViolations error = %v", err)
	}
	want := []retryladder.ScopeViolation{{Code: retryladder.ScopeFileOutside, File: "go/internal/core/other.go"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("violations = %+v, want %+v", got, want)
	}
}

func TestC2047_004_FixChangingLinesOutsideTheCitedLinesFailsTheScopeGuard(t *testing.T) {
	spec := citedSpec(t)
	got, err := retryladder.FixScopeViolations(spec, diffFor("go/internal/retryladder/audit_fix.go", "@@ -103,3 +103,3 @@\n-a\n+b"))
	if err != nil {
		t.Fatalf("FixScopeViolations error = %v", err)
	}
	if len(got) != 1 || got[0].Code != retryladder.ScopeLinesOutside || got[0].File != "go/internal/retryladder/audit_fix.go" {
		t.Errorf("violations = %+v, want one lines_out_of_scope on audit_fix.go", got)
	}
}

func TestC2047_005_FixInsideTheSpecPassesTheScopeGuard(t *testing.T) {
	spec := citedSpec(t)
	diff := diffFor("go/internal/retryladder/audit_fix.go", "@@ -100,5 +100,3 @@\n-a\n+b") +
		diffFor("go/internal/retryladder/decide.go", "@@ -20 +20 @@\n-a\n+b")
	got, err := retryladder.FixScopeViolations(spec, diff)
	if err != nil || len(got) != 0 {
		t.Errorf("violations = %+v, error = %v, want none: a guard that flags every diff passes the negative predicates", got, err)
	}
}

func TestC2047_006_NewExportsOfRetryladderAreNamedByApicoverGate(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	test := exec.CommandContext(ctx, "go", "test", "-count=1", "-coverprofile="+profile, "./internal/retryladder")
	test.Dir = goDir
	test.WaitDelay = 10 * time.Second
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test ./internal/retryladder failed: %v\n%s", err, out)
	}
	funcs := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+profile)
	funcs.Dir = goDir
	funcs.WaitDelay = 10 * time.Second
	funcOut, err := funcs.Output()
	if err != nil {
		t.Fatalf("go tool cover -func: %v", err)
	}
	funcFile := filepath.Join(t.TempDir(), "cover.func.txt")
	if err := os.WriteFile(funcFile, funcOut, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := apicover.Main([]string{"-enforce", "-cover", funcFile, filepath.Join(goDir, "internal", "retryladder")}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("apicover -enforce exit = %d, want 0: every export must be named by a test that runs\n%s%s", code, stdout.String(), stderr.String())
	}
}
