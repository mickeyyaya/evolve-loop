package commitgate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/sysexec"
)

// fixedClock returns a deterministic Now for reproducible `ts` fields.
func fixedClock() func() time.Time {
	t := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

// scriptRunner is a programmable sysexec.RunFunc: it matches commands by their
// "name arg0 arg1 ..." prefix and returns the configured (stdout, exit). The
// first matching rule wins; an unmatched command defaults to (exit 0, "").
type scriptRunner struct {
	rules []scriptRule
	calls []string
}

type scriptRule struct {
	matchPrefix string // matched against "name arg0 arg1 ..."
	stdout      string
	exit        int
	err         error
}

func (s *scriptRunner) run() sysexec.RunFunc {
	return func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		cmd := strings.TrimSpace(name + " " + strings.Join(args, " "))
		s.calls = append(s.calls, cmd)
		for _, r := range s.rules {
			if strings.HasPrefix(cmd, r.matchPrefix) {
				if stdout != nil && r.stdout != "" {
					_, _ = io.WriteString(stdout, r.stdout)
				}
				return r.exit, r.err
			}
		}
		return 0, nil
	}
}

// baseOpts builds an Options whose lookPath reports `present` tools as present
// (and everything else absent), with a deterministic clock.
func baseOpts(root string, present ...string) Options {
	have := map[string]bool{}
	for _, t := range present {
		have[t] = true
	}
	return Options{
		RepoRoot: root,
		Now:      fixedClock(),
		lookPath: func(tool string) (string, error) {
			if have[tool] {
				return "/usr/bin/" + tool, nil
			}
			return "", os.ErrNotExist
		},
	}
}

func TestDetectLangs_ExtensionMapping(t *testing.T) {
	t.Parallel()
	got := detectLangs([]string{
		"a.go", "b.py", "c.ts", "d.tsx", "e.js", "f.jsx", "g.mjs", "h.cjs", "i.rs",
		"README.md", "noext", "k.txt",
	})
	want := []string{"go", "js", "python", "rust", "ts"} // sorted-unique
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detectLangs = %v, want %v", got, want)
	}
}

func TestDetectLangs_Empty(t *testing.T) {
	t.Parallel()
	if got := detectLangs([]string{"x.md", "y"}); len(got) != 0 {
		t.Fatalf("want no langs, got %v", got)
	}
}

func TestReviewersSatisfied(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		reviewers string
		langs     []string
		want      bool
	}{
		{"simplify+general", "code-simplifier,code-reviewer", []string{"go"}, true},
		{"simplify+lang-reviewer", "code-simplifier,go-reviewer", []string{"go"}, true},
		{"ecc-prefix-stripped", "ecc:code-simplifier,ecc:go-reviewer", []string{"go"}, true},
		{"whitespace-stripped", "  code-simplifier , code-reviewer ", []string{"go"}, true},
		{"combined-capability", "code-review-simplify", []string{"go"}, true},
		{"refactor-as-simplify", "refactor,code-reviewer", []string{"python"}, true},
		{"missing-review", "code-simplifier", []string{"go"}, false},
		{"missing-simplify", "go-reviewer", []string{"go"}, false},
		{"wrong-lang-reviewer", "code-simplifier,go-reviewer", []string{"python"}, false},
		{"empty", "", []string{"go"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := Options{Reviewers: tc.reviewers}
			res := &Result{}
			if got := o.reviewersSatisfied(tc.langs, res); got != tc.want {
				t.Fatalf("reviewersSatisfied(%q, %v) = %v, want %v", tc.reviewers, tc.langs, got, tc.want)
			}
		})
	}
}

func TestSplitReviewers_RawVerbatim(t *testing.T) {
	t.Parallel()
	// Empties dropped, namespace prefixes KEPT (raw spelling), order preserved.
	got := splitReviewers("ecc:code-simplifier,,go-reviewer,")
	want := []string{"ecc:code-simplifier", "go-reviewer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitReviewers = %v, want %v", got, want)
	}
}

func TestRun_NoChangedFiles_ExitFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := baseOpts(root, "shasum")
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "git diff --name-only HEAD", stdout: "\n  \n"}}}
	o.Runner = sr.run()
	res := o.Run(context.Background())
	if res.ExitCode != ExitFail {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, ExitFail, res.Logs)
	}
}

func TestRun_ReviewerPreconditionFail_ExitFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = "code-simplifier" // missing review capability
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"}}}
	o.Runner = sr.run()
	res := o.Run(context.Background())
	if res.ExitCode != ExitFail {
		t.Fatalf("ExitCode = %d, want %d", res.ExitCode, ExitFail)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "missing required review capability") {
		t.Fatalf("expected DENY log, got %v", res.Logs)
	}
}

