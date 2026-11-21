package agentrequest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const testWeight = 0.3

type fileCall struct {
	root   string
	raw    []byte
	source string
}

type fakeFiler struct {
	root   string
	calls  []fileCall
	err    error
	bySrc  map[string]Filed
	idempo bool
}

func newFakeFiler(root string) *fakeFiler { return &fakeFiler{root: root, bySrc: map[string]Filed{}} }

func (f *fakeFiler) port() FileFunc { return f.file }

func (f *fakeFiler) file(_ context.Context, root string, raw []byte, source string) (Filed, error) {
	f.calls = append(f.calls, fileCall{root, raw, source})
	if f.err != nil {
		return Filed{}, f.err
	}
	if prior, ok := f.bySrc[source]; ok && f.idempo {
		return prior, nil
	}
	var item struct{ ID string }
	if err := json.Unmarshal(raw, &item); err != nil {
		return Filed{}, err
	}
	filed := Filed{ID: item.ID, Path: filepath.Join(root, ".evolve", "inbox", "2026-11-21T00-00-00Z-"+item.ID+".json")}
	f.bySrc[source] = filed
	return filed, nil
}

func (h *harness) handoff(inbox policy.RequestStage, cap int) {
	h.t.Helper()
	if err := h.twoKinds(policy.RequestStageOff, inbox, cap).Handoff(context.Background(), h.cell); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) lastDecision() signalcenter.Event {
	h.t.Helper()
	for i := len(h.events) - 1; i >= 0; i-- {
		if h.events[i].Kind == signalcenter.KindIsolationRequestDecision {
			return h.events[i]
		}
	}
	h.t.Fatalf("no decision signal in %+v", h.events)
	return signalcenter.Event{}
}

func TestInboxAdd_AValidRequestFilesTheItemWithItsSource(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))

	h.handoff(policy.RequestStageEnforce, 8)

	if len(h.filer.calls) != 1 {
		t.Fatalf("the filer ran %d times, want 1", len(h.filer.calls))
	}
	call := h.filer.calls[0]
	var item map[string]any
	if err := json.Unmarshal(call.raw, &item); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"id": "a-finding", "title": "A finding", "kind": "fix", "fix": "Fix it.", "priority_class": "correctness",
		"summary": "The build found a defect outside its scope.", "weight": testWeight,
		"acceptance": []any{"It is fixed."}, "files": []any{"go/internal/x/x.go"}}
	for key, w := range want {
		if fmt.Sprint(item[key]) != fmt.Sprint(w) {
			t.Errorf("item[%s] = %v, want %v", key, item[key], w)
		}
	}
	if _, has := item["origin"]; has || call.source != "request:7/inbox-add-1" || call.root != h.cell.ProjectRoot {
		t.Errorf("source %q root %q origin %v: the filer sets the origin, the source names cycle and request", call.source, call.root, item["origin"])
	}
}

func TestInboxAdd_AnAgentWeightArgIsRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", strings.Replace(inboxRequest("a-finding"), `"id":`, `"weight":"0.99","id":`, 1))

	h.handoff(policy.RequestStageEnforce, 8)

	r := h.record("inbox-add-1")
	if r.Decision != DecisionRefused || r.Precondition != PreArgs || !strings.Contains(r.Reason, "weight") || len(h.filer.calls) != 0 {
		t.Fatalf("an agent cannot steer the rank of its item: %+v, filer calls %d", r, len(h.filer.calls))
	}
}

func TestInboxAdd_NumberedArgsKeepTheirOrder(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	body := strings.Replace(inboxRequest("a-finding"), `"acceptance-1":"It is fixed."`, `"acceptance-10":"tenth","acceptance-2":"second","acceptance-1":"first"`, 1)
	h.request("inbox-add-1", body)

	h.handoff(policy.RequestStageEnforce, 8)

	var item struct{ Acceptance []string }
	if err := json.Unmarshal(h.filer.calls[0].raw, &item); err != nil || !slices.Equal(item.Acceptance, []string{"first", "second", "tenth"}) {
		t.Fatalf("acceptance = %v (%v), want the numeric order", item.Acceptance, err)
	}
}

