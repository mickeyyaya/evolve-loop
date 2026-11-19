//go:build acs

// Package cycle2536 holds the acceptance predicates of the S11 generator-template batch.
package cycle2536

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changeloggen"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/internal/versionbump"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	regionBegin  = regexp.MustCompile(`GENERATED:[a-z-]+ BEGIN`)
	regionEnd    = regexp.MustCompile(`GENERATED:[a-z-]+ END`)
	historyRow   = regexp.MustCompile(`(?m)^\| v[0-9]+\.[0-9]+ \| `)
	registryRow  = regexp.MustCompile("^\\| `([A-Za-z0-9_]+)` \\|")
	builtBinary  string
	buildOnce    sync.Once
	buildFailure error
)

func generatedRegions(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var kept []string
	inside := false
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case regionBegin.MatchString(line):
			inside = true
		case regionEnd.MatchString(line):
			inside = false
		case inside:
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func steFindings(t *testing.T, label, text string) []string {
	t.Helper()
	standard, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	lines := strings.Split(text, "\n")
	var out []string
	for _, f := range stelint.Check([]byte(text), stelint.Options{Words: standard}) {
		if f.IsSkipNotice() {
			continue
		}
		if f.Rule == stelint.RuleWord && f.Line >= 1 && f.Line <= len(lines) && strings.HasPrefix(lines[f.Line-1], "#") {
			continue
		}
		out = append(out, fmt.Sprintf("%s:%d [%s] %s", label, f.Line, f.Rule, f.Message))
	}
	return out
}

func requireSteClean(t *testing.T, label, text string) {
	t.Helper()
	if strings.TrimSpace(text) == "" {
		t.Fatalf("%s: no generated text found, so a lint pass proves nothing", label)
	}
	findings := steFindings(t, label, text)
	for i, f := range findings {
		if i == 5 {
			break
		}
		t.Log(f)
	}
	if len(findings) > 0 {
		t.Errorf("RED: %s has %d ASD-STE100 findings", label, len(findings))
	}
}

func nonTestGoSources(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	sources := map[string]string{}
	for _, dir := range dirs {
		walkErr := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel, _ := filepath.Rel(root, path)
			sources[filepath.ToSlash(rel)] = string(raw)
			return nil
		})
		if walkErr != nil {
			t.Fatalf("scan %s: %v", dir, walkErr)
		}
	}
	return sources
}

func blankRowsDefinedOnlyInProtectedFiles(t *testing.T, text string, dirs ...string) (string, int, int) {
	t.Helper()
	sources := nonTestGoSources(t, dirs...)
	lines := strings.Split(text, "\n")
	blanked, checked := 0, 0
	for i, line := range lines {
		m := registryRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		literal := `"` + m[1] + `"`
		holders, protected := 0, 0
		for rel, src := range sources {
			if strings.Contains(src, literal) {
				holders++
				if guards.IsProtectedScope(rel) {
					protected++
				}
			}
		}
		if holders > 0 && holders == protected {
			lines[i] = ""
			blanked++
			continue
		}
		checked++
	}
	return strings.Join(lines, "\n"), checked, blanked
}

func requireLaneReachableTextIsSteClean(t *testing.T, label, text string, minChecked int, dirs ...string) {
	t.Helper()
	kept, checked, blanked := blankRowsDefinedOnlyInProtectedFiles(t, text, dirs...)
	if checked < minChecked {
		t.Fatalf("%s: only %d lane-reachable rows were graded (need %d), so a lint pass proves little", label, checked, minChecked)
	}
	t.Logf("%s: graded %d lane-reachable rows; %d rows defined only in protected files go to the console follow-up", label, checked, blanked)
	requireSteClean(t, label, kept)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2536-evolve-")
		if err != nil {
			buildFailure = err
			return
		}
		builtBinary = filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		build := exec.CommandContext(ctx, "go", "build", "-o", builtBinary, "./cmd/evolve")
		build.Dir = acsassert.GoModuleDir(t)
		if out, err := build.CombinedOutput(); err != nil {
			buildFailure = fmt.Errorf("go build: %v\n%s", err, acsassert.OutputTail(string(out), 20))
		}
	})
	if buildFailure != nil {
		t.Fatalf("build the evolve binary: %v", buildFailure)
	}
	return builtBinary
}