func TestRun_GoLanePass_WritesAttestation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Real go.mod + .go file so findUp/gofmt path resolution works.
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")

	o := baseOpts(root, "shasum", "go")
	o.Reviewers = "code-simplifier,go-reviewer"
	o.Env = os.Environ()
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff --git a/x.go b/x.go\n", exit: 1}, // tree-state SHA
		{matchPrefix: "gofmt -s -l", stdout: ""},                                      // formatted
		{matchPrefix: "go vet", exit: 0},
		{matchPrefix: "go test", exit: 0},
	}}
	o.Runner = sr.run()

	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	if res.Attestation == nil {
		t.Fatal("expected an attestation on pass")
	}
	wantChecks := []string{"go:gofmt", "go:vet", "go:test"}
	if !reflect.DeepEqual(res.ChecksPassed, wantChecks) {
		t.Fatalf("ChecksPassed = %v, want %v", res.ChecksPassed, wantChecks)
	}
	// Attestation landed on disk.
	if _, err := os.Stat(filepath.Join(root, ".commit-gate", "attestation.json")); err != nil {
		t.Fatalf("attestation not written: %v", err)
	}
	// reviewers_run records the RAW spelling.
	if !reflect.DeepEqual(res.Attestation.ReviewersRun, []string{"code-simplifier", "go-reviewer"}) {
		t.Fatalf("ReviewersRun = %v", res.Attestation.ReviewersRun)
	}
}

// TestRun_GeneralReviewerAlone_ExitPass is the full-pipeline successor to bash
// commit-gate-test.sh T4: the general `code-reviewer` (no language reviewer)
// satisfies the "one review" precondition end-to-end, so a clean Go change runs
// to ExitPass. TestReviewersSatisfied covers the precondition decision in
// isolation; this proves it through the whole Run.
func TestRun_GeneralReviewerAlone_ExitPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = "code-simplifier,code-reviewer" // general reviewer, no go-reviewer
	o.Env = os.Environ()
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff\n", exit: 1},
		{matchPrefix: "gofmt -s -l", stdout: ""},
		{matchPrefix: "go vet", exit: 0},
		{matchPrefix: "go test", exit: 0},
	}}
	o.Runner = sr.run()
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	// Prove the Go lane actually RAN — a regression that bypassed it would still
	// satisfy the precondition and leave ExitPass, so assert the recorded checks.
	if !reflect.DeepEqual(res.ChecksPassed, []string{"go:gofmt", "go:vet", "go:test"}) {
		t.Fatalf("ChecksPassed = %v, want [go:gofmt go:vet go:test]", res.ChecksPassed)
	}
}

// TestRun_EccPrefixedReviewer_ExitPass is the full-pipeline successor to bash
// commit-gate-test.sh T5: ECC namespace prefixes are stripped for the precondition
// (ecc:go-reviewer counts as go-reviewer), so a clean Go change runs to ExitPass —
// while reviewers_run records the RAW ecc:-prefixed spelling.
func TestRun_EccPrefixedReviewer_ExitPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = "ecc:code-simplifier,ecc:go-reviewer"
	o.Env = os.Environ()
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff\n", exit: 1},
		{matchPrefix: "gofmt -s -l", stdout: ""},
		{matchPrefix: "go vet", exit: 0},
		{matchPrefix: "go test", exit: 0},
	}}
	o.Runner = sr.run()
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	if !reflect.DeepEqual(res.ChecksPassed, []string{"go:gofmt", "go:vet", "go:test"}) {
		t.Fatalf("ChecksPassed = %v, want [go:gofmt go:vet go:test]", res.ChecksPassed)
	}
	// Precondition strips ecc: prefixes, but reviewers_run keeps the raw spelling.
	if !reflect.DeepEqual(res.Attestation.ReviewersRun, []string{"ecc:code-simplifier", "ecc:go-reviewer"}) {
		t.Fatalf("ReviewersRun = %v, want raw ecc:-prefixed", res.Attestation.ReviewersRun)
	}
}

func TestRun_GofmtUnformatted_ExitFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = "code-simplifier,go-reviewer"
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "gofmt -s -l", stdout: "x.go\n"}, // gofmt reports it unformatted
	}}
	o.Runner = sr.run()
	res := o.Run(context.Background())
	if res.ExitCode != ExitFail {
		t.Fatalf("ExitCode = %d, want %d", res.ExitCode, ExitFail)
	}
}

func TestRun_GolangciLintRecordedWhenPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go", "golangci-lint")
	o.Reviewers = "code-simplifier,go-reviewer"
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff\n", exit: 1},
		{matchPrefix: "gofmt -s -l", stdout: ""},
	}}
	o.Runner = sr.run()
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	want := []string{"go:gofmt", "go:vet", "go:golangci-lint", "go:test"}
	if !reflect.DeepEqual(res.ChecksPassed, want) {
		t.Fatalf("ChecksPassed = %v, want %v", res.ChecksPassed, want)
	}
}

func TestRun_AcsPackagesExcluded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	// A file UNDER an acs subpackage (./acs/predicate) — the bash awk excludes
	// /^\.\/acs\//, i.e. subpackages of acs, NOT the ./acs package itself.
	mustWrite(t, filepath.Join(root, "acs", "predicate", "p.go"), "package predicate\n")
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = "code-simplifier,go-reviewer"
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "acs/predicate/p.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff\n", exit: 1},
		{matchPrefix: "gofmt -s -l", stdout: ""},
	}}
	o.Runner = sr.run()
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	// acs/ subpackage excluded → no go vet / go test command actually ran.
	for _, c := range sr.calls {
		if strings.HasPrefix(c, "go vet") || strings.HasPrefix(c, "go test") {
			t.Fatalf("acs subpackage should be excluded, but ran: %q", c)
		}
	}
	// ...yet the bash runner records go:vet/go:test UNCONDITIONALLY after the
	// (empty) module loop, so the attestation byte-format stays identical. We
	// preserve that exactly: the checks are recorded though no command ran.
	if !reflect.DeepEqual(res.ChecksPassed, []string{"go:gofmt", "go:vet", "go:test"}) {
		t.Fatalf("ChecksPassed = %v, want [go:gofmt go:vet go:test]", res.ChecksPassed)
	}
}

func TestEnsureTool_ForceMissingSeam(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "ruff")
	o.ForceMissing = "ruff"
	res := &Result{}
	// ruff is on PATH but forced-missing; install seam unset → ExitToolMissing.
	if code := o.ensureTool("ruff", "pip install ruff", "pip install ruff", res); code != ExitToolMissing {
		t.Fatalf("ensureTool with ForceMissing = %d, want %d", code, ExitToolMissing)
	}
}

func TestEnsureTool_TestInstallOK(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir()) // ruff absent
	o.TestInstall = "ok"
	res := &Result{}
	if code := o.ensureTool("ruff", "pip install ruff", "pip install ruff", res); code != ExitPass {
		t.Fatalf("ensureTool TestInstall=ok = %d, want %d", code, ExitPass)
	}
}

func TestEnsureTool_TestInstallFail(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir()) // ruff absent
	o.TestInstall = "fail"
	res := &Result{}
	if code := o.ensureTool("ruff", "pip install ruff", "pip install ruff", res); code != ExitToolMissing {
		t.Fatalf("ensureTool TestInstall=fail = %d, want %d", code, ExitToolMissing)
	}
}

func TestEnsureTool_NotAutoInstallable(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir()) // go absent, no install cmd
	res := &Result{}
	if code := o.ensureTool("go", "", "install Go", res); code != ExitToolMissing {
		t.Fatalf("ensureTool not-installable = %d, want %d", code, ExitToolMissing)
	}
}

func TestRun_AttestationStableBytes(t *testing.T) {
	t.Parallel()
	att := &Attestation{
		TreeStateSHA: "abc123",
		TS:           "2026-06-18T12:00:00Z",
		ChecksPassed: []string{"go:gofmt", "go:vet", "go:test"},
		ReviewersRun: []string{"code-simplifier", "go-reviewer"},
		Tool:         "shasum",
	}
	want := `{
  "tree_state_sha": "abc123",
  "ts": "2026-06-18T12:00:00Z",
  "checks_passed": ["go:gofmt","go:vet","go:test"],
  "reviewers_run": ["code-simplifier","go-reviewer"],
  "tool": "shasum"
}
`
	if got := string(att.Marshal()); got != want {
		t.Fatalf("Marshal mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRun_EmptyArraysMarshalInline(t *testing.T) {
	t.Parallel()
	att := &Attestation{TreeStateSHA: "z", TS: "t", Tool: "sha256sum"}
	want := `{
  "tree_state_sha": "z",
  "ts": "t",
  "checks_passed": [],
  "reviewers_run": [],
  "tool": "sha256sum"
}
`
	if got := string(att.Marshal()); got != want {
		t.Fatalf("empty-array Marshal mismatch:\n%q", got)
	}
}

func TestRunner_TypeAliasSatisfiesSysexec(t *testing.T) {
	t.Parallel()
	// Runner is sysexec.RunFunc; a value of one is assignable to the other.
	var r Runner = sysexec.DefaultRunner
	var _ sysexec.RunFunc = r
	if r == nil {
		t.Fatal("Runner alias should hold DefaultRunner")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
