//go:build acs

package cycle2254

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	itemID           = "stubborn-item"
	planRaise        = "raise-with-findings"
	planReplan       = "fresh-deep-plan"
	planField        = "retry_plan"
	findingsField    = "last_failure_findings"
	fingerprintField = "last_failure_fingerprint"
	routeConsole     = "console-manual"
)

var (
	sameReasons = []string{"alpha: build is red", "beta: coverage fell to 80%"}
	otherReason = []string{"gamma: a different breakage"}
)

type project struct {
	t         *testing.T
	root      string
	evolveDir string
	inbox     string
}

func newProject(t *testing.T, policyJSON string) project {
	t.Helper()
	root := t.TempDir()
	p := project{t: t, root: root, evolveDir: filepath.Join(root, ".evolve"), inbox: filepath.Join(root, ".evolve", "inbox")}
	if err := os.MkdirAll(p.inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	item, _ := json.Marshal(map[string]any{"id": itemID, "title": "fixture " + itemID, "kind": "feature"})
	if err := os.WriteFile(filepath.Join(p.inbox, "2026-10-26T00-00-00Z-"+itemID+".json"), item, 0o644); err != nil {
		t.Fatal(err)
	}
	if policyJSON != "" {
		if err := os.WriteFile(filepath.Join(p.evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func policyWith(ceiling int, onCeiling string) string {
	body := map[string]any{"thresholds": map[string]any{"task_retry_ceiling": ceiling}}
	if onCeiling != "" {
		body["on_task_retry_ceiling"] = onCeiling
	}
	raw, _ := json.Marshal(map[string]any{"failure_policy": body})
	return string(raw)
}

func (p project) fail(cycle int, reasons []string, systemLevel bool) {
	p.t.Helper()
	ws := filepath.Join(p.evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		p.t.Fatal(err)
	}
	decision, _ := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": itemID}}})
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), decision, 0o644); err != nil {
		p.t.Fatal(err)
	}
	failReason, _ := json.Marshal(map[string]any{"phase": "audit", "reasons": reasons})
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), failReason, 0o644); err != nil {
		p.t.Fatal(err)
	}
	in := cycleoutcome.FailureInputsFor(p.root, p.evolveDir, ws, cycle, io.Discard)
	in.SystemLevel = systemLevel
	if _, err := cycleoutcome.ApplyFailure(in); err != nil {
		p.t.Fatalf("ApplyFailure(cycle %d): %v", cycle, err)
	}
}

func (p project) item(dir string) (map[string]any, bool) {
	p.t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var doc map[string]any
		if json.Unmarshal(raw, &doc) == nil && doc["id"] == itemID {
			return doc, true
		}
	}
	return nil, false
}

func (p project) atRoot() (map[string]any, bool) { return p.item(p.inbox) }

func (p project) quarantined() (map[string]any, bool) {
	return p.item(filepath.Join(p.inbox, "quarantine"))
}

func number(doc map[string]any, key string) int {
	f, _ := doc[key].(float64)
	return int(f)
}

func text(doc map[string]any, key string) string {
	s, _ := doc[key].(string)
	return s
}

func TestC2254_001_ReplayOfTwoSameFingerprintFailuresChangesThePlanOfTheThirdRun(t *testing.T) {
	p := newProject(t, policyWith(3, ""))
	p.fail(5, sameReasons, false)
	afterFirst, ok := p.atRoot()
	if !ok {
		t.Fatal("after failure 1 the item is not back at the inbox root")
	}
	p.fail(6, sameReasons, false)
	afterSecond, ok := p.atRoot()
	if !ok {
		t.Fatal("after failure 2 (ceiling 3) the item is not back at the inbox root for its final run")
	}
	fpFirst, fpSecond := text(afterFirst, fingerprintField), text(afterSecond, fingerprintField)
	if fpFirst == "" || fpFirst != fpSecond {
		t.Fatalf("replay precondition: two failures must share one fingerprint, got %q then %q", fpFirst, fpSecond)
	}
	if got := number(afterSecond, "failure_count"); got != 2 {
		t.Fatalf("failure_count = %d, want 2", got)
	}
	if got := text(afterSecond, planField); got != planReplan {
		t.Errorf("third run of an item that failed twice with one fingerprint carries %s=%q, want %q: the same plan must not be retried", planField, got, planReplan)
	}
	if got := text(afterFirst, planField); got == planReplan {
		t.Errorf("second run already carries %q after one failure: the strategy change belongs to the later run", got)
	}
}

