package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	stopJourneyKey = baseattr.Key{Package: "example.com/m/dup", Test: "TestDup"}
	stopJourneyNow = time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC)
)

type stopEvent struct {
	root  string
	event signalcenter.Event
}

type stopJourney struct {
	*trainJourney
	t       *testing.T
	root    string
	red     []baseattr.Key
	runErr  error
	onRun   func()
	runs    []string
	events  []stopEvent
	fixItem string
}

func newStopJourney(t *testing.T, stage string) *stopJourney {
	t.Helper()
	s := &stopJourney{trainJourney: newTrainJourney(t), t: t, root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(s.evolveDir(), "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.write("policy.json", `{"gates":{"main_red_stop":"`+stage+`"}}`)
	s.env.Now = stopJourneyNow
	s.env.Emit = func(root string, console io.Writer, e signalcenter.Event) {
		fmt.Fprintf(console, "signal %s\n", e.Code)
		s.events = append(s.events, stopEvent{root: root, event: e})
	}
	s.env.KeyRun = func(root string, warn io.Writer) selfheal.KeyRun {
		return func(_ context.Context, rev string, reds baseattr.Reds) ([]baseattr.Key, error) {
			fmt.Fprintf(warn, "key run %s\n", rev)
			s.runs = append(s.runs, root+" "+rev+" "+baseattr.Names(reds.Keys)+" "+strings.Join(reds.Flags, " "))
			if s.onRun != nil {
				s.onRun()
			}
			return s.red, s.runErr
		}
	}
	return s
}

func (s *stopJourney) evolveDir() string { return paths.EvolveDirOf(s.root) }

func (s *stopJourney) write(rel, body string) {
	s.t.Helper()
	if err := os.WriteFile(filepath.Join(s.evolveDir(), rel), []byte(body), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

func (s *stopJourney) holdStop() {
	s.t.Helper()
	main := s.originMain()
	s.fixItem = mainstop.FixItemID(main)
	_, err := mainstop.Update(s.evolveDir(), func(stop mainstop.Stop, _ bool) (mainstop.Stop, error) {
		return stop.Opened(baseattr.Verdict{MainRed: []baseattr.Key{stopJourneyKey}, Main: main}, s.fixItem, stopJourneyNow.Add(-time.Hour)), nil
	})
	if err != nil {
		s.t.Fatal(err)
	}
	s.write(filepath.Join("inbox", s.fixItem+".json"), `{"id":"`+s.fixItem+`"}`)
}

func (s *stopJourney) docsPR(prs ...int) {
	s.t.Helper()
	for _, n := range prs {
		s.pr(s.t, n, "docs: change "+string(rune('a'+n)), string(rune('a'+n))+".md", "text\n")
	}
	s.recordDocs(s.t, prs...)
}

func (s *stopJourney) batch(prs ...string) (int, string, string) {
	return s.run(append([]string{"batch", "--project-root", s.root}, prs...)...)
}

func (s *stopJourney) stop() mainstop.Stop {
	s.t.Helper()
	stop, _, err := mainstop.Read(s.evolveDir())
	if err != nil {
		s.t.Fatal(err)
	}
	return stop
}

func (s *stopJourney) fixItemOpen() bool {
	_, err := os.Stat(filepath.Join(s.evolveDir(), "inbox", s.fixItem+".json"))
	return err == nil
}

func (s *stopJourney) codes() []signalcenter.Code {
	codes := make([]signalcenter.Code, len(s.events))
	for i, e := range s.events {
		codes[i] = e.event.Code
	}
	return codes
}

func TestStopJourney_EnforceHoldsARedBatchAndKeepsThePullRequest(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "enforce")
	s.docsPR(1)
	s.holdStop()
	before := s.originMain()
	s.gh.conclusion = "failure"

	rc, _, errb := s.batch("1")

	if rc != exitRefused || s.originMain() != before {
		t.Fatalf("rc=%d err=%q, want exit %d and a main that did not move", rc, errb, exitRefused)
	}
	for _, want := range []string{"main is red and a stop holds", "batch pull request 99", baseattr.Names([]baseattr.Key{stopJourneyKey}), "ship the fix", s.fixItem} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr %q lacks %q", errb, want)
		}
	}
	if len(s.gh.comments["1"]) != 0 || len(s.gh.closed) != 0 {
		t.Errorf("comments=%v closed=%v, want a pull request that the hold did not eject", s.gh.comments, s.gh.closed)
	}
	if !s.stop().Holds() || len(s.events) != 0 || len(s.runs) != 0 {
		t.Errorf("stop=%+v events=%v runs=%v, want a stop that holds, no event and no key run", s.stop(), s.events, s.runs)
	}
	s.gh.conclusion = "success"
	s.env.Wave = "j2"
	s.red = []baseattr.Key{stopJourneyKey}
	if rc, _, errb := s.batch("1"); rc != exitRefused || !strings.Contains(errb, "(not-a-fix)") {
		t.Errorf("second run: rc=%d err=%q, want the kept record to reach the key run", rc, errb)
	}
}

func TestStopJourney_ShadowSplitsTwoRedPullRequestsAndRecordsOneEvent(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "shadow")
	s.docsPR(1, 2)
	s.holdStop()
	before := s.originMain()
	s.gh.conclusion = "failure"

	rc, _, errb := s.batch("1", "2")

	if rc != exitRefused || s.originMain() != before || len(s.gh.comments["1"]) != 1 || len(s.gh.comments["2"]) != 1 {
		t.Fatalf("rc=%d err=%q comments=%v, want the split of today: each pull request briefed", rc, errb, s.gh.comments)
	}
	if strings.Contains(errb, "a stop holds") {
		t.Errorf("stderr %q: shadow must not hold the batch", errb)
	}
	if len(s.events) != 1 {
		t.Fatalf("%d events, want one would-refuse: %+v", len(s.events), s.events)
	}
	e := s.events[0]
	if e.root != s.root || e.event.Code != core.CodeMainRedStopWouldRefuse || e.event.Origin != batchEventOrigin || !strings.HasPrefix(e.event.Fields["landing"], "batch pull request ") {
		t.Errorf("event = %+v, want one would-refuse of the batch in the project root", e)
	}
}

