//go:build acs

package cycle1638

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/buildplanner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const advisorTier = "balanced"

func realRegistry(t *testing.T) config.RoutingConfig {
	t.Helper()
	root := acsassert.RepoRoot(t)
	cfg, warns := config.Load(filepath.Join(root, "docs", "architecture", "phase-registry.json"), map[string]string{})
	for _, w := range warns {
		t.Logf("registry warning: %+v", w)
	}
	if len(cfg.Order) == 0 && len(cfg.Conditional) == 0 {
		t.Fatalf("precondition: the live phase registry loaded empty — the predicate would assert nothing")
	}
	return cfg
}

func unifiedSignals(size string, members int) router.RoutingSignals {
	var sig router.RoutingSignals
	sig.Triage.UnifiedSize = size
	sig.Triage.UnifiedMemberCount = members
	return sig
}

func planningPlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "plan-review", Run: true, Tier: advisorTier, Justification: "advisor default"},
		{Phase: "build-planner", Run: true, Tier: advisorTier, Justification: "advisor default"},
		{Phase: "tdd", Run: true, Tier: advisorTier},
		{Phase: "build", Run: true, Tier: advisorTier},
		{Phase: "audit", Run: true, Tier: advisorTier},
		{Phase: "ship", Run: true, Tier: advisorTier},
	}}
}

func clampPlan(t *testing.T, sig router.RoutingSignals) (*router.PhasePlan, []router.Clamp) {
	t.Helper()
	in := router.RouteInput{
		Current:   "triage",
		Verdict:   "PASS",
		Signals:   sig,
		Cfg:       realRegistry(t),
		Completed: []string{"scout", "triage"},
	}
	return router.ClampPlanToFloorWith(in, planningPlan(), router.DefaultShipFloor(), false)
}

func tierOf(plan *router.PhasePlan, phase string) (string, bool) {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Tier, true
		}
	}
	return "", false
}

var planningPhases = []string{"plan-review", "build-planner"}

func TestC1638_001_ValidatedUnifiedCommitmentRaisesPlanningPhasesToDeepTier(t *testing.T) {
	for _, size := range []string{"small", "large"} {
		t.Run(size, func(t *testing.T) {
			plan, clamps := clampPlan(t, unifiedSignals(size, 3))
			for _, phase := range planningPhases {
				tier, present := tierOf(plan, phase)
				if !present {
					t.Fatalf("RED: %s was dropped from the clamped plan entirely", phase)
				}
				if tier != "deep" {
					t.Errorf("RED: a validated %s unified commitment must raise %s to the deep tier "+
						"(how_to_apply step 2: \"routes through buildplanner+plan-review at deep tier\"); got tier=%q",
						size, phase, tier)
				}
			}
			var recorded []string
			for _, c := range clamps {
				for _, phase := range planningPhases {
					if c.Phase == phase && strings.Contains(strings.ToLower(c.Forced), "deep") {
						recorded = append(recorded, c.Phase)
					}
				}
			}
			if len(recorded) < len(planningPhases) {
				t.Errorf("RED: the tier raise must be recorded as a Clamp for each planning phase "+
					"(proposed=%s → forced=deep); recorded only %v of %v; clamps=%+v",
					advisorTier, recorded, planningPhases, clamps)
			}
		})
	}
}

func TestC1638_002_NoValidatedCommitmentLeavesAdvisorTiersUntouched(t *testing.T) {
	plan, clamps := clampPlan(t, unifiedSignals("", 0))
	for _, phase := range planningPhases {
		tier, present := tierOf(plan, phase)
		if !present {
			continue
		}
		if tier != advisorTier {
			t.Errorf("with no validated unified commitment the advisor's tier must stand: %s tier=%q, want %q "+
				"(a blanket escalation would make 001 pass on a no-op)", phase, tier, advisorTier)
		}
	}
	for _, c := range clamps {
		for _, phase := range planningPhases {
			if c.Phase == phase && strings.Contains(strings.ToLower(c.Forced), "deep") {
				t.Errorf("no unified commitment must produce no tier clamp on %s; got %+v", phase, c)
			}
		}
	}
}

