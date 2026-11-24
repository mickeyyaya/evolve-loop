//go:build acs

package cycle2627

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	closeoutCycle = 41
	dossierJSON   = "knowledge-base/cycles/cycle-41.json"
	dossierMD     = "knowledge-base/cycles/cycle-41.md"
	dossierCommit = "dossier: cycle-41 closeout"
	keptCommit    = "chore(inbox): the loop's inbox stamps from the plane"
	stampPath     = ".evolve/inbox/item.json"
)

type closeout struct {
	repo  *gittest.Repo
	batch *core.BookkeepingBatch
}

type bookkeepingSetup struct {
	enabled bool
	stage   string
}

func runCloseout(t *testing.T, setup bookkeepingSetup) closeout {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".gitignore", ".evolve/\n")
	repo.Git("add", ".gitignore")
	repo.Git("commit", "-q", "-m", "base")
	batch := core.NewBookkeepingBatch()
	var opts []core.Option
	if setup.enabled {
		opts = append(opts, core.WithBookkeeping(setup.stage, batch))
	}
	storage := &fixtures.FakeStorage{State: core.State{LastCycleNumber: closeoutCycle - 1}}
	orchestrator := core.NewOrchestrator(storage, &fixtures.FakeLedger{}, fixtures.BuildRunners(nil), opts...)
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "runs", "cycle-"+strconv.Itoa(closeoutCycle)), 0o755); err != nil {
		t.Fatal(err)
	}
	req := core.CycleRequest{ProjectRoot: repo.Dir, GoalHash: "g", Env: map[string]string{}, DisableWorkspaceGuard: true}
	if _, err := orchestrator.RunCycle(context.Background(), req); err != nil {
		t.Logf("the stubbed cycle ended with %v; the closeout dossier is written either way", err)
	}
	return closeout{repo: repo, batch: batch}
}

func (c closeout) commitCount() int {
	n, err := strconv.Atoi(strings.TrimSpace(c.repo.Git("rev-list", "--count", "HEAD")))
	if err != nil {
		panic(err)
	}
	return n
}

func (c closeout) read(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(c.repo.Dir, rel))
	if err != nil {
		t.Fatalf("the closeout left no %s: %v", rel, err)
	}
	return string(body)
}

func TestC2627_001_AtEnforceTheDossierCloseoutGoesToTheBatchNotMain(t *testing.T) {
	c := runCloseout(t, bookkeepingSetup{enabled: true, stage: policy.FleetIntegrationEnforce})
	if got := c.repo.Git("log", "--format=%s", "--", "knowledge-base/cycles"); strings.TrimSpace(got) != "" {
		t.Errorf("main holds a dossier commit at enforce: %q", got)
	}
	if got := c.commitCount(); got != 1 {
		t.Errorf("commits on main = %d, want 1 (only the base): the closeout must not commit at enforce", got)
	}
	for _, rel := range []string{dossierJSON, dossierMD} {
		c.read(t, rel)
	}
	entries := c.batch.Drain()
	if len(entries) != 1 {
		t.Fatalf("batch entries = %d, want exactly 1 dossier closeout: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.Kind != core.BookkeepingDossier || e.Message != dossierCommit || strings.Join(e.Paths, ",") != dossierJSON+","+dossierMD {
		t.Errorf("entry = %+v, want kind %s, message %q, paths %s and %s", e, core.BookkeepingDossier, dossierCommit, dossierJSON, dossierMD)
	}
}

func TestC2627_001b_WithNoBookkeepingBatchTheCloseoutCommitsToMainAsToday(t *testing.T) {
	c := runCloseout(t, bookkeepingSetup{})
	if got := strings.TrimSpace(c.repo.Git("log", "-1", "--format=%s")); got != dossierCommit {
		t.Errorf("HEAD subject = %q, want %q: without a bookkeeping batch the closeout must commit as today", got, dossierCommit)
	}
	if c.batch.Len() != 0 {
		t.Errorf("batch entries = %d, want 0", c.batch.Len())
	}
}

func dirtyInboxStamp(t *testing.T) (*gittest.Repo, inboxstamps.Plan) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(stampPath, `{"id":"item","status":"open"}`+"\n")
	repo.Git("add", stampPath)
	repo.Git("commit", "-q", "-m", "seed the inbox item")
	repo.Write(stampPath, `{"id":"item","status":"open","claimed_by":"cycle-41"}`+"\n")
	stamp := inboxstamps.Stamp{Path: stampPath, Set: map[string]json.RawMessage{"claimed_by": json.RawMessage(`"cycle-41"`)}}
	return repo, inboxstamps.Plan{Kept: []inboxstamps.Stamp{stamp}}
}

