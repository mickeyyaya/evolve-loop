package agentrequest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func inboxRequest(id string) string {
	return `{"schema":"evolve.request/1","kind":"inbox-add","what":{"verb":"inbox add","args":{"id":"` + id + `","title":"A finding","kind":"fix","fix":"Fix it.","priority_class":"P2","acceptance-1":"It is fixed."}},` +
		`"why":"The build found a defect outside its scope.","inputs":[],"expected_output":{"artifact":"an inbox item"}}`
}

func (h *harness) twoKinds(dispatch, inbox policy.RequestStage, inboxCap int) Channel {
	c := h.channel(dispatch)
	c.Executors = append(c.Executors, NewInboxAdd(policy.InboxRequestsPolicy{Stage: inbox, PerCycle: inboxCap}))
	return c
}

func TestHandoff_AnInboxAddRequestIsReadWhileDispatchIsOff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	if err := h.twoKinds(policy.RequestStageOff, policy.RequestStageEnforce, 8).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	r := h.record("inbox-add-1")
	if r.Kind != agentclass.KindInboxAdd || r.Decision != DecisionRefused || r.Precondition != PreIntake || r.Stage != policy.RequestStageEnforce || !strings.Contains(r.Reason, "intake") {
		t.Fatalf("want a decision that names the missing intake at the inbox stage, got %+v", r)
	}
	last := h.events[len(h.events)-1]
	if !slices.Equal(h.codes(), []signalcenter.Code{isolation.CodeRequestRefused}) || last.Severity != signalcenter.SeverityWarn || last.Reason == "" {
		t.Fatalf("the decision signal is the refusal WARN with a reason, got %+v", h.events)
	}
}

func TestHandoff_AnOffKindIsNotReadWhileAnotherKindIs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", validRequest)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	if err := h.twoKinds(policy.RequestStageOff, policy.RequestStageEnforce, 8).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if recordCount(h.cell, agentclass.KindDispatch) != 0 || recordCount(h.cell, agentclass.KindInboxAdd) != 1 || len(h.judged) != 0 {
		t.Fatalf("the off kind leaves no record and no judgement")
	}
}

func TestHandoff_EachKindOffReadsNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	h.request("code-refresh-1", validRequest)
	if err := h.twoKinds(policy.RequestStageOff, policy.RequestStageOff, 8).Handoff(context.Background(), h.cell); err != nil || len(h.events) != 0 {
		t.Fatalf("each kind off reads nothing, got %v %+v", err, h.events)
	}
	if _, err := os.Stat(inputindex.RequestsDir(h.cell.ProjectRoot, h.cell.Cycle)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("each kind off writes no record, even for a kind with no executor: %v", err)
	}
	if err := (Channel{}).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
}

func TestHandoff_TheInboxShadowStageApprovesAValidRequest(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("inbox-add-1", inboxRequest("a-finding"))
	if err := h.twoKinds(policy.RequestStageOff, policy.RequestStageShadow, 8).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("inbox-add-1"); r.Decision != DecisionApproved || r.Stage != policy.RequestStageShadow {
		t.Fatalf("shadow approves a request that passes the preconditions, got %+v", r)
	}
}

func TestHandoff_AnInboxAddRequestNamesTheArgsItLacks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	body := strings.Replace(strings.Replace(inboxRequest("a-finding"), `"fix":"Fix it.",`, "", 1), `"title":"A finding"`, `"title":" "`, 1)
	h.request("inbox-add-1", body)
	if err := h.twoKinds(policy.RequestStageEnforce, policy.RequestStageEnforce, 8).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	r := h.record("inbox-add-1")
	if r.Decision != DecisionRefused || r.Precondition != PreArgs || !strings.Contains(r.Reason, "fix") || !strings.Contains(r.Reason, "title") || strings.Contains(r.Reason, "priority_class") {
		t.Fatalf("want a refusal that names each missing arg, got %+v", r)
	}
}