func TestC1638_003_EverySizeOfUnifiedCommitmentPinsThePlanningPhases(t *testing.T) {
	pol := router.NewPhasePolicy(realRegistry(t))
	for _, size := range []string{"small", "large"} {
		t.Run(size, func(t *testing.T) {
			for _, phase := range planningPhases {
				if !pol.Enabled(phase, unifiedSignals(size, 3)) {
					t.Errorf("RED: a validated %s unified commitment must pin %s to run "+
						"(how_to_apply step 2 names both planning phases and does not exempt the campaign path); Enabled=false",
						size, phase)
				}
			}
		})
	}
	t.Run("no-commitment", func(t *testing.T) {
		for _, phase := range planningPhases {
			if pol.Enabled(phase, unifiedSignals("", 0)) {
				t.Errorf("fail-open: with no validated commitment %s must not be pinned by the unified rule; Enabled=true", phase)
			}
		}
	})
}

// acs-predicate: config-check — the rubric is prompt text consumed by an LLM,
func TestC1638_004_PlanReviewPersonaRejectsPatchBundleDesigns(t *testing.T) {
	root := acsassert.RepoRoot(t)
	t.Setenv("EVOLVE_PROMPTS_DIR", "")
	persona, err := prompts.NewForProject(root).Agent("plan-reviewer")
	if err != nil {
		t.Fatalf("the production prompt loader must resolve the plan-review persona: %v", err)
	}
	body := strings.ToLower(persona.Raw)
	if !strings.Contains(body, "patch-bundle") && !strings.Contains(body, "patch bundle") {
		t.Errorf("RED: the plan-review persona must name the patch-bundle failure mode " +
			"(a unified commitment whose plan is N patches rather than one general abstraction)")
	}
	hasVerdict := strings.Contains(body, "revise") || strings.Contains(body, "abort")
	if !hasVerdict {
		t.Errorf("RED: the persona must bind the patch-bundle finding to a REVISE/ABORT verdict, not merely mention it")
	}
	wanted := []string{"single-source", "immutab", "kiss"}
	var missing []string
	for _, w := range wanted {
		if !strings.Contains(body, w) {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		t.Errorf("RED: the persona must state the general-abstraction rubric the directive names "+
			"(single-source-with-projection, Strategy/DI over flags, immutability, KISS floor); missing markers: %v", missing)
	}
}

func TestC1638_005_SalvagedCycle1637ContractStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pkgDir := filepath.Join(root, "go", "acs", "cycle1637")
	if !acsassert.FileExists(t, filepath.Join(pkgDir, "predicates_test.go")) {
		t.Fatalf("the salvaged predicate package must stay in the tree: %s", pkgDir)
	}
	cmd := exec.Command("go", "test", "-tags", "acs", "-count=1", "./acs/cycle1637/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the cycle-1637 unified-commitment contract regressed — the planning-teeth work must keep it GREEN:\n%s\n%v", out, err)
	}
	if !strings.Contains(string(out), "ok ") {
		t.Errorf("expected an `ok` line from the cycle1637 package; got:\n%s", out)
	}
}

func TestC1638_006_RouterPackageStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "./internal/router/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("internal/router regressed:\n%s\n%v", out, err)
	}
	if !strings.Contains(string(out), "ok ") {
		t.Errorf("expected an `ok` line from internal/router; got:\n%s", out)
	}
}

func TestC1638_007_CycleACSPackageIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "acs", "cycle1638", "predicates_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); err != nil || code != 0 {
		t.Errorf("RED: %s is untracked (git ls-files exit=%d err=%v) — the audit's predicate tree would carry an input absent from the ship tree", rel, code, err)
	}
}

func declinedPlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "triage", Run: true},
		{Phase: "plan-review", Run: false}, {Phase: "tdd", Run: true},
		{Phase: "build-planner", Run: false}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

func walkFrom(cfg config.RoutingConfig, sig router.RoutingSignals, from string, completed []string) []string {
	var visited []string
	cur := from
	done := append([]string(nil), completed...)
	for i := 0; i < 16; i++ {
		d := router.Route(router.RouteInput{Current: cur, Verdict: "PASS", Signals: sig, Cfg: cfg, Completed: done, Plan: declinedPlan()}, nil)
		if d.NextPhase == router.PhaseEnd {
			return visited
		}
		visited = append(visited, d.NextPhase)
		if d.NextPhase == "build" {
			return visited
		}
		done = append(done, d.NextPhase)
		cur = d.NextPhase
	}
	return visited
}

