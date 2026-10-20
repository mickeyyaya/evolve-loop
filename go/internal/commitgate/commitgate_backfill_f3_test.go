package commitgate

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const reviewedF3 = "code-simplifier,code-reviewer"

func runnerListingF3(files string, rules ...scriptRule) (*scriptRunner, sysexec.RunFunc) {
	base := []scriptRule{
		{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: strings.ReplaceAll(files, "\n", "\x00")},
		{matchPrefix: "git diff --name-only HEAD", stdout: files},
		{matchPrefix: "git diff HEAD", stdout: "diff --git a/f b/f\n", exit: 1},
	}
	sr := &scriptRunner{rules: append(rules, base...)}
	return sr, sr.run()
}

func gateF3(t *testing.T, files string, present []string, rules ...scriptRule) (Options, *scriptRunner, string) {
	t.Helper()
	root := t.TempDir()
	for _, f := range strings.Fields(files) {
		mustWrite(t, filepath.Join(root, f), "x\n")
	}
	o := baseOpts(root, present...)
	o.Reviewers = reviewedF3
	o.Env = os.Environ()
	sr, run := runnerListingF3(files, rules...)
	o.Runner = run
	return o, sr, root
}

func calledF3(sr *scriptRunner, prefix string) bool {
	for _, c := range sr.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func TestRun_EachLanguageDispatchesToItsOwnLaneAndStopsOnItsFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		file     string
		extra    map[string]string
		present  []string
		rule     scriptRule
		wantCode int
		wantCall string
	}{
		{name: "python lane fails", file: "a.py", present: []string{"shasum", "ruff"}, rule: scriptRule{matchPrefix: "ruff check", exit: 1}, wantCode: ExitFail, wantCall: "ruff check"},
		{name: "node lane fails", file: "a.js", present: []string{"shasum", "eslint"}, rule: scriptRule{matchPrefix: "eslint", exit: 1}, wantCode: ExitFail, wantCall: "eslint"},
		{name: "typescript routes to the node lane", file: "a.ts", present: []string{"shasum", "eslint"}, rule: scriptRule{matchPrefix: "eslint", exit: 1}, wantCode: ExitFail, wantCall: "eslint"},
		{name: "rust lane fails", file: "a.rs", extra: map[string]string{"Cargo.toml": "[package]\n"}, present: []string{"shasum", "cargo"}, rule: scriptRule{matchPrefix: "cargo fmt", exit: 1}, wantCode: ExitFail, wantCall: "cargo fmt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, sr, root := gateF3(t, tc.file, tc.present, tc.rule)
			for name, body := range tc.extra {
				mustWrite(t, filepath.Join(root, name), body)
			}

			res := o.Run(context.Background())

			if res.ExitCode != tc.wantCode || res.Attestation != nil {
				t.Fatalf("ExitCode=%d attestation=%v logs=%v", res.ExitCode, res.Attestation, res.Logs)
			}
			if !calledF3(sr, tc.wantCall) {
				t.Fatalf("the %s lane never ran: %v", tc.name, sr.calls)
			}
			if _, err := os.Stat(filepath.Join(root, ".commit-gate", "attestation.json")); err == nil {
				t.Fatal("a failing lane must not leave an attestation")
			}
		})
	}
}

func TestRun_APassingNonGoLaneStillStampsTheAttestation(t *testing.T) {
	t.Parallel()
	o, _, root := gateF3(t, "a.py", []string{"shasum", "ruff"}, scriptRule{matchPrefix: "ruff check"})

	res := o.Run(context.Background())

	if res.ExitCode != ExitPass || !reflect.DeepEqual(res.ChecksPassed, []string{"python:ruff"}) || res.Attestation == nil {
		t.Fatalf("ExitCode=%d checks=%v logs=%v", res.ExitCode, res.ChecksPassed, res.Logs)
	}
	if _, err := os.Stat(filepath.Join(root, ".commit-gate", "attestation.json")); err != nil {
		t.Fatalf("attestation: %v", err)
	}
}

