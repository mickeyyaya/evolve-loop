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
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var inboxAuditNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

type curatorBridge struct {
	mu       sync.Mutex
	requests []core.BridgeRequest
	report   func(in inboxAuditInput) string
}

func (b *curatorBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.mu.Lock()
	b.requests = append(b.requests, req)
	b.mu.Unlock()
	start := strings.Index(req.Prompt, `{"schema":"`+inboxAuditInputSchema+`"`)
	var in inboxAuditInput
	if start >= 0 {
		_ = json.NewDecoder(strings.NewReader(req.Prompt[start:])).Decode(&in)
	}
	report := b.report
	if report == nil {
		report = judgeByLandedCommits
	}
	return core.BridgeResponse{}, os.WriteFile(req.ArtifactPath, []byte(report(in)), 0o644)
}

func (b *curatorBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func judgeByLandedCommits(in inboxAuditInput) string {
	var verdicts []string
	for _, c := range in.Candidates {
		if len(c.Landed) > 0 {
			verdicts = append(verdicts, `{"id":"`+c.ID+`","verdict":"consume","reason":"the change landed","evidence":["`+c.Landed[0]+`"]}`)
			continue
		}
		verdicts = append(verdicts, `{"id":"`+c.ID+`","verdict":"keep","reason":"nothing landed"}`)
	}
	return `{"schema":"evolve.inbox.audit/1","proposals":[` + strings.Join(verdicts, ",") + `]}`
}

func curatorAgent(t *testing.T, b *curatorBridge) agentSeat {
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
			return inboxAuditNow.Add(tick)
		},
	})
}

type auditFixture struct {
	root, landed string
	items        map[string][]byte
	center       *signalcenter.Center
}

const auditPolicy = `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]},
	"inbox": {"curator": {"stage": "shadow", "audit": {"min_age_days": 14, "trigger_count": 2, "max_items": 5}}}}`

func auditItem(id, route, created, acceptance string, files ...string) string {
	raw, _ := json.Marshal(map[string]any{
		"id": id, "kind": "feature", "weight": 0.5, "priority_class": "correctness", "title": "the title of " + id, "summary": "s",
		"route": route, "created_at": created, "files": files, "acceptance": []string{acceptance},
	})
	return string(raw)
}

func newAuditFixture(t *testing.T, policyJSON string) *auditFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	root, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	src := repoRootForSkills(t)
	for _, rel := range []string{
		"docs/architecture/phase-registry.json",
		".evolve/phases/inbox-curator/phase.json",
		".evolve/profiles/inbox-curator.json",
		"agents/evolve-inbox-curator.md",
	} {
		copyFileForTest(t, filepath.Join(src, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel)))
	}
	f := &auditFixture{root: root, items: map[string][]byte{}}
	f.write(t, ".evolve/policy.json", policyJSON)
	for name, body := range map[string]string{
		"2026-01-01T00-00-00Z-landed-item.json": auditItem("landed-item", "console", "2026-01-01T00:00:00Z", "the feature exists", "go/feature.go"),
		"2026-01-02T00-00-00Z-open-item.json":   auditItem("open-item", "console", "2026-01-02T00:00:00Z", "the other part exists", "go/other.go"),
		"2026-11-02T00-00-00Z-young-item.json":  auditItem("young-item", "console", "2026-11-02T00:00:00Z", "the young part exists", "go/feature.go"),
		"2026-01-03T00-00-00Z-tested-item.json": auditItem("tested-item", "console", "2026-01-03T00:00:00Z", "`TestFeature_Exists` passes", "go/feature.go"),
		"2026-01-04T00-00-00Z-lane-item.json":   auditItem("lane-item", "", "2026-01-04T00:00:00Z", "the lane part exists", "go/feature.go"),
	} {
		rel := ".evolve/inbox/" + name
		f.write(t, rel, body)
		f.items[rel] = []byte(body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	f.write(t, "go/feature.go", "package feature\n")
	repo.Git("add", "go/feature.go")
	repo.Git("commit", "-q", "-m", "feat: the feature")
	f.landed = repo.Git("rev-parse", "HEAD")
	f.center = signalcenter.New()
	return f
}

func (f *auditFixture) write(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *auditFixture) env(agent agentSeat) inboxAuditEnv {
	return inboxAuditEnv{agent: agent, now: func() time.Time { return inboxAuditNow },
		newCenter: func(string, string, io.Writer) (*signalcenter.Center, func()) { return f.center, func() {} }}
}

func (f *auditFixture) assertNothingMoved(t *testing.T) {
	t.Helper()
	for rel, want := range f.items {
		got, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(rel)))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: the audit must not move or change an item: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.root, ".evolve", "inbox", "consumed")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the audit must not consume an item: %v", err)
	}
}