func TestC2254_002_TwoSameFingerprintFailuresAtTheDefaultCeilingNeverReturnToTheInbox(t *testing.T) {
	p := newProject(t, "")
	p.fail(5, sameReasons, false)
	p.fail(6, sameReasons, false)
	if _, back := p.atRoot(); back {
		t.Error("item failed twice with one fingerprint at the default ceiling and is back at the inbox root: it would be retried a third time")
	}
	if _, parked := p.quarantined(); !parked {
		t.Error("item failed twice with one fingerprint at the default ceiling and is not in quarantine/")
	}
}

func TestC2254_003_SecondRunCarriesTheFindingsAndARaisedEffortOfTheFirst(t *testing.T) {
	p := newProject(t, policyWith(4, ""))
	p.fail(5, sameReasons, false)
	doc, ok := p.atRoot()
	if !ok {
		t.Fatal("after failure 1 the item is not back at the inbox root")
	}
	if got := number(doc, "failure_count"); got != 1 {
		t.Fatalf("failure_count = %d, want 1", got)
	}
	if got := text(doc, planField); got != planRaise {
		t.Errorf("%s = %q after the first failure, want %q", planField, got, planRaise)
	}
	findings := text(doc, findingsField)
	for _, reason := range sameReasons {
		if !strings.Contains(findings, reason) {
			t.Errorf("%s = %q does not carry the finding %q of the first run", findingsField, findings, reason)
		}
	}
}

func TestC2254_004_AFailureOfTheSystemNeverStampsAPlanOrFindings(t *testing.T) {
	p := newProject(t, policyWith(4, ""))
	p.fail(5, sameReasons, true)
	doc, ok := p.atRoot()
	if !ok {
		t.Fatal("after a system-level failure the item is not back at the inbox root")
	}
	for _, key := range []string{planField, findingsField} {
		if _, stamped := doc[key]; stamped {
			t.Errorf("a system-level failure stamped %s on the item: the fault is not the task's, so the task must not change its plan", key)
		}
	}
	if got := number(doc, "failure_count"); got != 0 {
		t.Errorf("failure_count = %d after a system-level failure, want 0", got)
	}
}

func TestC2254_005_AfterTheFinalRunTheCeilingPolicyRoutesTheItemToTheConsole(t *testing.T) {
	p := newProject(t, policyWith(3, "console"))
	p.fail(5, sameReasons, false)
	p.fail(6, sameReasons, false)
	if doc, ok := p.atRoot(); !ok || text(doc, "route") == routeConsole {
		t.Fatalf("before the final run the item must wait at the inbox root unrouted: present=%v route=%q", ok, text(doc, "route"))
	}
	p.fail(7, sameReasons, false)
	doc, ok := p.atRoot()
	if !ok {
		t.Fatal("after the final run the console-routed item is not at the inbox root")
	}
	if got := text(doc, "route"); got != routeConsole {
		t.Errorf("route = %q after the final run with on_task_retry_ceiling=console, want %q", got, routeConsole)
	}
	if got := number(doc, "failure_count"); got != 3 {
		t.Errorf("failure_count = %d, want 3", got)
	}
	if strings.TrimSpace(text(doc, "routed_reason")) == "" {
		t.Error("routed_reason is empty: the console cannot see why the item came to it")
	}
	if _, parked := p.quarantined(); parked {
		t.Error("the item was quarantined although the policy routes it to the console")
	}
	_, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: p.root, Stderr: io.Discard}, itemID, "8")
	if !errors.Is(err, inboxmover.ErrConsoleRouted) {
		t.Errorf("a lane can still claim the item after the final run: Claim error = %v, want ErrConsoleRouted", err)
	}
}