func TestHandoff_TheCycleCapOfAKindCountsOnlyItsOwnRecords(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for i := 1; i <= 3; i++ {
		h.request(fmt.Sprintf("inbox-add-%d", i), inboxRequest("a-finding"))
	}
	h.request("dispatch-1", validRequest)
	c := h.twoKinds(policy.RequestStageEnforce, policy.RequestStageEnforce, 8)
	if err := c.Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("dispatch-1"); r.Decision != DecisionRun || len(h.runs) != 1 {
		t.Fatalf("three inbox records must not use the dispatch cap, got %+v", r)
	}
	capped := newHarness(t)
	for i := 1; i <= 3; i++ {
		capped.request(fmt.Sprintf("inbox-add-%d", i), inboxRequest("a-finding"))
	}
	capped.request("dispatch-1", validRequest)
	if err := capped.twoKinds(policy.RequestStageEnforce, policy.RequestStageEnforce, 2).Handoff(context.Background(), capped.cell); err != nil {
		t.Fatal(err)
	}
	if r := capped.record("inbox-add-3"); r.Precondition != PreCycleCap {
		t.Fatalf("the third inbox request is over the cap of 2, got %+v", r)
	}
	if r := capped.record("dispatch-1"); r.Decision != DecisionRun {
		t.Fatalf("the dispatch request is not counted against the inbox cap, got %+v", r)
	}
}

func TestRecordCount_CountsTheRecordsOfOneKind(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, name := range []string{"dispatch-1", "dispatch-2", "inbox-add-1", "dispatch-3.inbox-add-1", "dispatch-1~0123456789ab", "inbox-add-2~0123456789ab"} {
		if err := writeRecord(h.cell, Record{Request: name}); err != nil {
			t.Fatal(err)
		}
	}
	h.write(filepath.Join(h.cell.ProjectRoot, ".evolve", "runs", "cycle-7", "requests", "stray.json"), "{}")
	if d, i := recordCount(h.cell, agentclass.KindDispatch), recordCount(h.cell, agentclass.KindInboxAdd); d != 3 || i != 3 {
		t.Fatalf("dispatch %d, inbox-add %d; want 3 and 3 (the leaf of the name gives the kind)", d, i)
	}
}

func TestHandoff_AMissingRoleIsRefusedForTheArgsBeforeThePolicyOfTheRole(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.request("dispatch-1", strings.Replace(validRequest, `{"role":"fault-localization"}`, `{"role":" "}`, 1))
	if err := h.channel(policy.RequestStageEnforce).Handoff(context.Background(), h.cell); err != nil {
		t.Fatal(err)
	}
	if r := h.record("dispatch-1"); r.Precondition != PreArgs || !strings.Contains(r.Reason, "[role]") || !strings.Contains(r.Reason, "dispatch") {
		t.Fatalf("want the args refusal that names the role, got %+v", r)
	}
}

func TestKindTable_EveryKindWithRequiredArgsHasAnExecutorAndTheOtherWayRound(t *testing.T) {
	t.Parallel()
	var kinds []string
	for _, e := range Executors(policy.Policy{}, DispatchDeps{}) {
		kinds = append(kinds, e.Kind())
	}
	slices.Sort(kinds)
	if !slices.Equal(kinds, agentclass.ArgKinds()) {
		t.Fatalf("executors %v, kinds with required args %v; the two tables name the same kinds", kinds, agentclass.ArgKinds())
	}
	caps := Caps(policy.Policy{})
	d, i := caps[agentclass.KindDispatch].(dispatchCap), caps[agentclass.KindInboxAdd].(inboxAddCap)
	if len(caps) != len(kinds) || d.Stage != policy.RequestStageShadow || !slices.Equal(d.RequiredArgs, []string{"role"}) || i.Stage != policy.RequestStageEnforce || i.PerCycle != 8 {
		t.Fatalf("caps = %+v, want the policy view and the required args of each kind", caps)
	}
}

func TestRecordKind_ReadsTheKindOfTheLeafAndIgnoresTheReuseSuffix(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		"/x/dispatch-1.json":                 agentclass.KindDispatch,
		"/x/dispatch-3.inbox-add-1.json":     agentclass.KindInboxAdd,
		"/x/inbox-add-2~0123456789ab.json":   agentclass.KindInboxAdd,
		"/x/dispatch-1.dispatch-1~aaaa.json": agentclass.KindDispatch,
		"/x/stray.json":                      "",
	} {
		if got := recordKind(path); got != want {
			t.Errorf("recordKind(%q) = %q, want %q", path, got, want)
		}
	}
	if got := reusedName("dispatch-1", []byte("x")); originalName(got) != "dispatch-1" || got == "dispatch-1" || originalName("dispatch-1") != "dispatch-1" {
		t.Fatalf("reusedName = %q must add a suffix that originalName removes", got)
	}
}
