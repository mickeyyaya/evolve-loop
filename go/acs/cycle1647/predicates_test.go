//go:build acs

package cycle1647

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	taskSlug               = "overlay-family-name-transport-ambiguity"
	focusedRunnerTest      = "TestResolveRouting_AdvisorOverlayPreservesFamilyTransport"
	contractEscalationTest = "TestRunner_ContractEscalation_RedispatchesOnEscalatedCLIWithDirective"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runGo(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go %s: %v", strings.Join(args, " "), err)
	return "", -1
}

type advisorHooks struct{ phase, agent string }

func (h advisorHooks) PhaseName() string                              { return h.phase }
func (h advisorHooks) AgentPromptName() string                        { return h.agent }
func (h advisorHooks) ArtifactFilename(core.PhaseRequest) string      { return h.phase + "-report.md" }
func (h advisorHooks) DefaultModel() string                           { return "sonnet" }
func (h advisorHooks) ComposePrompt(string, core.PhaseRequest) string { return "x" }
func (h advisorHooks) Classify(string, core.PhaseRequest, core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, ""
}

type recordingBridge struct {
	calls   []string
	trigger map[string]int
}

func (b *recordingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls = append(b.calls, req.CLI)
	if code, ok := b.trigger[req.CLI]; ok {
		return core.BridgeResponse{ExitCode: code, Stderr: "scripted: REPL boot timeout"}, errors.New("bridge: launch exit=" + strconv.Itoa(code))
	}
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("ok-from-"+req.CLI), 0o644)
	}
	return core.BridgeResponse{Stdout: "ok-from-" + req.CLI}, nil
}

func (b *recordingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func writeProfile(t *testing.T, agent, cli string, fallback []string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	fb := ""
	if len(fallback) > 0 {
		fb = `,"cli_fallback":["` + strings.Join(fallback, `","`) + `"]`
	}
	body := `{"name":"` + agent + `","cli":"` + cli + `","model_tier_default":"sonnet"` + fb + `}`
	name := strings.TrimPrefix(agent, "evolve-") + ".json"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

func dispatchThroughRunner(t *testing.T, root, overlayCLI string, trigger map[string]int) []string {
	t.Helper()
	t.Setenv("EVOLVE_CLI", "")
	hooks := advisorHooks{phase: "scout", agent: "evolve-scout"}
	bridge := &recordingBridge{trigger: trigger}
	loader := prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-scout.md": &fstest.MapFile{Data: []byte("---\nname: evolve-scout\n---\nx")},
	})
	r := runner.New(runner.Options{Hooks: hooks, Bridge: bridge, Prompts: loader})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot:     root,
		Workspace:       t.TempDir(),
		ModelRoutingCLI: overlayCLI,
	}); err != nil {
		t.Fatalf("runner.Run(overlay=%q): %v", overlayCLI, err)
	}
	return bridge.calls
}

// acs-predicate: config-check — this criterion IS a documentation-presence
func TestC1647_001_PackageDocDecidesOverlayFamilyVsDriverSemantics(t *testing.T) {
	out, code := runGo(t, goDir(t), "doc", "./internal/llmroute")
	if code != 0 {
		t.Fatalf("go doc ./internal/llmroute exit=%d:\n%s", code, out)
	}
	pkgDoc := packageCommentOnly(out)
	lower := strings.ToLower(pkgDoc)
	for _, want := range []string{"overlay", "family selector", "driver selector"} {
		if !strings.Contains(lower, want) {
			t.Errorf("RED: llmroute PACKAGE doc does not contain %q — the family-vs-driver decision is not documented at the package boundary (only on the ApplySoftOverlay function comment).\n--- go doc package comment ---\n%s", want, pkgDoc)
		}
	}
}

