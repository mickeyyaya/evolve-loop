package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	fixtureDir        = "../../attemptpostmortem/testdata/"
	session1853First  = "c61dea20-648d-4002-a794-85f794e80ba0"
	cycle1853Composed = "BODY OF THE BUILDER" + cycleContextBoundary + "- cycle: 1853\n"
)

type launchStep struct {
	exit       int
	cause      string
	transcript string
	pane       string
	exhausted  bool
	endAt      string
}

type stepBridge struct {
	t       *testing.T
	home    string
	clock   *time.Time
	steps   []launchStep
	prompts []string
	clis    []string
}

func (s *stepBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	i := len(s.prompts)
	s.prompts, s.clis = append(s.prompts, req.Prompt), append(s.clis, req.CLI)
	step := s.steps[i]
	if step.transcript != "" {
		s.writeTranscript(req, step.transcript)
	}
	s.writePane(req.Workspace, step)
	if step.endAt != "" {
		*s.clock = mustStamp(s.t, step.endAt)
	}
	if step.exit != 0 {
		return core.BridgeResponse{ExitCode: step.exit, CauseCode: step.cause, UsageExhausted: step.exhausted}, errors.New("bridge: launch exit=81: artifact-timeout: cause=pane_lost")
	}
	if err := os.WriteFile(req.ArtifactPath, []byte("# Build Report\n"), 0o644); err != nil {
		s.t.Fatal(err)
	}
	return core.BridgeResponse{Stdout: "ok"}, nil
}

func (s *stepBridge) Probe(context.Context) (core.BridgeProbe, error) { return core.BridgeProbe{}, nil }

func (s *stepBridge) writeTranscript(req core.BridgeRequest, fixture string) {
	body, err := os.ReadFile(fixtureDir + fixture)
	if err != nil {
		s.t.Fatal(err)
	}
	head := `{"type":"user","timestamp":"2026-10-09T09:19:29Z","message":{"content":"Artifact path: ` + req.ArtifactPath + `"}}` + "\n"
	dir := filepath.Join(s.home, ".claude", "projects", "-wt-cycle-1853")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, session1853First+".jsonl"), append([]byte(head), body...), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