func runAuditJSON(t *testing.T, env inboxAuditEnv, root string, args ...string) (int, inboxAuditReport, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runInboxAuditWith(env, root, append([]string{"--json"}, args...), &stdout, &stderr)
	var r inboxAuditReport
	if rc == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
			t.Fatalf("rc=%d: no JSON report: %v\n%s\n%s", rc, err, stdout.String(), stderr.String())
		}
	}
	return rc, r, stderr.String()
}

func TestCmd_InboxAudit_AFixtureItemWithALandedCommitYieldsAConsumeProposalThroughTheBridgeAndNothingMoves(t *testing.T) {
	f := newAuditFixture(t, auditPolicy)
	b := &curatorBridge{}

	rc, r, stderr := runAuditJSON(t, f.env(curatorAgent(t, b)), f.root)

	if rc != 0 || !r.Ran || r.Schema != inboxAuditSchema || r.Count != 2 || r.Threshold != 2 || len(b.requests) != 1 {
		t.Fatalf("rc=%d report=%+v launches=%d\n%s", rc, r, len(b.requests), stderr)
	}
	if !slices.Equal(r.Candidates, []string{"landed-item", "open-item"}) {
		t.Errorf("candidates = %v: only old console items with no probe, oldest first", r.Candidates)
	}
	req := b.requests[0]
	if req.CLI == "" || !strings.Contains(req.Prompt, f.landed) || !strings.Contains(req.Prompt, `"gone_paths":["go/other.go"]`) {
		t.Errorf("the seat must be routed and get the landed commit and the gone path: cli=%q\n%s", req.CLI, req.Prompt)
	}
	if len(r.Proposals) != 2 || len(r.Refused) != 0 || r.RunLabel == "" {
		t.Fatalf("proposals = %+v refused = %v label = %q", r.Proposals, r.Refused, r.RunLabel)
	}
	consume := r.Proposals[0]
	if consume.Kind != inboxcurator.ProposeConsume || consume.ID != "landed-item" || !slices.Equal(consume.Evidence, []string{f.landed}) ||
		consume.Trigger != inboxcurator.TriggerAudit || consume.Deterministic || consume.Path != "2026-01-01T00-00-00Z-landed-item.json" {
		t.Errorf("consume = %+v, want a judgment consume of landed-item that cites %s", consume, f.landed)
	}
	if r.Proposals[1].Kind != inboxcurator.ProposeKeep || r.Proposals[1].ID != "open-item" {
		t.Errorf("keep = %+v", r.Proposals[1])
	}
	f.assertNothingMoved(t)
	raw, err := os.ReadFile(r.Record)
	var stored inboxAuditReport
	if err != nil || json.Unmarshal(raw, &stored) != nil || len(stored.Proposals) != 2 || !strings.HasPrefix(r.Record, filepath.Join(f.root, ".evolve", "inbox-audit")+string(filepath.Separator)) {
		t.Errorf("the proposals must be stored under .evolve/inbox-audit: %s %v", r.Record, err)
	}
	var signals []signalcenter.Event
	for _, e := range f.center.Recent() {
		if e.Code == inboxcurator.CodeProposal {
			signals = append(signals, e)
		}
	}
	if len(signals) != 1 || signals[0].Fields["id"] != "landed-item" || signals[0].Fields["trigger"] != "audit" || signals[0].Fields["applied"] != "false" || signals[0].Fields["stage"] != "shadow" {
		t.Errorf("one unapplied proposal signal for the consume, none for a keep: %+v", signals)
	}
}

func TestCmd_InboxAudit_BelowTheThresholdNoSeatRunsUnlessForced(t *testing.T) {
	f := newAuditFixture(t, strings.Replace(auditPolicy, `"trigger_count": 2`, `"trigger_count": 3`, 1))
	b := &curatorBridge{}
	env := f.env(curatorAgent(t, b))

	rc, r, stderr := runAuditJSON(t, env, f.root)
	if rc != 0 || r.Ran || r.Count != 2 || r.Threshold != 3 || len(b.requests) != 0 || r.Proposals == nil || r.Refused == nil {
		t.Fatalf("below the threshold: rc=%d report=%+v launches=%d\n%s", rc, r, len(b.requests), stderr)
	}

	rc, r, stderr = runAuditJSON(t, env, f.root, "--force")
	if rc != 0 || !r.Ran || len(b.requests) != 1 || len(r.Proposals) != 2 {
		t.Fatalf("--force runs the seat below the threshold: rc=%d report=%+v launches=%d\n%s", rc, r, len(b.requests), stderr)
	}
	f.assertNothingMoved(t)
}