func packageCommentOnly(goDocOut string) string {
	var kept []string
	for _, line := range strings.Split(goDocOut, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ") ||
			strings.HasPrefix(trimmed, "var ") || strings.HasPrefix(trimmed, "const ") {
			break
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func TestC1647_002_RunnerAdvisorBareFamilyOverlayKeepsHeadlessTransport(t *testing.T) {
	t.Run("resolver-named-test", func(t *testing.T) {
		name := "TestApplySoftOverlay_BareFamilyDoesNotCrossTransportWhenTheChainHoldsANonDefaultDriver"
		out, code := runGo(t, goDir(t), "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/llmroute")
		if code != 0 || !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass in ./internal/llmroute (exit=%d):\n%s", name, code, out)
		}
	})
	t.Run("runner-production-path", func(t *testing.T) {
		root := writeProfile(t, "evolve-scout", "claude-p", []string{"codex"})
		calls := dispatchThroughRunner(t, root, "claude", nil)
		if len(calls) == 0 {
			t.Fatal("RED: the runner dispatched nothing")
		}
		if calls[0] != "claude-p" {
			t.Errorf("RED: runner dispatched %q first for bare overlay \"claude\" over chain [claude-p codex]; want claude-p — the advisor projection (resolveDispatchPlan, routing.go) must preserve the phase's resolved headless transport (dispatch order %v)", calls[0], calls)
		}
		for _, c := range calls {
			if c == "claude-tmux" {
				t.Errorf("RED: claude-tmux was dispatched (%v) — the bare-family overlay was normalized onto the family's default TMUX driver, the exact transport crossing this task forbids", calls)
			}
		}
	})
}

func TestC1647_003_RunnerAdvisorExplicitDriverOverlayWinsAndKeepsHeadlessFallback(t *testing.T) {
	t.Run("resolver-named-test", func(t *testing.T) {
		name := "TestApplySoftOverlay_DriverQualifiedOverlayWinsOverSameFamilyChainEntry"
		out, code := runGo(t, goDir(t), "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/llmroute")
		if code != 0 || !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass in ./internal/llmroute (exit=%d):\n%s", name, code, out)
		}
	})
	t.Run("runner-production-path", func(t *testing.T) {
		root := writeProfile(t, "evolve-scout", "claude-p", nil)
		calls := dispatchThroughRunner(t, root, "claude-tmux", map[string]int{"claude-tmux": 80})
		if len(calls) == 0 {
			t.Fatal("RED: the runner dispatched nothing")
		}
		if calls[0] != "claude-tmux" {
			t.Errorf("RED: runner dispatched %q first for explicit overlay \"claude-tmux\" over chain [claude-p]; want claude-tmux — an explicit driver request was satisfied by naive family matching (dispatch order %v)", calls[0], calls)
		}
		if len(calls) < 2 || calls[1] != "claude-p" {
			t.Errorf("RED: after claude-tmux exited 80 the walk did not reach claude-p (dispatch order %v) — the explicit overlay must be SOFT: the chain's headless entry is retained as fallback, not replaced", calls)
		}
	})
}

func TestC1647_004_FocusedRunnerRegressionCoversAdvisorProjectionAndNamesCaller(t *testing.T) {
	dir := goDir(t)
	out, code := runGo(t, dir, "test", "-count=1", "-v",
		"-run", "^("+focusedRunnerTest+"|"+contractEscalationTest+")$", "./internal/phases/runner")
	for _, name := range []string{focusedRunnerTest, contractEscalationTest} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not PASS in ./internal/phases/runner (exit=%d)", name, code)
		}
	}
	if code != 0 {
		t.Errorf("RED: go test ./internal/phases/runner (narrowed) exit=%d:\n%s", code, out)
	}
	if t.Failed() {
		t.Logf("--- go test output ---\n%s", out)
	}

	file := fileDefiningTest(t, filepath.Join(dir, "internal", "phases", "runner"), focusedRunnerTest)
	if file == "" {
		t.Fatalf("RED: no *_test.go under internal/phases/runner defines func %s", focusedRunnerTest)
	}
	for _, want := range []string{"resolveDispatchPlan", "routing.go", "ModelRoutingCLI", `"claude-p"`, `"codex"`, `"claude-tmux"`} {
		if !acsassert.FileContains(t, file, want) {
			t.Errorf("RED: %s does not mention %q — the focused test must name the production caller and drive the AC's literal inputs", filepath.Base(file), want)
		}
	}
	if !acsassert.FileContainsAny(file, "ContractEscalation", "contract-escalation", "contract_escalation") {
		t.Errorf("RED: %s does not distinguish the advisor projection from the contract-escalation projection (both reach llmroute.ApplySoftOverlay through resolveDispatchPlan; AC4 asks the caller to be named for each)", filepath.Base(file))
	}
}

