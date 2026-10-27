package agentrequest

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var t0 = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

const validRequest = `{"schema":"evolve.request/1","kind":"dispatch","what":{"verb":"agent run","args":{"role":"fault-localization"}},` +
	`"why":"The build fails in a parser that the scout report does not name.","inputs":["build-report.md"],` +
	`"expected_output":{"artifact":"the suspect files of the failure"}}`

type harness struct {
	t        *testing.T
	cell     Cell
	events   []signalcenter.Event
	judged   []Pending
	verdicts map[string]string
	runs     []Pending
	nested   map[string]string
	runErr   error
	maxDepth int
	judgeErr error
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, cell: Cell{ProjectRoot: root, Workspace: filepath.Join(root, ".evolve", "runs", "cycle-7"), Cycle: 7, Phase: "build"},
		verdicts: map[string]string{}, nested: map[string]string{}}
	h.write(filepath.Join(h.cell.Workspace, "build-report.md"), "# Build FAIL\n")
	return h
}

func (h *harness) write(path, body string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) request(name, body string) {
	h.write(filepath.Join(h.cell.Workspace, "requests", name+".json"), body)
}

func approve(name string) string {
	return `{"schema":"evolve.diagnosis/1","incident_id":"` + name + `","hypothesis":"the build needs the suspect files","evidence":["q1"],` +
		`"action":{"kind":"dispatch-phase-agent","params":{"request":"` + name + `"}},"expected_effect":"the build gets the suspect files","confidence":"high"}`
}

func refuse(name, reason string) string {
	return `{"schema":"evolve.diagnosis/1","incident_id":"` + name + `","hypothesis":"` + reason + `","evidence":["q1"],` +
		`"action":{"kind":"halt-and-escalate","params":{"reason":"` + reason + `"}},"expected_effect":"no agent runs","confidence":"high"}`
}

func (h *harness) channel(stage string) Channel {
	pol := policy.Policy{Isolation: &policy.IsolationPolicy{DispatchRequests: &policy.DispatchRequestsPolicy{Stage: stage, Roles: []string{"fault-localization", "premise-challenge"}, PerCycle: 3, MaxDepth: cmp.Or(h.maxDepth, 1)}}}
	return Channel{
		Policy: pol.IsolationDispatchRequests(),
		Judge: func(_ context.Context, p Pending) (string, error) {
			h.judged = append(h.judged, p)
			if h.judgeErr != nil {
				return "", h.judgeErr
			}
			if v, ok := h.verdicts[p.Name]; ok {
				return v, nil
			}
			return approve(p.Name), nil
		},
		Dispatch: func(_ context.Context, p Pending) (Result, error) {
			h.runs = append(h.runs, p)
			ws := filepath.Join(h.cell.ProjectRoot, ".evolve", "ephemeral", "agent-runs", "run-"+p.Name)
			h.write(filepath.Join(ws, "report.md"), "x")
			if h.runErr != nil {
				return Result{Workspace: ws}, h.runErr
			}
			if body, ok := h.nested[p.Name]; ok {
				h.write(filepath.Join(ws, "requests", "dispatch-1.json"), body)
			}
			return Result{Report: "# Suspects\nparser.go\n", RunLabel: "agent-" + p.Request.Role(), Workspace: ws}, nil
		},
		Emit: func(e signalcenter.Event) { h.events = append(h.events, e) },
		Now:  func() time.Time { return t0 },
	}
}

func (h *harness) record(name string) Record {
	h.t.Helper()
	raw, err := os.ReadFile(filepath.Join(inputindex.RequestsDir(h.cell.ProjectRoot, h.cell.Cycle), name+".json"))
	if err != nil {
		h.t.Fatalf("no record of %s: %v", name, err)
	}
	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		h.t.Fatal(err)
	}
	return r
}

func (h *harness) codes() []signalcenter.Code {
	var out []signalcenter.Code
	for _, e := range h.events {
		if e.Code != "" {
			out = append(out, e.Code)
		}
	}
	return out
}

