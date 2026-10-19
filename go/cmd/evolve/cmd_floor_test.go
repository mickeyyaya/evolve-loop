package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var floorTiers = []string{"fmt", "vet", "integration", "e2e", "acs-durable", "apicover", "cover", "ratchet"}

type floorCIStep struct {
	tier     string
	workflow string
	job      string
	want     string
	without  string
}

var floorCIParity = []floorCIStep{
	{tier: "fmt", workflow: "go.yml", job: "lint", want: "gofmt -d -s ."},
	{tier: "vet", workflow: "go.yml", job: "lint", want: "go vet ./...", without: "GOOS="},
	{tier: "vet", workflow: "go.yml", job: "lint", want: "GOOS=darwin go vet ./..."},
	{tier: "integration", workflow: "go.yml", job: "build-test", want: "make test-integration"},
	{tier: "e2e", workflow: "go.yml", job: "build-test", want: "make test-e2e"},
	{tier: "apicover", workflow: "go.yml", job: "build-test", want: "make apicover-check"},
	{tier: "cover", workflow: "go.yml", job: "build-test", want: "make cover-strict"},
	{tier: "acs-durable", workflow: "ci.yml", job: "acs-durable", want: "go test -count=1 -tags acs ./acs/regression/..."},
}

type floorWorkflow struct {
	Jobs map[string]struct {
		Uses  string `yaml:"uses"`
		Steps []struct {
			Run  string         `yaml:"run"`
			With map[string]any `yaml:"with"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func floorRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for ; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "required.yml")); err == nil {
			return dir
		}
		if dir == filepath.Dir(dir) {
			t.Fatal("no .github/workflows/required.yml above the test directory")
		}
	}
}

func loadFloorWorkflow(t *testing.T, root, name string) floorWorkflow {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var wf floorWorkflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return wf
}

func (wf floorWorkflow) jobRuns(job string) string {
	var runs []string
	for _, step := range wf.Jobs[job].Steps {
		runs = append(runs, step.Run)
	}
	return strings.Join(runs, "\n")
}

func (wf floorWorkflow) jobGoVersion(job string) string {
	for _, step := range wf.Jobs[job].Steps {
		if v, ok := step.With["go-version"]; ok {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func floorRun(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	cmd := lookupCommand("floor")
	if cmd == nil {
		t.Fatal("RED: no `floor` command in the registry")
	}
	var out, errOut bytes.Buffer
	code = cmd.Run(args, nil, &out, &errOut)
	return code, out.String(), errOut.String()
}

func floorDryRunLines(t *testing.T, args ...string) map[string][]string {
	t.Helper()
	code, stdout, stderr := floorRun(t, args...)
	if code != 0 {
		t.Fatalf("floor %v exit=%d, want 0\nstdout:\n%s\nstderr:\n%s", args, code, stdout, stderr)
	}
	byTier := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		tier, command, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatalf("dry-run line %q is not `<tier>: <command>`", line)
		}
		byTier[tier] = append(byTier[tier], command)
	}
	return byTier
}

func plannedTiers(byTier map[string][]string) []string {
	var tiers []string
	for tier := range byTier {
		tiers = append(tiers, tier)
	}
	slices.Sort(tiers)
	return tiers
}

func TestFloorRun_IsRegisteredAndDispatchesThroughTheRegistry(t *testing.T) {
	t.Parallel()
	cmd := lookupCommand("floor")
	if cmd == nil || cmd.Run == nil {
		t.Fatalf("lookupCommand(\"floor\") = %+v, want a runnable command", cmd)
	}
	if cmd.Summary == "" {
		t.Error("floor has an empty Summary, so `evolve help` cannot describe it")
	}
}

func TestFloorRun_DryRunDefaultCoversEveryTier(t *testing.T) {
	t.Parallel()
	byTier := floorDryRunLines(t, "run", "--dry-run", "--root", floorRepoRoot(t))
	got := plannedTiers(byTier)
	want := slices.Clone(floorTiers)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("default tiers = %v, want %v", got, want)
	}
}

func TestFloorRun_DryRunTierFilter(t *testing.T) {
	t.Parallel()
	root := floorRepoRoot(t)
	cases := []struct {
		name  string
		tiers string
		want  []string
	}{
		{"single", "fmt", []string{"fmt"}},
		{"pair", "fmt,vet", []string{"fmt", "vet"}},
		{"hyphenated", "acs-durable,ratchet", []string{"acs-durable", "ratchet"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			byTier := floorDryRunLines(t, "run", "--dry-run", "--root", root, "--tier", tc.tiers)
			got := plannedTiers(byTier)
			if !slices.Equal(got, tc.want) {
				t.Errorf("--tier %s planned %v, want %v", tc.tiers, got, tc.want)
			}
		})
	}
}

func TestFloorRun_RefusesWithExit2(t *testing.T) {
	t.Parallel()
	root := floorRepoRoot(t)
	cases := []struct {
		name     string
		args     []string
		mentions string
	}{
		{"unknown tier", []string{"run", "--dry-run", "--root", root, "--tier", "fmt,bogus"}, "bogus"},
		{"no verb", nil, "usage"},
		{"unknown verb", []string{"bogus"}, "usage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := floorRun(t, tc.args...)
			if code != 2 {
				t.Errorf("exit=%d, want 2\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if !strings.Contains(stderr, tc.mentions) {
				t.Errorf("stderr %q does not mention %q", stderr, tc.mentions)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("a refused invocation printed a plan or verdict:\n%s", stdout)
			}
		})
	}
}

func TestFloorRun_TierCommandsMatchRequiredCI(t *testing.T) {
	t.Parallel()
	root := floorRepoRoot(t)
	required := loadFloorWorkflow(t, root, "required.yml")
	byTier := floorDryRunLines(t, "run", "--dry-run", "--root", root)

	for _, step := range floorCIParity {
		t.Run(step.tier+"/"+step.want, func(t *testing.T) {
			t.Parallel()
			reusedByRequired := false
			for _, job := range required.Jobs {
				reusedByRequired = reusedByRequired || job.Uses == "./.github/workflows/"+step.workflow
			}
			if !reusedByRequired {
				t.Fatalf("required.yml no longer calls %s, so the table is stale", step.workflow)
			}
			wf := loadFloorWorkflow(t, root, step.workflow)
			if !strings.Contains(wf.jobRuns(step.job), step.want) {
				t.Fatalf("%s job %q has no step running %q, so the table is stale", step.workflow, step.job, step.want)
			}
			found := slices.ContainsFunc(byTier[step.tier], func(line string) bool {
				return strings.Contains(line, step.want) && (step.without == "" || !strings.Contains(line, step.without))
			})
			if !found {
				t.Errorf("tier %s plans %q, none runs CI's %q", step.tier, byTier[step.tier], step.want)
			}
		})
	}
}

func TestFloorRun_FmtTierUsesTheToolchainCIPins(t *testing.T) {
	t.Parallel()
	root := floorRepoRoot(t)
	pinned := loadFloorWorkflow(t, root, "go.yml").jobGoVersion("lint")
	if pinned == "" {
		t.Fatal("go.yml lint job pins no go-version, so the table is stale")
	}
	lines := floorDryRunLines(t, "run", "--dry-run", "--root", root, "--tier", "fmt")["fmt"]
	if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, pinned) }) {
		t.Errorf("fmt plan %q never names CI's pinned toolchain %q", lines, pinned)
	}
}

func TestFloorRun_RatchetTierRunsTheRatchetCheck(t *testing.T) {
	t.Parallel()
	lines := floorDryRunLines(t, "run", "--dry-run", "--root", floorRepoRoot(t), "--tier", "ratchet")["ratchet"]
	if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, "ratchet check") }) {
		t.Errorf("ratchet plan %q does not run `ratchet check`", lines)
	}
}
