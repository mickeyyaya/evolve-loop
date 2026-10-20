package cycleoutcome

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const cycle1953Decision = `{"category":"intent-malformed","level":"task","evidence":"lane item files miss go/internal/inboxmover/lifecycle/item.go:189","action":"defer-or-quarantine","fix_type":"reintent","schema_version":1}`

func deferredCycle(t *testing.T, decision string) (string, string, FailureInputs, *bytes.Buffer) {
	t.Helper()
	root, inbox, ws := seedProject(t, "lane-item", []string{"lane-item"})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "lane-item", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if decision != "" {
		if err := os.WriteFile(filepath.Join(ws, "failure-decision.json"), []byte(decision), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stderr bytes.Buffer
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, &stderr)
	in.Deferred, in.SystemLevel = true, false
	return root, inbox, in, &stderr
}

func TestApplyFailure_Cycle1953ReintentRoutesTheItemToTheConsole(t *testing.T) {
	root, inbox, in, _ := deferredCycle(t, cycle1953Decision)

	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}

	rec := itemRecord(t, inbox, "lane-item")
	reason, _ := rec["routed_reason"].(string)
	if rec["route"] != "console-manual" || rec["routed_cycle"] != float64(7) {
		t.Errorf("item = %v, want route console-manual by cycle 7", rec)
	}
	for _, want := range []string{"cycle 7", "intent-malformed", "fix_type reintent"} {
		if !strings.Contains(reason, want) {
			t.Errorf("routed_reason = %q, want it to name %q", reason, want)
		}
	}
	if _, bumped := rec["failure_count"]; bumped {
		t.Errorf("a routed item gets no failure_count: %v", rec)
	}
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "lane-item", "8"); err == nil {
		t.Error("the next lane claimed the item that the reintent decision routed to the console")
	}
}

func TestApplyFailure_PipelineRepairMakesTheItemWaitOnACompanionConsoleItem(t *testing.T) {
	decision := strings.Replace(cycle1953Decision, `"fix_type":"reintent"`, `"fix_type":"pipeline-repair"`, 1)
	root, inbox, in, _ := deferredCycle(t, decision)

	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}

	companion := itemRecord(t, inbox, "pipeline-repair-cycle-7")
	reason, _ := companion["routed_reason"].(string)
	if companion["route"] != "console-manual" || !strings.Contains(reason, "cycle 7") || !strings.Contains(reason, "intent-malformed") || !strings.Contains(reason, "fix_type pipeline-repair") {
		t.Errorf("companion = %v, want a console item whose routed_reason names the cycle, the category and the fix_type", companion)
	}
	rec := itemRecord(t, inbox, "lane-item")
	deps, _ := rec["deps"].([]any)
	if len(deps) != 1 || deps[0] != "pipeline-repair-cycle-7" || rec["route"] == "console-manual" {
		t.Errorf("item = %v, want it to wait on the companion and keep its lane route: the fix_type, not the category, decides", rec)
	}
	if _, bumped := rec["failure_count"]; bumped {
		t.Errorf("a pipeline defect is not the item's failed attempt: %v", rec)
	}
	_, err := inboxmover.ClaimDispatchable(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "lane-item", "8")
	if !errors.Is(err, inboxmover.ErrWaitingOnDependency) {
		t.Errorf("claim err = %v, want the item to wait on its companion", err)
	}
}

func TestApplyFailure_ASecondPipelineRepairCloseoutReusesTheCompanion(t *testing.T) {
	decision := strings.Replace(cycle1953Decision, `"fix_type":"reintent"`, `"fix_type":"pipeline-repair"`, 1)
	root, inbox, in, _ := deferredCycle(t, decision)
	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("first ApplyFailure: %v", err)
	}
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "lane-item", "7"); err != nil {
		t.Fatalf("reclaim: %v", err)
	}

	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("second ApplyFailure: %v", err)
	}

	deps, _ := itemRecord(t, inbox, "lane-item")["deps"].([]any)
	if len(deps) != 1 {
		t.Errorf("deps = %v, want the one companion once", deps)
	}
}

func TestApplyFailure_ADeferWithNoRouteKeepsTheReleaseAndSaysWhy(t *testing.T) {
	unknown := strings.Replace(cycle1953Decision, `"fix_type":"reintent"`, `"fix_type":"rewrite-the-world"`, 1)
	for name, c := range map[string]struct {
		decision string
		deferred bool
		warn     string
	}{
		"no recorded decision":           {"", true, "no recorded defer-or-quarantine failure-decision.json"},
		"an unknown fix_type":            {unknown, true, `fix_type "rewrite-the-world" has no route`},
		"a cycle not ended by the defer": {cycle1953Decision, false, ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, inbox, in, stderr := deferredCycle(t, c.decision)
			in.Deferred = c.deferred

			if _, err := ApplyFailure(in); err != nil {
				t.Fatalf("ApplyFailure: %v", err)
			}

			rec := itemRecord(t, inbox, "lane-item")
			if rec["route"] == "console-manual" || rec["failure_count"] != float64(1) {
				t.Errorf("item = %v, want the plain release with one failure", rec)
			}
			saysWhy := strings.Contains(stderr.String(), "defer route")
			if saysWhy != (c.warn != "") || !strings.Contains(stderr.String(), c.warn) {
				t.Errorf("stderr = %q, want %q", stderr.String(), c.warn)
			}
		})
	}
}

func TestApplyFailure_ACompanionThatCannotBeFiledKeepsTheReleaseAndSaysWhy(t *testing.T) {
	decision := strings.Replace(cycle1953Decision, `"fix_type":"reintent"`, `"fix_type":"pipeline-repair"`, 1)
	_, inbox, in, stderr := deferredCycle(t, decision)
	in.PriorityClasses = []string{"feature"}

	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}

	if hasItem(t, inbox, "pipeline-repair-cycle-7") || itemRecord(t, inbox, "lane-item")["failure_count"] != float64(1) {
		t.Error("want no companion and the plain release with one failure")
	}
	if !strings.Contains(stderr.String(), "defer route: the companion item pipeline-repair-cycle-7 was not filed and stamped") {
		t.Errorf("stderr = %q, want the filing failure", stderr.String())
	}
}

func TestApplyFailure_ADepsEditThatFailsIsAnError(t *testing.T) {
	decision := strings.Replace(cycle1953Decision, `"fix_type":"reintent"`, `"fix_type":"pipeline-repair"`, 1)
	_, _, in, _ := deferredCycle(t, decision)
	edit := depsEdit
	t.Cleanup(func() { depsEdit = edit })
	depsEdit = func(inboxmover.Options, string, []inboxmover.FieldEdit) (string, error) {
		return "", errors.New("disk full")
	}

	_, err := ApplyFailure(in)

	if err == nil || !strings.Contains(err.Error(), "lane-item") || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("err = %v, want the failed deps edit of lane-item", err)
	}
}

func TestRouteEachConsole_AnIdWithNoItemIsNotRouted(t *testing.T) {
	root, inbox, _, _ := deferredCycle(t, cycle1953Decision)
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}

	routed := routeEachConsole(opts, []string{"lane-item", "ghost"}, "cycle 7 ended by a defer-or-quarantine decision", 7)

	if routed || itemRecord(t, filepath.Join(inbox, "processing", "cycle-7"), "lane-item")["route"] != "console-manual" {
		t.Errorf("routed = %t, want false for the ghost id and the real item routed", routed)
	}
}
