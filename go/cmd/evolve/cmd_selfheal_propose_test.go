package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	liveIncidentFixtures = "../../internal/selfheal/testdata/incidents-2026-10-27"
	incCIHang            = "inc-2026-10-27-235078cd"
	incAuditStall        = "inc-2026-10-27-56437896"
	incBuildStall        = "inc-2026-10-27-9abfa496"
	incChurn             = "inc-2026-10-27-408110e5"
)

var incidentIDPattern = regexp.MustCompile(`inc-\d{4}-\d{2}-\d{2}-[0-9a-f]{8}`)

var diagnosisActionOf = map[string]string{
	incCIHang: `{"kind":"open-repair-item","params":{"files":["go/internal/gc/sweep.go"],"repro":"a boundary run after a merge hangs"}}`,
	incChurn:  `{"kind":"run-shell","params":{"reason":"rm -rf .evolve/inbox"}}`,
}

type diagnosisBridge struct {
	mu       sync.Mutex
	requests []core.BridgeRequest
}

func (b *diagnosisBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.mu.Lock()
	b.requests = append(b.requests, req)
	b.mu.Unlock()
	id := incidentIDPattern.FindString(req.Prompt)
	action, ok := diagnosisActionOf[id]
	if !ok {
		action = `{"kind":"quarantine-item","params":{"item":"retro-minting-files-through-inbox-add-lane"}}`
	}
	evidence, confidence := `["i1"]`, "medium"
	if id == incBuildStall {
		evidence = `["i1","s9"]`
	}
	if id == incAuditStall {
		confidence = "low"
	}
	body := `{"schema":"evolve.diagnosis/1","incident_id":"` + id + `","hypothesis":"the cause of ` + id + `","evidence":` + evidence +
		`,"action":` + action + `,"expected_effect":"the signal stops","confidence":"` + confidence + `"}`
	return core.BridgeResponse{}, os.WriteFile(req.ArtifactPath, []byte(body), 0o644)
}

