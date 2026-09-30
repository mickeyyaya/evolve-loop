//go:build acs

package cycle1633

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/buildplanner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func threeItems() []inboxbatch.Item {
	return []inboxbatch.Item{
		{ID: "alpha", Title: "alpha", Weight: 0.8, Files: []string{"go/internal/alpha/a.go"}},
		{ID: "beta", Title: "beta", Weight: 0.7, Files: []string{"go/internal/beta/b.go"}},
		{ID: "gamma", Title: "gamma", Weight: 0.6, Files: []string{"go/internal/gamma/c.go"}},
	}
}

func chainItems(n int) []inboxbatch.Item {
	items := make([]inboxbatch.Item, 0, n)
	for i := 1; i <= n; i++ {
		it := inboxbatch.Item{
			ID:         fmt.Sprintf("delta%d", i),
			Title:      fmt.Sprintf("delta %d", i),
			Weight:     0.5,
			Files:      []string{fmt.Sprintf("go/internal/delta%d/d.go", i)},
			Acceptance: []string{fmt.Sprintf("delta%d acceptance: its own retry call site uses the shared Retrier", i)},
		}
		if i > 1 {
			it.Deps = []string{fmt.Sprintf("delta%d", i-1)}
		}
		items = append(items, it)
	}
	return items
}

func completeCommitment(items []inboxbatch.Item) inboxbatch.UnifiedCommitment {
	members := make([]inboxbatch.UnifiedMember, 0, len(items))
	for _, it := range items {
		members = append(members, inboxbatch.UnifiedMember{
			ID:       it.ID,
			Evidence: it.ID + " re-implements the retry loop in " + it.Files[0] + " — one of N copies of the same bug",
		})
	}
	return inboxbatch.UnifiedCommitment{
		RootCauseHypothesis: "N callers each hand-roll the same retry/backoff loop; every member is one copy of that disease",
		SharedSeam:          "go/internal/retry (single Retrier with policy injection)",
		DesignRequirements:  []string{"single source with projection", "Strategy over flags", "immutable policy values"},
		Members:             members,
	}
}