func runVerb(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "EVOLVE_WORKTREE_ROOT="+root, "EVOLVE_PROJECT_ROOT="+root)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("run evolve %v: %v", args, err)
	return "", -1
}

func requireNoDrift(t *testing.T, args ...string) {
	t.Helper()
	out, code := runVerb(t, args...)
	if code != 0 {
		t.Errorf("RED: evolve %s exited %d:\n%s", strings.Join(args, " "), code, acsassert.OutputTail(out, 8))
	}
}

func runPackageTests(t *testing.T, pkg string, extra ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	args := append([]string{"test", "-count=1"}, extra...)
	cmd := exec.CommandContext(ctx, "go", append(args, pkg)...)
	cmd.Dir = acsassert.GoModuleDir(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("RED: go test %s failed: %v\n%s", pkg, err, acsassert.OutputTail(string(out), 15))
	}
}

func TestC2536_001_SignalCodesGeneratorOutputIsSteClean(t *testing.T) {
	requireLaneReachableTextIsSteClean(t, "signalcenter.RenderCodes", signalcenter.RenderCodes(), 5, "go/internal", "go/cmd")
}

func TestC2536_002_SignalCodesDocumentRegionIsSteClean(t *testing.T) {
	requireLaneReachableTextIsSteClean(t, "docs/architecture/signal-codes.md", generatedRegions(t, "docs/architecture/signal-codes.md"), 200, "go/internal", "go/cmd")
}

func TestC2536_003_FlagIndexGeneratorOutputIsSteClean(t *testing.T) {
	requireLaneReachableTextIsSteClean(t, "flagregistry.RenderIndex", flagregistry.RenderIndex(), 0, "go/internal/flagregistry")
}

func TestC2536_004_FlagIndexDocumentRegionIsSteClean(t *testing.T) {
	requireLaneReachableTextIsSteClean(t, "docs/architecture/control-flags.md", generatedRegions(t, "docs/architecture/control-flags.md"), 0, "go/internal/flagregistry")
}

func TestC2536_005_ResearchIndexDocumentRegionIsSteClean(t *testing.T) {
	requireSteClean(t, "docs/research-index.md", generatedRegions(t, "docs/research-index.md"))
}

func TestC2536_006_SkillGeneratedRegionsAreSteClean(t *testing.T) {
	for _, rel := range []string{
		"CLAUDE.md", "GEMINI.md", "agents/evolve-router.md",
		"skills/scout/SKILL.md", "skills/intent/SKILL.md", "skills/audit/SKILL.md", "skills/retro/SKILL.md",
		"skills/build/SKILL.md", "skills/ship/SKILL.md", "skills/tdd/SKILL.md", "skills/plan-review/SKILL.md",
		".claude/agents/code-reviewer.md", ".claude/agents/code-simplifier.md",
		".claude/agents/architecture-reviewer.md", ".claude/agents/go-reviewer.md",
	} {
		requireSteClean(t, rel, generatedRegions(t, rel))
	}
}

func TestC2536_007_CommandStubTemplateIsSteClean(t *testing.T) {
	stub := skillcheck.RenderCommandStub("tdd", "Write the failing tests for a task", "[task]")
	var prose []string
	for _, line := range strings.Split(stub, "\n") {
		if !strings.HasPrefix(line, "<!--") {
			prose = append(prose, line)
		}
	}
	requireSteClean(t, "skillcheck.RenderCommandStub", strings.Join(prose, "\n"))
}

func TestC2536_008_ChangelogEntryTemplateIsSteClean(t *testing.T) {
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	populated := changeloggen.ClassifyAll([]changeloggen.Commit{
		{Subject: "feat: add the tdd lane"},
		{Subject: "fix: close the drift"},
		{Subject: "docs: explain the lane"},
		{Subject: "chore: tidy"},
		{Subject: "refactor: split the gate"},
	})
	entries := map[string]string{
		"changeloggen.RenderEntry populated": changeloggen.RenderEntry("1.2.3", "v1.2.2", "HEAD", now, populated),
		"changeloggen.RenderEntry empty":     changeloggen.RenderEntry("1.2.3", "v1.2.2", "HEAD", now, changeloggen.Buckets{}),
		"changeloggen.DefaultFreshContent":   changeloggen.DefaultFreshContent,
	}
	for label, entry := range entries {
		requireSteClean(t, label, entry)
	}
}