func TestDispatchRequest_AValidRequestIsDispatchedByThePipeline(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)

	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	if len(h.runs) != 1 || h.runs[0].Request.Role() != "fault-localization" || !slices.Equal(h.runs[0].Inputs, []string{filepath.Join(h.cell.Workspace, "build-report.md")}) {
		t.Fatalf("the pipeline must dispatch the role of the request once with its inputs, got %+v", h.runs)
	}
	r := h.record("dispatch-1")
	if r.Decision != DecisionRun || r.Role != "fault-localization" || r.Phase != "build" || r.Depth != 1 || r.RunLabel != "agent-fault-localization" || !strings.Contains(r.Report, "parser.go") {
		t.Fatalf("the record must hold the run and its result, got %+v", r)
	}
	idx, err := inputindex.Build(inputindex.Request{ProjectRoot: h.cell.ProjectRoot, Workspace: h.cell.Workspace, Cycle: 7, Phase: "audit"})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(idx.Entries, func(e inputindex.Entry) bool { return e.Kind == inputindex.KindRequest })
	if i < 0 || !strings.Contains(idx.Entries[i].Summary, "dispatch-1 to fault-localization: run") {
		t.Fatalf("the result must reach the index of the requester, got %+v", idx.Entries)
	}
	evidence, err := inputindex.Read(h.runs[0].Evidence)
	if err != nil || h.runs[0].Evidence != EvidenceWorkspace(h.cell, "dispatch-1") || r.Schema != RecordSchema || !slices.Equal(evidence.Required, []string{"q1"}) ||
		len(evidence.Entries) != 2 || !evidence.Entries[0].Required || evidence.Entries[1].Required || !slices.Equal(h.judged[0].EntryIDs, []string{"q1", "i1"}) {
		t.Fatalf("the orchestrator gets an evidence index where only the request q1 is required, and the record has its schema: %v %+v %+v", err, evidence, r)
	}
	if len(h.events) != 2 || h.events[0].Kind != signalcenter.KindIsolationRequest || h.events[1].Kind != signalcenter.KindIsolationRequestDecision || h.events[1].Severity != signalcenter.SeverityInfo || h.events[1].Fields["decision"] != DecisionRun {
		t.Fatalf("one request signal and one INFO decision signal, got %+v", h.events)
	}
}

func TestHandoff_ADecidedRequestIsNotReadAgain(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	c := h.channel(policy.DispatchRequestStageEnforce)
	for range 2 {
		if err := c.Handoff(context.Background(), h.cell); err != nil {
			t.Fatal(err)
		}
	}
	if len(h.runs) != 1 || len(h.judged) != 1 {
		t.Fatalf("a request runs once, got %d runs and %d judgements", len(h.runs), len(h.judged))
	}
}

func TestHandoff_AnUnreasonableRequestIsRefusedWithARecordedReason(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	h.verdicts["dispatch-1"] = refuse("dispatch-1", "the build report already names the suspect file")

	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	r := h.record("dispatch-1")
	if len(h.runs) != 0 || r.Decision != DecisionRefused || r.Precondition != PreJudged || !strings.Contains(r.Reason, "already names the suspect file") {
		t.Fatalf("the orchestrator refused, so no agent runs and the reason is recorded, got %+v runs=%d", r, len(h.runs))
	}
	if !slices.Equal(h.codes(), []signalcenter.Code{isolation.CodeRequestRefused}) || h.events[1].Fields["reason"] != r.Reason || h.events[1].Severity != signalcenter.SeverityWarn {
		t.Fatalf("a refusal emits ISOLATION_REQUEST_REFUSED with the reason, got %+v", h.events)
	}
}

func TestHandoff_ARequestOfADispatchedAgentIsRefusedPastTheDepthCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	h.nested["dispatch-1"] = strings.Replace(validRequest, `"inputs":["build-report.md"]`, `"inputs":[]`, 1)

	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	child := h.record("dispatch-1.dispatch-1")
	if child.Decision != DecisionRefused || child.Precondition != PreDepthCap || child.Depth != 2 || len(h.runs) != 1 {
		t.Fatalf("the request of a dispatched agent has depth 2 and the cap is 1, got %+v runs=%d", child, len(h.runs))
	}
	if _, err := os.Stat(filepath.Join(h.cell.ProjectRoot, ".evolve", "ephemeral", "agent-runs", "run-dispatch-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the pipeline removes the kept workspace of the dispatched agent after it reads its requests: %v", err)
	}
}

func TestHandoff_ARequestPastTheCycleCapIsRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, name := range []string{"dispatch-1", "dispatch-2", "dispatch-3", "dispatch-10"} {
		h.request(name, validRequest)
	}

	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	if len(h.runs) != 3 {
		t.Fatalf("the cap of 3 for each cycle lets 3 requests run, got %d", len(h.runs))
	}
	if r := h.record("dispatch-10"); r.Decision != DecisionRefused || r.Precondition != PreCycleCap {
		t.Fatalf("the fourth request in number order is past the cycle cap, got %+v", r)
	}
}

