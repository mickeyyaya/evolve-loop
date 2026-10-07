package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

var l2RoundsPath = filepath.Join("..", "..", "internal", "convergence", "testdata", "l2-rounds.json")

func convergenceRoot(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if policyJSON != "" {
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		writePolicy(t, root, policyJSON)
	}
	return root
}

func runDecide(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runConvergence(append([]string{"decide"}, args...), nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func decodeDecision(t *testing.T, stdout string) convergence.Decision {
	t.Helper()
	var d convergence.Decision
	if err := json.Unmarshal([]byte(stdout), &d); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	return d
}

func l2Prefix(t *testing.T, round int) string {
	t.Helper()
	data, err := os.ReadFile(l2RoundsPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["round"] = round
	doc["rounds"] = doc["rounds"].([]any)[:round+1]
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rounds.json")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCmd_ConvergenceDecide_TheL2ReplayLandsAtRoundThreeThroughRungTwo(t *testing.T) {
	convergenceRoot(t, "")

	rc, stdout, stderr := runDecide(t, "--input", l2RoundsPath, "--json")

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	d := decodeDecision(t, stdout)
	if d.Action != convergence.ActionLand || d.Rung != 2 || d.LandRound != 3 || d.BlockingBar != convergence.SeverityHigh {
		t.Fatalf("decision %+v, want Land at round 3 through rung 2 at the HIGH bar", d)
	}
	if !reflect.DeepEqual(d.Defer, []string{"r7-MEDIUM-1", "r7-LOW-1", "r7-LOW-2", "r7-LOW-3", "r7-LOW-4"}) ||
		!reflect.DeepEqual(d.File, []string{"r7-INFO"}) || !reflect.DeepEqual(d.Redesign, []string{"bridge:model-check"}) {
		t.Fatalf("defer %q file %q redesign %q", d.Defer, d.File, d.Redesign)
	}
}

func TestCmd_ConvergenceDecide_HumanTextNamesTheDecisionAndEveryReason(t *testing.T) {
	convergenceRoot(t, "")

	rc, stdout, stderr := runDecide(t, "--input", l2RoundsPath)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	for _, want := range []string{
		"convergence: console-lane after round 3: Land (rung 2, bar HIGH)",
		"land round: 3",
		"defer: r7-MEDIUM-1, r7-LOW-1, r7-LOW-2, r7-LOW-3, r7-LOW-4",
		"file: r7-INFO",
		"redesign: bridge:model-check",
		"  CONVERGENCE_RUNG loop=console-lane round=3 rung=2 bar=HIGH action=Land cause=schedule",
		"  CONVERGENCE_CONCENTRATION loop=console-lane component=bridge:model-check share=0.647 prev_share=0.636 window=2",
	} {
		if !strings.Contains(stdout, want+"\n") {
			t.Fatalf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "split:") || strings.Contains(stdout, "raise:") {
		t.Fatalf("stdout prints empty fields:\n%s", stdout)
	}
}

func TestCmd_ConvergenceDecide_AContinueNamesItsChanges(t *testing.T) {
	convergenceRoot(t, "")

	rc, stdout, stderr := runDecide(t, "--input", l2Prefix(t, 2))

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	for _, want := range []string{"Continue (rung 2, bar HIGH)", "fresh context: yes", "verify-only: yes"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestCmd_ConvergenceDecide_TodaysClaudeTierTableHasNoDeepToTopHeadroom(t *testing.T) {
	convergenceRoot(t, "")

	rc, stdout, stderr := runDecide(t, "--input", l2Prefix(t, 1), "--json")

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	d := decodeDecision(t, stdout)
	if d.Rung != 1 || d.JudgeRaise != (convergence.TierEffort{}) || d.FixerRaise != (convergence.TierEffort{}) {
		t.Fatalf("decision %+v, want rung 1 with no raise: claude-tmux deep and top are both opus", d)
	}
	for _, role := range []string{"judge", "fixer"} {
		want := "CONVERGENCE_NO_HEADROOM loop=console-lane role=" + role + " family=claude-tmux from=deep to=top"
		if !strings.Contains(strings.Join(d.Reasons, "\n"), want) {
			t.Fatalf("reasons %q lack %q", d.Reasons, want)
		}
	}
}

func TestCmd_ConvergenceDecide_ThePolicysConvergenceBlockSetsTheBudget(t *testing.T) {
	convergenceRoot(t, `{"workflow": {"convergence": {"max_fix_rounds": 2, "stage": "loud"}}}`)

	rc, stdout, stderr := runDecide(t, "--input", l2Prefix(t, 1), "--json")

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	if d := decodeDecision(t, stdout); d.Rung != 2 || !d.FreshContext {
		t.Fatalf("decision %+v, want round 2 as the final round under max_fix_rounds 2", d)
	}
	if !strings.Contains(stderr, `convergence decide: WARN workflow.convergence.stage: unknown value "loud", falling back to "shadow"`) {
		t.Fatalf("stderr %q, want the unknown stage warned", stderr)
	}
}

func TestCmd_ConvergenceDecide_BadInputExits10(t *testing.T) {
	root := convergenceRoot(t, "")
	write := func(name, body string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for name, args := range map[string][]string{
		"no subverb":      nil,
		"unknown subverb": {"explain"},
		"no input flag":   {"decide"},
		"stray argument":  {"decide", "--input", l2RoundsPath, "extra"},
		"missing file":    {"decide", "--input", filepath.Join(root, "absent.json")},
		"malformed json":  {"decide", "--input", write("bad.json", `{"loop": `)},
		"unknown field":   {"decide", "--input", write("field.json", `{"loop": "console-lane", "round": 0, "rounds": [], "max_rounds": 3}`)},
		"round mismatch":  {"decide", "--input", write("round.json", `{"loop": "console-lane", "round": 2, "rounds": [{"index": 0, "findings": []}]}`)},
		"unknown family": {"decide", "--input", write("family.json",
			`{"loop": "console-lane", "round": 0, "rounds": [{"index": 0, "findings": []}], "judge": {"family": "nosuch-cli", "tier": "deep"}}`)},
		"family without a tier table": {"decide", "--input", write("tierless.json",
			`{"loop": "console-lane", "round": 0, "rounds": [{"index": 0, "findings": []}], "fixer": {"family": "claude-p", "tier": "balanced"}}`)},
		"deferred critical": {"decide", "--input", write("deferred.json",
			`{"loop": "console-lane", "round": 0, "rounds": [{"index": 0, "findings": [{"id": "c", "severity": "CRITICAL", "status": "DEFERRED"}]}]}`)},
		"filed critical": {"decide", "--input", write("filed.json",
			`{"loop": "console-lane", "round": 0, "rounds": [{"index": 0, "findings": [{"id": "c", "severity": "CRITICAL", "status": "FILED"}]}]}`)},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			rc := runConvergence(args, nil, &stdout, &stderr)

			if rc != 10 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "convergence") {
				t.Fatalf("rc = %d stdout = %q stderr = %q, want exit 10 with the cause on stderr", rc, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCmd_ConvergenceDecide_EveryLoopButTheConsoleLaneExits10NamingItsInProcessCaller(t *testing.T) {
	root := convergenceRoot(t, "")
	for loop, caller := range map[string]string{
		"audit-repair": "V5", "explanation-reauthor": "V5", "code-review": "V6", "cycle": "V10", "inbox-item": "V11", "ship-recovery": "V12",
	} {
		t.Run(loop, func(t *testing.T) {
			path := filepath.Join(root, loop+".json")
			body := `{"loop": "` + loop + `", "round": 0, "rounds": [{"index": 0, "findings": [{"id": "h", "severity": "HIGH", "status": "OPEN"}]}]}`
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			rc, stdout, stderr := runDecide(t, "--input", path)

			if rc != 10 || stdout != "" || !strings.Contains(stderr, "console-lane") || !strings.Contains(stderr, caller) {
				t.Fatalf("rc = %d stdout = %q stderr = %q, want exit 10 naming console-lane and the in-process caller %s", rc, stdout, stderr, caller)
			}
		})
	}
}

func TestCmd_ConvergenceDecide_AnOutputWriteFailureExits1(t *testing.T) {
	convergenceRoot(t, "")
	closed, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	rc := runConvergence([]string{"decide", "--input", l2RoundsPath, "--json"}, nil, closed, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "convergence decide: encode") {
		t.Fatalf("rc = %d stderr = %q, want exit 1 naming the failed write", rc, stderr.String())
	}
}

func TestCmd_ConvergenceDecide_AnUnreadablePolicyExits2(t *testing.T) {
	convergenceRoot(t, `{"workflow": {"convergence": {"max_rounds": 2}}}`)

	rc, stdout, stderr := runDecide(t, "--input", l2RoundsPath)

	if rc != 2 || stdout != "" || !strings.Contains(stderr, "max_rounds") {
		t.Fatalf("rc = %d stdout = %q stderr = %q, want exit 2 naming the refused key", rc, stdout, stderr)
	}
}

func TestCmd_Convergence_IsARegisteredVerb(t *testing.T) {
	convergenceRoot(t, "")
	var stdout, stderr bytes.Buffer

	rc := dispatch([]string{"convergence", "decide", "--input", l2RoundsPath, "--json"}, nil, &stdout, &stderr)

	if rc != 0 || decodeDecision(t, stdout.String()).Action != convergence.ActionLand {
		t.Fatalf("rc = %d stdout = %q stderr = %q", rc, stdout.String(), stderr.String())
	}
}