func TestInboxAdd_SixRequestsInOneCycleFileSixItems(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for i := 1; i <= 6; i++ {
		h.request(fmt.Sprintf("inbox-add-%d", i), inboxRequest(fmt.Sprintf("move-item-%d", i)))
	}

	h.handoff(policy.RequestStageEnforce, 8)

	for i := 1; i <= 6; i++ {
		if r := h.record(fmt.Sprintf("inbox-add-%d", i)); r.Decision != DecisionFiled || r.ItemID != fmt.Sprintf("move-item-%d", i) {
			t.Errorf("request %d: %+v, want a filed item", i, r)
		}
	}
	if len(h.filer.calls) != 6 {
		t.Fatalf("the filer ran %d times, want 6 (cycle 2460 lost its 6 move items)", len(h.filer.calls))
	}
}

func TestInboxAdd_TheRecordNamesTheFiledItem(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))

	h.handoff(policy.RequestStageEnforce, 8)

	wantPath := filepath.Join(h.cell.ProjectRoot, ".evolve", "inbox", "2026-11-21T00-00-00Z-a-finding.json")
	r := h.record("inbox-add-1")
	if r.Decision != DecisionFiled || r.ItemID != "a-finding" || r.ItemPath != wantPath || !strings.Contains(r.Summary, "a-finding") || r.Report != "" {
		t.Fatalf("record = %+v, want the item id and path in their own fields and the id in the summary", r)
	}
	e := h.lastDecision()
	if e.Severity != signalcenter.SeverityInfo || e.Code != "" || e.Fields["item_id"] != "a-finding" || e.Fields["item_path"] != wantPath {
		t.Fatalf("a filed request is an INFO signal that names the item, got %+v", e)
	}
}

func TestInboxAdd_AMissingArgIsRefusedWithAnIncidentSignalNamingTheRequestPath(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", strings.Replace(inboxRequest("a-finding"), `"fix":"Fix it.",`, "", 1))

	h.handoff(policy.RequestStageEnforce, 8)

	e := h.lastDecision()
	wantPath := filepath.Join(h.cell.Workspace, "requests", "inbox-add-1.json")
	if e.Severity != signalcenter.SeverityIncident || e.Code != isolation.CodeRequestRefused || e.Fields["request_path"] != wantPath || e.Fields["precondition"] != PreArgs {
		t.Fatalf("an invalid inbox-add is a loud refusal with the request path, got %+v", e)
	}
}

func TestInboxAdd_AMalformedRequestFileIsRefusedLoudlyWithItsPath(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", `{"schema":`)

	h.handoff(policy.RequestStageEnforce, 8)

	e := h.lastDecision()
	if r := h.record("inbox-add-1"); r.Precondition != PreSchema {
		t.Fatalf("record = %+v, want a schema refusal", r)
	}
	if e.Severity != signalcenter.SeverityIncident || e.Fields["request_path"] != filepath.Join(h.cell.Workspace, "requests", "inbox-add-1.json") {
		t.Fatalf("a malformed inbox-add file is as loud as a missing arg, got %+v", e)
	}
}

func TestInboxAdd_AFilerRefusalIsRefusedWithTheCause(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	h.channelWithFiler(t, fmt.Errorf("%w: the item overlaps open item rolling-pool", ErrItemRefused))

	e := h.lastDecision()
	r := h.record("inbox-add-1")
	if r.Decision != DecisionRefused || r.Precondition != PreIntake || !strings.Contains(r.Reason, "rolling-pool") || r.ItemID != "" {
		t.Fatalf("record = %+v, want a refusal at the intake with the cause", r)
	}
	if e.Severity != signalcenter.SeverityIncident || e.Code != isolation.CodeRequestRefused {
		t.Fatalf("signal = %+v, want the loud refusal", e)
	}
}

