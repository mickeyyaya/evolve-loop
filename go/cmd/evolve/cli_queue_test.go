package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const queueEnforcePolicy = `{"isolation":{"i15":{"stage":"enforce"}}}`

func queuedItemRoot(t *testing.T, policyJSON string) (root, item string) {
	t.Helper()
	root = emptyInboxRoot(t)
	if policyJSON != "" {
		contractUnitWrite(t, root, ".evolve/policy.json", policyJSON)
	}
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add", "--file", "-", "--project-root", root}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 0 {
		t.Fatalf("file the item: rc %d %s", rc, stderr.String())
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-cli-inbox-show.json"))
	return root, matches[0]
}

func queuedRequests(t *testing.T, root string) []queuedRequest {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, ".evolve", "queue", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := []queuedRequest{}
	for _, p := range paths {
		var r queuedRequest
		raw, err := os.ReadFile(p)
		if err != nil || json.Unmarshal(raw, &r) != nil {
			t.Fatalf("read %s: %v %s", p, err, raw)
		}
		out = append(out, r)
	}
	return out
}

func TestInboxEdit_DuringAWaveQueuesAndExitsFour(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root, item := queuedItemRoot(t, queueEnforcePolicy)
	liveLaneLease(t, root, 0)
	before, _ := os.ReadFile(item)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4", "--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitQueued || !strings.Contains(stdout.String(), "queued") {
		t.Fatalf("rc %d stdout %q stderr %q, want exit 4 and the queue file named", rc, stdout.String(), stderr.String())
	}
	if after, _ := os.ReadFile(item); !bytes.Equal(before, after) {
		t.Errorf("a queued edit changed the item now: %s", after)
	}
	reqs := queuedRequests(t, root)
	if len(reqs) != 1 || reqs[0].Schema != queueRequestSchema || reqs[0].Verb != "inbox edit" || strings.Join(reqs[0].Args, " ") != "cli-inbox-show --set weight=0.4" {
		t.Errorf("queue = %+v, want one inbox edit request with the edit arguments", reqs)
	}
}

func TestQueuedVerbs_TheShadowStageRefusesAndNamesTheFileItWouldWrite(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root, _ := queuedItemRoot(t, "")
	liveLaneLease(t, root, 0)
	for _, args := range [][]string{
		{"edit", "cli-inbox-show", "--set", "weight=0.4"},
		{"withdraw", "cli-inbox-show", "mistake"},
		{"verify", "cli-inbox-show", "--evidence", "checked"},
		{"route-console", "cli-inbox-show", "operator owns it", "0"},
		{"route-lane", "cli-inbox-show", "a lane may take it"},
	} {
		var stdout, stderr bytes.Buffer
		rc := runInbox(append(args, "--project-root", root), nil, &stdout, &stderr)
		if rc != cliExitRefused || !strings.Contains(stderr.String(), "a loop lane is live") || !strings.Contains(stderr.String(), "shadow: the stage enforce writes "+filepath.Join(root, ".evolve", "queue")) {
			t.Errorf("%v: rc %d stderr %q, want exit 3 and the file the enforce stage writes", args, rc, stderr.String())
		}
	}
	if reqs := queuedRequests(t, root); len(reqs) != 0 {
		t.Errorf("the shadow stage wrote %d requests", len(reqs))
	}
}

func TestQueuedVerbs_InsideAPhasePaneStillRefuse(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "1")
	root, _ := queuedItemRoot(t, queueEnforcePolicy)
	liveLaneLease(t, root, 0)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4", "--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitRefused || !strings.Contains(stderr.String(), "inside a phase") || len(queuedRequests(t, root)) != 0 {
		t.Fatalf("rc %d stderr %q, want exit 3, the phase refusal and no queue file", rc, stderr.String())
	}
}

func TestQueuedVerbs_JSONPrintsTheReceipt(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root, _ := queuedItemRoot(t, queueEnforcePolicy)
	liveLaneLease(t, root, 0)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"route-lane", "cli-inbox-show", "a lane may take it", "--json", "--project-root", root}, nil, &stdout, &stderr)

	var receipt queueReceipt
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil || rc != cliExitQueued || receipt.Verb != "inbox route-lane" || !strings.HasPrefix(receipt.Path, filepath.Join(root, ".evolve", "queue")) {
		t.Fatalf("rc %d receipt %+v (%v) stdout %q", rc, receipt, err, stdout.String())
	}
}

