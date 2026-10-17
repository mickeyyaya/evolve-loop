//go:build acs

package cycle1938

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	digestHeading      = "## Upstream Handoff Digest"
	auditLeakSentinel  = "AUDIT-LEAK-1938"
	buildLeakSentinel  = "BUILD-LEAK-1938"
	packageDefaultCap  = 1024
	scoutSizeSentinel  = "scout-size-1938"
	triageSizeSentinel = "triage-size-1938"
)

func fullUpstream() phaseio.Handoffs {
	return phaseio.NewHandoffs(phaseio.HandoffsInit{
		Scout:  &phaseio.ScoutView{CycleSizeEstimate: scoutSizeSentinel, ItemCount: 2},
		Triage: &phaseio.TriageView{CycleSize: triageSizeSentinel, PhaseSkip: []string{"memo"}},
		Build:  &phaseio.BuildView{Verdict: buildLeakSentinel, ACSGreen: 3, ACSTotal: 3},
		Audit:  &phaseio.AuditView{Verdict: auditLeakSentinel, Confidence: 0.9},
		Generic: map[string]any{
			"scout.note":   "scout-generic-1938",
			"audit.secret": auditLeakSentinel,
			"build.secret": buildLeakSentinel,
		},
	})
}

func activeRequest(phase core.Phase, up phaseio.Handoffs) core.PhaseRequest {
	return core.PhaseRequest{Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
		Phase:    string(phase),
		Upstream: up,
	})}
}

func fencedDigest(prompt string) string {
	_, after, ok := strings.Cut(prompt, digestHeading)
	if !ok {
		return ""
	}
	_, after, ok = strings.Cut(after, "```text\n")
	if !ok {
		return ""
	}
	block, _, ok := strings.Cut(after, "\n```")
	if !ok {
		return ""
	}
	return block
}