func fileDefiningTest(t *testing.T, pkgDir, name string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", pkgDir, err)
	}
	decl := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(`)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if decl.Match(raw) {
			return f
		}
	}
	return ""
}

func TestC1647_005_LlmrouteRouterRaceCleanAndApicoverEnforceClean(t *testing.T) {
	dir := goDir(t)
	for _, pkg := range []string{"./internal/llmroute", "./internal/router"} {
		pkg := pkg
		t.Run("race-"+strings.TrimPrefix(pkg, "./internal/"), func(t *testing.T) {
			out, code := runGo(t, dir, "test", "-race", "-count=1", pkg)
			if code != 0 {
				t.Errorf("RED: go test -race %s exit=%d:\n%s", pkg, code, out)
			}
		})
	}
	t.Run("apicover-enforce-llmroute", func(t *testing.T) {
		tmp := t.TempDir()
		bin := filepath.Join(tmp, "apicover")
		if out, code := runGo(t, dir, "build", "-o", bin, "./cmd/apicover"); code != 0 {
			t.Fatalf("build apicover exit=%d:\n%s", code, out)
		}
		profile := filepath.Join(tmp, "coverage.txt")
		if out, code := runGo(t, dir, "test", "-count=1", "-tags", "integration", "-coverprofile="+profile, "./internal/llmroute"); code != 0 {
			t.Fatalf("coverprofile run exit=%d:\n%s", code, out)
		}
		funcOut, code := runGo(t, dir, "tool", "cover", "-func="+profile)
		if code != 0 {
			t.Fatalf("go tool cover exit=%d:\n%s", code, funcOut)
		}
		funcFile := filepath.Join(tmp, "coverage.func.txt")
		if err := os.WriteFile(funcFile, []byte(funcOut), 0o644); err != nil {
			t.Fatal(err)
		}
		pkgDir := filepath.Join(dir, "internal", "llmroute")
		cmd := exec.Command(bin, "-enforce", "-cover", funcFile, pkgDir)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("RED: apicover -enforce internal/llmroute failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "0 uncovered, 0 false-green") {
			t.Errorf("RED: apicover -enforce internal/llmroute is not clean:\n%s", out)
		}
	})
}

func TestC1647_006_PredicatePackageAndEvalAreGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		"go/acs/cycle1647/predicates_test.go",
		".evolve/evals/" + taskSlug + ".md",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, rel)) {
			t.Errorf("RED: %s missing on disk", rel)
			continue
		}
		if _, stderr, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("RED: %s is not git-tracked (may be gitignored — dropped at ship): %s", rel, strings.TrimSpace(stderr))
		}
	}
}

const thisPackage = "cycle1647"

func shippingExplanation(t *testing.T) (string, string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	const dir = "docs/explain/builds"
	isRecord := func(p string) bool {
		return strings.HasPrefix(p, dir+"/cycle-") && strings.HasSuffix(p, ".md")
	}
	var found []string
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--untracked-files=all", "--", dir)
	if err != nil || code != 0 {
		t.Fatalf("git status -- %s: exit=%d err=%v stderr=%s", dir, code, err, stderr)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if i := strings.LastIndex(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		if isRecord(path) {
			found = append(found, path)
		}
	}
	if len(found) == 0 {
		out, stderr, code, err = acsassert.SubprocessOutput("git", "-C", root, "log", "-1", "--diff-filter=A", "--name-only", "--format=", "--", dir+"/cycle-*.md")
		if err != nil || code != 0 {
			t.Fatalf("git log -- %s: exit=%d err=%v stderr=%s", dir, code, err, stderr)
		}
		for _, line := range strings.Split(out, "\n") {
			if path := strings.TrimSpace(line); isRecord(path) {
				found = append(found, path)
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one shipping build explanation under %s/ (the record this tree adds; unshipped predecessors are archived under docs/private/); got %v", dir, found)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(found[0])))
	if err != nil {
		t.Fatalf("read %s: %v", found[0], err)
	}
	return found[0], string(raw)
}

func explanationBase(t *testing.T, body string) string {
	t.Helper()
	in := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "## ")), "Build Binding")
			continue
		}
		if !in {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if rest, ok := strings.CutPrefix(line, "Base SHA:"); ok {
			if sha := strings.Trim(strings.TrimSpace(rest), "`"); sha != "" {
				return sha
			}
		}
	}
	t.Fatal("the build explanation's `## Build Binding` must carry a `- Base SHA:` line")
	return ""
}