func TestStopJourney_AFixBatchLandsReleasesTheStopAndClosesTheFixItem(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "enforce")
	s.docsPR(1)
	s.holdStop()
	before := s.originMain()

	rc, _, errb := s.batch("1")

	tip := s.originMain()
	if rc != 0 || tip == before {
		t.Fatalf("rc=%d err=%q, want a landing that moves main", rc, errb)
	}
	wantRun := s.root + " " + tip + " " + baseattr.Names([]baseattr.Key{stopJourneyKey}) + " " + strings.Join(core.StopProofReds(nil).Flags, " ")
	if !slices.Equal(s.runs, []string{wantRun}) {
		t.Errorf("key runs = %v, want %q", s.runs, wantRun)
	}
	stop := s.stop()
	if stop.Holds() || stop.ReleasedBy != "batch main "+tip || stop.ReleasedAt == nil || !stop.ReleasedAt.Equal(stopJourneyNow) {
		t.Errorf("stop = %+v, want a release by the batch tip at the clock of the command", stop)
	}
	if s.fixItemOpen() || !inboxProcessed(t, filepath.Join(s.evolveDir(), "inbox"), s.fixItem) {
		t.Error("the fix item is not in processed")
	}
	if want := "key run " + tip + "\n[inbox-mover] promoted: " + s.fixItem + ".json → processed/\nsignal " + string(core.CodeMainRedStopReleased) + "\n"; errb != want {
		t.Errorf("stderr = %q, want the key run, the mover and the signal on the stderr of the command: %q", errb, want)
	}
	if len(s.events) != 1 || s.events[0].root != s.root || s.events[0].event.Code != core.CodeMainRedStopReleased || s.events[0].event.Origin != batchEventOrigin || s.events[0].event.Severity != signalcenter.SeverityInfo {
		t.Errorf("events = %+v, want one info release of the batch in the project root", s.events)
	}
}