func visits(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

var entryFor = map[string]struct {
	from      string
	completed []string
}{
	"plan-review":   {from: "triage", completed: []string{"scout", "triage"}},
	"build-planner": {from: "tdd", completed: []string{"scout", "triage", "plan-review", "tdd"}},
}

func TestC1638_008_BothRoutingAuthoritiesAgreeOnEverySizeOfCommitment(t *testing.T) {
	cfg := realRegistry(t)
	pol := router.NewPhasePolicy(cfg)
	for _, size := range []string{"small", "large"} {
		t.Run(size, func(t *testing.T) {
			sig := unifiedSignals(size, 3)
			for _, phase := range planningPhases {
				entry := entryFor[phase]
				walk := walkFrom(cfg, sig, entry.from, entry.completed)
				enabled, visited := pol.Enabled(phase, sig), visits(walk, phase)
				if enabled != visited {
					t.Fatalf("RED: the two production routing authorities disagree for a %s commitment on %s: "+
						"PhasePolicy.Enabled=%t but the router walk from %q visited %v -- one tree cannot satisfy both "+
						"(this is the cycle-1638 audit H1 shape)", size, phase, enabled, entry.from, walk)
				}
				if !enabled {
					t.Errorf("RED: a validated %s unified commitment must pin %s in BOTH authorities; both report not-pinned (walk from %q visited %v)",
						size, phase, entry.from, walk)
				}
			}
		})
	}
	t.Run("no-commitment", func(t *testing.T) {
		sig := unifiedSignals("", 0)
		for _, phase := range planningPhases {
			entry := entryFor[phase]
			walk := walkFrom(cfg, sig, entry.from, entry.completed)
			if enabled, visited := pol.Enabled(phase, sig), visits(walk, phase); enabled || visited {
				t.Errorf("fail-open: with no validated commitment %s must be pinned by neither authority; Enabled=%t walkVisited=%t (%v)",
					phase, enabled, visited, walk)
			}
		}
	})
}

func TestC1638_009_BuildPlannerReportsADegradedDigestInsteadOfSelfSkippingSilently(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bp := buildplanner.New(buildplanner.Config{})
	request := func(ws string) core.PhaseRequest {
		return core.PhaseRequest{Cycle: 1638, ProjectRoot: root, Workspace: ws, Env: map[string]string{}}
	}
	mentionsDegradation := func(diags []core.Diagnostic) bool {
		for _, d := range diags {
			m := strings.ToLower(d.Message)
			if strings.Contains(m, "digest") || strings.Contains(m, "degrad") {
				return true
			}
		}
		return false
	}

	t.Run("degraded-digest-is-reported", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.Mkdir(filepath.Join(ws, "triage-decision.json"), 0o755); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		sig, _ := router.Digest(ws, []string{"triage"})
		if len(sig.DigestDegraded) == 0 {
			t.Fatalf("precondition: router.Digest must record the unreadable decision in DigestDegraded; got none")
		}
		_, verdict, _, diags := bp.ShouldSkip(request(ws))
		if len(diags) == 0 {
			t.Errorf("RED: build-planner must SURFACE the degraded routing digest %v as a diagnostic instead of "+
				"self-skipping on zero-valued signals (verdict=%q, diagnostics=none) -- an unreported read failure "+
				"silently disarms the conditional_mandatory pin", sig.DigestDegraded, verdict)
		} else if !mentionsDegradation(diags) {
			t.Errorf("RED: the diagnostic must name the digest degradation so an operator can act on it; got %+v (degraded=%v)", diags, sig.DigestDegraded)
		}
	})

	t.Run("clean-absence-stays-silent", func(t *testing.T) {
		ws := t.TempDir()
		sig, err := router.Digest(ws, []string{"triage"})
		if err != nil || len(sig.DigestDegraded) != 0 {
			t.Fatalf("precondition: an empty workspace must be a CLEAN absence; err=%v degraded=%v", err, sig.DigestDegraded)
		}
		skipped, _, _, diags := bp.ShouldSkip(request(ws))
		if !skipped {
			t.Error("build-planner must stay opt-in (skipped) when no commitment is present")
		}
		if len(diags) != 0 {
			t.Errorf("a clean no-commitment cycle must stay silent; got %+v", diags)
		}
	})

	t.Run("validated-commitment-runs-the-phase", func(t *testing.T) {
		ws := t.TempDir()
		decision := `{"top_n":[{"id":"a"},{"id":"b"},{"id":"c"}],"unified_projection":{"size":"small","member_count":3}}`
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(decision), 0o644); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		sig, _ := router.Digest(ws, []string{"triage"})
		if sig.Triage.UnifiedSize != "small" {
			t.Fatalf("precondition: the fixture must project UnifiedSize=small; got %q (degraded=%v)", sig.Triage.UnifiedSize, sig.DigestDegraded)
		}
		skipped, verdict, _, diags := bp.ShouldSkip(request(ws))
		if skipped {
			t.Errorf("a validated unified commitment must RUN build-planner; ShouldSkip=true verdict=%q", verdict)
		}
		if len(diags) != 0 {
			t.Errorf("a healthy validated cycle must report nothing; got %+v", diags)
		}
	})
}