func TestInboxAdd_AFilerFaultIsAFailureOfThePipelineNotARefusal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	h.channelWithFiler(t, errors.New("lock the inbox: timed out"))

	e := h.lastDecision()
	r := h.record("inbox-add-1")
	if r.Decision != DecisionFailed || r.Precondition != "" || !strings.Contains(r.Reason, "timed out") {
		t.Fatalf("record = %+v, want a failure with the cause", r)
	}
	if e.Severity != signalcenter.SeverityIncident || e.Code != isolation.CodeRequestFailed {
		t.Fatalf("signal = %+v, want the loud failure", e)
	}
}

func (h *harness) channelWithFiler(t *testing.T, err error) {
	t.Helper()
	c := h.twoKinds(policy.RequestStageOff, policy.RequestStageEnforce, 8)
	h.filer.err = err
	if herr := c.Handoff(context.Background(), h.cell); herr != nil {
		t.Fatal(herr)
	}
}

func TestInboxAdd_ShadowFilesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))

	h.handoff(policy.RequestStageShadow, 8)

	if r := h.record("inbox-add-1"); r.Decision != DecisionApproved || r.ItemID != "" || len(h.filer.calls) != 0 {
		t.Fatalf("shadow records that the item would be filed and files nothing: %+v, calls %d", r, len(h.filer.calls))
	}
}

func TestExecutors_InboxAddAtEnforceNeedsAFiler(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	c := Channel{Executors: Executors(policy.Policy{}, DispatchDeps{}), Emit: func(e signalcenter.Event) { h.events = append(h.events, e) }, Now: h.channel(policy.RequestStageOff).Now}

	if err := c.Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	e := h.lastDecision()
	if r := h.record("inbox-add-1"); r.Decision != DecisionRefused || r.Precondition != PreIntake || !strings.Contains(r.Reason, "no filer") || e.Severity != signalcenter.SeverityIncident {
		t.Fatalf("an enforce stage with no filer must fail loudly, got %+v %+v", r, e)
	}
}

func TestInboxAdd_AReplayAfterACrashIsRecordedAsTheFiledItem(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	c := h.twoKinds(policy.RequestStageOff, policy.RequestStageEnforce, 8)
	h.filer.idempo = true
	if err := c.Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	first := h.record("inbox-add-1")
	if err := os.Remove(recordPath(h.cell, "inbox-add-1")); err != nil {
		t.Fatal(err)
	}

	if err := c.Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	again := h.record("inbox-add-1")
	if again.ItemID != first.ItemID || again.ItemPath != first.ItemPath || again.Decision != DecisionFiled || len(h.filer.bySrc) != 1 {
		t.Fatalf("replay = %+v, first = %+v; want the same item and one source", again, first)
	}
}

func TestInboxAdd_AFiledRequestReadsNoChildRequestFromTheProcessDirectory(t *testing.T) {
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	dir := t.TempDir()
	h.write(filepath.Join(dir, "requests", "dispatch-1.json"), validRequest)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	h.handoff(policy.RequestStageEnforce, 8)

	if recordCount(h.cell, agentclass.KindDispatch) != 0 || len(h.judged) != 0 {
		t.Fatalf("a filed item has no workspace, so no request of the process directory is read")
	}
}

func TestDispatchRefusalStaysAWarning(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", strings.Replace(validRequest, `"role":"fault-localization"`, `"role":"nobody"`, 1))

	if err := h.channel(policy.RequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	if e := h.lastDecision(); e.Severity != signalcenter.SeverityWarn || e.Code != isolation.CodeRequestRefused || e.Fields["request_path"] == "" {
		t.Fatalf("a refused dispatch request stays a WARN and now names its path: %+v", e)
	}
}

func TestInboxAdd_AWeightThatCannotBeEncodedIsRefusedBeforeTheFiler(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	filer := newFakeFiler(h.cell.ProjectRoot)
	c := h.channel(policy.RequestStageOff)
	c.Executors = append(c.Executors, NewInboxAdd(policy.InboxRequestsPolicy{Stage: policy.RequestStageEnforce, PerCycle: 8, DefaultWeight: math.NaN()}, filer.file))

	if err := c.Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}

	if r := h.record("inbox-add-1"); r.Decision != DecisionRefused || r.Precondition != PreIntake || len(filer.calls) != 0 {
		t.Fatalf("record = %+v, want a refusal at the intake and no filing", r)
	}
}