func TestCLIRoutingSet_DuringAWaveQueuesAndADryRunStillRuns(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}, "isolation": {"i15": {"stage": "enforce"}}}`)
	_, before := readRoutingPolicy(t, root)
	liveLaneLease(t, root, 0)

	rc, stdout, stderr := runRoutingVerb(t, inRoot(root, "set", "default", "claude")...)
	if _, after := readRoutingPolicy(t, root); rc != cliExitQueued || !bytes.Equal(before, after) {
		t.Fatalf("rc %d stdout %q stderr %q, want exit 4 and the policy unchanged", rc, stdout, stderr)
	}
	if reqs := queuedRequests(t, root); len(reqs) != 1 || reqs[0].Verb != "cli-routing set" || strings.Join(reqs[0].Args, " ") != "default claude" {
		t.Errorf("queue = %+v, want the set request without --project-root", reqs)
	}
	if rc, stdout, _ := runRoutingVerb(t, inRoot(root, "set", "default", "claude", "--dry-run")...); rc != 0 || !strings.Contains(stdout, "would become") {
		t.Errorf("a dry run under a live lease: rc %d stdout %q, want the preview", rc, stdout)
	}
}

func TestQueueApply_AppliesTheRequestsInTimeOrderAndEmitsOneSignalEach(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root, item := queuedItemRoot(t, queueEnforcePolicy)
	at := time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)
	for i, weight := range []string{"0.3", "0.4"} {
		req := queuedRequest{Schema: queueRequestSchema, Verb: "inbox edit", Args: []string{"cli-inbox-show", "--set", "weight=" + weight}, QueuedAt: at.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339Nano)}
		if _, err := writeQueuedRequest(root, req); err != nil {
			t.Fatal(err)
		}
	}
	bad := queuedRequest{Schema: queueRequestSchema, Verb: "inbox withdraw", Args: []string{"absent", "mistake"}, QueuedAt: at.Add(time.Minute).Format(time.RFC3339Nano)}
	if _, err := writeQueuedRequest(root, bad); err != nil {
		t.Fatal(err)
	}
	events := captureQueueEvents(t)
	var stdout, stderr bytes.Buffer

	rc := dispatchBoundaryVerb(queueApplyVerb, []string{"--project-root", root}, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "applied 2 of 3") {
		t.Fatalf("rc %d stdout %q stderr %q, want exit 0 and the count", rc, stdout.String(), stderr.String())
	}
	if got := itemFields(t, item)["weight"]; got != 0.3 {
		t.Errorf("weight = %v, want 0.3: the request queued last applies last", got)
	}
	if reqs := queuedRequests(t, root); len(reqs) != 0 {
		t.Errorf("the queue still holds %d requests", len(reqs))
	}
	failed, _ := filepath.Glob(filepath.Join(root, ".evolve", "queue", "failed", "*.json"))
	if len(failed) != 1 {
		t.Errorf("failed = %v, want the withdraw of an absent item kept for the operator", failed)
	}
	var applied, refused int
	for _, e := range events() {
		if e.Kind != signalcenter.KindCLIQueueApplied {
			continue
		}
		if e.Severity == signalcenter.SeverityWarn && e.Code == codeCLIQueueFailed {
			refused++
		} else {
			applied++
		}
	}
	if applied != 2 || refused != 1 {
		t.Errorf("signals: %d applied, %d failed; want 2 and 1", applied, refused)
	}
}

func TestQueueApply_AnEmptyQueueDoesNothing(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := dispatchBoundaryVerb(queueApplyVerb, []string{"--project-root", t.TempDir()}, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "no queued request") {
		t.Fatalf("rc %d stdout %q stderr %q", rc, stdout.String(), stderr.String())
	}
}

func TestBoundarySteps_ApplyTheQueueRightAfterSyncMain(t *testing.T) {
	t.Parallel()
	steps := boundarySteps(boundaryArgs{goalText: "g"}, "/plane")
	var verbs []string
	for _, s := range steps {
		verbs = append(verbs, s.verb)
	}
	got := strings.Join(verbs, " ")
	if !strings.Contains(got, "sync-main "+queueApplyVerb+" quality") {
		t.Fatalf("steps = %s, want %s right after sync-main", got, queueApplyVerb)
	}
}

func captureQueueEvents(t *testing.T) func() []signalcenter.Event {
	t.Helper()
	var mu sync.Mutex
	var events []signalcenter.Event
	prev := attachRootEventsFn
	t.Cleanup(func() { attachRootEventsFn = prev })
	attachRootEventsFn = func(c *signalcenter.Center, role, _ string, _ io.Writer) func() {
		if role == roleQueue {
			c.Subscribe(func(e signalcenter.Event) {
				mu.Lock()
				defer mu.Unlock()
				events = append(events, e)
			})
		}
		return c.Flush
	}
	return func() []signalcenter.Event {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(events)
	}
}

func TestQueuedVerbs_AnUnwritableQueueOrAnUnreadablePolicyIsLoud(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root, _ := queuedItemRoot(t, queueEnforcePolicy)
	liveLaneLease(t, root, 0)
	contractUnitWrite(t, root, ".evolve/queue", "a file\n")
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitIO || !strings.Contains(stderr.String(), "write the queue request") {
		t.Errorf("an unwritable queue: rc %d stderr %q, want exit 2", rc, stderr.String())
	}
	contractUnitWrite(t, root, ".evolve/policy.json", "{bad")
	stderr.Reset()
	if rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitRefused || !strings.Contains(stderr.String(), "the policy is unreadable, so the queue stays at stage shadow") {
		t.Errorf("an unreadable policy: rc %d stderr %q, want the shadow refusal and the WARN", rc, stderr.String())
	}
}

func TestQueueApply_RefusesBadArgumentsAndAnUnreadableQueue(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runQueueApply([]string{"extra"}, nil, &stdout, &stderr); rc != cliExitUsage || !strings.Contains(stderr.String(), `unexpected argument "extra"`) {
		t.Errorf("an extra argument: rc %d stderr %q", rc, stderr.String())
	}
	root := t.TempDir()
	contractUnitWrite(t, root, ".evolve/queue", "a file\n")
	stderr.Reset()
	if rc := runQueueApply([]string{"--project-root", root}, nil, &stdout, &stderr); rc != cliExitIO || !strings.Contains(stderr.String(), "list the queue") {
		t.Errorf("a queue that is a file: rc %d stderr %q", rc, stderr.String())
	}
}

func TestQueueApply_AnUnreadableCwdIsAnIOFailure(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	e2StubProcessGetwd(t, os.ErrPermission)
	var stdout, stderr bytes.Buffer
	if rc := runQueueApply(nil, nil, &stdout, &stderr); rc != cliExitIO || !strings.Contains(stderr.String(), queueApplyVerb+": cwd: ") {
		t.Fatalf("rc %d stderr %q", rc, stderr.String())
	}
}

func TestQueueApply_KeepsEachBadRequestAndNamesAShelfItCannotWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractUnitWrite(t, root, ".evolve/queue/broken.json", "{bad")
	contractUnitWrite(t, root, ".evolve/queue/old-schema.json", `{"schema":"evolve.queue.request/0","verb":"inbox edit"}`)
	contractUnitWrite(t, root, ".evolve/queue/notes.txt", "not a request\n")
	contractUnitWrite(t, root, ".evolve/queue/nested/x.json", "{}")
	if _, err := writeQueuedRequest(root, queuedRequest{Schema: queueRequestSchema, Verb: "", QueuedAt: "2026-10-27T01:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := writeQueuedRequest(root, queuedRequest{Schema: queueRequestSchema, Verb: "wave note", QueuedAt: "2026-10-27T01:00:01Z"}); err != nil {
		t.Fatal(err)
	}
	contractUnitWrite(t, root, ".evolve/queue/failed/broken.json/keep", "x")
	var stdout, stderr bytes.Buffer

	rc := runQueueApply([]string{"--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitOK || !strings.Contains(stdout.String(), "applied 0 of 4") {
		t.Fatalf("rc %d stdout %q stderr %q, want 0 of 4 applied", rc, stdout.String(), stderr.String())
	}
	for _, want := range []string{"broken.json: invalid character", `old-schema.json: schema "evolve.queue.request/0" is not evolve.queue.request/1`, `"" is not a queued verb`, `"wave note" is not a queued verb`, "move broken.json to failed: "} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	shelved, _ := filepath.Glob(filepath.Join(root, ".evolve", "queue", "failed", "*.json"))
	if len(shelved) != 4 {
		t.Errorf("failed shelf = %v, want the four requests (the blocked one is a directory)", shelved)
	}
}

func TestShelveQueuedRequest_AShelfThatIsAFileFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractUnitWrite(t, root, ".evolve/queue/applied", "a file\n")
	if _, err := shelveQueuedRequest(root, filepath.Join(root, ".evolve", "queue", "x.json"), "applied"); err == nil {
		t.Fatal("a shelf that is a file must fail")
	}
}

func TestQueueApply_WaitsWhileARunLeaseIsLive(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root, item := queuedItemRoot(t, queueEnforcePolicy)
	liveLaneLease(t, root, 0)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitQueued {
		t.Fatalf("queue the edit: rc %d %s", rc, stderr.String())
	}
	before, _ := os.ReadFile(item)
	stdout.Reset()

	rc := runQueueApply([]string{"--project-root", root}, nil, &stdout, &stderr)

	if after, _ := os.ReadFile(item); rc != 0 || !bytes.Equal(before, after) || len(queuedRequests(t, root)) != 1 || !strings.Contains(stdout.String(), "1 request(s) wait, because cycle-1790 holds a live run lease") {
		t.Fatalf("rc %d stdout %q, want the queue kept while the lane is live", rc, stdout.String())
	}
}

func TestQueuedRequest_TheTimeHasAFixedWidthSoTheTextOrderIsTheTimeOrder(t *testing.T) {
	t.Parallel()
	a := time.Date(2026, 10, 27, 1, 0, 5, 450_000_000, time.UTC).Format(queueTimeLayout)
	b := time.Date(2026, 10, 27, 1, 0, 5, 500_000_000, time.UTC).Format(queueTimeLayout)
	if len(a) != len(b) || !(a < b) {
		t.Fatalf("%q and %q: want equal widths and text order equal to time order", a, b)
	}
}

func TestSettleQueuedRequest_ARequestTheVerbQueuedAgainMovesToRequeued(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path, err := writeQueuedRequest(root, queuedRequest{Schema: queueRequestSchema, Verb: "inbox edit", QueuedAt: "2026-10-27T01:00:00.000000000Z"})
	if err != nil {
		t.Fatal(err)
	}
	signals := newRootSignalCenter(root, filepath.Join(root, ".evolve"), &bytes.Buffer{})
	var stderr bytes.Buffer
	if done := settleQueuedRequest(signals, root, queueEntry{path: path, req: queuedRequest{Verb: "inbox edit"}}, cliExitQueued, &stderr); done {
		t.Fatal("a requeued request is not applied")
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "queue", queueShelfRequeued, filepath.Base(path))); err != nil {
		t.Errorf("the request is not on the requeued shelf: %v (stderr %q)", err, stderr.String())
	}
}

func TestQueueApply_AClaimItCannotMakeKeepsTheRequestQueued(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := writeQueuedRequest(root, queuedRequest{Schema: queueRequestSchema, Verb: "inbox edit", QueuedAt: "2026-10-27T01:00:00.000000000Z"}); err != nil {
		t.Fatal(err)
	}
	contractUnitWrite(t, root, ".evolve/queue/"+queueShelfProcessing, "a file\n")
	var stdout, stderr bytes.Buffer
	if rc := runQueueApply([]string{"--project-root", root}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stderr.String(), "it stays queued") || len(queuedRequests(t, root)) != 1 {
		t.Fatalf("rc %d stderr %q, want the request kept in the queue", rc, stderr.String())
	}
}

func TestQueueApply_NamesTheTrackedChangesThatTheNextSyncMainRefuses(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	repo := gittest.Fixture(t)
	root := repo.Dir
	contractUnitWrite(t, root, ".evolve/inbox/2026-10-27T00-00-00Z-cli-inbox-show.json", cliInboxItem)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "an item")
	req := queuedRequest{Schema: queueRequestSchema, Verb: "inbox edit", Args: []string{"cli-inbox-show", "--set", "weight=0.4"}, QueuedAt: "2026-10-27T01:00:00.000000000Z"}
	if _, err := writeQueuedRequest(root, req); err != nil {
		t.Fatal(err)
	}
	events := captureQueueEvents(t)
	var stdout, stderr bytes.Buffer

	rc := runQueueApply([]string{"--project-root", root}, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "land these tracked changes before the next sync-main") || !strings.Contains(stdout.String(), "cli-inbox-show.json") {
		t.Fatalf("rc %d stdout %q stderr %q", rc, stdout.String(), stderr.String())
	}
	var unlanded int
	for _, e := range events() {
		if e.Code == codeCLIQueueUnlanded {
			unlanded++
		}
	}
	if unlanded != 1 {
		t.Errorf("%d CLI_QUEUE_UNLANDED signals, want 1", unlanded)
	}
}

func TestQueuedVerbs_AReceiptThatCannotBeWrittenIsAnIOFailure(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	root, _ := queuedItemRoot(t, queueEnforcePolicy)
	liveLaneLease(t, root, 0)
	var stderr bytes.Buffer
	if rc := runInbox([]string{"route-lane", "cli-inbox-show", "a lane may take it", "--json", "--project-root", root}, nil, &failingWriter{}, &stderr); rc != cliExitIO {
		t.Fatalf("rc %d stderr %q, want exit 2", rc, stderr.String())
	}
}