func TestHandoff_TheValidatorRefusesBeforeTheOrchestratorJudges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body, want string
	}{
		{"not json", `{"schema":`, PreSchema},
		{"trailing data", validRequest + `{}`, PreSchema},
		{"an unknown key", strings.Replace(validRequest, `"why"`, `"tier":"T0","why"`, 1), PreSchema},
		{"another schema", strings.Replace(validRequest, "evolve.request/1", "evolve.request/2", 1), PreSchema},
		{"no reason", strings.Replace(validRequest, `"The build fails in a parser that the scout report does not name."`, `" "`, 1), PreSchema},
		{"no expected output", strings.Replace(validRequest, `"the suspect files of the failure"`, `""`, 1), PreSchema},
		{"a kind with no executor", strings.Replace(validRequest, `"kind":"dispatch"`, `"kind":"inbox-add"`, 1), PreKind},
		{"a role that the policy does not allow", strings.Replace(validRequest, "fault-localization", "builder", 1), PreRole},
		{"no role", strings.Replace(validRequest, `{"role":"fault-localization"}`, `{}`, 1), PreRole},
		{"an input outside the workspace", strings.Replace(validRequest, `"build-report.md"`, `"../cycle-6/audit-report.md"`, 1), PreInput},
		{"an input that does not exist", strings.Replace(validRequest, `"build-report.md"`, `"scout-report.md"`, 1), PreInput},
		{"an input path that is not clean", strings.Replace(validRequest, `"build-report.md"`, `"./build-report.md"`, 1), PreInput},
		{"an input that is a directory", strings.Replace(validRequest, `"build-report.md"`, `"requests"`, 1), PreInput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.write(filepath.Join(h.cell.ProjectRoot, ".evolve", "runs", "cycle-6", "audit-report.md"), "# Audit\n")
			h.request("dispatch-1", c.body)
			if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
				t.Fatal(err)
			}
			r := h.record("dispatch-1")
			if r.Decision != DecisionRefused || r.Precondition != c.want || r.Reason == "" || len(h.judged) != 0 || len(h.runs) != 0 {
				t.Fatalf("want a refusal of %s before a judgement, got %+v judged=%d", c.want, r, len(h.judged))
			}
			last := h.events[len(h.events)-1]
			if !slices.Equal(h.codes(), []signalcenter.Code{isolation.CodeRequestRefused}) || last.Severity != signalcenter.SeverityWarn {
				t.Fatalf("each refusal emits ISOLATION_REQUEST_REFUSED as a WARN, got %+v", h.events)
			}
		})
	}
}

func TestHandoff_TheCatalogDisposesOfTheJudgement(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, verdict, want, reason string
		judgeErr                    error
	}{
		{"a judgement that fails the schema", `{"schema":"evolve.diagnosis/1"}`, PreJudge, "diagnosis.json", nil},
		{"the judge seat fails", "", PreJudge, "exit 1", errors.New("evolve agent run selfheal-orchestrator: exit 1")},
		{"an approval of another request", strings.Replace(approve("dispatch-1"), `"request":"dispatch-1"`, `"request":"dispatch-9"`, 1), PreCatalog, "request-validated: ", nil},
		{"an action that is not a dispatch", strings.Replace(refuse("dispatch-1", "x"), `"kind":"halt-and-escalate","params":{"reason":"x"}`, `"kind":"quarantine-item","params":{"item":"a"}`, 1), PreCatalog, "item-known: ", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.request("dispatch-1", validRequest)
			h.verdicts["dispatch-1"], h.judgeErr = c.verdict, c.judgeErr
			if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
				t.Fatal(err)
			}
			if r := h.record("dispatch-1"); r.Decision != DecisionRefused || r.Precondition != c.want || !strings.Contains(r.Reason, c.reason) || len(h.runs) != 0 {
				t.Fatalf("want a refusal of %s that names %q, got %+v", c.want, c.reason, r)
			}
		})
	}
}

func TestHandoff_ShadowJudgesAndRecordsButDispatchesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	if err := h.channel(policy.DispatchRequestStageShadow).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("dispatch-1"); r.Decision != DecisionApproved || r.Stage != policy.DispatchRequestStageShadow || len(h.runs) != 0 || len(h.judged) != 1 {
		t.Fatalf("shadow approves and records, and runs no agent, got %+v runs=%d", r, len(h.runs))
	}
}