func diffPaths(t *testing.T, root, base, filter, pathspec string) []string {
	t.Helper()
	var tail []string
	if filter != "" {
		tail = append(tail, "--diff-filter="+filter)
	}
	tail = append(tail, base, "--", pathspec)
	out, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root, "diff", "--name-only"}, tail...)...)
	if err != nil || code != 0 {
		t.Fatalf("git diff --name-only %s -- %s: exit=%d err=%v stderr=%s", base, pathspec, code, err, stderr)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

func failLines(out string) string {
	var keep []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "FAIL") || strings.Contains(line, "RED:") || strings.Contains(line, "expected exactly one") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

func TestC1647_007_InheritedPredicatePackagesAddedByThisDiffStayGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := goDir(t)
	_, body := shippingExplanation(t)
	base := explanationBase(t, body)
	seen := map[string]bool{}
	var pkgs []string
	for _, path := range diffPaths(t, root, base, "A", "go/acs/") {
		pkg := strings.Split(strings.TrimPrefix(path, "go/acs/"), "/")[0]
		if !strings.HasPrefix(pkg, "cycle") || pkg == thisPackage || seen[pkg] {
			continue
		}
		seen[pkg] = true
		pkgs = append(pkgs, pkg)
	}
	if len(pkgs) == 0 {
		t.Fatalf("precondition: the base-bound diff against %s adds no inherited go/acs/cycle* package -- on a continuation tree that means the resolution is broken, not that the contract is vacuously met", base[:8])
	}
	for _, pkg := range pkgs {
		t.Run(pkg, func(t *testing.T) {
			out, code := runGo(t, dir, "test", "-tags", "acs", "-count=1", "-v", "./acs/"+pkg+"/")
			if code != 0 {
				t.Errorf("RED: inherited predicate package go/acs/%s is red on the shipping tree (exit=%d):\n%s", pkg, code, failLines(out))
			}
		})
	}
}

func TestC1647_008_ExplanationCitesNoLineIntoAPathThisDiffDeletes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath, body := shippingExplanation(t)
	deleted := diffPaths(t, root, explanationBase(t, body), "D", ".")
	if len(deleted) == 0 {
		t.Logf("the base-bound diff deletes no path; no line citation can be dead")
		return
	}
	for _, path := range deleted {
		re := regexp.MustCompile(regexp.QuoteMeta(path) + `:[0-9]+`)
		for _, cite := range re.FindAllString(body, -1) {
			t.Errorf("RED: %s cites `%s`, but this diff DELETES %s -- the line is unreadable on the shipped tree; cite the surviving copy (e.g. its .evolve/inbox/consumed/ record) at a real line instead", docPath, cite, path)
		}
	}
}