func TestC2627_002_AtEnforceAnInboxStampGoesToTheBatchNotMain(t *testing.T) {
	repo, plan := dirtyInboxStamp(t)
	before := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	batch := core.NewBookkeepingBatch()
	if err := plan.HoldKept(batch); err != nil {
		t.Fatalf("HoldKept: %v", err)
	}
	if after := strings.TrimSpace(repo.Git("rev-parse", "HEAD")); after != before {
		t.Errorf("HEAD moved %s -> %s: a held stamp must not commit to main", before, after)
	}
	entries := batch.Drain()
	if len(entries) != 1 || entries[0].Kind != core.BookkeepingInboxStamp || strings.Join(entries[0].Paths, ",") != stampPath || entries[0].Message != keptCommit {
		t.Fatalf("entries = %+v, want one %s entry for %s with message %q", entries, core.BookkeepingInboxStamp, stampPath, keptCommit)
	}
}

type refusingSink struct{}

func (refusingSink) Add(core.BookkeepingEntry) error { return errors.New("sink down") }

func TestC2627_002b_AHeldStampThatTheBatchRefusesFailsLoudAndAnEmptyPlanAddsNothing(t *testing.T) {
	_, plan := dirtyInboxStamp(t)
	if err := plan.HoldKept(refusingSink{}); err == nil {
		t.Error("HoldKept swallowed a refusing sink")
	}
	batch := core.NewBookkeepingBatch()
	if err := (inboxstamps.Plan{}).HoldKept(batch); err != nil || batch.Len() != 0 {
		t.Errorf("an empty plan: err=%v, batch entries=%d, want nil and 0", err, batch.Len())
	}
}

func TestC2627_003_TheBookkeepingBatchSkipsL2AndACodeBatchDoesNot(t *testing.T) {
	if !integration.SkipsL2(false) {
		t.Error("a batch that holds no code must skip L2")
	}
	if integration.SkipsL2(true) {
		t.Error("a batch that holds code must go through L2")
	}
}

func TestC2627_004_TheBookkeepingBatchFlushesAloneAtEachCodeFlushAndAtWaveEnd(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	flushPolicy := integration.DefaultFlushPolicy()
	deliverable := func(id string, risky bool) integration.Deliverable {
		return integration.Deliverable{ID: id, DeliveredAt: now, Risky: risky}
	}
	capReached := make([]integration.Deliverable, flushPolicy.Cap)
	for i := range capReached {
		capReached[i] = deliverable("d"+strconv.Itoa(i), false)
	}
	cases := []struct {
		name  string
		batch []integration.Deliverable
		wave  bool
		want  integration.FlushTrigger
	}{
		{"wave end", []integration.Deliverable{deliverable("a", false)}, true, integration.TriggerWaveEnd},
		{"cap reached", capReached, false, integration.TriggerCapReached},
		{"risky alone", []integration.Deliverable{deliverable("a", false), deliverable("b", true)}, false, integration.TriggerRiskyAlone},
	}
	for _, c := range cases {
		trigger := integration.DecideFlush(c.batch, c.wave, flushPolicy, now).Trigger
		if trigger != c.want {
			t.Fatalf("%s: code batch trigger = %q, want %q", c.name, trigger, c.want)
		}
		if !integration.FlushDue(2, trigger) {
			t.Errorf("%s: a pending bookkeeping batch is not due at this code flush", c.name)
		}
	}
	held := integration.DecideFlush([]integration.Deliverable{deliverable("a", false)}, false, flushPolicy, now)
	if held.Trigger != integration.TriggerNone || integration.FlushDue(2, held.Trigger) {
		t.Errorf("a held code batch (trigger %q) must not flush the bookkeeping batch", held.Trigger)
	}
	if integration.FlushDue(0, integration.TriggerWaveEnd) {
		t.Error("an empty bookkeeping batch must never be due")
	}
}

type recordingFlusher struct {
	calls   int
	flushed []core.BookkeepingEntry
	err     error
}

func (f *recordingFlusher) Flush(entries []core.BookkeepingEntry) error {
	f.calls++
	f.flushed = append(f.flushed, entries...)
	return f.err
}