func explanationDoc(t *testing.T) (string, string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	rel := shippingCycleRecord(t, root)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return rel, string(raw)
}

func shippingCycleRecord(t *testing.T, root string) string {
	t.Helper()
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
	return found[0]
}

func docSection(t *testing.T, body, title string) string {
	t.Helper()
	lines := strings.Split(body, "\n")
	var out []string
	in := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			in = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "## ")), title)
			continue
		}
		if in {
			out = append(out, line)
		}
	}
	if !in && len(out) == 0 {
		t.Fatalf("the build explanation has no `## %s` section", title)
	}
	return strings.Join(out, "\n")
}

func baseSHA(t *testing.T, body string) string {
	t.Helper()
	for _, line := range strings.Split(docSection(t, body, "Build Binding"), "\n") {
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

func changedSinceBase(t *testing.T, root, base string) []string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base)
	if err != nil || code != 0 {
		t.Fatalf("git diff --name-only %s: exit=%d err=%v stderr=%s", base, code, err, stderr)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	if len(paths) == 0 {
		t.Fatalf("precondition: the base-bound diff against %s is empty -- the predicate would assert nothing", base)
	}
	return paths
}

func loadBearing(path string) bool {
	switch {
	case strings.HasSuffix(path, "_test.go"):
		return false
	case strings.HasPrefix(path, "go/internal/"), strings.HasPrefix(path, "go/cmd/"):
		return strings.HasSuffix(path, ".go")
	case strings.HasPrefix(path, "docs/architecture/") && strings.HasSuffix(path, ".json"):
		return true
	}
	return false
}

func TestC1638_010_ExplanationNamesEveryLoadBearingFileTheBuildChanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath, body := explanationDoc(t)
	areas := docSection(t, body, "Changed Areas")
	var missing []string
	for _, path := range changedSinceBase(t, root, baseSHA(t, body)) {
		if loadBearing(path) && !strings.Contains(areas, path) {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		t.Errorf("RED: %s `## Changed Areas` omits load-bearing file(s) the base-bound diff changed: %v -- "+
			"a change a reader cannot find in the explanation is an unexplained change", docPath, missing)
	}
}

var provenanceClaims = []string{"preserved", "pre-existing", "preexisting", "inherited", "historical", "older", "prior cycle", "earlier cycle"}

func TestC1638_011_ExplanationDoesNotAttributeThisDiffsOwnPredicatesToHistory(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath, body := explanationDoc(t)
	base := baseSHA(t, body)
	lower := strings.ToLower(body)
	for _, path := range changedSinceBase(t, root, base) {
		if !strings.HasPrefix(path, "go/acs/cycle") {
			continue
		}
		pkg := strings.Split(strings.TrimPrefix(path, "go/acs/"), "/")[0]
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "cat-file", "-e", base+":"+path); code == 0 {
			continue
		}
		hyphen := strings.Replace(pkg, "cycle", "cycle-", 1)
		for _, line := range strings.Split(lower, "\n") {
			for _, sentence := range strings.Split(line, ". ") {
				if !strings.Contains(sentence, pkg) && !strings.Contains(sentence, hyphen) {
					continue
				}
				for _, claim := range provenanceClaims {
					if strings.Contains(sentence, claim) {
						t.Errorf("RED: %s attributes %s to history (%q) in: %q -- but `git cat-file -e %s:%s` is ABSENT, "+
							"so that package is ADDED by this diff and its contract is this cycle's to reconcile",
							docPath, path, claim, strings.TrimSpace(sentence), base[:8], path)
					}
				}
			}
		}
	}
}