func (s *stepBridge) writePane(workspace string, step launchStep) {
	path := filepath.Join(workspace, "tmux-final-scrollback.txt")
	if step.pane == "" {
		return
	}
	if err := os.WriteFile(path, []byte(step.pane), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

func mustStamp(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

type postmortemRun struct {
	events   []string
	bridge   *stepBridge
	ws       string
	warnings []signalcenter.Event
	gitCalls [][]string
	resp     core.PhaseResponse
}

func runWithSteps(t *testing.T, req core.PhaseRequest, git func(args []string, stdout io.Writer) int, steps ...launchStep) postmortemRun {
	t.Helper()
	clock := mustStamp(t, "2026-10-09T09:19:20.147Z")
	run := postmortemRun{ws: req.Workspace}
	run.bridge = &stepBridge{t: t, home: req.Env["HOME"], clock: &clock, steps: steps}
	center := signalcenter.New()
	center.Subscribe(func(e signalcenter.Event) {
		if e.Code == CodeAttemptPostmortemFailed {
			run.warnings = append(run.warnings, e)
		}
	})
	if req.ProjectRoot == "" {
		req.ProjectRoot = writeFallbackProfile(t, "evolve-builder", "claude-tmux", []string{"agy-claude-tmux"})
	}
	r := New(Options{
		Hooks:        &fakeHooks{phase: "build", agent: "evolve-builder", model: "deep", prompt: cycle1853Composed, verdict: core.VerdictPASS},
		Bridge:       run.bridge,
		Prompts:      fakePromptsFS("evolve-builder", "x"),
		AttemptClock: func() time.Time { return clock },
		EventsProducer: func(_, _, _ string, _ int, prompt string) error {
			run.events = append(run.events, prompt)
			return nil
		},
		Signals: func() *signalcenter.Center { return center },
		SleepFn: func(time.Duration) {},
		VerifyFn: func(phase string, _ phasecontract.Roots) (deliverable.Result, error) {
			return deliverable.Result{OK: true, Phase: phase, Content: "# Build Report\n"}, nil
		},
		GitExec: func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
			run.gitCalls = append(run.gitCalls, append([]string{name}, args...))
			return git(args, stdout), nil
		},
	})
	run.resp, _ = r.Run(context.Background(), req)
	return run
}

func cycle1853Request(t *testing.T) core.PhaseRequest {
	t.Helper()
	return core.PhaseRequest{Cycle: 1853, Workspace: t.TempDir(), Env: map[string]string{"HOME": t.TempDir()}}
}

func noGit(t *testing.T) func([]string, io.Writer) int {
	return func(args []string, _ io.Writer) int {
		t.Errorf("git %v ran with no worktree", args)
		return 0
	}
}

func paneLostAttempt1() launchStep {
	return launchStep{exit: 81, cause: "pane_lost", transcript: "cycle1853-build-attempt1.jsonl", pane: "no server running on /private/tmp/tmux-501/evolve-bridge-p21421\n", endAt: "2026-10-09T09:59:52.115Z"}
}

func cleanAttempt() launchStep { return launchStep{endAt: "2026-10-09T10:05:00Z"} }

func TestDispatch_APaneLostAttemptGivesTheRetryThePostmortemOfCycle1853(t *testing.T) {
	run := runWithSteps(t, cycle1853Request(t), noGit(t), paneLostAttempt1(), cleanAttempt())

	if len(run.bridge.prompts) != 2 || len(run.warnings) != 0 {
		t.Fatalf("launches = %v warnings = %+v, want 2 launches and no warning", run.bridge.clis, run.warnings)
	}
	first, second := run.bridge.prompts[0], run.bridge.prompts[1]
	if first != cycle1853Composed {
		t.Fatalf("the first prompt changed:\n%s", first)
	}
	for _, want := range []string{
		attemptpostmortem.SectionHeading,
		"### Attempt 1",
		"cause `pane_lost`, exit code 81",
		"session `" + session1853First + "`",
		"Suspect command (reason `end_window`",
		"TMUX_TMPDIR=$d tmux -f /dev/null new-session -d -s probe; echo rc=$?; TMUX_TMPDIR=$d tmux kill-server",
		"Final pane tail: none recorded.",
		"Do not run the suspect command again unchanged until you know why the earlier dispatch ended.",
	} {
		if !strings.Contains(second, want) {
			t.Errorf("the retry prompt lacks %q:\n%s", want, second)
		}
	}
	if !strings.HasPrefix(second, strings.TrimRight(first, "\n")) || StaticPrefix(second) != StaticPrefix(first) {
		t.Fatal("the section must follow the cycle context and keep the cache-stable prefix")
	}
	if strings.Index(second, attemptpostmortem.SectionHeading) < strings.Index(second, cycleContextBoundary) {
		t.Fatal("the section precedes the cycle context boundary")
	}
	if len(run.events) != 2 || run.events[1] != second {
		t.Fatal("the events producer must get the prompt that the bridge got, for its echo veto")
	}
	records, err := attemptpostmortem.ReadAll(run.ws, "build")
	if err != nil || len(records) != 1 {
		t.Fatalf("records = %+v, %v; want one", records, err)
	}
	rec := records[0]
	if rec.Number != 1 || rec.CLI != "claude-tmux" || rec.CauseCode != "pane_lost" || rec.CommandSource != attemptpostmortem.SourceTranscript || rec.Suspect == nil {
		t.Fatalf("record = %+v", rec)
	}
	if !rec.StartedAt.Equal(mustStamp(t, "2026-10-09T09:19:20.147Z")) || !rec.EndedAt.Equal(mustStamp(t, "2026-10-09T09:59:52.115Z")) {
		t.Fatalf("times = %s .. %s, want the dispatch window", rec.StartedAt, rec.EndedAt)
	}
}

func TestDispatch_ACleanEndWritesNoRecordAndAddsNoSection(t *testing.T) {
	run := runWithSteps(t, cycle1853Request(t), noGit(t), cleanAttempt())

	if len(run.bridge.prompts) != 1 || run.bridge.prompts[0] != cycle1853Composed {
		t.Fatalf("prompts = %q, want the composed prompt once", run.bridge.prompts)
	}
	if matches, _ := filepath.Glob(filepath.Join(run.ws, "*postmortem*")); len(matches) != 0 || len(run.warnings) != 0 {
		t.Fatalf("records = %v warnings = %+v, want none", matches, run.warnings)
	}
}

func TestRun_AResumeCarriesTheStoredRecordInItsFirstPrompt(t *testing.T) {
	req := cycle1853Request(t)
	stored := attemptpostmortem.Record{
		Schema: attemptpostmortem.SchemaVersion,
		Attempt: attemptpostmortem.Attempt{
			Phase: "build", Cycle: 1853, Number: 2, CLI: "claude-tmux", Session: "86c13298-bb96-466f-9c7e-60cdb0d6f387",
			StartedAt: mustStamp(t, "2026-10-09T10:00:40Z"), EndedAt: mustStamp(t, "2026-10-09T10:02:20Z"), CauseCode: "pane_lost", ExitCode: 81,
		},
		CommandSource: attemptpostmortem.SourceTranscript,
		Commands:      []attemptpostmortem.Command{{Text: "tmux kill-server", Status: attemptpostmortem.StatusSignal, ExitCode: 137}},
	}
	if err := attemptpostmortem.Write(req.Workspace, stored); err != nil {
		t.Fatal(err)
	}

	run := runWithSteps(t, req, noGit(t), cleanAttempt())

	if len(run.bridge.prompts) != 1 || !strings.Contains(run.bridge.prompts[0], "### Attempt 2") || !strings.Contains(run.bridge.prompts[0], "`86c13298-bb96-466f-9c7e-60cdb0d6f387`") {
		t.Fatalf("the resumed prompt lacks the stored record:\n%s", run.bridge.prompts)
	}
}

func TestDispatch_ACollectErrorIsAWarnAndTheRetryProceedsWithoutTheSection(t *testing.T) {
	req := cycle1853Request(t)
	req.Cycle = 0

	run := runWithSteps(t, req, noGit(t), paneLostAttempt1(), cleanAttempt())

	if len(run.bridge.prompts) != 2 || run.bridge.prompts[1] != cycle1853Composed {
		t.Fatalf("prompts = %q, want a retry with the plain prompt", run.bridge.prompts)
	}
	if run.resp.Verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want the retry to pass: %+v", run.resp.Verdict, run.resp.Diagnostics)
	}
	if len(run.warnings) != 1 || run.warnings[0].Severity != signalcenter.SeverityWarn || run.warnings[0].Fields["step"] != "collect" || !strings.Contains(run.warnings[0].Reason, "invalid record field cycle") || run.warnings[0].Phase != "build" {
		t.Fatalf("warnings = %+v, want one collect WARN that names the refused field", run.warnings)
	}
}

func TestDispatch_AnUnreadableRecordIsAWarnAndTheLaunchProceeds(t *testing.T) {
	req := cycle1853Request(t)
	if err := os.WriteFile(attemptpostmortem.Path(req.Workspace, "build", 1), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := runWithSteps(t, req, noGit(t), cleanAttempt())

	if len(run.bridge.prompts) != 1 || run.bridge.prompts[0] != cycle1853Composed {
		t.Fatalf("prompts = %q, want the plain prompt", run.bridge.prompts)
	}
	if len(run.warnings) != 1 || run.warnings[0].Fields["step"] != "read" {
		t.Fatalf("warnings = %+v, want one read WARN", run.warnings)
	}
}

func TestDispatch_TheRecordCarriesTheWorktreeDeltaFromGit(t *testing.T) {
	req := cycle1853Request(t)
	req.Worktree = t.TempDir()
	stat := " go/internal/swarm/dispatcher.go | 31 ++++---\n 1 file changed\n"

	run := runWithSteps(t, req, func(_ []string, stdout io.Writer) int {
		_, _ = io.WriteString(stdout, stat)
		return 0
	}, paneLostAttempt1(), cleanAttempt())

	records, err := attemptpostmortem.ReadAll(run.ws, "build")
	if err != nil || len(records) != 1 || !strings.Contains(records[0].WorktreeDelta, "dispatcher.go | 31") {
		t.Fatalf("records = %+v, %v; want the git delta", records, err)
	}
	if !containsCall(run.gitCalls, "git diff --stat HEAD") {
		t.Fatalf("git calls = %v, want git diff --stat HEAD", run.gitCalls)
	}
}

func TestDispatch_AFailedGitDiffIsStatedAsTheDelta(t *testing.T) {
	req := cycle1853Request(t)
	req.Worktree = t.TempDir()

	run := runWithSteps(t, req, func(_ []string, _ io.Writer) int { return 128 }, paneLostAttempt1(), cleanAttempt())

	records, err := attemptpostmortem.ReadAll(run.ws, "build")
	if err != nil || len(records) != 1 || !strings.Contains(records[0].WorktreeDelta, "git diff --stat failed") {
		t.Fatalf("records = %+v, %v; want the stated git failure", records, err)
	}
}

func TestDispatch_TheSharedScrollbackIsNeverAttachedBecauseItNamesNoDispatch(t *testing.T) {
	run := runWithSteps(t, cycle1853Request(t), noGit(t), paneLostAttempt1(), cleanAttempt())

	records, err := attemptpostmortem.ReadAll(run.ws, "build")
	if err != nil || len(records) != 1 || records[0].PaneTail != "" || len(records[0].EvidencePaths) != 1 {
		t.Fatalf("records = %+v, %v; want no pane tail and only the transcript as evidence", records, err)
	}
	if strings.Contains(run.bridge.prompts[1], "no server running") {
		t.Fatal("the retry prompt quotes a pane that another dispatch may have written")
	}
}

func TestDispatch_ThePolicyBlockSetsTheCaps(t *testing.T) {
	req := cycle1853Request(t)
	req.ProjectRoot = writeFallbackProfile(t, "evolve-builder", "claude-tmux", []string{"agy-claude-tmux"})
	writePolicy(t, req.ProjectRoot, `{"attempt_postmortem":{"max_commands":1}}`)

	run := runWithSteps(t, req, noGit(t), paneLostAttempt1(), cleanAttempt())

	if got := strings.Count(run.bridge.prompts[1], "`ok` exit"); got != 1 || len(run.warnings) != 0 {
		t.Fatalf("listed commands = %d warnings = %+v, want max_commands 1 and no warning", got, run.warnings)
	}
}

func containsCall(calls [][]string, want string) bool {
	for _, c := range calls {
		if strings.Join(c, " ") == want {
			return true
		}
	}
	return false
}

func TestDispatch_AWarnWithNoSignalCenterOnlyLogs(t *testing.T) {
	req := cycle1853Request(t)
	req.ProjectRoot = writeFallbackProfile(t, "evolve-builder", "claude-tmux", nil)
	if err := os.WriteFile(attemptpostmortem.Path(req.Workspace, "build", 1), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	bridge := &fakeBridge{writeArtifact: "# Build Report\n"}
	r := New(Options{
		Hooks:  &fakeHooks{phase: "build", agent: "evolve-builder", model: "deep", prompt: cycle1853Composed, verdict: core.VerdictPASS},
		Bridge: bridge, Prompts: fakePromptsFS("evolve-builder", "x"),
		EventsProducer: func(string, string, string, int, string) error { return nil },
	})
	if r.signals != nil {
		t.Fatal("the fake bridge must expose no signal center")
	}

	_, _ = r.Run(context.Background(), req)

	if bridge.gotReq.Prompt != cycle1853Composed {
		t.Fatalf("prompt = %q, want the plain prompt", bridge.gotReq.Prompt)
	}
}

func TestLocateTranscript_WithNoHomeReadsNoRelativeClaudeDirectory(t *testing.T) {
	cwd := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore the working directory: %v", err)
		}
	})
	t.Setenv("HOME", "")
	dir := filepath.Join(cwd, ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","cwd":"/wt","timestamp":"2026-10-09T10:00:00Z","message":{"content":"x"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(Options{Hooks: &fakeHooks{phase: "build"}})
	w := launchWindow{cli: "claude-tmux", start: mustStamp(t, "2026-10-09T09:59:00Z"), end: mustStamp(t, "2026-10-09T10:01:00Z")}
	s := postmortemScope{req: core.PhaseRequest{Worktree: "/wt", Env: map[string]string{}}}

	if got := r.locateTranscript(s, w); got != "" {
		t.Fatalf("locateTranscript with no HOME = %q, want none", got)
	}
	s.req.Env["HOME"] = cwd
	if err := os.Rename(filepath.Join(cwd, ".claude"), filepath.Join(cwd, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(cwd, "moved", "projects"), filepath.Join(cwd, ".claude", "projects")); err != nil {
		t.Fatal(err)
	}
	if got := r.locateTranscript(s, w); got != filepath.Join(cwd, ".claude", "projects", "p", "s.jsonl") {
		t.Fatalf("control: locateTranscript with HOME = %q, want the session", got)
	}
}

func TestDispatch_OnlyAnEndInTheAgentSessionWritesARecord(t *testing.T) {
	cases := []struct {
		name   string
		first  launchStep
		record bool
	}{
		{"a quota wall at exit 85", launchStep{exit: 85, cause: "rate_limit"}, false},
		{"a pane loss on an exhausted account", launchStep{exit: 81, cause: "pane_lost", exhausted: true}, false},
		{"a model mismatch at exit 87", launchStep{exit: 87, cause: "model_mismatch"}, false},
		{"a REPL boot timeout at exit 80", launchStep{exit: 80, cause: "repl_boot_timeout"}, false},
		{"a missing binary at exit 127", launchStep{exit: 127, cause: "missing_binary"}, false},
		{"a lost pane", launchStep{exit: 81, cause: "pane_lost"}, true},
		{"a review pause", launchStep{exit: 81, cause: "review_pause"}, true},
		{"a dead shell", launchStep{exit: 81, cause: "dead_shell"}, true},
		{"a crash or a kill", launchStep{exit: -1, cause: "driver_error"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first := tc.first
			first.transcript, first.endAt = "cycle1853-build-attempt1.jsonl", "2026-10-09T09:59:52.115Z"

			run := runWithSteps(t, cycle1853Request(t), noGit(t), first, cleanAttempt())

			records, err := attemptpostmortem.ReadAll(run.ws, "build")
			if err != nil || (len(records) == 1) != tc.record || len(records) > 1 {
				t.Fatalf("records = %d (%v), want a record: %v", len(records), err, tc.record)
			}
			if len(run.bridge.prompts) == 2 && strings.Contains(run.bridge.prompts[1], attemptpostmortem.SectionHeading) != tc.record {
				t.Fatalf("the retry section is present = %v, want %v", !tc.record, tc.record)
			}
		})
	}
}

func TestDispatch_ARecordFromAnEarlierRunOfThePhaseIsKept(t *testing.T) {
	req := cycle1853Request(t)
	earlier := attemptpostmortem.Record{
		Schema:        attemptpostmortem.SchemaVersion,
		Attempt:       attemptpostmortem.Attempt{Phase: "build", Cycle: 1853, Number: 1, CLI: "agy-tmux", CauseCode: "review_pause", ExitCode: 81},
		CommandSource: attemptpostmortem.SourcePaneTail,
	}
	if err := attemptpostmortem.Write(req.Workspace, earlier); err != nil {
		t.Fatal(err)
	}

	run := runWithSteps(t, req, noGit(t), paneLostAttempt1(), cleanAttempt())

	records, err := attemptpostmortem.ReadAll(run.ws, "build")
	if err != nil || len(records) != 2 || records[0].CLI != "agy-tmux" || records[1].Number != 2 || records[1].CauseCode != "pane_lost" {
		t.Fatalf("records = %+v, %v; want the earlier record 1 and a new record 2", records, err)
	}
	if !strings.Contains(run.bridge.prompts[1], "### Attempt 1") || !strings.Contains(run.bridge.prompts[1], "### Attempt 2") {
		t.Fatalf("the retry prompt must state both records:\n%s", run.bridge.prompts[1])
	}
}