func TestC2254_006_AfterTheFinalRunTheDefaultAndAnUnknownPolicyQuarantineTheItem(t *testing.T) {
	for name, onCeiling := range map[string]string{"default": "", "quarantine": "quarantine", "unknown value": "teleport"} {
		t.Run(name, func(t *testing.T) {
			p := newProject(t, policyWith(3, onCeiling))
			p.fail(5, sameReasons, false)
			p.fail(6, sameReasons, false)
			p.fail(7, sameReasons, false)
			if _, back := p.atRoot(); back {
				t.Error("item is at the inbox root after the final run: it would be retried")
			}
			doc, parked := p.quarantined()
			if !parked {
				t.Fatal("item is not in quarantine/ after the final run")
			}
			if got := number(doc, "failure_count"); got != 3 {
				t.Errorf("quarantined failure_count = %d, want 3", got)
			}
		})
	}
}

func TestC2254_007_ReleaseFromQuarantineClearsThePlanAndTheFindings(t *testing.T) {
	p := newProject(t, policyWith(2, ""))
	p.fail(5, otherReason, false)
	p.fail(6, otherReason, false)
	parked, ok := p.quarantined()
	if !ok {
		t.Fatal("precondition: the item is not quarantined at ceiling 2")
	}
	if _, stamped := parked[findingsField]; !stamped {
		t.Fatalf("precondition: the quarantined item keeps no %s for the operator", findingsField)
	}
	if _, err := inboxmover.ReleaseFromQuarantine(inboxmover.Options{ProjectRoot: p.root, Stderr: io.Discard}, itemID); err != nil {
		t.Fatalf("ReleaseFromQuarantine: %v", err)
	}
	doc, back := p.atRoot()
	if !back {
		t.Fatal("released item is not at the inbox root")
	}
	for _, key := range []string{planField, findingsField} {
		if _, stale := doc[key]; stale {
			t.Errorf("released item keeps the stale %s although its failure_count is reset", key)
		}
	}
	if got := number(doc, "failure_count"); got != 0 {
		t.Errorf("failure_count = %d after release, want 0", got)
	}
}

func TestC2254_008_PlanRoundSchedulesTheRoundsOfAnItem(t *testing.T) {
	tests := []struct {
		name                         string
		failures, ceiling            int
		try                          int
		carry, raise, changeStrategy bool
	}{
		{"first run is plain", 0, 4, 1, false, false, false},
		{"second run carries findings and raises effort", 1, 4, 2, true, true, false},
		{"third run changes the strategy", 2, 4, 3, true, true, true},
		{"later run keeps the changed strategy", 3, 4, 4, true, true, true},
		{"the final run of a ceiling of two changes the strategy", 1, 2, 2, true, true, true},
		{"no ceiling still changes the strategy on the third run", 2, 0, 3, true, true, true},
		{"no ceiling raises on the second run only", 1, 0, 2, true, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lifecycle.PlanRound(tc.failures, tc.ceiling)
			if got.Try != tc.try || got.CarryFindings != tc.carry || got.RaiseEffort != tc.raise || got.ChangeStrategy != tc.changeStrategy {
				t.Errorf("PlanRound(%d, %d) = %+v, want try=%d carry=%v raise=%v change=%v",
					tc.failures, tc.ceiling, got, tc.try, tc.carry, tc.raise, tc.changeStrategy)
			}
		})
	}
}

func TestC2254_009_TheRoundStampsBelongToTheLoopNotToTheAuthor(t *testing.T) {
	for _, key := range []string{planField, findingsField} {
		if got := inboxbatch.RoleOf(key).Owner; got != inboxbatch.LoopStampOwned {
			t.Errorf("RoleOf(%q).Owner = %q, want %q: a curator edit must not rewrite a host stamp", key, got, inboxbatch.LoopStampOwned)
		}
	}
}