func TestStopJourney_AGreenBatchThatFixesNoKeyIsHeldAtEnforceAndLandsAtShadow(t *testing.T) {
	t.Parallel()
	for stage, held := range map[string]bool{"enforce": true, "shadow": false} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			s := newStopJourney(t, stage)
			s.docsPR(1)
			s.holdStop()
			before := s.originMain()
			s.red = []baseattr.Key{stopJourneyKey}

			rc, _, errb := s.batch("1")

			if (rc == exitRefused) != held || (rc == 0) == held || (s.originMain() == before) != held {
				t.Fatalf("rc=%d err=%q moved=%v, want held=%v", rc, errb, s.originMain() != before, held)
			}
			if !s.stop().Holds() || !s.fixItemOpen() {
				t.Errorf("stop=%+v open=%v, want a stop that holds and a fix item in the inbox", s.stop(), s.fixItemOpen())
			}
			if held && (!strings.Contains(errb, "(not-a-fix)") || !strings.Contains(errb, "is held") || len(s.events) != 0) {
				t.Errorf("stderr %q events %v, want the hold and no event", errb, s.events)
			}
			if !held && !slices.Equal(s.codes(), []signalcenter.Code{core.CodeMainRedStopWouldRefuse}) {
				t.Errorf("codes = %v, want one would-refuse", s.codes())
			}
		})
	}
}

func TestStopJourney_AStopRecordThatDoesNotParseHoldsNoBatch(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "enforce")
	s.docsPR(1)
	s.write(filepath.Base(mainstop.Path(s.evolveDir())), "{not json")
	s.gh.conclusion = "failure"

	rc, _, errb := s.batch("1")

	if rc != exitRefused || len(s.gh.comments["1"]) != 1 || strings.Contains(errb, "a stop holds") {
		t.Fatalf("rc=%d err=%q comments=%v, want the split of today", rc, errb, s.gh.comments)
	}
	if !strings.Contains(errb, "WARN main-red stop: the record could not be read") {
		t.Errorf("stderr %q lacks the warning of the read", errb)
	}
}

func TestStopJourney_AReleaseThatFailsStillLandsTheFixBatchAndWarns(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "enforce")
	s.docsPR(1)
	s.holdStop()
	before := s.originMain()
	s.onRun = func() {
		if err := os.Chmod(s.evolveDir(), 0o555); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { _ = os.Chmod(s.evolveDir(), 0o755) })

	rc, _, errb := s.batch("1")

	if rc != 0 || s.originMain() == before {
		t.Fatalf("rc=%d err=%q, want a landing", rc, errb)
	}
	if !strings.Contains(errb, "[integration] WARN main-red stop release: mainstop:") || !s.stop().Holds() || len(s.events) != 0 {
		t.Errorf("stderr %q stop %+v events %v, want the release warning and a stop that holds", errb, s.stop(), s.events)
	}
}

func TestStopJourney_AnAbsentProjectRootStopsTheBatchBeforeAnyLanding(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "enforce")
	s.docsPR(1)
	before := s.originMain()
	creates := s.gh.creates
	absent := filepath.Join(t.TempDir(), "nowhere")

	rc, out, errb := s.run("batch", "--project-root", absent, "1")

	if rc != exitIO || out != "" || s.originMain() != before || s.gh.creates != creates {
		t.Fatalf("rc=%d out=%q creates=%d, want exit %d and no landing", rc, out, s.gh.creates, exitIO)
	}
	for _, want := range []string{"integration batch", absent, "pass --project-root"} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr %q lacks %q", errb, want)
		}
	}
}

