package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

const (
	incKernelRepair = "inc-2026-10-27-99f19b25"
	openRepairOf    = `{"kind":"open-repair-item","params":{"files":["go/internal/gc/sweep.go"],"repro":"the boundary hangs"}}`
)

func repairProject(t *testing.T, healing string) string {
	t.Helper()
	root := validatedProject(t, healing)
	dir := selfheal.Dir(paths.EvolveDirOf(root))
	incidents, err := selfheal.LoadIncidents(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, inc := range incidents {
		if inc.ID == incKernelRepair {
			inc.Tier = selfheal.TierKernel
			if err := selfheal.WriteIncident(dir, inc); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func repairEnv(t *testing.T, refuse bool) selfhealEnv {
	t.Helper()
	env := selfhealTestEnv(fixtureGH)
	env.agent = fakeBridgeAgent(t, &diagnosisBridge{actions: map[string]string{incCIHang: openRepairOf, incChurn: openRepairOf, incKernelRepair: openRepairOf}})
	env.auditRefused = func(string) (bool, error) { return refuse, nil }
	return env
}

func repairItems(t *testing.T, root string) map[string]inboxbatch.Item {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(paths.EvolveDirOf(root), "inbox", "*-repair-inc-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]inboxbatch.Item{}
	for _, f := range files {
		it, _, err := inboxbatch.LoadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimPrefix(it.ID, "repair-")] = it
	}
	return out
}

func TestSelfhealPropose_AnAcceptedOpenRepairItemIsFiledConsoleRoutedWithItsTierAndRecordedInShadow(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8}}`)

	rc, e, stderr := runProposeJSON(t, repairEnv(t, true), root)

	if rc != 0 || len(e.Data.Proposals) != 8 {
		t.Fatalf("rc=%d %+v\n%s", rc, e, stderr)
	}
	items := repairItems(t, root)
	if len(items) != 3 {
		t.Fatalf("one item for each accepted open-repair-item, got %d: %v", len(items), items)
	}
	want := map[string]struct {
		tier      gateclass.Tier
		stamped   string
		wantRoute string
	}{
		incCIHang:       {gateclass.T1, "T1", "lane"},
		incChurn:        {gateclass.T2, "T2", "console"},
		incKernelRepair: {gateclass.T0, "", "console"},
	}
	for id, w := range want {
		it, ok := items[id]
		if !ok {
			t.Fatalf("no item for %s", id)
		}
		if it.RepairTier() != w.tier || stampOf(it) != w.stamped || it.Route != "console" || it.Kind != inboxbatch.KindPipelineRepair {
			t.Errorf("%s: tier %q stamp %q route %q kind %q", id, it.RepairTier(), it.Tier, it.Route, it.Kind)
		}
		if routed, _ := inboxbatch.ConsoleRouted(it, nil); !routed {
			t.Errorf("%s: the filed item must be console-routed even with no protected predicate", id)
		}
	}
	if len(items[incKernelRepair].ProtectedFiles) == 0 || len(items[incCIHang].ProtectedFiles) != 0 {
		t.Errorf("only the T0 item carries protected_files: %v %v", items[incKernelRepair].ProtectedFiles, items[incCIHang].ProtectedFiles)
	}
	routed := 0
	for _, ev := range channelEvents(t, root, "selfheal.decision") {
		if ev.Code != selfheal.CodeRepairRouted {
			continue
		}
		routed++
		w := want[ev.Fields["incident"]]
		if ev.Fields["stage"] != "shadow" || ev.Fields["tier"] != string(w.tier) || ev.Fields["would_route"] != w.wantRoute || ev.Fields["would_refuse_audit"] != "true" {
			t.Errorf("event fields = %v", ev.Fields)
		}
	}
	if routed != 3 {
		t.Errorf("want 3 HEALING_REPAIR_ROUTED events, got %d", routed)
	}
	intake := map[string]bool{}
	for _, ev := range channelEvents(t, root, "inbox") {
		if ev.Code == lifecycle.CodeItemFiledConsole {
			intake[ev.Fields["task_id"]] = true
		}
	}
	for id := range want {
		if !intake["repair-"+id] {
			t.Errorf("no INBOX_ITEM_FILED_CONSOLE event for repair-%s: the filing must carry the pass Center", id)
		}
	}
}

func TestSelfhealPropose_RepairCycleStageOffFilesNothing(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8},"repair_cycle":{"stage":"off"}}`)

	rc, _, stderr := runProposeJSON(t, repairEnv(t, false), root)

	if rc != 0 || len(repairItems(t, root)) != 0 {
		t.Fatalf("rc=%d items=%v\n%s", rc, repairItems(t, root), stderr)
	}
	for _, ev := range channelEvents(t, root, "selfheal.decision") {
		if ev.Code == selfheal.CodeRepairRouted {
			t.Errorf("stage off records no repair route: %+v", ev)
		}
	}
	for _, ev := range channelEvents(t, root, "inbox") {
		if ev.Code == lifecycle.CodeItemFiledConsole {
			t.Errorf("stage off files no item, so no intake event: %+v", ev)
		}
	}
}

func TestSelfhealPropose_ARefusedProposalFilesNoRepairItem(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8},"validator":{"stage":"off"}}`)

	rc, _, stderr := runProposeJSON(t, repairEnv(t, false), root)

	if rc != 0 || len(repairItems(t, root)) != 0 {
		t.Fatalf("a validator at off decides nothing, so nothing is filed: rc=%d %v\n%s", rc, repairItems(t, root), stderr)
	}
}

func TestSelfhealPropose_AnAuditSeatFaultIsAWarningAndTheItemIsStillFiled(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8}}`)
	env := repairEnv(t, false)
	env.auditRefused = func(string) (bool, error) { return false, errors.New("table refuses") }

	rc, e, stderr := runProposeJSON(t, env, root)

	if rc != 0 || len(repairItems(t, root)) != 3 || !strings.Contains(strings.Join(e.Warnings, "\n"), "table refuses") {
		t.Fatalf("rc=%d warnings=%v\n%s", rc, e.Warnings, stderr)
	}
}

