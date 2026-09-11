//go:build integration

package verifyeval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("/bin/bash not present")
	}
}

func TestVerify_MultilineBashFenceSharesShellState(t *testing.T) {
	requireBash(t)
	workspace := t.TempDir()
	path := writeEval(t, "```bash\n"+
		"name=hello\\\n"+
		"world\n"+
		"mkdir child\n"+
		"cd child\n"+
		"test \"$name\" = helloworld &&\n"+
		"test \"$(basename \"$PWD\")\" = child\n"+
		"```\n")

	res, err := Verify(Options{Path: path, Workspace: workspace})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != "PASS" {
		t.Fatalf("Verdict = %q, want PASS; commands=%+v", res.Verdict, res.Commands)
	}
	if len(res.Commands) != 1 {
		t.Fatalf("executed scripts = %d, want one fence execution", len(res.Commands))
	}
}

func TestVerify_MultilineBashFenceCanInspectPriorFailure(t *testing.T) {
	requireBash(t)
	path := writeEval(t, "```bash\nprobe() { return 7; }\nprobe\nstatus=$?\ntest \"$status\" -eq 7\n```\n")

	res, err := Verify(Options{Path: path, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != "PASS" {
		t.Fatalf("Verdict = %q, want PASS; commands=%+v", res.Verdict, res.Commands)
	}
}

func TestVerify_FinalPipelineFailureIsNotMasked(t *testing.T) {
	requireBash(t)
	path := writeEval(t, "```bash\nprobe() { return 7; }\nprobe | cat\n```\n")

	res, err := Verify(Options{Path: path, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != "FAIL" {
		t.Fatalf("Verdict = %q, want FAIL; commands=%+v", res.Verdict, res.Commands)
	}
}

func TestVerify_SeparateBashFencesUseSeparateShells(t *testing.T) {
	requireBash(t)
	path := writeEval(t, "```bash\nsecret=private\n```\n```bash\ntest -z \"${secret:-}\"\n```\n")

	res, err := Verify(Options{Path: path, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != "PASS" {
		t.Fatalf("Verdict = %q, want PASS; commands=%+v", res.Verdict, res.Commands)
	}
	if len(res.Commands) != 2 {
		t.Fatalf("executed scripts = %d, want two isolated fences", len(res.Commands))
	}
}

func TestVerify_ExplicitNonzeroAppliesToWholeScript(t *testing.T) {
	requireBash(t)
	path := writeEval(t, "```bash\nexit 3\nprintf 'masked success\\n'\n```\n\n## Expected\nexit_code: 3\n")

	res, err := Verify(Options{Path: path, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != "PASS" {
		t.Fatalf("Verdict = %q, want PASS; commands=%+v", res.Verdict, res.Commands)
	}
	if len(res.Commands) != 1 {
		t.Fatalf("executed scripts = %d, want one fence execution", len(res.Commands))
	}
}

func TestVerify_DefaultRunnerRequiresNarrowedGoTestExecution(t *testing.T) {
	requireBash(t)
	t.Setenv("GOCACHE", t.TempDir())
	t.Setenv("GOENV", "off")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "off")
	workspace := writeGoVerificationFixture(t)
	runner := func(ctx context.Context, workdir, command string) (string, string, int, error) {
		bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		return DefaultRunner(bounded, workdir, command)
	}
	tests := []struct {
		name        string
		command     string
		wantVerdict string
	}{
		{name: "local package has no match", command: "go test -count=1 -timeout=10s -run '^TestMissing$'", wantVerdict: "FAIL"},
		{name: "quoted capture has no match", command: "output=\"$(go test -count=1 -timeout=10s -run '^TestMissing$')\"\nprintf '%s\\n' \"$output\"", wantVerdict: "FAIL"},
		{name: "local package has a match", command: "go test -count=1 -timeout=10s -run '^TestPresent$'", wantVerdict: "PASS"},
		{name: "selected package has no test files", command: "go test -count=1 -timeout=10s -run '^TestMissing$' ./empty", wantVerdict: "FAIL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeEval(t, "```bash\n"+test.command+"\n```\n")
			res, err := Verify(Options{Path: path, Workspace: workspace, Runner: runner})
			if err != nil {
				t.Fatal(err)
			}
			if res.Verdict != test.wantVerdict {
				t.Fatalf("Verdict = %q, want %s; commands=%+v", res.Verdict, test.wantVerdict, res.Commands)
			}
		})
	}
}

func writeGoVerificationFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":         "module example.com/verifyevalfixture\n\ngo 1.23\n",
		"sample_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestPresent(t *testing.T) {}\n",
		"empty/empty.go": "package empty\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