func containsAll(list []string, want ...string) bool {
	set := map[string]bool{}
	for _, v := range list {
		set[v] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func forbiddenHits(list []string, unwanted ...string) []string {
	var hits []string
	for _, v := range list {
		for _, u := range unwanted {
			if v == u {
				hits = append(hits, v)
			}
		}
	}
	return hits
}

func TestC1938_001_EveryComposePromptEdgeIsExplicitConfig(t *testing.T) {
	cases := []struct {
		phase      core.Phase
		mustHave   []string
		mustNotGet []string
	}{
		{core.PhaseScout, nil, []string{"triage", "build", "audit"}},
		{core.PhaseTriage, []string{"scout"}, []string{"build", "audit"}},
		{core.PhaseTDD, []string{"scout", "triage"}, []string{"build", "audit"}},
		{core.PhaseBuild, []string{"scout", "triage"}, []string{"build", "audit"}},
		{core.PhaseAudit, nil, []string{"build", "audit"}},
	}
	for _, tc := range cases {
		upstreams, declared := phaseio.EdgeUpstreams(string(tc.phase))
		if !declared {
			t.Errorf("edge into %q is not declared in phaseio's edge config — every ComposePrompt edge must be explicit", tc.phase)
			continue
		}
		if !containsAll(upstreams, tc.mustHave...) {
			t.Errorf("edge into %q declares upstreams %v; want at least %v", tc.phase, upstreams, tc.mustHave)
		}
		if hits := forbiddenHits(upstreams, tc.mustNotGet...); len(hits) > 0 {
			t.Errorf("edge into %q declares forbidden upstreams %v (a phase never digests itself or a later/graded phase)", tc.phase, hits)
		}
	}
	for _, phase := range []string{"intent", "build-planner", "swarm-plan", "debugger"} {
		upstreams, declared := phaseio.EdgeUpstreams(phase)
		if !declared {
			t.Errorf("ComposePrompt edge into %q is not declared in phaseio's edge config", phase)
		}
		if hits := forbiddenHits(upstreams, phase, "audit"); len(hits) > 0 {
			t.Errorf("edge into %q declares forbidden upstreams %v", phase, hits)
		}
	}
	if upstreams, declared := phaseio.EdgeUpstreams("no-such-phase-1938"); declared || len(upstreams) != 0 {
		t.Errorf("an undeclared phase reports declared=%v upstreams=%v; want declared=false and no upstreams", declared, upstreams)
	}
}

func TestC1938_002_DigestForRendersOnlyTheDeclaredSections(t *testing.T) {
	up := fullUpstream()
	triageDigest := up.DigestFor(string(core.PhaseTriage))
	if !strings.Contains(triageDigest, scoutSizeSentinel) || !strings.Contains(triageDigest, "scout-generic-1938") {
		t.Errorf("triage digest omits its declared scout section (typed and generic):\n%s", triageDigest)
	}
	for _, leak := range []string{triageSizeSentinel, buildLeakSentinel, auditLeakSentinel} {
		if strings.Contains(triageDigest, leak) {
			t.Errorf("triage digest carries undeclared upstream data %q:\n%s", leak, triageDigest)
		}
	}
	buildDigest := up.DigestFor(string(core.PhaseBuild))
	for _, want := range []string{scoutSizeSentinel, triageSizeSentinel} {
		if !strings.Contains(buildDigest, want) {
			t.Errorf("build digest omits declared upstream data %q:\n%s", want, buildDigest)
		}
	}
	for _, leak := range []string{buildLeakSentinel, auditLeakSentinel} {
		if strings.Contains(buildDigest, leak) {
			t.Errorf("build digest carries undeclared upstream data %q:\n%s", leak, buildDigest)
		}
	}
	for _, phase := range []string{string(core.PhaseScout), string(core.PhaseAudit), "no-such-phase-1938"} {
		if got := up.DigestFor(phase); got != "" {
			t.Errorf("DigestFor(%q) = %q; an edge with no declared upstream renders nothing, never a raw fallback", phase, got)
		}
	}
	if got := phaseio.NewHandoffs(phaseio.HandoffsInit{}).DigestFor(string(core.PhaseBuild)); got != "" {
		t.Errorf("DigestFor on empty handoffs = %q; want empty", got)
	}
}

func TestC1938_003_DigestForStaysUnderThePackageDefaultCap(t *testing.T) {
	generic := map[string]any{"scout.huge": strings.Repeat("x", 20*packageDefaultCap)}
	for i := 0; i < 40; i++ {
		generic[fmt.Sprintf("triage.k%02d", i)] = strings.Repeat("z", 200)
	}
	up := phaseio.NewHandoffs(phaseio.HandoffsInit{
		Scout:   &phaseio.ScoutView{CycleSizeEstimate: scoutSizeSentinel},
		Triage:  &phaseio.TriageView{CycleSize: triageSizeSentinel},
		Generic: generic,
	})
	got := up.DigestFor(string(core.PhaseBuild))
	if n := len([]rune(got)); n == 0 || n > packageDefaultCap {
		t.Fatalf("build digest is %d runes; want 1..%d (the package-default cap)", n, packageDefaultCap)
	}
	if !strings.Contains(got, "triage:") {
		t.Errorf("an oversized scout scalar evicted the triage section from the bounded build digest:\n%s", got)
	}
}

func TestC1938_004_TriagePromptConsumesItsConfiguredDigest(t *testing.T) {
	phase := triage.New(triage.Config{})
	req := activeRequest(core.PhaseTriage, fullUpstream())
	prompt := phase.ComposePrompt("BODY", req)
	if n := strings.Count(prompt, digestHeading); n != 1 {
		t.Fatalf("triage prompt carries %d digest blocks; want exactly one", n)
	}
	if got, want := fencedDigest(prompt), req.Input.Upstream().DigestFor(string(core.PhaseTriage)); got != want || want == "" {
		t.Errorf("triage prompt digest = %q; want its configured edge digest %q", got, want)
	}
	for _, leak := range []string{buildLeakSentinel, auditLeakSentinel} {
		if strings.Contains(prompt, leak) {
			t.Errorf("triage prompt carries undeclared upstream data %q", leak)
		}
	}
	inactive := phase.ComposePrompt("BODY", core.PhaseRequest{})
	if strings.Contains(inactive, digestHeading) {
		t.Errorf("triage prompt renders a digest block for an inactive PhaseInput (off/advisory must stay byte-compatible)")
	}
}

func TestC1938_005_BuildPromptConsumesItsConfiguredDigest(t *testing.T) {
	phase := build.New(build.Config{})
	req := activeRequest(core.PhaseBuild, fullUpstream())
	prompt := phase.ComposePrompt("BODY", req)
	if n := strings.Count(prompt, digestHeading); n != 1 {
		t.Fatalf("build prompt carries %d digest blocks; want exactly one", n)
	}
	if got, want := fencedDigest(prompt), req.Input.Upstream().DigestFor(string(core.PhaseBuild)); got != want || want == "" {
		t.Errorf("build prompt digest = %q; want its configured edge digest %q", got, want)
	}
	for _, leak := range []string{buildLeakSentinel, auditLeakSentinel} {
		if strings.Contains(prompt, leak) {
			t.Errorf("build prompt carries undeclared upstream data %q", leak)
		}
	}
	inactive := phase.ComposePrompt("BODY", core.PhaseRequest{})
	if strings.Contains(inactive, digestHeading) {
		t.Errorf("build prompt renders a digest block for an inactive PhaseInput (off/advisory must stay byte-compatible)")
	}
}

func TestC1938_006_AuditPromptReceivesNoBuilderSelfReport(t *testing.T) {
	phase := audit.New(audit.Config{})
	prompt := phase.ComposePrompt("BODY", activeRequest(core.PhaseAudit, fullUpstream()))
	if strings.Contains(prompt, digestHeading) {
		t.Errorf("audit prompt renders an upstream digest; its declared edge is empty so the auditor grades without the builder's self-report")
	}
	for _, leak := range []string{buildLeakSentinel, auditLeakSentinel} {
		if strings.Contains(prompt, leak) {
			t.Errorf("audit prompt carries upstream handoff data %q", leak)
		}
	}
}

const coreBatchTestPattern = "^TestCoreCoverageBatch_"

var coreBatchFunctions = []string{
	"phase_bindings.go:worktreeBaseTreeSHA",
	"phase_bindings.go:recordBuildBinding",
	"phase_bindings.go:recordGenericBinding",
	"phase_bindings_graduation.go:packageNewThisCycle",
	"phase_bindings_selfcheck.go:writeBuildSelfCheckArtifact",
	"ship_recovery_budget.go:contentionBackoff",
}

func sourceRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv(ipcenv.WorktreeRootKey); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func funcCoverage(report string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		file := filepath.Base(strings.SplitN(fields[0], ":", 2)[0])
		out[file+":"+fields[1]] = fields[len(fields)-1]
	}
	return out
}

func TestC1938_007_CoreBindingsBatchReachesFullCoverage(t *testing.T) {
	goDir := filepath.Join(sourceRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "core-batch.cov")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1",
		"-run", coreBatchTestPattern, "-coverprofile="+profile, "./internal/core")
	if code != 0 {
		t.Fatalf("go test -run %s ./internal/core: code=%d err=%v\n%s%s", coreBatchTestPattern, code, err, stdout, stderr)
	}
	if strings.Contains(stdout, "no tests to run") {
		t.Fatalf("no internal/core test matches %s — the batch's behavior tests are absent", coreBatchTestPattern)
	}
	report, rerr, rcode, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if rcode != 0 {
		t.Fatalf("go tool cover -func: code=%d err=%v\n%s", rcode, err, rerr)
	}
	got := funcCoverage(report)
	for _, fn := range coreBatchFunctions {
		pct, ok := got[fn]
		if !ok {
			t.Errorf("%s is absent from the coverage report — renamed or removed instead of tested", fn)
			continue
		}
		if pct != "100.0%" {
			t.Errorf("%s is at %s under the %s batch tests; want 100.0%% (one behavior test per uncovered branch)", fn, pct, coreBatchTestPattern)
		}
	}
}