func decisionJSON(t *testing.T, topN []string, c *inboxbatch.UnifiedCommitment) string {
	t.Helper()
	top := make([]map[string]string, 0, len(topN))
	for _, id := range topN {
		top = append(top, map[string]string{"id": id})
	}
	doc := map[string]any{"cycle": 1633, "top_n": top}
	if c != nil {
		doc["unified_commitment"] = c
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func reportMD(topN []string) string {
	var b strings.Builder
	b.WriteString("cycle_size_estimate: medium\ndeliverable_kind: code\n\n## top_n\n")
	for _, id := range topN {
		fmt.Fprintf(&b, "- %s: close %s, files=go/internal/%s/x.go\n", id, id, id)
	}
	b.WriteString("\n## deferred\n- none\n\n## dropped\n- none\n")
	return b.String()
}

type fakeBridge struct{ report, decision string }

func (b *fakeBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755); err != nil {
		return core.BridgeResponse{}, err
	}
	if err := os.WriteFile(req.ArtifactPath, []byte(b.report), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	dec := filepath.Join(filepath.Dir(req.ArtifactPath), "triage-decision.json")
	if err := os.WriteFile(dec, []byte(b.decision), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	return core.BridgeResponse{Stdout: b.report}, nil
}

func (b *fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func writeInbox(t *testing.T, items []inboxbatch.Item) string {
	t.Helper()
	proj := t.TempDir()
	dir := filepath.Join(proj, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		raw, err := json.Marshal(it)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, it.ID+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return proj
}

func runTriage(t *testing.T, items []inboxbatch.Item, topN []string, c *inboxbatch.UnifiedCommitment) (core.PhaseResponse, string) {
	t.Helper()
	proj := writeInbox(t, items)
	ws := t.TempDir()
	fb := &fakeBridge{report: reportMD(topN), decision: decisionJSON(t, topN, c)}
	ph := triage.New(triage.Config{
		Bridge: fb,
		Prompts: prompts.NewFromFS(fstest.MapFS{
			"agents/evolve-triage.md": &fstest.MapFile{Data: []byte("---\nname: evolve-triage\n---\ntriage body")},
		}),
	})
	resp, err := ph.Run(context.Background(), core.PhaseRequest{Cycle: 1633, ProjectRoot: proj, Workspace: ws})
	if err != nil {
		t.Fatalf("triage runner: %v", err)
	}
	return resp, ws
}

func digestTriage(t *testing.T, ws string) router.RoutingSignals {
	t.Helper()
	sig, err := router.Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("router.Digest: %v", err)
	}
	return sig
}

func realRegistryConfig(t *testing.T) config.RoutingConfig {
	t.Helper()
	root := acsassert.RepoRoot(t)
	cfg, _ := config.Load(filepath.Join(root, "docs", "architecture", "phase-registry.json"), map[string]string{})
	return cfg
}

func declinedPlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "triage", Run: true},
		{Phase: "plan-review", Run: false}, {Phase: "tdd", Run: true},
		{Phase: "build-planner", Run: false}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

func walkPhases(cfg config.RoutingConfig, sig router.RoutingSignals, plan *router.PhasePlan, from string, completed []string) []string {
	var visited []string
	cur := from
	done := append([]string(nil), completed...)
	for i := 0; i < 16; i++ {
		d := router.Route(router.RouteInput{Current: cur, Verdict: "PASS", Signals: sig, Cfg: cfg, Completed: done, Plan: plan}, nil)
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

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func hasDiagMentioning(diags []core.Diagnostic, needle string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, needle) {
			return true
		}
	}
	return false
}

func memberIDs(items []inboxbatch.Item) []string {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

func TestC1633_001_UnifiedCommitmentAcceptsCompleteEvidenceCitedMembers(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	if err := c.Validate(items); err != nil {
		t.Fatalf("complete, evidence-cited commitment over known items must validate; got %v", err)
	}
	if got := c.Size(); got != "small" {
		t.Errorf("3 members (<= DefaultMaxItems=%d) must be size %q; got %q", inboxbatch.DefaultMaxItems, "small", got)
	}
}

func TestC1633_002_UnifiedCommitmentRejectsMissingEvidence(t *testing.T) {
	items := threeItems()
	for _, blank := range []string{"", "   \t"} {
		c := completeCommitment(items)
		c.Members[1].Evidence = blank
		err := c.Validate(items)
		if err == nil {
			t.Fatalf("member %q with evidence %q must be rejected (evidence-cited, not vibes)", c.Members[1].ID, blank)
		}
		if !strings.Contains(err.Error(), "beta") {
			t.Errorf("rejection must name the offending member %q; got %v", "beta", err)
		}
	}
}

func TestC1633_003_UnifiedCommitmentRejectsUnknownMember(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	c.Members = append(c.Members, inboxbatch.UnifiedMember{ID: "phantom", Evidence: "cites the root cause"})
	err := c.Validate(items)
	if err == nil {
		t.Fatal("a member that is not a known inbox item must be rejected")
	}
	if !strings.Contains(err.Error(), "phantom") {
		t.Errorf("rejection must name the unknown member; got %v", err)
	}
}

func TestC1633_004_UnifiedCommitmentRejectsDuplicateMember(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	c.Members = append(c.Members, c.Members[0])
	err := c.Validate(items)
	if err == nil {
		t.Fatal("a duplicated member id must be rejected (double-counting one item as two closures)")
	}
	if !strings.Contains(err.Error(), "alpha") {
		t.Errorf("rejection must name the duplicated member; got %v", err)
	}
}

func TestC1633_005_UnifiedCommitmentRejectsIncompleteShape(t *testing.T) {
	items := threeItems()
	cases := []struct {
		name   string
		mutate func(c *inboxbatch.UnifiedCommitment)
	}{
		{"empty root cause", func(c *inboxbatch.UnifiedCommitment) { c.RootCauseHypothesis = "" }},
		{"blank root cause", func(c *inboxbatch.UnifiedCommitment) { c.RootCauseHypothesis = "  " }},
		{"empty shared seam", func(c *inboxbatch.UnifiedCommitment) { c.SharedSeam = "" }},
		{"no design requirements", func(c *inboxbatch.UnifiedCommitment) { c.DesignRequirements = nil }},
		{"blank design requirement", func(c *inboxbatch.UnifiedCommitment) { c.DesignRequirements = []string{" "} }},
		{"single member is not unified", func(c *inboxbatch.UnifiedCommitment) { c.Members = c.Members[:1] }},
		{"zero members", func(c *inboxbatch.UnifiedCommitment) { c.Members = nil }},
		{"empty member id", func(c *inboxbatch.UnifiedCommitment) { c.Members[0].ID = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := completeCommitment(items)
			tc.mutate(&c)
			if err := c.Validate(items); err == nil {
				t.Errorf("%s: incomplete commitment must be rejected (fail-open to independent top_n)", tc.name)
			}
		})
	}
	if err := (inboxbatch.UnifiedCommitment{}).Validate(items); err == nil {
		t.Error("zero-value commitment must be rejected")
	}
}

func TestC1633_006_UnifiedCommitmentRejectsHeterogeneousMembers(t *testing.T) {
	t.Run("distinct campaigns", func(t *testing.T) {
		items := threeItems()
		items[0].Campaign = "retry-2026-08"
		items[2].Campaign = "docs-2026-09"
		c := completeCommitment(items)
		if err := c.Validate(items); err == nil {
			t.Fatal("members spanning two distinct non-empty campaigns must be rejected as heterogeneous")
		}
	})
	t.Run("one campaign is homogeneous", func(t *testing.T) {
		items := threeItems()
		for i := range items {
			items[i].Campaign = "retry-2026-08"
		}
		c := completeCommitment(items)
		if err := c.Validate(items); err != nil {
			t.Fatalf("members sharing one campaign must validate; got %v", err)
		}
	})
	t.Run("mixed deliverable kinds", func(t *testing.T) {
		items := threeItems()
		items[1].DeliverableKind = "document"
		c := completeCommitment(items)
		if err := c.Validate(items); err == nil {
			t.Fatal("members mixing code and document deliverable kinds must be rejected as heterogeneous")
		}
	})
}

func TestC1633_007_UnifiedCommitmentSizeBoundaryIsDefaultMaxItems(t *testing.T) {
	small := chainItems(inboxbatch.DefaultMaxItems)
	cs := completeCommitment(small)
	if err := cs.Validate(small); err != nil {
		t.Fatalf("%d-member commitment must validate; got %v", len(small), err)
	}
	if got := cs.Size(); got != "small" {
		t.Errorf("exactly DefaultMaxItems members must be %q; got %q", "small", got)
	}
	large := chainItems(inboxbatch.DefaultMaxItems + 1)
	cl := completeCommitment(large)
	if err := cl.Validate(large); err != nil {
		t.Fatalf("%d-member commitment must validate; got %v", len(large), err)
	}
	if got := cl.Size(); got != "large" {
		t.Errorf("DefaultMaxItems+1 members must be %q; got %q", "large", got)
	}
}

func TestC1633_008_DefaultBatchRulesUnchangedBySynthesis(t *testing.T) {
	if got := len(inboxbatch.DefaultRules()); got != 2 {
		t.Errorf("DefaultRules must remain the 2 structural signals (campaign, file-area); got %d", got)
	}
	batches := inboxbatch.Classify(threeItems(), inboxbatch.Config{})
	if len(batches) != 3 {
		t.Errorf("unrelated items must stay independent batches: got %d, want 3", len(batches))
	}
}

func TestC1633_009_TriagePhaseFailsOpenOnInvalidCommitment(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	c.Members[2] = inboxbatch.UnifiedMember{ID: "phantom", Evidence: "not an inbox item"}
	resp, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("an invalid synthesis claim must fail OPEN to the independent top_n (verdict PASS); got %q diags=%v", resp.Verdict, resp.Diagnostics)
	}
	if !hasDiagMentioning(resp.Diagnostics, "unified_commitment") {
		t.Errorf("fail-open must be LOUD: expected a diagnostic mentioning unified_commitment; got %v", resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "" {
		t.Errorf("router must see NO unified signal after an invalid claim; got UnifiedSize=%q", sig.Triage.UnifiedSize)
	}
	if sig.Triage.CommittedCount != 3 {
		t.Errorf("independent top_n must survive fail-open: CommittedCount=%d, want 3", sig.Triage.CommittedCount)
	}
}

func TestC1633_010_TriagePhaseProjectsValidSmallCommitment(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	resp, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("valid commitment: verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "small" {
		t.Errorf("router.Digest must project the validated commitment: UnifiedSize=%q, want %q", sig.Triage.UnifiedSize, "small")
	}
	if sig.Triage.UnifiedMemberCount != 3 {
		t.Errorf("UnifiedMemberCount=%d, want 3", sig.Triage.UnifiedMemberCount)
	}
	if sig.Triage.CommittedCount != 3 {
		t.Errorf("per-member acceptance stays separate: the independent top_n count must be preserved (got %d, want 3)", sig.Triage.CommittedCount)
	}
}

func TestC1633_011_TriagePhaseRejectsMemberOutsideTopN(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	resp, ws := runTriage(t, items, []string{"alpha", "beta"}, &c)
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS (fail-open); diags=%v", resp.Verdict, resp.Diagnostics)
	}
	if !hasDiagMentioning(resp.Diagnostics, "gamma") {
		t.Errorf("rejection must name the uncommitted member gamma; diags=%v", resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "" {
		t.Errorf("a claim whose members exceed top_n must not project: UnifiedSize=%q", sig.Triage.UnifiedSize)
	}
	if sig.Triage.CommittedCount != 2 {
		t.Errorf("the independent top_n must survive the rejection untouched: CommittedCount=%d, want 2", sig.Triage.CommittedCount)
	}
}

func TestC1633_012_SmallCommitmentPinsPlanReviewViaRegistry(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	_, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	sig := digestTriage(t, ws)
	cfg := realRegistryConfig(t)
	visited := walkPhases(cfg, sig, declinedPlan(), "triage", []string{"scout", "triage"})
	if !contains(visited, "plan-review") {
		t.Errorf("a validated small commitment must pin plan-review even against an advisor plan that declined it; walk visited %v", visited)
	}
	_, ws0 := runTriage(t, items, []string{"alpha", "beta", "gamma"}, nil)
	base := walkPhases(cfg, digestTriage(t, ws0), declinedPlan(), "triage", []string{"scout", "triage"})
	if contains(base, "plan-review") {
		t.Errorf("no commitment must leave routing unchanged (plan-review not pinned); baseline walk visited %v", base)
	}
	large := chainItems(inboxbatch.DefaultMaxItems + 1)
	cl := completeCommitment(large)
	_, wsL := runTriage(t, large, memberIDs(large), &cl)
	if got := walkPhases(cfg, digestTriage(t, wsL), declinedPlan(), "triage", []string{"scout", "triage"}); !contains(got, "plan-review") {
		t.Errorf("a validated LARGE unified commitment must pin plan-review exactly as a small one does; walk visited %v", got)
	}
}

func TestC1633_013_SmallCommitmentPinsBuildPlannerViaRegistry(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	_, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	sig := digestTriage(t, ws)
	cfg := realRegistryConfig(t)
	done := []string{"scout", "triage", "plan-review", "tdd"}
	visited := walkPhases(cfg, sig, declinedPlan(), "tdd", done)
	if !contains(visited, "build-planner") {
		t.Errorf("a validated small commitment must pin build-planner even against an advisor plan that declined it; walk from tdd visited %v", visited)
	}
	_, ws0 := runTriage(t, items, []string{"alpha", "beta", "gamma"}, nil)
	base := walkPhases(cfg, digestTriage(t, ws0), declinedPlan(), "tdd", done)
	if contains(base, "build-planner") {
		t.Errorf("no commitment must leave build-planner opt-in (skipped); baseline walk visited %v", base)
	}
	large := chainItems(inboxbatch.DefaultMaxItems + 1)
	cl := completeCommitment(large)
	_, wsL := runTriage(t, large, memberIDs(large), &cl)
	sigL := digestTriage(t, wsL)
	if sigL.Triage.UnifiedSize != "large" {
		t.Fatalf("precondition: large commitment must project UnifiedSize=large; got %q", sigL.Triage.UnifiedSize)
	}
	if got := walkPhases(cfg, sigL, declinedPlan(), "tdd", done); !contains(got, "build-planner") {
		t.Errorf("a validated LARGE unified commitment must ALSO pin build-planner — the campaign path is the one design that must not outrun planning (how_to_apply step 2 names both planning phases without a size qualifier); walk from tdd visited %v", got)
	}
}

func TestC1633_014_BuildPlannerPhaseRunsForSmallCommitment(t *testing.T) {
	root := acsassert.RepoRoot(t)
	items := threeItems()
	c := completeCommitment(items)
	_, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	bp := buildplanner.New(buildplanner.Config{})
	skipped, verdict, _, _ := bp.ShouldSkip(core.PhaseRequest{Cycle: 1633, ProjectRoot: root, Workspace: ws, Env: map[string]string{}})
	if skipped {
		t.Errorf("build-planner must RUN for a validated small commitment; ShouldSkip=true verdict=%q", verdict)
	}
	_, ws0 := runTriage(t, items, []string{"alpha", "beta", "gamma"}, nil)
	skipped0, _, _, _ := bp.ShouldSkip(core.PhaseRequest{Cycle: 1633, ProjectRoot: root, Workspace: ws0, Env: map[string]string{}})
	if !skipped0 {
		t.Error("build-planner must stay opt-in (skipped) when no commitment is present")
	}
}

func TestC1633_015_LargeCommitmentProjectsToVerifiedCampaignPlan(t *testing.T) {
	items := chainItems(inboxbatch.DefaultMaxItems + 1)
	c := completeCommitment(items)
	plan, err := campaign.PlanFromUnifiedCommitment(c, items)
	if err != nil {
		t.Fatalf("PlanFromUnifiedCommitment: %v", err)
	}
	if err := plan.Verify(); err != nil {
		t.Fatalf("projected plan must pass the deterministic trust boundary: %v", err)
	}
	if strings.TrimSpace(plan.Goal) == "" {
		t.Error("plan goal must carry the commitment's root cause (non-empty)")
	}
	if len(plan.Cycles) != len(items) {
		t.Fatalf("one cycle per member: got %d, want %d", len(plan.Cycles), len(items))
	}
	byID := map[string]string{}
	for _, cy := range plan.Cycles {
		byID[cy.ID] = cy.OutputContract
	}
	for _, it := range items {
		contract, ok := byID[it.ID]
		if !ok {
			t.Errorf("member %q has no cycle in the projected plan", it.ID)
			continue
		}
		if !strings.Contains(contract, it.Acceptance[0]) {
			t.Errorf("cycle %q must carry its member's own acceptance %q; got contract %q", it.ID, it.Acceptance[0], contract)
		}
	}
	waves, err := plan.Waves()
	if err != nil {
		t.Fatalf("Waves: %v", err)
	}
	if len(waves) != len(items) {
		t.Errorf("a %d-long dep chain must yield %d waves; got %d", len(items), len(items), len(waves))
	}
	for i, w := range waves {
		want := fmt.Sprintf("delta%d", i+1)
		if len(w) != 1 || len(w[0].Scope) != 1 || w[0].Scope[0] != want {
			t.Errorf("wave %d must hold exactly one cycle scoped to %q; got %+v", i+1, want, w)
		}
	}
	bad := completeCommitment(items)
	bad.Members[0].Evidence = ""
	if _, err := campaign.PlanFromUnifiedCommitment(bad, items); err == nil {
		t.Error("an invalid commitment must be refused by the campaign projection")
	}
}

func TestC1633_016_TriagePhaseEmitsCampaignPlanForLargeCommitment(t *testing.T) {
	items := chainItems(inboxbatch.DefaultMaxItems + 1)
	c := completeCommitment(items)
	ids := memberIDs(items)
	resp, ws := runTriage(t, items, ids, &c)
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "large" {
		t.Errorf("UnifiedSize=%q, want %q", sig.Triage.UnifiedSize, "large")
	}
	planPath := filepath.Join(ws, "campaign-plan.json")
	if !acsassert.FileExists(t, planPath) {
		t.Fatalf("a validated LARGE commitment must emit %s for `evolve campaign run --plan`", planPath)
	}
	plan, err := campaign.LoadFile(planPath)
	if err != nil {
		t.Fatalf("emitted campaign plan must load: %v", err)
	}
	if err := plan.Verify(); err != nil {
		t.Fatalf("emitted campaign plan must verify: %v", err)
	}
	got := make([]string, 0, len(plan.Cycles))
	for _, cy := range plan.Cycles {
		got = append(got, cy.ID)
	}
	for _, id := range ids {
		if !contains(got, id) {
			t.Errorf("emitted plan must carry every member as a cycle; missing %q in %v", id, got)
		}
	}
	small := threeItems()
	cs := completeCommitment(small)
	_, wsS := runTriage(t, small, []string{"alpha", "beta", "gamma"}, &cs)
	if _, err := os.Stat(filepath.Join(wsS, "campaign-plan.json")); err == nil {
		t.Error("a SMALL commitment must not emit campaign-plan.json")
	}
}

func TestC1633_017_CycleACSPackageIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "acs", "cycle1633", "predicates_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); err != nil || code != 0 {
		t.Errorf("RED: %s is untracked (git ls-files exit=%d err=%v) — the audit's predicate tree would carry an input absent from the ship tree", rel, code, err)
	}
}
