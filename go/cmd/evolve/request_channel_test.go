package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentrequest"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type agentCall struct {
	args  []string
	input string
}

type fakeAgentSeats struct {
	calls []agentCall
	reply func(args []string) (int, agentcmd.Envelope)
}

func (f *fakeAgentSeats) run(args []string, stdin io.Reader, stdout, _ io.Writer) int {
	raw, _ := io.ReadAll(stdin)
	f.calls = append(f.calls, agentCall{args: args, input: string(raw)})
	rc, env := f.reply(args)
	if rc < 0 {
		_, _ = io.WriteString(stdout, "not json")
		return 1
	}
	out, _ := json.Marshal(env)
	_, _ = stdout.Write(out)
	return rc
}

const channelRequest = `{"schema":"evolve.request/1","kind":"dispatch","what":{"verb":"agent run","args":{"role":"fault-localization"}},` +
	`"why":"The build fails in a parser.","inputs":["build-report.md"],"expected_output":{"artifact":"the suspect files"}}`

func requestCell(t *testing.T) core.RequestCell {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	for path, body := range map[string]string{"build-report.md": "# Build FAIL\n", "requests/dispatch-1.json": channelRequest} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(ws, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return core.RequestCell{ProjectRoot: root, Workspace: ws, Cycle: 7, Phase: "build"}
}

func enforcePolicy() policy.Policy {
	return policy.Policy{Isolation: &policy.IsolationPolicy{DispatchRequests: &policy.DispatchRequestsPolicy{Stage: policy.DispatchRequestStageEnforce}}}
}

func TestAgentRequest_AnApprovedRequestIsRunByThePipeline(t *testing.T) {
	t.Parallel()
	cell := requestCell(t)
	kept := filepath.Join(cell.ProjectRoot, ".evolve", "ephemeral", "agent-runs", "agent-fault-localization-1")
	if err := os.MkdirAll(kept, 0o755); err != nil {
		t.Fatal(err)
	}
	seats := &fakeAgentSeats{reply: func(args []string) (int, agentcmd.Envelope) {
		if args[1] == orchestratorProfile {
			return 0, agentcmd.Envelope{Report: `{"schema":"evolve.diagnosis/1","incident_id":"dispatch-1","hypothesis":"the build needs it","evidence":["q1","i1"],` +
				`"action":{"kind":"dispatch-phase-agent","params":{"request":"dispatch-1"}},"expected_effect":"suspects","confidence":"high"}`}
		}
		return 0, agentcmd.Envelope{Report: "# Suspects\nparser.go\n", RunLabel: "agent-fault-localization-1", Workspace: kept}
	}}
	var events []signalcenter.Event

	handoff := requestHandoff(enforcePolicy(), seats.run, func(e signalcenter.Event) { events = append(events, e) }, io.Discard)
	if err := handoff(context.Background(), cell); err != nil {
		t.Fatal(err)
	}

	if len(seats.calls) != 2 {
		t.Fatalf("the pipeline runs the orchestrator, then the role, got %+v", seats.calls)
	}
	judge, run := seats.calls[0], seats.calls[1]
	evidence := agentrequest.EvidenceWorkspace(agentrequest.Cell(cell), "dispatch-1")
	if !slices.Equal(judge.args, []string{"run", orchestratorProfile, "--json", "--project-root", cell.ProjectRoot}) || !strings.Contains(judge.input, "dispatch-1") || !strings.Contains(judge.input, evidence) {
		t.Fatalf("the orchestrator must judge the request through its evidence workspace, got %+v", judge)
	}
	wantRun := []string{"run", "fault-localization", "--json", "--keep", "--project-root", cell.ProjectRoot, "--file", filepath.Join(cell.Workspace, "build-report.md")}
	if !slices.Equal(run.args, wantRun) || !strings.Contains(run.input, "The build fails in a parser.") || !strings.Contains(run.input, "the suspect files") {
		t.Fatalf("the pipeline must run the role with the inputs of the request, got %+v", run)
	}
	raw, err := os.ReadFile(filepath.Join(inputindex.RequestsDir(cell.ProjectRoot, 7), "dispatch-1.json"))
	var rec agentrequest.Record
	if err == nil {
		err = json.Unmarshal(raw, &rec)
	}
	if err != nil || rec.Decision != agentrequest.DecisionRun || !strings.Contains(rec.Report, "parser.go") {
		t.Fatalf("the result goes into the record that the index lists: %s %v", raw, err)
	}
	if _, err := os.Stat(kept); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the kept workspace of the run is removed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("one request signal and one decision signal, got %+v", events)
	}
}