func TestHandoff_AFailedDispatchIsRecordedAndSignalled(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	h.runErr = errors.New("evolve agent run fault-localization: exit 81")
	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("dispatch-1"); r.Decision != DecisionFailed || !strings.Contains(r.Reason, "exit 81") {
		t.Fatalf("a failed dispatch is recorded, got %+v", r)
	}
	if _, err := os.Stat(filepath.Join(h.cell.ProjectRoot, ".evolve", "ephemeral", "agent-runs", "run-dispatch-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the channel removes the kept workspace of a failed run too: %v", err)
	}
	if !slices.Equal(h.codes(), []signalcenter.Code{isolation.CodeRequestFailed}) || h.events[1].Severity != signalcenter.SeverityWarn {
		t.Fatalf("a failed dispatch emits ISOLATION_REQUEST_FAILED, got %+v", h.events)
	}
}

func TestHandoff_OffReadsNothingAndAWorkspaceWithNoRequestsIsQuiet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	h.write(filepath.Join(h.cell.Workspace, "requests", "notes.txt"), "not a request")
	if err := h.channel(policy.DispatchRequestStageOff).Handoff(context.Background(), h.cell); err != nil || len(h.events) != 0 {
		t.Fatalf("off reads nothing, got %v %+v", err, h.events)
	}
	empty := newHarness(t)
	if err := empty.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), empty.cell); err != nil || len(empty.events) != 0 {
		t.Fatalf("a cell with no requests directory is quiet, got %v %+v", err, empty.events)
	}
	h.write(filepath.Join(h.cell.Workspace, "requests", "inbox-add-1.json"), validRequest)
	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(inputindex.RequestsDir(h.cell.ProjectRoot, 7), "notes.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a file that is not a request file is not read")
	}
	if r := h.record("inbox-add-1"); r.Decision != DecisionRefused || r.Precondition != PreKind {
		t.Fatalf("the kind of the file name must match the kind of the request, got %+v", r)
	}
}

func TestHandoff_AnUnwritableRecordStoreFailsLoudly(t *testing.T) {
	t.Parallel()
	unreadable := newHarness(t)
	unreadable.request("dispatch-1", validRequest)
	unreadable.write(filepath.Join(unreadable.cell.ProjectRoot, ".evolve", "lineage", "requests"), "a file where the store must be")
	if err := unreadable.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), unreadable.cell); err == nil || !strings.Contains(err.Error(), "dispatch-1") {
		t.Fatalf("a store that cannot be read is an error that names the request, got %v", err)
	}
	readOnly := newHarness(t)
	readOnly.request("dispatch-1", validRequest)
	dir := inputindex.RequestsDir(readOnly.cell.ProjectRoot, 7)
	if err := errors.Join(os.MkdirAll(dir, 0o755), os.Chmod(dir, 0o500)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	err := readOnly.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), readOnly.cell)
	if err == nil || !strings.Contains(err.Error(), "request dispatch-1") || len(readOnly.events) != 1 {
		t.Fatalf("a record that cannot be written is an error that names the request, and no decision signal claims it, got %v %+v", err, readOnly.events)
	}
	if _, err := os.Stat(filepath.Join(readOnly.cell.ProjectRoot, ".evolve", "ephemeral", "agent-runs", "run-dispatch-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the kept workspace is removed when the record fails: %v", err)
	}
}

func TestHandoff_AnInputThatIsNotAPlainFileIsRefusedBeforeTheJudge(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	report := filepath.Join(h.cell.Workspace, "build-report.md")
	if err := os.Link(report, filepath.Join(h.cell.Workspace, "linked.md")); err != nil {
		t.Fatal(err)
	}
	h.request("dispatch-1", strings.Replace(validRequest, `"build-report.md"`, `"linked.md"`, 1))
	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("dispatch-1"); r.Decision != DecisionRefused || r.Precondition != PreInput || !strings.Contains(r.Reason, "not a plain file") || len(h.judged) != 0 {
		t.Fatalf("an input with a second hard link is no plain file, so the orchestrator is not asked, got %+v", r)
	}
}

func TestParse_DecodesTheRequestShapeOfTheCLIContract(t *testing.T) {
	t.Parallel()
	got, err := parse([]byte(validRequest))
	want := Request{Schema: "evolve.request/1", Kind: agentclass.KindDispatch, What: What{Verb: "agent run", Args: map[string]string{"role": "fault-localization"}},
		Why: "The build fails in a parser that the scout report does not name.", Inputs: []string{"build-report.md"},
		ExpectedOutput: ExpectedOutput{Artifact: "the suspect files of the failure"}}
	if err != nil || !reflect.DeepEqual(got, want) || got.Role() != "fault-localization" {
		t.Fatalf("parse = %+v %v, want %+v", got, err, want)
	}
}