func TestC2627_004b_AFlushHandsOverOnlyBookkeepingEntriesOnceAndKeepsThemWhenItFails(t *testing.T) {
	batch := core.NewBookkeepingBatch()
	dossier := core.BookkeepingEntry{Kind: core.BookkeepingDossier, Paths: []string{dossierJSON, dossierMD}, Message: dossierCommit}
	stamp := core.BookkeepingEntry{Kind: core.BookkeepingInboxStamp, Paths: []string{stampPath}, Message: keptCommit}
	for _, e := range []core.BookkeepingEntry{dossier, stamp} {
		if err := batch.Add(e); err != nil {
			t.Fatal(err)
		}
	}
	failing := &recordingFlusher{err: errors.New("push refused")}
	if err := batch.FlushTo(failing); err == nil {
		t.Fatal("FlushTo swallowed a failing flusher")
	}
	if batch.Len() != 2 {
		t.Fatalf("batch entries after a failed flush = %d, want 2 kept for the next flush", batch.Len())
	}
	flusher := &recordingFlusher{}
	if err := batch.FlushTo(flusher); err != nil {
		t.Fatal(err)
	}
	if flusher.calls != 1 || len(flusher.flushed) != 2 || batch.Len() != 0 {
		t.Fatalf("calls=%d flushed=%d left=%d, want 1 call, 2 entries, 0 left", flusher.calls, len(flusher.flushed), batch.Len())
	}
	for _, e := range flusher.flushed {
		if e.Kind != core.BookkeepingDossier && e.Kind != core.BookkeepingInboxStamp {
			t.Errorf("the flush carries a %q entry: it must carry bookkeeping only", e.Kind)
		}
		for _, p := range e.Paths {
			if strings.HasSuffix(p, ".go") {
				t.Errorf("the flush carries code path %s", p)
			}
		}
	}
	if err := batch.FlushTo(flusher); err != nil || flusher.calls != 1 {
		t.Errorf("an empty batch reached the flusher: err=%v calls=%d", err, flusher.calls)
	}
}

var volatileFields = []*regexp.Regexp{
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}T[\d:.]+Z`),
	regexp.MustCompile(`\b[0-9A-Z]{26}\b`),
	regexp.MustCompile(`\|[0-9a-f]{12}\b`),
}

func withoutVolatileFields(body string) string {
	for _, re := range volatileFields {
		body = re.ReplaceAllString(body, "~")
	}
	return body
}

func TestC2627_005_AtShadowTheCloseoutCommitIsByteIdenticalToToday(t *testing.T) {
	today := runCloseout(t, bookkeepingSetup{})
	shadow := runCloseout(t, bookkeepingSetup{enabled: true, stage: policy.FleetIntegrationShadow})
	if shadow.batch.Len() != 0 {
		t.Errorf("shadow fed the batch %d entries, want 0", shadow.batch.Len())
	}
	if shadow.commitCount() != today.commitCount() {
		t.Errorf("commits: shadow %d, today %d", shadow.commitCount(), today.commitCount())
	}
	for _, format := range []string{"--format=%s", "--format=%B"} {
		if s, d := shadow.repo.Git("log", "-1", format), today.repo.Git("log", "-1", format); s != d {
			t.Errorf("HEAD %s differs: shadow %q, today %q", format, s, d)
		}
	}
	if s, d := shadow.repo.Git("show", "-1", "--name-status", "--format="), today.repo.Git("show", "-1", "--name-status", "--format="); s != d || !strings.Contains(s, dossierJSON) || !strings.Contains(s, dossierMD) {
		t.Errorf("committed files differ or miss the pair:\nshadow:\n%s\ntoday:\n%s", s, d)
	}
	for _, rel := range []string{dossierJSON, dossierMD} {
		s, d := withoutVolatileFields(shadow.read(t, rel)), withoutVolatileFields(today.read(t, rel))
		if s != d {
			t.Errorf("%s differs at shadow beyond the run id, the timestamps and the fingerprint hash:\n--- shadow ---\n%s\n--- today ---\n%s", rel, s, d)
		}
		if !strings.Contains(s, "FAIL") {
			t.Errorf("%s has no verdict: the comparison saw an empty dossier", rel)
		}
	}
}

func TestC2627_006_AtShadowAnInboxStampStillCommitsToMainAsToday(t *testing.T) {
	repo, plan := dirtyInboxStamp(t)
	if err := plan.CommitKept(context.Background(), gitexec.Default(repo.Dir)); err != nil {
		t.Fatalf("CommitKept: %v", err)
	}
	if got := strings.TrimSpace(repo.Git("log", "-1", "--format=%s")); got != keptCommit {
		t.Errorf("HEAD subject = %q, want %q", got, keptCommit)
	}
	if got := strings.TrimSpace(repo.Git("show", "-1", "--name-only", "--format=")); got != stampPath {
		t.Errorf("the stamp commit holds %q, want only %s", got, stampPath)
	}
	if status := strings.TrimSpace(repo.Git("status", "--porcelain", "--", stampPath)); status != "" {
		t.Errorf("the stamp is still dirty after its commit: %q", status)
	}
}