func (b *diagnosisBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func proposeProject(t *testing.T, healing string) string {
	t.Helper()
	repo := repoRootForSkills(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"docs/architecture/phase-registry.json",
		".evolve/phases/selfheal-orchestrator/phase.json",
		".evolve/profiles/selfheal-orchestrator.json",
		"agents/evolve-selfheal-orchestrator.md",
	} {
		copyFileForTest(t, filepath.Join(repo, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel)))
	}
	policyJSON := `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}, "healing": ` + healing + `}`
	selfhealFile(t, filepath.Join(paths.EvolveDirOf(root), paths.PolicyFile), policyJSON)
	incidents, err := selfheal.LoadIncidents(liveIncidentFixtures)
	if err != nil || len(incidents) != 8 {
		t.Fatalf("the 8 records of the live pass of #1186: %d %v", len(incidents), err)
	}
	for _, inc := range incidents {
		if err := selfheal.WriteIncident(selfheal.Dir(paths.EvolveDirOf(root)), inc); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func fakeBridgeAgent(t *testing.T, b *diagnosisBridge) func([]string, io.Reader, io.Writer, io.Writer) int {
	t.Helper()
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	var tick time.Duration
	return agentcmd.New(agentcmd.Deps{
		InstallRouter: rootRouterInstaller,
		Bridge:        func(string, *signalcenter.Center) core.Bridge { return b },
		Signals:       quietSignals,
		TreeState:     fixedTreeState("tree"),
		Getenv:        func(string) string { return "" },
		Now: func() time.Time {
			tick += time.Second
			return selfhealNow.Add(tick)
		},
	})
}

type proposeEnvelope struct {
	Schema   string              `json:"schema"`
	OK       bool                `json:"ok"`
	Data     proposeReport       `json:"data"`
	Error    *struct{ Code int } `json:"error"`
	Warnings []string            `json:"warnings"`
}

func runProposeJSON(t *testing.T, env selfhealEnv, root string) (int, proposeEnvelope, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runSelfhealWith(env, []string{"propose", "--json", "--project-root", root}, &stdout, &stderr)
	var e proposeEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &e); err != nil {
		t.Fatalf("rc=%d: no JSON envelope: %v\n%s\n%s", rc, err, stdout.String(), stderr.String())
	}
	return rc, e, stderr.String()
}

func proposalOf(t *testing.T, ps []selfheal.Proposal, id string) selfheal.Proposal {
	t.Helper()
	i := slices.IndexFunc(ps, func(p selfheal.Proposal) bool { return p.IncidentID == id })
	if i < 0 {
		t.Fatalf("no proposal for %s", id)
	}
	return ps[i]
}

func TestSelfhealPropose_TheLiveIncidentsEachGetOneCheckedProposalThroughTheBridge(t *testing.T) {
	root := proposeProject(t, `{"orchestrator":{"max_per_pass":8}}`)
	b := &diagnosisBridge{}
	env := selfhealTestEnv(fixtureGH)
	env.agent = fakeBridgeAgent(t, b)

	rc, e, stderr := runProposeJSON(t, env, root)

	if rc != 0 || !e.OK || e.Schema != selfhealSchemas[selfhealVerbPropose] || e.Data.Stage != "shadow" || len(e.Data.Proposals) != 8 || len(b.requests) != 8 {
		t.Fatalf("rc=%d envelope=%+v launches=%d\n%s", rc, e, len(b.requests), stderr)
	}
	for _, req := range b.requests {
		id := incidentIDPattern.FindString(req.Prompt)
		ws := selfheal.EvidenceWorkspace(paths.EvolveDirOf(root), id)
		if req.CLI == "" || !strings.Contains(req.Prompt, "evolve context index --workspace "+ws) {
			t.Errorf("%s: the seat must be routed and name its evidence workspace: cli=%q", id, req.CLI)
		}
		if _, err := inputindex.Read(ws); err != nil {
			t.Errorf("%s: no evidence index: %v", id, err)
		}
	}
	if p := proposalOf(t, e.Data.Proposals, incCIHang); p.State != selfheal.ProposalProposed || p.Action.Kind != phasecontract.ActionOpenRepairItem || p.Tier != "T1" {
		t.Errorf("ci-hang: %+v", p)
	}
	if p := proposalOf(t, e.Data.Proposals, incAuditStall); p.State != selfheal.ProposalProposed || p.Action.Kind != phasecontract.ActionHaltAndEscalate || p.Diagnosis.Action.Kind != phasecontract.ActionQuarantineItem {
		t.Errorf("a low diagnosis must propose halt-and-escalate and keep its own action: %+v", p)
	}
	if p := proposalOf(t, e.Data.Proposals, incChurn); p.State != selfheal.ProposalRefused || p.Action != nil || !strings.Contains(p.Error, `"run-shell" is not in the catalog`) {
		t.Errorf("an action outside the catalog must be refused: %+v", p)
	}
	if p := proposalOf(t, e.Data.Proposals, incBuildStall); p.State != selfheal.ProposalRefused || !strings.Contains(p.Error, `"s9" is not an entry`) {
		t.Errorf("an evidence id outside the index must be refused: %+v", p)
	}
	if len(e.Warnings) != 2 || !strings.Contains(stderr, "WARN: "+incChurn+" refused") {
		t.Errorf("each refusal is a warning: %v\n%s", e.Warnings, stderr)
	}
	stored, err := selfheal.LoadProposals(selfheal.ProposalsDir(paths.EvolveDirOf(root)))
	if err != nil || len(stored) != 8 {
		t.Fatalf("one record for each incident: %d %v", len(stored), err)
	}

	b.requests = nil
	rc, again, _ := runProposeJSON(t, env, root)
	if rc != 0 || len(again.Data.Proposals) != 0 || len(b.requests) != 0 {
		t.Fatalf("a second pass with no change launches nothing: rc=%d proposals=%d launches=%d", rc, len(again.Data.Proposals), len(b.requests))
	}
}

func TestSelfhealPropose_TheDefaultPassDiagnosesTheTwoMostSevereIncidents(t *testing.T) {
	root := proposeProject(t, `{}`)
	b := &diagnosisBridge{}
	env := selfhealTestEnv(fixtureGH)
	env.agent = fakeBridgeAgent(t, b)
	var stdout, stderr bytes.Buffer

	rc := runSelfhealWith(env, []string{"propose", "--project-root", root}, &stdout, &stderr)

	if rc != 0 || len(b.requests) != 2 {
		t.Fatalf("rc=%d launches=%d\n%s", rc, len(b.requests), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "selfheal propose: stage shadow, 2 proposal(s)") || !strings.Contains(out, incChurn+" T2 refused: diagnosis.json") || !strings.Contains(out, "inc-2026-10-27-6f89f94c T2 proposed: quarantine-item") {
		t.Errorf("the text report:\n%s", out)
	}
}

func TestSelfhealPropose_StageOffLaunchesNoAgent(t *testing.T) {
	t.Parallel()
	root := proposeProject(t, `{"orchestrator":{"stage":"off"}}`)
	env := selfhealTestEnv(fixtureGH)
	env.agent = func([]string, io.Reader, io.Writer, io.Writer) int {
		t.Error("stage off must launch no agent")
		return 0
	}
	rc, e, _ := runProposeJSON(t, env, root)
	if rc != 0 || e.Data.Stage != "off" || len(e.Data.Proposals) != 0 {
		t.Fatalf("rc=%d %+v", rc, e)
	}
}

func scriptedAgent(stdout string, rc int) func([]string, io.Reader, io.Writer, io.Writer) int {
	return func(_ []string, _ io.Reader, w io.Writer, _ io.Writer) int {
		io.WriteString(w, stdout)
		return rc
	}
}

func TestSelfhealPropose_AFailedRunIsRecordedAndTriedAgain(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		agent func([]string, io.Reader, io.Writer, io.Writer) int
		want  string
	}{
		"a dispatch error": {scriptedAgent(`{"run_label":"agent-x","error":"bridge down"}`, 21), "exit 21: bridge down"},
		"no envelope":      {scriptedAgent("not json", 3), "exit 3 with no JSON envelope"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := proposeProject(t, `{"orchestrator":{"max_per_pass":1}}`)
			env := selfhealTestEnv(fixtureGH)
			env.agent = c.agent
			rc, e, _ := runProposeJSON(t, env, root)
			if rc != 0 || len(e.Data.Proposals) != 1 || e.Data.Proposals[0].State != selfheal.ProposalFailed || !strings.Contains(e.Data.Proposals[0].Error, c.want) {
				t.Fatalf("rc=%d %+v", rc, e.Data.Proposals)
			}
			_, again, _ := runProposeJSON(t, env, root)
			if len(again.Data.Proposals) != 1 || again.Data.Proposals[0].IncidentID != e.Data.Proposals[0].IncidentID {
				t.Fatalf("a failed run is tried again at the next pass: %+v", again.Data.Proposals)
			}
		})
	}
}