func TestStopJourney_ALinkedWorktreeReadsTheStopOfTheProjectRoot(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		flag, variable bool
		held           bool
	}{
		"the flag":                      {flag: true, held: true},
		"the variable":                  {variable: true, held: true},
		"the working directory as root": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newStopJourney(t, "enforce")
			linked := filepath.Join(t.TempDir(), "linked")
			s.work.Git("worktree", "add", "-q", "--detach", linked, "main")
			s.env.Root = linked
			s.docsPR(1)
			s.holdStop()
			before := s.originMain()
			s.red = []baseattr.Key{stopJourneyKey}
			args := []string{"batch", "1"}
			if tc.flag {
				args = []string{"batch", "--project-root", s.root, "1"}
			}
			if tc.variable {
				s.env.ProjectRootEnv = s.root
			}

			rc, _, errb := s.run(args...)

			if (rc == exitRefused) != tc.held || (rc == 0) == tc.held || (s.originMain() == before) != tc.held {
				t.Errorf("rc=%d err=%q moved=%v, want held=%v", rc, errb, s.originMain() != before, tc.held)
			}
		})
	}
}

func TestStopJourney_TheTrainOfTheCommandPutsTheStopGateOnMainAndOnCI(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "enforce")
	s.holdStop()
	s.red = []baseattr.Key{stopJourneyKey}
	s.gh.conclusion = "failure"
	var warn bytes.Buffer
	train := s.env.train(s.root, &warn)
	tip := s.originMain()

	ffErr := train.Main.FastForward(context.Background(), tip)
	verdict, watchErr := train.CI.Watch(context.Background(), integration.PR{Number: 7, Tip: tip})

	if !errors.Is(ffErr, selfheal.ErrMainRedHold) || !strings.Contains(ffErr.Error(), "batch "+tip) {
		t.Errorf("FastForward = %v, want the hold of the batch tip", ffErr)
	}
	if verdict != integration.Red || !errors.Is(watchErr, selfheal.ErrMainRedHold) || !strings.Contains(watchErr.Error(), "batch pull request 7") {
		t.Errorf("Watch = %v, %v, want a red verdict and the hold of pull request 7", verdict, watchErr)
	}
}

func TestIntegrationTrainIsBuiltOnlyBehindTheStopGate(t *testing.T) {
	t.Parallel()
	literal := regexp.MustCompile(`integration\.Train\{`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var holders []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for range literal.FindAll(src, -1) {
			holders = append(holders, name)
		}
	}
	if !slices.Equal(holders, []string{"cmd_integration_batch.go"}) {
		t.Fatalf("integration.Train literals in %v, want one, in integrationEnv.train", holders)
	}
	src, err := os.ReadFile("cmd_integration_batch.go")
	if err != nil {
		t.Fatal(err)
	}
	body := regexp.MustCompile(`(?s)func \(e integrationEnv\) train\(.*?\n}\n`).Find(src)
	for _, want := range []string{"integration.Train{", "gate := e.stopGate(projectRoot, warn)", "gate.CI(p.CI)", "gate.Main(p.Main)"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("integrationEnv.train lacks %q:\n%s", want, body)
		}
	}
}