func TestC2536_009_ReadmeVersionRowIsSteCleanAndKeepsItsPrefix(t *testing.T) {
	readme := filepath.Join(t.TempDir(), "README.md")
	seed := "# Project\n\n## History\n\n| Version | Date | Notes |\n|---|---|---|\n| v1.0 | Jan 2 | See the [1.0.0] section of CHANGELOG.md. |\n"
	if err := os.WriteFile(readme, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := versionbump.BumpReadmeHistory(readme, "1.1.0", time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC), false)
	if err != nil || !changed {
		t.Fatalf("BumpReadmeHistory changed=%v err=%v", changed, err)
	}
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	var newRow string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "| v1.1 |") {
			newRow = line
		}
	}
	if newRow == "" {
		t.Fatalf("the new row lost its `| v1.1 |` prefix:\n%s", raw)
	}
	if !historyRow.MatchString(newRow + "\n") {
		t.Errorf("the new row does not match the machine-read row pattern: %q", newRow)
	}
	if has, err := versionbump.HasHistoryRow(readme, "1.1"); err != nil || !has {
		t.Errorf("HasHistoryRow(1.1) = %v, %v after the bump", has, err)
	}
	requireSteClean(t, "versionbump README row", newRow)
}

func TestC2536_010_ReadmeVersionRowBumpIsIdempotent(t *testing.T) {
	readme := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(readme, []byte("| v1.0 | Jan 2 | See the [1.0.0] section of CHANGELOG.md. |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	if changed, err := versionbump.BumpReadmeHistory(readme, "1.1.0", now, false); err != nil || !changed {
		t.Fatalf("first bump changed=%v err=%v", changed, err)
	}
	if changed, err := versionbump.BumpReadmeHistory(readme, "1.1.0", now, false); err != nil || changed {
		t.Errorf("second bump changed=%v err=%v, want no change", changed, err)
	}
}

func TestC2536_011_CommittedReadmeVersionRowsKeepTheirPrefix(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if rows := historyRow.FindAllString(string(raw), -1); len(rows) < 3 {
		t.Errorf("README.md keeps %d `| vX.Y |` rows, want at least 3", len(rows))
	}
	if got, err := versionbump.CurrentReadmeCurrent(filepath.Join(root, "README.md")); err != nil || got == "" {
		t.Errorf("CurrentReadmeCurrent = %q, %v", got, err)
	}
	requireSteClean(t, "README.md", string(raw))
}

func TestC2536_012_SignalCodesCheckReportsNoDrift(t *testing.T) {
	requireNoDrift(t, "signals", "codes", "check")
}

func TestC2536_013_FlagsCheckReportsNoDrift(t *testing.T) {
	requireNoDrift(t, "flags", "check")
}

func TestC2536_014_SkillsCheckReportsNoDrift(t *testing.T) {
	requireNoDrift(t, "skills", "check")
}

func TestC2536_015_DocsIndexCheckReportsNoDrift(t *testing.T) {
	requireNoDrift(t, "docs", "index", "check")
}

func TestC2536_016_VersionbumpTestsPass(t *testing.T) {
	runPackageTests(t, "./internal/versionbump")
}

func TestC2536_017_ChangeloggenTestsPass(t *testing.T) {
	runPackageTests(t, "./internal/changeloggen")
}

func TestC2536_018_SignalCenterRenderTestsPass(t *testing.T) {
	runPackageTests(t, "./internal/signalcenter", "-run", "Render")
}

func TestC2536_019_FlagRegistryTestsPass(t *testing.T) {
	runPackageTests(t, "./internal/flagregistry")
}

func TestC2536_020_SkillcheckTestsPass(t *testing.T) {
	runPackageTests(t, "./internal/skillcheck")
}