func TestSelfhealPropose_AnEvidenceFaultIsAFailedProposalWithNoLaunch(t *testing.T) {
	t.Parallel()
	root := proposeProject(t, `{"orchestrator":{"max_per_pass":1}}`)
	selfhealFile(t, filepath.Join(paths.EvolveDirOf(root), "runs"), "a file")
	env := selfhealTestEnv(fixtureGH)
	env.agent = func([]string, io.Reader, io.Writer, io.Writer) int {
		t.Error("no launch without an evidence index")
		return 0
	}
	rc, e, _ := runProposeJSON(t, env, root)
	if rc != 0 || len(e.Data.Proposals) != 1 || e.Data.Proposals[0].State != selfheal.ProposalFailed || e.Data.Proposals[0].RunLabel != "" {
		t.Fatalf("rc=%d %+v", rc, e.Data.Proposals)
	}
}

func TestSelfhealPropose_IOFaultsAreExitTwo(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, root string){
		"a corrupt incident record": func(t *testing.T, root string) {
			selfhealFile(t, filepath.Join(selfheal.Dir(paths.EvolveDirOf(root)), "inc-bad.json"), "{")
		},
		"a corrupt proposal record": func(t *testing.T, root string) {
			selfhealFile(t, filepath.Join(selfheal.ProposalsDir(paths.EvolveDirOf(root)), "inc-bad.json"), "{")
		},
		"an unreadable inbox channel": func(t *testing.T, root string) {
			selfhealFile(t, filepath.Join(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "inbox"), "a file")
		},
		"a proposal that cannot be written": func(t *testing.T, root string) {
			for _, id := range []string{"inc-2026-10-27-408110e5", "inc-2026-10-27-6f89f94c"} {
				if err := os.MkdirAll(filepath.Join(selfheal.ProposalsDir(paths.EvolveDirOf(root)), id+".json"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := proposeProject(t, `{}`)
			setup(t, root)
			env := selfhealTestEnv(fixtureGH)
			env.agent = scriptedAgent("not json", 21)
			rc, e, _ := runProposeJSON(t, env, root)
			if rc != exitIO || e.OK || e.Error == nil || e.Error.Code != exitIO {
				t.Fatalf("rc=%d envelope=%+v", rc, e)
			}
		})
	}
}

func TestSelfhealPropose_NoPendingIncidentReadsNoSignal(t *testing.T) {
	t.Parallel()
	root := proposeProject(t, `{}`)
	if err := os.RemoveAll(selfheal.Dir(paths.EvolveDirOf(root))); err != nil {
		t.Fatal(err)
	}
	selfhealFile(t, filepath.Join(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "inbox"), "a file")
	rc, e, _ := runProposeJSON(t, selfhealTestEnv(fixtureGH), root)
	if rc != 0 || len(e.Data.Proposals) != 0 {
		t.Fatalf("rc=%d %+v", rc, e)
	}
}

func TestRunAgentCommand_IsTheRegisteredAgentVerb(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runAgentCommand(nil, nil, &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), "evolve agent run <profile>") {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	if cmd := lookupCommand("agent"); cmd == nil {
		t.Fatal("evolve agent must stay registered")
	}
}