func TestAttributedKeyRun_RunsTheKeysInTheTreeOfTheRevision(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("state.txt", "first\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "first")
	first := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Write("state.txt", "second\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "second")
	var seen string
	var got baseattr.Reds
	runner := func(_ context.Context, moduleDir string, reds baseattr.Reds) ([]baseattr.Key, error) {
		raw, err := os.ReadFile(filepath.Join(moduleDir, "state.txt"))
		seen, got = string(raw), reds
		return []baseattr.Key{stopJourneyKey}, err
	}
	want := core.StopProofReds([]baseattr.Key{stopJourneyKey})

	red, err := attributedKeyRun(runner)(repo.Dir, io.Discard)(context.Background(), first, want)

	if err != nil || !slices.Equal(red, []baseattr.Key{stopJourneyKey}) {
		t.Fatalf("red = %v, err = %v, want the keys of the runner", red, err)
	}
	if seen != "first\n" || !slices.Equal(got.Keys, want.Keys) || !slices.Equal(got.Flags, want.Flags) || len(got.Flags) == 0 {
		t.Errorf("tree state %q reds %+v, want the tree of %s and the flags of the stop proof", seen, got, first)
	}
}

func TestAttributedKeyRun_ReadsTheRerunBudgetOfTheProjectRoot(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("state.txt", "first\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "first")
	if err := os.MkdirAll(paths.EvolveDirOf(repo.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.EvolveDirOf(repo.Dir), "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	runner := func(context.Context, string, baseattr.Reds) ([]baseattr.Key, error) { return nil, nil }

	if _, err := attributedKeyRun(runner)(repo.Dir, &warn)(context.Background(), "HEAD", baseattr.Reds{}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(warn.String(), "policy.json") {
		t.Errorf("warning = %q, want the policy of the project root to be read with the writer of the caller", warn.String())
	}
}

func TestEmitIntegrationSignal_SendsTheEventThroughACenterOfTheIntegrationRole(t *testing.T) {
	var mu sync.Mutex
	var events []signalcenter.Event
	var roles, dirs []string
	closed := 0
	prev := attachRootEventsFn
	t.Cleanup(func() { attachRootEventsFn = prev })
	attachRootEventsFn = func(c *signalcenter.Center, role, evolveDir string, _ io.Writer) func() {
		roles, dirs = append(roles, role), append(dirs, evolveDir)
		c.Subscribe(func(e signalcenter.Event) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, e)
		})
		return func() {
			c.Flush()
			closed++
		}
	}
	root := t.TempDir()

	emitIntegrationSignal(root, io.Discard, signalcenter.Event{
		Module: signalcenter.ModuleGateRepo, Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn,
		Code: core.CodeMainRedStopWouldRefuse, Origin: batchEventOrigin, Reason: "a batch",
	})

	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(roles, []string{"integration"}) || !slices.Equal(dirs, []string{paths.EvolveDirOf(root)}) || closed != 1 {
		t.Fatalf("roles=%v dirs=%v closed=%d, want one center of the integration role on the project root, closed one time", roles, dirs, closed)
	}
	if len(events) != 1 || events[0].Code != core.CodeMainRedStopWouldRefuse || events[0].Origin != "evolve.integrationBatch" {
		t.Errorf("events = %+v, want the one event of the batch", events)
	}
}

func TestBatchUsage_NamesTheProjectRootFlagAndTheHoldExit(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if rc := runIntegrationEnv(integrationEnv{Root: t.TempDir()}, []string{"batch", "--help"}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "batch [--dry-run] [--project-root P] [PR...]") {
		t.Errorf("rc=%d out=%q", rc, out.String())
	}
	for _, want := range []string{"evolve integration batch [--dry-run] [--project-root P] [PR...]", "a main-red stop holds the batch", "2 = git or gh failed or an absent root"} {
		if !strings.Contains(integrationUsage, want) {
			t.Errorf("family usage lacks %q", want)
		}
	}
}

func TestIntegrationProductionEnv_TheKeyRunRunsGoTestOnTheRevision(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	key := baseattr.Key{Package: "example.com/fx/p", Test: "TestState"}
	commit := func(body string) string {
		repo.Write("go/go.mod", "module example.com/fx\n\ngo 1.23\n")
		repo.Write("go/p/p_test.go", "package p\n\nimport \"testing\"\n\nfunc TestState(t *testing.T) {"+body+"}\n")
		repo.Git("add", ".")
		repo.Git("commit", "-q", "-m", "state")
		return strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	}
	red := commit(" t.Fatal(\"red\") ")
	green := commit("")
	run := productionIntegrationEnv(repo.Dir, stopJourneyNow).KeyRun(repo.Dir, io.Discard)

	onRed, redErr := run(context.Background(), red, baseattr.Reds{Keys: []baseattr.Key{key}})
	onGreen, greenErr := run(context.Background(), green, baseattr.Reds{Keys: []baseattr.Key{key}})

	if redErr != nil || greenErr != nil || !slices.Equal(onRed, []baseattr.Key{key}) || len(onGreen) != 0 {
		t.Errorf("red commit: %v, %v; green commit: %v, %v; want the key red on the first commit and green on the second", onRed, redErr, onGreen, greenErr)
	}
}