func lifecycleCoverage(t *testing.T) map[string]string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	run := exec.CommandContext(ctx, "go", "test", "-count=1", "-coverprofile="+profile, "./internal/inboxmover/lifecycle")
	run.Dir = goDir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("go test lifecycle: %v\n%s", err, acsassert.OutputTail(string(out), 30))
	}
	funcs := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+profile)
	funcs.Dir = goDir
	out, err := funcs.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, out)
	}
	percentOf := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		cols := strings.Fields(line)
		if len(cols) == 3 && strings.Contains(cols[0], "/inboxmover/lifecycle/") {
			percentOf[cols[0]+" "+cols[1]] = cols[2]
		}
	}
	return percentOf
}

func TestC2254_010_EveryFunctionOfTheRoundLifecycleFilesIsFullyCovered(t *testing.T) {
	roundFiles := []string{"/round.go:", "/item.go:", "/release.go:", "/quarantine.go:"}
	seen := map[string]bool{}
	for key, pct := range lifecycleCoverage(t) {
		for _, file := range roundFiles {
			if !strings.Contains(key, file) {
				continue
			}
			seen[file] = true
			if pct != "100.0%" {
				t.Errorf("%s coverage = %s, want 100.0%%", key, pct)
			}
		}
	}
	for _, file := range roundFiles {
		if !seen[file] {
			t.Errorf("no covered function found for %s: the coverage profile did not list the file", strings.Trim(file, "/:"))
		}
	}
}

func TestC2254_011_ARouteFailureAtTheCeilingNeverReturnsTheItemToTheRetryPool(t *testing.T) {
	p := newProject(t, policyWith(1, "console"))
	const finalCycle = 9
	claimedDir := filepath.Join(p.inbox, "processing", "cycle-"+strconv.Itoa(finalCycle))
	staleDir := filepath.Join(p.inbox, "processing", "cycle-2")
	for _, dir := range []string{claimedDir, staleDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	body, _ := json.Marshal(map[string]any{"id": itemID, "title": "fixture " + itemID})
	for _, dir := range []string{claimedDir, staleDir} {
		if err := os.WriteFile(filepath.Join(dir, "2026-10-26T00-00-00Z-"+itemID+".json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(p.inbox, "2026-10-26T00-00-00Z-"+itemID+".json")); err != nil {
		t.Fatal(err)
	}
	p.fail(finalCycle, sameReasons, false)
	if doc, back := p.atRoot(); back {
		t.Errorf("the route to the console failed at the ceiling and the item went back to the inbox root (route=%q, failure_count=%d): a lane would pick it up again",
			text(doc, "route"), number(doc, "failure_count"))
	}
}

func TestC2254_012_TheCarriedFindingsStayValidTextWhenTheCutFallsInsideACharacter(t *testing.T) {
	p := newProject(t, policyWith(4, ""))
	threeByteCharacter := "\u754c"
	longReason := strings.Repeat(threeByteCharacter, 3000)
	p.fail(5, []string{longReason}, false)
	doc, ok := p.atRoot()
	if !ok {
		t.Fatal("after failure 1 the item is not back at the inbox root")
	}
	findings := text(doc, findingsField)
	if findings == "" {
		t.Fatalf("%s is empty: a long finding must be cut, not dropped", findingsField)
	}
	if !utf8.ValidString(findings) || strings.ContainsRune(findings, utf8.RuneError) {
		t.Errorf("%s holds a broken character after the cut (%d bytes)", findingsField, len(findings))
	}
	if len(findings) > 4096 || !strings.HasPrefix(longReason, findings) {
		t.Errorf("%s must be a prefix of the finding of at most 4096 bytes, got %d bytes", findingsField, len(findings))
	}
}

func TestC2254_013_TheTaskContractOfTheNextRunCarriesThePriorFindingsAndPlan(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	run := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", "^TestComposeTaskContract_PriorRun", "./internal/core")
	run.Dir = goDir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("the contract of a retried item does not carry the prior run: %v\n%s", err, acsassert.OutputTail(string(out), 40))
	}
}