func TestCmd_InboxAudit_ForceWithNoOldItemRunsNoSeat(t *testing.T) {
	f := newAuditFixture(t, strings.Replace(auditPolicy, `"min_age_days": 14`, `"min_age_days": 100000`, 1))
	b := &curatorBridge{}

	rc, r, stderr := runAuditJSON(t, f.env(curatorAgent(t, b)), f.root, "--force")

	if rc != 0 || r.Ran || r.Count != 0 || len(b.requests) != 0 {
		t.Fatalf("--force with no candidate must start no seat: rc=%d report=%+v launches=%d\n%s", rc, r, len(b.requests), stderr)
	}
}

func TestCmd_InboxAudit_ThePromptCarriesTheContractOfTheCurator(t *testing.T) {
	t.Parallel()
	prompt := inboxAuditPrompt(nil)
	if !strings.Contains(prompt, inboxcurator.AuditContract()) || !strings.Contains(prompt, `"schema":"`+inboxAuditInputSchema+`"`) {
		t.Errorf("the prompt must carry the contract of inboxcurator and the input:\n%s", prompt)
	}
}

func TestShouldAudit_StartsTheSeatAtTheThresholdOrWhenForcedAndNeverWithNoCandidate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		count, threshold int
		force, want      bool
	}{{0, 1, true, false}, {1, 2, false, false}, {1, 2, true, true}, {2, 2, false, true}} {
		if got := shouldAudit(tc.count, tc.threshold, tc.force); got != tc.want {
			t.Errorf("shouldAudit(%d, %d, %v) = %v, want %v", tc.count, tc.threshold, tc.force, got, tc.want)
		}
	}
}

func TestCmd_InboxAudit_TheSeatGetsAtMostMaxItemsOldestFirst(t *testing.T) {
	f := newAuditFixture(t, strings.Replace(auditPolicy, `"max_items": 5`, `"max_items": 1`, 1))
	b := &curatorBridge{}

	rc, r, stderr := runAuditJSON(t, f.env(curatorAgent(t, b)), f.root)

	if rc != 0 || r.Count != 2 || !slices.Equal(r.Candidates, []string{"landed-item"}) || len(r.Proposals) != 1 {
		t.Fatalf("rc=%d report=%+v\n%s", rc, r, stderr)
	}
}

func TestCmd_InboxAudit_ARefusedVerdictIsAWarningAndTheRestStands(t *testing.T) {
	f := newAuditFixture(t, auditPolicy)
	b := &curatorBridge{report: func(inboxAuditInput) string {
		return `{"schema":"evolve.inbox.audit/1","proposals":[{"id":"open-item","verdict":"consume","reason":"it landed","evidence":["0123456789abcdef"]},` +
			`{"id":"landed-item","verdict":"retire","reason":"replaced"}]}`
	}}
	var stdout, stderr bytes.Buffer

	rc := runInboxAuditWith(f.env(curatorAgent(t, b)), f.root, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stderr.String(), "inbox audit: WARN: open-item: a consume cites no commit of the checked-out branch") {
		t.Fatalf("rc=%d\n%s", rc, stderr.String())
	}
	for _, want := range []string{"inbox audit: 2 old console item(s) with no probe; the threshold is 2", "retire landed-item: replaced", "1 refused", "nothing moved"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout must contain %q:\n%s", want, stdout.String())
		}
	}
	f.assertNothingMoved(t)
}

func TestCmd_InboxAudit_ThePlainOutputSaysWhyNoSeatRan(t *testing.T) {
	f := newAuditFixture(t, strings.Replace(auditPolicy, `"trigger_count": 2`, `"trigger_count": 9`, 1))
	var stdout, stderr bytes.Buffer

	rc := runInboxAuditWith(f.env(curatorAgent(t, &curatorBridge{})), f.root, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "below the threshold; no seat ran (--force runs it)") {
		t.Fatalf("rc=%d\n%s\n%s", rc, stdout.String(), stderr.String())
	}
}

func TestCmd_InboxAudit_EachFailureHasItsExitCode(t *testing.T) {
	f := newAuditFixture(t, auditPolicy)
	failing := func([]string, io.Reader, io.Writer, io.Writer) int { return 3 }
	for name, tc := range map[string]struct {
		env  inboxAuditEnv
		root string
		args []string
		rc   int
		want string
	}{
		"an unknown flag":         {f.env(failing), f.root, []string{"--apply"}, 10, "usage: evolve inbox audit"},
		"an extra argument":       {f.env(failing), f.root, []string{"x"}, 10, "usage: evolve inbox audit"},
		"a seat that fails":       {f.env(failing), f.root, nil, 2, "inbox audit: evolve agent run inbox-curator"},
		"a report that is bad":    {f.env(curatorAgent(t, &curatorBridge{report: func(inboxAuditInput) string { return `{"schema":"evolve.diagnosis/1","proposals":[]}` }})), f.root, nil, 1, "inbox audit: inboxcurator: the audit report"},
		"a policy that is bad":    {f.env(failing), badPolicyRoot(t), nil, 2, "inbox audit: policy"},
		"an inbox that is a file": {f.env(failing), inboxFileRoot(t), nil, 2, "inbox audit: "},
		"a root with no git":      {f.env(failing), noGitRoot(t), nil, 2, "inbox audit: inboxcurator: read the landings of old-item"},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInboxAuditWith(tc.env, tc.root, tc.args, &stdout, &stderr); rc != tc.rc || !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("%s: rc=%d, want %d with %q\n%s", name, rc, tc.rc, tc.want, stderr.String())
		}
	}
	f.assertNothingMoved(t)
}