func TestHandoff_ACancelledCycleJudgesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(ctx, h.cell); !errors.Is(err, context.Canceled) || len(h.judged) != 0 {
		t.Fatalf("a cancelled cycle reads no request, got %v and %d judgements", err, len(h.judged))
	}
}

func TestHandoff_ANewRequestUnderADecidedNameIsRefusedOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	c := h.channel(policy.DispatchRequestStageEnforce)
	if err := c.Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	other := strings.Replace(validRequest, "fault-localization", "premise-challenge", 1)
	h.request("dispatch-1", other)
	h.events = nil
	for range 2 {
		if err := c.Handoff(context.Background(), h.cell); err != nil {
			t.Fatal(err)
		}
	}
	r := h.record("dispatch-1~" + digest([]byte(other))[:12])
	if r.Decision != DecisionRefused || r.Precondition != PreName || len(h.runs) != 1 || !slices.Equal(h.codes(), []signalcenter.Code{isolation.CodeRequestRefused}) {
		t.Fatalf("a request that reuses a decided name is refused loudly once, got %+v runs=%d events=%+v", r, len(h.runs), h.events)
	}
}

func TestHandoff_ANestedRequestUnderTheDepthCapIsJudgedAndRun(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.maxDepth = 2
	h.request("dispatch-1", validRequest)
	h.nested["dispatch-1"] = strings.Replace(validRequest, `"inputs":["build-report.md"]`, `"inputs":[]`, 1)
	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("dispatch-1.dispatch-1"); r.Decision != DecisionRun || r.Depth != 2 || len(h.runs) != 2 || h.judged[1].Name != "dispatch-1.dispatch-1" {
		t.Fatalf("the child name passes the grammar of the judgement and runs, got %+v runs=%d", r, len(h.runs))
	}
}

func TestHandoff_AnUnreadableRequestStoreOrRecordFailsLoudly(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.write(filepath.Join(h.cell.Workspace, "requests"), "a file where the requests directory must be")
	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err == nil || !strings.Contains(err.Error(), "requests of") {
		t.Fatalf("an unreadable requests directory is an error, got %v", err)
	}
	corrupt := newHarness(t)
	corrupt.request("dispatch-1", validRequest)
	corrupt.write(filepath.Join(inputindex.RequestsDir(corrupt.cell.ProjectRoot, 7), "dispatch-1.json"), "{")
	if err := corrupt.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), corrupt.cell); err == nil || !strings.Contains(err.Error(), "request dispatch-1") || len(corrupt.judged) != 0 {
		t.Fatalf("a corrupt record is an error and decides nothing, got %v", err)
	}
}

func TestHandoff_ARequestWhoseEvidenceIsNoPlainFileIsRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	runs := filepath.Join(h.cell.ProjectRoot, ".evolve", "runs")
	moved := filepath.Join(h.cell.ProjectRoot, "elsewhere")
	if err := errors.Join(os.Rename(runs, moved), os.Symlink(moved, runs)); err != nil {
		t.Fatal(err)
	}
	h.request("dispatch-1", strings.Replace(validRequest, `"inputs":["build-report.md"]`, `"inputs":[]`, 1))
	if err := h.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("dispatch-1"); r.Decision != DecisionRefused || r.Precondition != PreEvidence || !strings.Contains(r.Reason, "not a plain file") || len(h.judged) != 0 {
		t.Fatalf("evidence under a symlinked runs directory is refused before the judge, got %+v", r)
	}
}

func TestHandoff_TheRequestsOfAFailedRunAreNotRead(t *testing.T) {
	t.Parallel()
	nested := strings.Replace(validRequest, `"inputs":["build-report.md"]`, `"inputs":[]`, 1)
	failed := newHarness(t)
	failed.maxDepth = 2
	failed.request("dispatch-1", validRequest)
	failed.runErr = errors.New("evolve agent run fault-localization: exit 81")
	failed.nested["dispatch-1"] = nested
	failed.write(filepath.Join(failed.cell.ProjectRoot, ".evolve", "ephemeral", "agent-runs", "run-dispatch-1", "requests", "dispatch-1.json"), nested)
	if err := failed.channel(policy.DispatchRequestStageEnforce).Handoff(context.Background(), failed.cell); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(inputindex.RequestsDir(failed.cell.ProjectRoot, 7), "dispatch-1.dispatch-1.json")); !errors.Is(err, os.ErrNotExist) || len(failed.judged) != 1 {
		t.Fatalf("the requests of a failed run are not read: %v, %d judgements", err, len(failed.judged))
	}
}