func TestSelfhealPropose_AnItemAlreadyFiledIsAWarningNotAFault(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8}}`)
	selfhealFile(t, filepath.Join(paths.EvolveDirOf(root), "inbox", "2026-10-27T00-00-00Z-repair-"+incCIHang+".json"), `{"id":"repair-`+incCIHang+`"}`)

	rc, e, stderr := runProposeJSON(t, repairEnv(t, false), root)

	if rc != 0 || len(repairItems(t, root)) != 3 || !strings.Contains(strings.Join(e.Warnings, "\n"), "repair-"+incCIHang) {
		t.Fatalf("rc=%d warnings=%v\n%s", rc, e.Warnings, stderr)
	}
}

func TestAuditRefusedWith_TheCheckedInTableHasAnotherFamilyForTheAuditSeat(t *testing.T) {
	t.Parallel()
	_, root := checkedInPolicy(t)

	refused, err := auditRefusedWith(cliroute.Host{LookPath: everyBinaryPresent}, root)

	if err != nil || refused {
		t.Fatalf("the operator table routes the auditor to another family than the builder: refused=%v err=%v", refused, err)
	}
}

func TestAuditRefusedWith_ASingleFamilyTableRefuses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"builder.json", "auditor.json", "tool-policy.json"} {
		copyFileForTest(t, filepath.Join(repoRootForSkills(t), ".evolve", "profiles", name), filepath.Join(root, ".evolve", "profiles", name))
	}
	selfhealFile(t, filepath.Join(paths.EvolveDirOf(root), paths.PolicyFile), `{"cli_routing":{"clis":["claude"],"default":["claude"]}}`)

	refused, err := auditRefusedWith(cliroute.Host{LookPath: everyBinaryPresent}, root)

	if err != nil || !refused {
		t.Fatalf("one family cannot judge its own build: refused=%v err=%v", refused, err)
	}
}

func TestProductionAuditRefused_ABrokenPolicyIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	selfhealFile(t, filepath.Join(paths.EvolveDirOf(root), paths.PolicyFile), `{"cli_routing":{"clis":["claude"],"default":["agy"]}}`)

	if _, err := productionAuditRefused(root); err == nil {
		t.Fatal("a table whose default names a CLI outside clis must be refused")
	}
}

func TestRepairAuditRefusal_EachSeatFaultIsReportedWithItsSeat(t *testing.T) {
	t.Parallel()
	boom := errors.New("no route")
	for _, seat := range []string{builderSeat, auditorSeat} {
		var asked []string
		resolve := func(agent string) (cliroute.Decision, error) {
			asked = append(asked, agent)
			if agent == seat {
				return cliroute.Decision{}, boom
			}
			return cliroute.Decision{Plan: llmroute.Plan{Candidates: []string{"claude"}}}, nil
		}
		if _, err := repairAuditRefusal(resolve); !errors.Is(err, boom) || asked[len(asked)-1] != seat {
			t.Errorf("%s: err = %v after asking %v", seat, err, asked)
		}
	}
}

func stampOf(it inboxbatch.Item) string {
	var tier string
	if len(it.Tier) == 0 {
		return ""
	}
	if err := json.Unmarshal(it.Tier, &tier); err != nil {
		return "unreadable"
	}
	return tier
}

func TestSelfhealPropose_TheAuditSeatIsReadOncePerPass(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8}}`)
	env := repairEnv(t, false)
	calls := 0
	env.auditRefused = func(string) (bool, error) {
		calls++
		return false, errors.New("table refuses")
	}

	rc, e, stderr := runProposeJSON(t, env, root)

	if rc != 0 || len(repairItems(t, root)) != 3 || calls != 1 {
		t.Fatalf("rc=%d items=%d calls=%d\n%s", rc, len(repairItems(t, root)), calls, stderr)
	}
	if n := strings.Count(strings.Join(e.Warnings, "\n"), "table refuses"); n != 1 {
		t.Errorf("the seat fault is one warning for the pass, got %d", n)
	}
}

