//go:build acs

package cycle1942

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/buildplanner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/swarmplan"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	digestHeading         = "## Upstream Handoff Digest"
	scoutSizeSentinel     = "scout-size-1942"
	scoutGenericSentinel  = "scout-generic-1942"
	triageSizeSentinel    = "triage-size-1942"
	buildVerdictSentinel  = "build-verdict-1942"
	buildGenericSentinel  = "build-generic-1942"
	auditVerdictSentinel  = "audit-verdict-1942"
	auditGenericSentinel  = "audit-generic-1942"
	coreBatchTestPattern  = "^TestCoreCoverageBatch_"
	coverageBatchesParent = "core-package-coverage-remaining-batches"
	coverageUmbrella      = "core-package-coverage-to-100"
)

var coreBatchFunctions = []string{
	"mergerung2.go:parseUnifiedDiffToHunks",
	"mergerung2.go:parseDiffPath",
	"mergerung2.go:parseHunkHeader",
	"mergerung2.go:parseDiffRange",
	"failure_digest.go:defectHead",
	"failure_digest.go:causeHead",
	"runid.go:Verify",
}

type composer interface {
	ComposePrompt(body string, req core.PhaseRequest) string
}

func everyUpstream() phaseio.Handoffs {
	return phaseio.NewHandoffs(phaseio.HandoffsInit{
		Scout:  &phaseio.ScoutView{CycleSizeEstimate: scoutSizeSentinel, ItemCount: 2},
		Triage: &phaseio.TriageView{CycleSize: triageSizeSentinel, PhaseSkip: []string{"memo"}},
		Build:  &phaseio.BuildView{Verdict: buildVerdictSentinel, ACSGreen: 3, ACSTotal: 3},
		Audit:  &phaseio.AuditView{Verdict: auditVerdictSentinel, Confidence: 0.9},
		Generic: map[string]any{
			"scout.note":   scoutGenericSentinel,
			"build.note":   buildGenericSentinel,
			"audit.secret": auditGenericSentinel,
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

func assertEdgeDigest(t *testing.T, phase core.Phase, c composer, declared, undeclared []string) {
	t.Helper()
	req := activeRequest(phase, everyUpstream())
	prompt := c.ComposePrompt("BODY", req)
	if n := strings.Count(prompt, digestHeading); n != 1 {
		t.Fatalf("%s prompt carries %d upstream digest blocks; want exactly one:\n%s", phase, n, prompt)
	}
	want := req.Input.Upstream().DigestFor(string(phase))
	if got := fencedDigest(prompt); got != want || want == "" {
		t.Errorf("%s prompt digest = %q; want its configured edge digest %q", phase, got, want)
	}
	for _, sentinel := range declared {
		if !strings.Contains(prompt, sentinel) {
			t.Errorf("%s prompt omits declared upstream data %q", phase, sentinel)
		}
	}
	for _, leak := range undeclared {
		if strings.Contains(prompt, leak) {
			t.Errorf("%s prompt carries upstream data %q that phaseio.EdgeUpstreams does not declare for it", phase, leak)
		}
	}
	if inactive := c.ComposePrompt("BODY", core.PhaseRequest{}); strings.Contains(inactive, digestHeading) {
		t.Errorf("%s prompt renders a digest block for an inactive PhaseInput", phase)
	}
}

func TestC1942_001_BuildPlannerPromptConsumesItsEdgeDigest(t *testing.T) {
	phase := buildplanner.New(buildplanner.Config{}).BaseRunner()
	assertEdgeDigest(t, core.PhaseBuildPlanner, phase,
		[]string{scoutSizeSentinel, scoutGenericSentinel, triageSizeSentinel},
		[]string{buildVerdictSentinel, buildGenericSentinel, auditVerdictSentinel, auditGenericSentinel})
	inactiveReq := core.PhaseRequest{Cycle: 7, Workspace: "/ws"}
	if got, want := phase.ComposePrompt("BODY", inactiveReq), runner.BaseCycleContext("BODY", inactiveReq); got != want {
		t.Errorf("build-planner prompt for an inactive PhaseInput changed; want byte-identical BaseCycleContext\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestC1942_002_SwarmPlanPromptConsumesItsEdgeDigest(t *testing.T) {
	phase := swarmplan.New(swarmplan.Config{}).BaseRunner()
	assertEdgeDigest(t, core.PhaseSwarmPlan, phase,
		[]string{scoutSizeSentinel, scoutGenericSentinel, triageSizeSentinel},
		[]string{buildVerdictSentinel, buildGenericSentinel, auditVerdictSentinel, auditGenericSentinel})
	inactiveReq := core.PhaseRequest{Cycle: 7, Workspace: "/ws"}
	if got, want := phase.ComposePrompt("BODY", inactiveReq), runner.BaseCycleContext("BODY", inactiveReq); got != want {
		t.Errorf("swarm-plan prompt for an inactive PhaseInput changed; want byte-identical BaseCycleContext\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestC1942_003_DebuggerPromptConsumesItsEdgeDigest(t *testing.T) {
	phase := debugger.New(debugger.Config{})
	assertEdgeDigest(t, core.PhaseDebugger, phase,
		[]string{triageSizeSentinel, buildVerdictSentinel, buildGenericSentinel},
		[]string{scoutSizeSentinel, scoutGenericSentinel, auditVerdictSentinel, auditGenericSentinel})
	prompt := phase.ComposePrompt("BODY", activeRequest(core.PhaseDebugger, everyUpstream()))
	if !strings.Contains(prompt, "## Ship Failure Envelope") {
		t.Errorf("debugger prompt lost its Ship Failure Envelope section:\n%s", prompt)
	}
}

func TestC1942_004_IntentPromptCarriesNoUpstreamDigestOnItsEmptyEdge(t *testing.T) {
	upstreams, declared := phaseio.EdgeUpstreams(string(core.PhaseIntent))
	if !declared || len(upstreams) != 0 {
		t.Fatalf("intent edge = %v (declared=%v); want a declared empty edge — intent runs before every other phase", upstreams, declared)
	}
	phase := intent.New(intent.Config{})
	prompt := phase.ComposePrompt("BODY", activeRequest(core.PhaseIntent, everyUpstream()))
	if strings.Contains(prompt, digestHeading) {
		t.Errorf("intent prompt renders a digest block on its empty edge:\n%s", prompt)
	}
	for _, leak := range []string{scoutSizeSentinel, scoutGenericSentinel, triageSizeSentinel, buildVerdictSentinel, auditVerdictSentinel, auditGenericSentinel} {
		if strings.Contains(prompt, leak) {
			t.Errorf("intent prompt carries upstream data %q", leak)
		}
	}
	if !strings.Contains(prompt, "- mode: full") {
		t.Errorf("intent prompt lost its mode line:\n%s", prompt)
	}
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

func TestC1942_005_CoreDiffParserAndDigestHeadBatchReachesFullCoverage(t *testing.T) {
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

func TestC1942_006_UnmetCoverageRemainderIsQueuedAsTrackedInboxItem(t *testing.T) {
	src := sourceRoot(t)
	inboxDir := filepath.Join(src, ".evolve", "inbox")
	items, _, err := inboxbatch.LoadDir(inboxDir)
	if err != nil {
		t.Fatalf("inboxbatch.LoadDir(%s): %v", inboxDir, err)
	}
	var found *inboxbatch.Item
	for i := range items {
		it := items[i]
		if it.ID == coverageBatchesParent || it.ID == coverageUmbrella {
			continue
		}
		for _, edge := range append(append([]string{}, it.ConnectsTo...), it.Deps...) {
			if edge == coverageBatchesParent || edge == coverageUmbrella {
				found = &it
			}
		}
	}
	if found == nil {
		t.Fatalf("no new inbox-root item links to %s or %s (connects_to/deps) — a PASS retires this batch with the rest of internal/core and the .cover-strict registration recorded nowhere", coverageBatchesParent, coverageUmbrella)
	}
	if found.Title == "" || found.Weight <= 0 || len(found.Acceptance) == 0 {
		t.Errorf("remainder %s needs a title, a positive weight and acceptance; got title=%q weight=%v acceptance=%d", found.ID, found.Title, found.Weight, len(found.Acceptance))
	}
	path, ferr := inboxmover.FindFileByTaskID(inboxDir, found.ID)
	if ferr != nil {
		t.Fatalf("remainder %s is not claimable: %v", found.ID, ferr)
	}
	rel, _ := filepath.Rel(src, path)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", src, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is on disk but not git-tracked — it would be dropped at ship", rel)
	}
}