func TestC1938_008_UnmetRemaindersAreQueuedAsTrackedInboxItems(t *testing.T) {
	src := sourceRoot(t)
	inboxDir := filepath.Join(src, ".evolve", "inbox")
	items, _, err := inboxbatch.LoadDir(inboxDir)
	if err != nil {
		t.Fatalf("inboxbatch.LoadDir(%s): %v", inboxDir, err)
	}
	for _, parent := range []string{"tokenopt-handoff-digests-per-edge-remainder", "core-package-coverage-to-100"} {
		var found *inboxbatch.Item
		for i := range items {
			it := items[i]
			if it.ID == parent {
				continue
			}
			for _, edge := range append(append([]string{}, it.ConnectsTo...), it.Deps...) {
				if edge == parent {
					found = &it
				}
			}
		}
		if found == nil {
			t.Errorf("no inbox-root item links to %s (connects_to/deps) — a PASS retires the parent with its unmet remainder recorded nowhere", parent)
			continue
		}
		if found.Title == "" || found.Weight <= 0 || len(found.Acceptance) == 0 {
			t.Errorf("remainder %s of %s needs a title, a positive weight and acceptance; got title=%q weight=%v acceptance=%d", found.ID, parent, found.Title, found.Weight, len(found.Acceptance))
		}
		path, ferr := inboxmover.FindFileByTaskID(inboxDir, found.ID)
		if ferr != nil {
			t.Errorf("remainder %s is not claimable: %v", found.ID, ferr)
			continue
		}
		rel, _ := filepath.Rel(src, path)
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", src, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("%s is on disk but not git-tracked — it would be dropped at ship", rel)
		}
	}
}