func TestRun_AttestationFailuresExitGitFatalWithTheirReason(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		present []string
		tweak   func(t *testing.T, o *Options, root string)
		rules   []scriptRule
		wantLog string
	}{
		{name: "tree sha refused by git", present: []string{"shasum"}, rules: []scriptRule{{matchPrefix: "git diff HEAD", exit: 128}}, wantLog: "cannot compute tree SHA"},
		{name: "tree sha runner error", present: []string{"shasum"}, rules: []scriptRule{{matchPrefix: "git diff HEAD", err: errors.New("spawn failed")}}, wantLog: "cannot compute tree SHA"},
		{name: "no hasher on the host", present: nil, wantLog: "no shasum/sha256sum"},
		{name: "attestation dir is a file", present: []string{"shasum"}, tweak: func(t *testing.T, o *Options, root string) {
			blocker := filepath.Join(root, "blocker")
			mustWrite(t, blocker, "file")
			o.AttestDir = filepath.Join(blocker, "inside")
		}, wantLog: "cannot write attestation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, _, root := gateF3(t, "README.md", tc.present, tc.rules...)
			o.Reviewers = "docs-review"
			if tc.tweak != nil {
				tc.tweak(t, &o, root)
			}

			res := o.Run(context.Background())

			if res.ExitCode != ExitGitFatal || res.Attestation != nil {
				t.Fatalf("ExitCode=%d attestation=%v logs=%v", res.ExitCode, res.Attestation, res.Logs)
			}
			if !strings.Contains(strings.Join(res.Logs, "\n"), tc.wantLog) {
				t.Fatalf("logs %v lack %q", res.Logs, tc.wantLog)
			}
		})
	}
}

func TestHasherName_PrefersShasumThenSha256sumThenNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		present []string
		want    string
	}{
		{[]string{"shasum", "sha256sum"}, "shasum"},
		{[]string{"sha256sum"}, "sha256sum"},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := baseOpts(t.TempDir(), tc.present...).hasherName(); got != tc.want {
			t.Errorf("hasherName with %v = %q, want %q", tc.present, got, tc.want)
		}
	}
}

func TestRun_Sha256sumHostStampsItsToolName(t *testing.T) {
	t.Parallel()
	o, _, _ := gateF3(t, "README.md", []string{"sha256sum"})
	o.Reviewers = "docs-review"

	res := o.Run(context.Background())

	if res.ExitCode != ExitPass || res.Attestation == nil || res.Attestation.Tool != "sha256sum" {
		t.Fatalf("ExitCode=%d attestation=%+v logs=%v", res.ExitCode, res.Attestation, res.Logs)
	}
}

func TestRunCmd_ARunnerErrorIsAFailureThatCarriesTheErrorText(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir())
	o.Runner = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return -1, errors.New("spawn exploded")
	}

	out, ok := o.runCmd(context.Background(), o.RepoRoot, "tool")

	if ok || !strings.Contains(out, "spawn exploded") {
		t.Fatalf("out=%q ok=%v", out, ok)
	}
}

func TestRunCmd_SuccessJoinsStdoutAndStderrAndReportsExitZeroOnly(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir())
	exit := 0
	o.Runner = func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, "out")
		_, _ = io.WriteString(stderr, "err")
		return exit, nil
	}

	out, ok := o.runCmd(context.Background(), o.RepoRoot, "tool")
	if !ok || out != "out\nerr" {
		t.Fatalf("out=%q ok=%v", out, ok)
	}
	exit = 3
	if _, ok := o.runCmd(context.Background(), o.RepoRoot, "tool"); ok {
		t.Fatal("a non-zero exit is a failure")
	}
}

func TestAttestationMarshalWith_ReturnsTheFailingFieldAndNoBytes(t *testing.T) {
	t.Parallel()
	boom := errors.New("marshal boom")
	calls := 0
	a := &Attestation{TreeStateSHA: "s", TS: "t", Tool: "shasum"}

	raw, err := a.marshalWith(func(v any) ([]byte, error) {
		calls++
		if calls == 3 {
			return nil, boom
		}
		return []byte(`""`), nil
	})

	if raw != nil || !errors.Is(err, boom) || !strings.Contains(err.Error(), "attestation field checks_passed") {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
}

func TestReviewWaiver_RefusesWhenTheChangeCannotBeListed(t *testing.T) {
	t.Parallel()
	cases := map[string]scriptRule{
		"runner error":   {matchPrefix: "git diff --name-only --no-renames -z HEAD", err: errors.New("spawn failed")},
		"git fatal exit": {matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 128},
	}
	for name, rule := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := baseOpts(t.TempDir())
			o.Runner = (&scriptRunner{rules: []scriptRule{rule}}).run()

			waived, refused := o.reviewWaiver(context.Background())

			if waived != "" || refused != "the change could not be listed" {
				t.Fatalf("waived=%q refused=%q", waived, refused)
			}
		})
	}
}

