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

	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
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
	{tier: "acs-durable", workflow: "ci.yml", job: "acs-durable", want: "make vet-acs"},
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

func makeRecipe(t *testing.T, goRoot, target string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goRoot, "Makefile"))
	if err != nil {
		t.Fatalf("read go/Makefile: %v", err)
	}
	var recipe []string
	found, inTarget := false, false
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "\t") {
			inTarget = strings.HasPrefix(line, target+":")
			found = found || inTarget
			continue
		}
		if !inTarget {
			continue
		}
		line = strings.TrimSpace(line)
		if n := len(recipe); n > 0 && strings.HasSuffix(recipe[n-1], "\\") {
			recipe[n-1] = strings.TrimSuffix(recipe[n-1], "\\") + " " + line
			continue
		}
		recipe = append(recipe, line)
	}
	if !found {
		t.Fatalf("go/Makefile has no %s target", target)
	}
	return recipe
}

func floorAcsVetPatterns(recipe []string) []string {
	var patterns []string
	for _, line := range recipe {
		fields := strings.Fields(line)
		vet := slices.Index(fields, "vet")
		if vet < 0 || !slices.Equal(fields[vet+1:min(vet+3, len(fields))], []string{"-tags", "acs"}) {
			continue
		}
		for _, f := range fields[vet+3:] {
			if gopkgpattern.IsPackagePattern(f) {
				patterns = append(patterns, f)
			}
		}
	}
	return patterns
}

func floorPatternCovers(patterns []string, dir string) bool {
	dirKey := gopkgpattern.Key(dir)
	return slices.ContainsFunc(patterns, func(p string) bool {
		key := gopkgpattern.Key(p)
		if !gopkgpattern.IsRecursive(p) {
			return dirKey == key
		}
		return key == "" || dirKey == key || strings.HasPrefix(dirKey, key+"/")
	})
}

func floorGoListSkips(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".")
}

func floorAcsPackageDirs(t *testing.T, goRoot string) []string {
	t.Helper()
	seen := map[string]bool{}
	root := filepath.Join(goRoot, "acs")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root && floorGoListSkips(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(goRoot, filepath.Dir(path))
		seen["./"+filepath.ToSlash(rel)] = true
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	return dirs
}

func TestMakefile_VetAcsCoversEveryAcsPackage(t *testing.T) {
	t.Parallel()
	goRoot := filepath.Join(floorRepoRoot(t), "go")
	patterns := floorAcsVetPatterns(makeRecipe(t, goRoot, "vet-acs"))
	if len(patterns) == 0 {
		t.Fatal("go/Makefile vet-acs runs no `vet -tags acs <pattern>`: the compile guard is gone, or the recipe form changed and this parser must follow it")
	}
	dirs := floorAcsPackageDirs(t, goRoot)
	if len(dirs) < 2 {
		t.Fatalf("found %d acs package dirs, want the regression tier and the frozen cycle packages", len(dirs))
	}
	for _, dir := range dirs {
		if !floorPatternCovers(patterns, dir) {
			t.Errorf("vet-acs patterns %q skip %s, so a broken frozen predicate there reaches main", patterns, dir)
		}
	}
}

func TestFloorAcsVetPatterns_ReadsOnlyTheAcsTaggedVet(t *testing.T) {
	t.Parallel()
	recipe := []string{
		"$(GO) vet -tags acs ./acs/... ./x $(ACS_PKGS) main.go",
		"$(GO) vet ./other/...",
		"$(GO) vet -tags",
		"echo vet -tags e2e ./e2e/...",
	}
	if got, want := floorAcsVetPatterns(recipe), []string{"./acs/...", "./x"}; !slices.Equal(got, want) {
		t.Errorf("patterns = %q, want %q", got, want)
	}
	for dir, want := range map[string]bool{"./acs": true, "./acs/cycle16": true, "./acsx": false, "./x": true, "./x/y": false} {
		if got := floorPatternCovers([]string{"./acs/...", "./x"}, dir); got != want {
			t.Errorf("covers(%s) = %v, want %v", dir, got, want)
		}
	}
	if !floorPatternCovers([]string{"./..."}, "./acs/cycle16") {
		t.Error("the whole-module pattern does not cover ./acs/cycle16")
	}
	for name, want := range map[string]bool{"testdata": true, "_old": true, ".git": true, "cycle16": false} {
		if got := floorGoListSkips(name); got != want {
			t.Errorf("skips(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestMakeRecipe_JoinsContinuedLinesAndRequiresTheTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mk := "other:\n\techo no\nvet-acs: ## doc\n\t$(GO) vet -tags acs \\\n\t\t./acs/...\n\techo done\n\nnext:\n\techo no\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(mk), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{"$(GO) vet -tags acs  ./acs/...", "echo done"}
	if got := makeRecipe(t, dir, "vet-acs"); !slices.Equal(got, want) {
		t.Errorf("recipe = %q, want %q", got, want)
	}
	if got := floorAcsVetPatterns(makeRecipe(t, dir, "vet-acs")); !slices.Equal(got, []string{"./acs/..."}) {
		t.Errorf("patterns of a continued line = %q, want [./acs/...]", got)
	}
}
