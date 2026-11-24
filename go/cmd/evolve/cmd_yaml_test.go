package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	validWorkflowYAML  = "name: ci\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n"
	brokenWorkflowYAML = "on: push\njobs: a: b\n"
	yamlFloorCommand   = "go run ./cmd/evolve yaml check"
)

func yamlRepo(t *testing.T, files map[string]string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	for path, body := range files {
		repo.Write(path, body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed", "--allow-empty")
	return repo
}

func yamlRun(dir string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = runYAMLWith(yamlEnv{dir: dir, run: sysexec.DefaultRunner}, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestYAMLCheck_FailsOnASyntaxErrorAndNamesFileAndLine(t *testing.T) {
	t.Parallel()
	repo := yamlRepo(t, map[string]string{".github/workflows/bad.yml": brokenWorkflowYAML})

	code, stdout, stderr := yamlRun(repo.Dir, "check")

	if code != 1 || !strings.Contains(stdout, ".github/workflows/bad.yml:2: ") {
		t.Fatalf("exit=%d stdout=%q stderr=%q; want exit 1 naming .github/workflows/bad.yml:2", code, stdout, stderr)
	}
}

func TestYAMLCheck_FailsOnAWorkflowThatBreaksARule(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ body, want string }{
		"no jobs":          {"on: push\n", "missing key jobs"},
		"no runner in job": {"on: push\njobs:\n  a:\n    name: x\n", "job a needs runs-on or uses"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := yamlRepo(t, map[string]string{".github/workflows/w.yml": tc.body})

			code, stdout, _ := yamlRun(repo.Dir, "check")

			if code != 1 || !strings.Contains(stdout, tc.want) {
				t.Errorf("exit=%d stdout=%q; want exit 1 with %q", code, stdout, tc.want)
			}
		})
	}
}

func TestYAMLCheck_PassesAValidWorkflowAndAPlainFile(t *testing.T) {
	t.Parallel()
	repo := yamlRepo(t, map[string]string{".github/workflows/w.yml": validWorkflowYAML, "data/x.yaml": "a: 1\n", "notes.txt": "[not yaml"})

	code, stdout, stderr := yamlRun(repo.Dir, "check")

	if code != 0 || !strings.Contains(stdout, "2 files") {
		t.Errorf("exit=%d stdout=%q stderr=%q; want exit 0 over the 2 yaml files", code, stdout, stderr)
	}
}

func TestYAMLCheck_ChecksAnUntrackedFileAndSkipsAnUnstagedDeletion(t *testing.T) {
	t.Parallel()
	repo := yamlRepo(t, map[string]string{".github/workflows/w.yml": validWorkflowYAML})
	repo.Write(".github/workflows/fresh.yml", brokenWorkflowYAML)

	code, stdout, _ := yamlRun(repo.Dir, "check")
	if code != 1 || !strings.Contains(stdout, "fresh.yml:2") {
		t.Fatalf("untracked broken workflow: exit=%d stdout=%q; want exit 1 naming fresh.yml", code, stdout)
	}

	if err := os.Remove(filepath.Join(repo.Dir, ".github/workflows/fresh.yml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repo.Dir, ".github/workflows/w.yml")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := yamlRun(repo.Dir, "check")
	if code != 0 {
		t.Errorf("unstaged deletion: exit=%d stdout=%q stderr=%q; want exit 0, a deleted file is not read", code, stdout, stderr)
	}
}

func TestYAMLCheck_DefaultRootIsTheTopLevelOfTheWorkingDirectory(t *testing.T) {
	t.Parallel()
	repo := yamlRepo(t, map[string]string{".github/workflows/bad.yml": brokenWorkflowYAML, "go/main.go": "package main\n"})

	code, stdout, _ := yamlRun(filepath.Join(repo.Dir, "go"), "check")

	if code != 1 || !strings.Contains(stdout, ".github/workflows/bad.yml:2") {
		t.Errorf("from go/: exit=%d stdout=%q; want the workflow above go/ checked", code, stdout)
	}
}

func TestYAMLCheck_ExplicitRootAndPathsSelectTheFiles(t *testing.T) {
	t.Parallel()
	repo := yamlRepo(t, map[string]string{"bad.yml": "ok: 1\nb: c: d\n", "good.yml": "a: 1\n"})

	code, stdout, _ := yamlRun(t.TempDir(), "check", "--root", repo.Dir, "good.yml")
	if code != 0 {
		t.Errorf("good.yml only: exit=%d stdout=%q; want 0", code, stdout)
	}
	code, stdout, _ = yamlRun(t.TempDir(), "check", "good.yml", "bad.yml", "--root", repo.Dir)
	if code != 1 || !strings.Contains(stdout, "bad.yml:2") || strings.Contains(stdout, "good.yml:") {
		t.Errorf("both paths: exit=%d stdout=%q; want exit 1 naming only bad.yml", code, stdout)
	}
	code, stdout, _ = yamlRun(t.TempDir(), "check", "--root", repo.Dir, "missing.yml")
	if code != 1 || !strings.Contains(stdout, "missing.yml:0: ") {
		t.Errorf("unreadable path: exit=%d stdout=%q; want exit 1 naming missing.yml", code, stdout)
	}
}

func TestYAMLCheck_RefusesBadUsageWithExit10(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"no verb":      nil,
		"unknown verb": {"lint"},
		"bad flag":     {"check", "--nope"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := yamlRun(t.TempDir(), args...)
			if code != 10 || stdout != "" || !strings.Contains(stderr, "usage: evolve yaml check") {
				t.Errorf("exit=%d stdout=%q stderr=%q; want exit 10 and the usage line on stderr", code, stdout, stderr)
			}
		})
	}
}

func TestYAMLCheck_ReportsAGitFailureWithExit2(t *testing.T) {
	t.Parallel()
	notARepo := t.TempDir()

	code, _, stderr := yamlRun(notARepo, "check")
	if code != 2 || !strings.Contains(stderr, "evolve yaml") {
		t.Errorf("default root outside a repository: exit=%d stderr=%q; want exit 2", code, stderr)
	}
	code, _, stderr = yamlRun(notARepo, "check", "--root", notARepo)
	if code != 2 || !strings.Contains(stderr, "evolve yaml") {
		t.Errorf("explicit root outside a repository: exit=%d stderr=%q; want exit 2", code, stderr)
	}
}

func TestYAMLCheck_IsRegisteredAndRunsThroughTheRegistry(t *testing.T) {
	t.Parallel()
	cmd := lookupCommand("yaml")
	if cmd == nil || cmd.Run == nil || cmd.Summary == "" {
		t.Fatalf("lookupCommand(\"yaml\") = %+v, want a runnable command with a summary", cmd)
	}
	repo := yamlRepo(t, map[string]string{"bad.yml": "ok: 1\nb: c: d\n"})
	var out, errOut bytes.Buffer

	code := cmd.Run([]string{"check", "--root", repo.Dir}, nil, &out, &errOut)

	if code != 1 || !strings.Contains(out.String(), "bad.yml:2") {
		t.Errorf("registry run: exit=%d stdout=%q stderr=%q; want exit 1 naming bad.yml", code, out.String(), errOut.String())
	}
}

func TestYAMLCheck_TheRealTreeIsClean(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer

	code := lookupCommand("yaml").Run([]string{"check", "--root", floorRepoRoot(t)}, nil, &out, &errOut)

	if code != 0 {
		t.Errorf("real tree: exit=%d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
}

func TestFloorRun_YAMLTierIsAStaticCommandWithNoRoot(t *testing.T) {
	t.Parallel()
	lines := floorDryRunLines(t, "run", "--dry-run", "--root", floorRepoRoot(t), "--tier", "yaml")["yaml"]

	if !slices.Equal(lines, []string{yamlFloorCommand}) {
		t.Errorf("yaml plan = %q, want exactly %q", lines, yamlFloorCommand)
	}
	if !strings.Contains(floorTierNames(), "yaml") {
		t.Errorf("floorTierNames() = %q, want it to include yaml", floorTierNames())
	}
}

func TestFloorRun_YAMLTierReachesTheWorkflowFilesFromEveryWorkingDirectory(t *testing.T) {
	t.Parallel()
	repo := yamlRepo(t, map[string]string{".github/workflows/bad.yml": brokenWorkflowYAML, "go/main.go": "package main\n"})
	cases := map[string]struct{ root, cwd string }{
		"repo root with --root .":   {".", repo.Dir},
		"go/ with --root ..":        {"..", filepath.Join(repo.Dir, "go")},
		"anywhere with an absolute": {repo.Dir, t.TempDir()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var ran []string
			exec := func(dir, command string) (string, int, error) {
				ran = append(ran, command)
				var out, errOut bytes.Buffer
				code := runYAMLWith(yamlEnv{dir: yamlResolve(tc.cwd, dir), run: sysexec.DefaultRunner}, strings.Fields(strings.TrimPrefix(command, "go run ./cmd/evolve yaml")), &out, &errOut)
				return out.String() + errOut.String(), code, nil
			}
			var out, errOut bytes.Buffer

			code := runFloorWith(exec, []string{"run", "--tier", "yaml", "--root", tc.root}, &out, &errOut)

			if code != 1 || !strings.Contains(out.String(), "RED yaml") || !strings.Contains(out.String(), ".github/workflows/bad.yml:2") {
				t.Errorf("exit=%d out=%q; want RED yaml naming .github/workflows/bad.yml:2", code, out.String())
			}
			if !slices.Equal(ran, []string{yamlFloorCommand}) {
				t.Errorf("ran %q, want only %q", ran, yamlFloorCommand)
			}
		})
	}
}

var floorTierLine = regexp.MustCompile("`([a-z0-9-]+)`")

func TestFloorRun_RegistrySummaryAndRuntimeReferenceNameEveryTierInOrder(t *testing.T) {
	t.Parallel()
	var want []string
	for _, tier := range floorTierTable {
		want = append(want, tier.name)
	}

	summary := lookupCommand("floor").Summary
	if !strings.Contains(summary, "("+floorTierNames()+")") {
		t.Errorf("floor summary = %q, want it to hold the tier list (%s)", summary, floorTierNames())
	}

	raw, err := os.ReadFile(filepath.Join(floorRepoRoot(t), "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(raw), "The tiers are ")
	if !ok {
		t.Fatal("runtime-reference.md has no `The tiers are ...` sentence for evolve floor run")
	}
	sentence, _, _ := strings.Cut(after, ". ")
	var got []string
	for _, m := range floorTierLine.FindAllStringSubmatch(sentence, -1) {
		got = append(got, m[1])
	}
	if !slices.Equal(got, want) {
		t.Errorf("runtime-reference tier line = %v, want floorTierTable order %v", got, want)
	}
	if !strings.Contains(string(raw), "`ratchet` and `yaml` are floor-only tiers") {
		t.Error("runtime-reference.md does not mark `ratchet` and `yaml` as floor-only tiers")
	}
}

func yamlResolve(cwd, dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(cwd, dir)
}