func TestReviewWaiver_ARefusalNamesWhyTheGoFileCannotBeProven(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		atHead string
		onDisk string
		want   string
	}{
		{name: "after does not parse", atHead: commentedAtHead, onDisk: "package x\nfunc {\n", want: "x.go: after:"},
		{name: "before does not parse", atHead: "package x\nfunc {\n", onDisk: plainTrimmed, want: "x.go: before:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, "x.go"), tc.onDisk)
			o := baseOpts(root)
			o.Runner = (&scriptRunner{rules: []scriptRule{
				{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: "x.go\x00"},
				{matchPrefix: "git show HEAD:x.go", stdout: tc.atHead},
			}}).run()

			waived, refused := o.reviewWaiver(context.Background())

			if waived != "" || !strings.HasPrefix(refused, tc.want) {
				t.Fatalf("waived=%q refused=%q", waived, refused)
			}
		})
	}
}

func TestReviewWaiver_AnUnreadableGoFileIsRefusedWithTheOsError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file; the unreadable-file refusal cannot be arranged")
	}
	root := t.TempDir()
	path := filepath.Join(root, "x.go")
	mustWrite(t, path, plainTrimmed)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	o := baseOpts(root)
	o.Runner = (&scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: "x.go\x00"},
		{matchPrefix: "git show HEAD:x.go", stdout: commentedAtHead},
	}}).run()

	waived, refused := o.reviewWaiver(context.Background())

	if waived != "" || !strings.HasPrefix(refused, "x.go: ") || !strings.Contains(refused, "permission denied") {
		t.Fatalf("waived=%q refused=%q", waived, refused)
	}
}

func TestRunGolangciLint_FailureAndDeadlineAreToldApart(t *testing.T) {
	t.Parallel()
	t.Run("lint failure carries the output", func(t *testing.T) {
		t.Parallel()
		o := baseOpts(t.TempDir())
		o.Runner = (&scriptRunner{rules: []scriptRule{{matchPrefix: "golangci-lint run", exit: 1, stdout: "x.go:1: unused\n"}}}).run()
		res := &Result{}

		ok := o.runGolangciLint(context.Background(), o.RepoRoot, []string{"./."}, res)

		joined := strings.Join(res.Logs, "\n")
		if ok || !strings.Contains(joined, "golangci-lint failed") || !strings.Contains(joined, "x.go:1: unused") || strings.Contains(joined, "was stopped") {
			t.Fatalf("ok=%v logs=%v", ok, res.Logs)
		}
	})
	t.Run("a run past the budget is stopped and named", func(t *testing.T) {
		t.Parallel()
		o := baseOpts(t.TempDir())
		o.LintBudget = 20 * time.Millisecond
		o.Runner = func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
			<-ctx.Done()
			return -1, ctx.Err()
		}
		res := &Result{}

		ok := o.runGolangciLint(context.Background(), o.RepoRoot, []string{"./."}, res)

		if ok || !strings.Contains(strings.Join(res.Logs, "\n"), "waited 20ms") {
			t.Fatalf("ok=%v logs=%v", ok, res.Logs)
		}
	})
	t.Run("a clean run passes", func(t *testing.T) {
		t.Parallel()
		o := baseOpts(t.TempDir())
		o.Runner = (&scriptRunner{}).run()
		res := &Result{}

		if !o.runGolangciLint(context.Background(), o.RepoRoot, []string{"./."}, res) || len(res.Logs) != 0 {
			t.Fatalf("logs=%v", res.Logs)
		}
	})
}

func goModuleF3(t *testing.T, tools ...string) (Options, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	return baseOpts(root, tools...), root
}

func TestLaneGo_StopsAtTheFirstFailingStep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		present  []string
		rules    []scriptRule
		wantCode int
		wantLog  string
		wantNot  string
	}{
		{name: "go is not installed", present: nil, wantCode: ExitToolMissing, wantLog: "install Go from"},
		{name: "go vet fails", present: []string{"go"}, rules: []scriptRule{{matchPrefix: "go vet", exit: 1, stdout: "vet says no"}}, wantCode: ExitFail, wantLog: "go vet failed in", wantNot: "go test"},
		{name: "go test fails", present: []string{"go"}, rules: []scriptRule{{matchPrefix: "go test", exit: 1, stdout: "FAIL x"}}, wantCode: ExitFail, wantLog: "go test failed in"},
		{name: "golangci-lint fails", present: []string{"go", "golangci-lint"}, rules: []scriptRule{{matchPrefix: "golangci-lint run", exit: 1}}, wantCode: ExitFail, wantLog: "golangci-lint failed", wantNot: "go test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, _ := goModuleF3(t, tc.present...)
			sr := &scriptRunner{rules: tc.rules}
			o.Runner = sr.run()
			res := &Result{}

			code := o.laneGo(context.Background(), []string{"x.go"}, res)

			if code != tc.wantCode || !strings.Contains(strings.Join(res.Logs, "\n"), tc.wantLog) {
				t.Fatalf("code=%d logs=%v", code, res.Logs)
			}
			if tc.wantNot != "" && calledF3(sr, tc.wantNot) {
				t.Fatalf("%s must not run after the failure: %v", tc.wantNot, sr.calls)
			}
			if slicesContainsF3(res.ChecksPassed, "go:test") {
				t.Fatalf("a failed lane records no go:test: %v", res.ChecksPassed)
			}
		})
	}
}