func TestAgentRequest_AFailedSeatIsAnError(t *testing.T) {
	t.Parallel()
	kept := t.TempDir()
	cases := []struct {
		name  string
		reply func(args []string) (int, agentcmd.Envelope)
		want  string
	}{
		{"the judge prints no envelope", func([]string) (int, agentcmd.Envelope) { return -1, agentcmd.Envelope{} }, "no JSON envelope"},
		{"the judge exits non-zero", func([]string) (int, agentcmd.Envelope) { return 2, agentcmd.Envelope{Error: "refused"} }, "exit 2: refused"},
		{"the role run fails", func(args []string) (int, agentcmd.Envelope) {
			if args[1] == orchestratorProfile {
				return 0, agentcmd.Envelope{Report: `{"schema":"evolve.diagnosis/1","incident_id":"dispatch-1","hypothesis":"h","evidence":["q1"],` +
					`"action":{"kind":"dispatch-phase-agent","params":{"request":"dispatch-1"}},"expected_effect":"e","confidence":"high"}`}
			}
			return 81, agentcmd.Envelope{Error: "artifact timeout", Workspace: kept}
		}, "exit 81: artifact timeout"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cell := requestCell(t)
			seats := &fakeAgentSeats{reply: c.reply}
			if err := requestHandoff(enforcePolicy(), seats.run, func(signalcenter.Event) {}, io.Discard)(context.Background(), cell); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(inputindex.RequestsDir(cell.ProjectRoot, 7), "dispatch-1.json"))
			if err != nil || !strings.Contains(string(raw), c.want) {
				t.Fatalf("the record must carry %q: %s %v", c.want, raw, err)
			}
		})
	}
	if _, err := os.Stat(kept); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the kept workspace of a failed run is removed too: %v", err)
	}
}

func TestAgentRequest_ADirectAgentRunInACellIsRefusedAndNamesTheFileThatThePipelineReads(t *testing.T) {
	t.Parallel()
	cell := requestCell(t)
	var stderr bytes.Buffer
	rc := dispatchWith(dispatchGate{open: unheldStore(t, "enforce")}, []string{"agent", "run", "fault-localization"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	named := agentclass.RequestPath(cell.Workspace, agentclass.KindDispatch)
	if rc != exitDispatchRefused || !strings.Contains(stderr.String(), agentclass.RequestPath("", agentclass.KindDispatch)) {
		t.Fatalf("a direct agent run in a cell = exit %d %q, want exit %d naming the dispatch request path", rc, stderr.String(), exitDispatchRefused)
	}
	if err := errors.Join(os.Remove(filepath.Join(cell.Workspace, "requests", "dispatch-1.json")), os.WriteFile(strings.Replace(named, "<n>", "7", 1), []byte(channelRequest), 0o644)); err != nil {
		t.Fatal(err)
	}
	seats := &fakeAgentSeats{reply: func([]string) (int, agentcmd.Envelope) { return 2, agentcmd.Envelope{Error: "stop"} }}
	if err := requestHandoff(enforcePolicy(), seats.run, func(signalcenter.Event) {}, io.Discard)(context.Background(), cell); err != nil || len(seats.calls) != 1 {
		t.Fatalf("the pipeline must read the request at the path that the refusal names: %v, %d seat calls", err, len(seats.calls))
	}
	if _, err := os.Stat(filepath.Join(inputindex.RequestsDir(cell.ProjectRoot, 7), "dispatch-7.json")); err != nil {
		t.Fatalf("the request at the named path must get its record: %v", err)
	}
}

func TestWireOrchestratorDeps_RequestChannelWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	if d.CloseSignals != nil {
		t.Cleanup(d.CloseSignals)
	}
	if d.Orchestrator == nil || !d.Orchestrator.RequestHandoffWired() {
		t.Fatal("the production composition root must give the orchestrator the request channel (llm-isolation.md §7.2.1)")
	}
}