func TestCmd_InboxAudit_AReportThatIsNotTheSchemaNamesTheRunOfThePaidSeat(t *testing.T) {
	f := newAuditFixture(t, auditPolicy)
	b := &curatorBridge{report: func(inboxAuditInput) string { return `{"schema":"evolve.diagnosis/1","proposals":[]}` }}
	var stdout, stderr bytes.Buffer

	rc := runInboxAuditWith(f.env(curatorAgent(t, b)), f.root, nil, &stdout, &stderr)

	if rc != cliExitFailed || !strings.Contains(stderr.String(), `(run "agent-inbox-curator-`) {
		t.Fatalf("rc=%d: the failure must name the run label of the seat\n%s", rc, stderr.String())
	}
}

func TestCmd_InboxAudit_ARecordThatCannotBeWrittenIsAnIOFailure(t *testing.T) {
	f := newAuditFixture(t, auditPolicy)
	f.write(t, ".evolve/inbox-audit", "a file\n")
	var stdout, stderr bytes.Buffer

	rc := runInboxAuditWith(f.env(curatorAgent(t, &curatorBridge{})), f.root, nil, &stdout, &stderr)

	if rc != 2 || !strings.Contains(stderr.String(), "inbox audit: store the proposals") {
		t.Fatalf("rc=%d\n%s", rc, stderr.String())
	}
}

func TestCmd_InboxAudit_AJSONEncodeFailureIsAnIOFailure(t *testing.T) {
	f := newAuditFixture(t, strings.Replace(auditPolicy, `"trigger_count": 2`, `"trigger_count": 9`, 1))
	var stderr bytes.Buffer

	rc := runInboxAuditWith(f.env(curatorAgent(t, &curatorBridge{})), f.root, []string{"--json"}, &failingWriter{}, &stderr)

	if rc != cliExitIO || !strings.Contains(stderr.String(), "evolve inbox audit: encode the JSON result") {
		t.Fatalf("rc=%d\n%s", rc, stderr.String())
	}
}

func TestInboxVerbs_ListAudit(t *testing.T) {
	t.Parallel()
	if got := inboxUsage("audit"); got != "usage: evolve inbox audit [--force] [--json] [--project-root P]" {
		t.Errorf("inboxUsage(audit) = %q", got)
	}
}

func badPolicyRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	contractUnitWrite(t, root, ".evolve/policy.json", "{")
	return root
}

func inboxFileRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	contractUnitWrite(t, root, ".evolve/inbox", "a file\n")
	return root
}

func noGitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	contractUnitWrite(t, root, ".evolve/policy.json", auditPolicy)
	for _, id := range []string{"old-item", "older-item"} {
		contractUnitWrite(t, root, ".evolve/inbox/"+id+".json", auditItem(id, "console", "2026-01-01T00:00:00Z", "it exists", "go/x.go"))
	}
	return root
}

func TestCmd_InboxAudit_TheInboxVerbRunsTheAuditWithTheDefaultEnvironment(t *testing.T) {
	t.Parallel()
	f := newAuditFixture(t, strings.Replace(auditPolicy, `"trigger_count": 2`, `"trigger_count": 9`, 1))
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"audit", "--json", "--project-root", f.root}, nil, &stdout, &stderr)

	var r inboxAuditReport
	if rc != 0 || json.Unmarshal(stdout.Bytes(), &r) != nil || r.Ran || r.Count != 2 {
		t.Fatalf("rc=%d report=%+v\n%s", rc, r, stderr.String())
	}
	env := defaultInboxAuditEnv()
	if env.agent == nil || env.now == nil {
		t.Fatal("the default environment must run the agent through evolve agent run and read the clock")
	}
	center, closeCenter := env.newCenter(f.root, filepath.Join(f.root, ".evolve"), io.Discard)
	defer closeCenter()
	if center == nil {
		t.Fatal("the default environment must open the inbox signal center of the root")
	}
}