func TestSelfhealPropose_RepairCycleStageOffReadsNoAuditSeat(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8},"repair_cycle":{"stage":"off"}}`)
	env := repairEnv(t, false)
	env.auditRefused = func(string) (bool, error) {
		t.Error("the audit seat must not be read when no repair item is filed")
		return false, nil
	}

	if rc, _, stderr := runProposeJSON(t, env, root); rc != 0 {
		t.Fatalf("rc=%d\n%s", rc, stderr)
	}
}

func TestRepairAuditRefusal_ABuilderSeatWithNoCandidateIsAnError(t *testing.T) {
	t.Parallel()
	resolve := func(string) (cliroute.Decision, error) { return cliroute.Decision{}, nil }
	if _, err := repairAuditRefusal(resolve); err == nil {
		t.Fatal("an empty builder plan must be an error, not a panic")
	}
}

func TestSelfhealPropose_NoAcceptedRepairReadsNoAuditSeatAndFilesNothing(t *testing.T) {
	root := repairProject(t, `{"orchestrator":{"max_per_pass":8}}`)
	halt := `{"kind":"halt-and-escalate","params":{"reason":"the operator decides"}}`
	env := selfhealTestEnv(fixtureGH)
	env.agent = fakeBridgeAgent(t, &diagnosisBridge{actions: map[string]string{incCIHang: halt, incChurn: halt, incKernelRepair: halt}})
	env.auditRefused = func(string) (bool, error) {
		t.Error("the audit seat must not be read when no repair is accepted")
		return false, nil
	}

	rc, e, stderr := runProposeJSON(t, env, root)

	if rc != 0 || len(repairItems(t, root)) != 0 || strings.Contains(strings.Join(e.Warnings, "\n"), "audit seat") {
		t.Fatalf("rc=%d items=%d warnings=%v\n%s", rc, len(repairItems(t, root)), e.Warnings, stderr)
	}
	for _, ev := range channelEvents(t, root, "selfheal.decision") {
		if ev.Code == selfheal.CodeRepairRouted {
			t.Errorf("no repair was filed, so no route is recorded: %+v", ev)
		}
	}
}