func slicesContainsF3(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}

func TestLaneGo_NoExistingGoFileIsANoOp(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "go")
	sr := &scriptRunner{}
	o.Runner = sr.run()

	if code := o.laneGo(context.Background(), []string{"gone.go", "README.md"}, &Result{}); code != ExitPass || len(sr.calls) != 0 {
		t.Fatalf("code=%d calls=%v", code, sr.calls)
	}
}

func TestGoPackagesByModule_ARelativizeFailureFailsTheLaneNamingTheFile(t *testing.T) {
	t.Parallel()
	o, _ := goModuleF3(t)
	o.relPath = func(string, string) (string, error) { return "", errors.New("no relative path") }
	res := &Result{}

	byMod, inModule, code := o.goPackagesByModule([]string{"x.go"}, res)

	if code != ExitFail || !inModule || byMod != nil || !strings.Contains(strings.Join(res.Logs, "\n"), "go: cannot relativize x.go") {
		t.Fatalf("byMod=%v inModule=%v code=%d logs=%v", byMod, inModule, code, res.Logs)
	}
}

func TestOptionsRel_UsesTheSeamWhenSetAndFilepathRelOtherwise(t *testing.T) {
	t.Parallel()
	got, err := Options{}.rel("/a", "/a/b/c")
	if err != nil || got != filepath.Join("b", "c") {
		t.Fatalf("default rel = %q %v", got, err)
	}
	seam := Options{relPath: func(base, target string) (string, error) { return base + "|" + target, nil }}
	if got, _ := seam.rel("p", "q"); got != "p|q" {
		t.Fatalf("seam rel = %q", got)
	}
}

func TestLanePython_PytestMissingIsAToolMissingExit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "test_a.py"), "def test_x(): pass\n")
	o := baseOpts(root, "ruff")
	o.NoInstall = true
	o.Runner = (&scriptRunner{}).run()
	res := &Result{}

	code := o.lanePython(context.Background(), []string{"test_a.py"}, res)

	if code != ExitToolMissing || !strings.Contains(strings.Join(res.Logs, "\n"), "pip install pytest") || !reflect.DeepEqual(res.ChecksPassed, []string{"python:ruff"}) {
		t.Fatalf("code=%d logs=%v checks=%v", code, res.Logs, res.ChecksPassed)
	}
}

func TestLaneRust_EachCargoStepFailsTheLaneAndSkipsTheRest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		failing  string
		wantCall []string
		wantNot  string
	}{
		{name: "fmt", failing: "cargo fmt --check", wantCall: []string{"cargo fmt --check"}, wantNot: "cargo clippy"},
		{name: "clippy", failing: "cargo clippy", wantCall: []string{"cargo fmt --check", "cargo clippy"}, wantNot: "cargo test"},
		{name: "test", failing: "cargo test", wantCall: []string{"cargo fmt --check", "cargo clippy", "cargo test"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, "Cargo.toml"), "[package]\n")
			mustWrite(t, filepath.Join(root, "src", "main.rs"), "fn main() {}\n")
			o := baseOpts(root, "cargo")
			sr := &scriptRunner{rules: []scriptRule{{matchPrefix: tc.failing, exit: 1, stdout: "bad"}}}
			o.Runner = sr.run()
			res := &Result{}

			code := o.laneRust(context.Background(), []string{"src/main.rs"}, res)

			if code != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "cargo checks failed in "+root) || len(res.ChecksPassed) != 0 {
				t.Fatalf("code=%d logs=%v checks=%v", code, res.Logs, res.ChecksPassed)
			}
			for _, want := range tc.wantCall {
				if !calledF3(sr, want) {
					t.Fatalf("%s never ran: %v", want, sr.calls)
				}
			}
			if tc.wantNot != "" && calledF3(sr, tc.wantNot) {
				t.Fatalf("%s must not run after %s failed: %v", tc.wantNot, tc.failing, sr.calls)
			}
		})
	}
}
