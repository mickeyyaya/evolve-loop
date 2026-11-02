package auditfixports

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const (
	citedTest = "go/internal/core/retro_remediation_filter_test.go"
	root      = "/hub/runtime"
)

var errSeat = errors.New("seat down")

type agentCall struct {
	args  []string
	input string
}

type fakeAgent struct {
	env   SeatResult
	err   error
	calls []agentCall
}

func (f *fakeAgent) run(_ context.Context, args []string, input string) (SeatResult, error) {
	f.calls = append(f.calls, agentCall{args: args, input: input})
	return f.env, f.err
}

func routes(families map[string]string) Route {
	return func(profile string) (auditfix.Seat, error) {
		family, ok := families[profile]
		if !ok {
			return auditfix.Seat{}, errSeat
		}
		return auditfix.Seat{Family: family, Tier: "deep"}, nil
	}
}

func prescriptions() []auditfix.Prescription {
	return []auditfix.Prescription{
		{Finding: "H1", Defect: "H1: " + citedTest + ":29 inboxFiles counts every entry", Text: "Make inboxFiles in " + citedTest + ":29 count only *.json items"},
		{Finding: "M1", Defect: "M1: " + citedTest + ":40 the lock is counted", Text: "Skip the lock file"},
	}
}

func TestFixer_RunsTheConsoleBuilderOnlyOnTheCitedFilesOfTheWorktree(t *testing.T) {
	t.Parallel()
	agent := &fakeAgent{env: SeatResult{Verdict: core.VerdictPASS}}
	f := Fixer{Run: agent.run, Route: routes(map[string]string{FixerProfile: "claude"}), ProjectRoot: root}
	req := auditfix.FixRequest{Cycle: 2035, Workspace: "/runs/cycle-2035", Worktree: "/wt/2035", Prescriptions: prescriptions(), Cited: []string{citedTest},
		Bounds: retryladder.AuditFixBounds{MaxFiles: 3, MaxLines: 40, MaxCodeLines: 10, NoNewProductionLogic: true}}

	reasons, err := f.ApplyFix(context.Background(), req)

	if err != nil || reasons[citedTest] != prescriptions()[0].Text || len(reasons) != 1 {
		t.Fatalf("ApplyFix = %v, %v, want the first prescription as the reason of the cited file", reasons, err)
	}
	want := []string{FixerProfile, "--json", "--project-root", root, "--worktree", "/wt/2035", "--file", "/runs/cycle-2035/audit-report.md", "--writable", citedTest}
	if len(agent.calls) != 1 || !slices.Equal(agent.calls[0].args, want) {
		t.Fatalf("agent run args = %v, want %v", agent.calls, want)
	}
	brief := agent.calls[0].input
	for _, part := range []string{"cycle 2035", "at most 3 files, 40 changed lines and 10 lines", "Add no new production logic", prescriptions()[1].Defect, "Prescription: Skip the lock file"} {
		if !strings.Contains(brief, part) {
			t.Errorf("the brief lacks %q:\n%s", part, brief)
		}
	}
	if seat, err := f.Seat(); err != nil || seat != (auditfix.Seat{Profile: FixerProfile, Family: "claude", Tier: "deep", Session: "fresh:" + FixerProfile}) {
		t.Fatalf("Seat = %+v, %v", seat, err)
	}
}

func TestFixer_ErrorsAreLoud(t *testing.T) {
	t.Parallel()
	agent := &fakeAgent{err: errSeat}
	f := Fixer{Run: agent.run, Route: routes(nil), ProjectRoot: root}
	if _, err := f.ApplyFix(context.Background(), auditfix.FixRequest{}); !errors.Is(err, errSeat) {
		t.Fatalf("ApplyFix = %v, want the agent run error", err)
	}
	if _, err := f.Seat(); !errors.Is(err, errSeat) || !strings.Contains(err.Error(), FixerProfile) {
		t.Fatalf("Seat = %v, want the routing error with the profile", err)
	}
	brief := fixBrief(auditfix.FixRequest{Prescriptions: []auditfix.Prescription{{Finding: "H9", Text: "fix it"}}})
	if strings.Contains(brief, "no new production logic") || !strings.Contains(brief, "- H9\n") {
		t.Fatalf("brief = %q, want the finding id when there is no defect text and no logic rule", brief)
	}
}

func TestReviewer_PicksTheFirstProfileOfAnotherFamily(t *testing.T) {
	t.Parallel()
	r := Reviewer{Route: routes(map[string]string{"code-reviewer": "claude", "unmapped": "", "go-reviewer": "agy", "adversarial-review": "codex"}),
		Profiles: []string{"code-reviewer", "missing", "unmapped", "go-reviewer", "adversarial-review"}}

	seat, err := r.ReviewSeat(context.Background(), auditfix.Seat{Family: "claude", Session: "fresh:" + FixerProfile})

	if err != nil || seat != (auditfix.Seat{Profile: "go-reviewer", Family: "agy", Tier: "deep", Session: "fresh:go-reviewer"}) {
		t.Fatalf("ReviewSeat = %+v, %v, want go-reviewer on agy", seat, err)
	}
}

func TestReviewer_GivesNoSeatWhenEveryProfileIsOfTheFixerFamily(t *testing.T) {
	t.Parallel()
	r := Reviewer{Route: routes(map[string]string{"code-reviewer": "claude"}), Profiles: []string{"code-reviewer"}}
	if seat, err := r.ReviewSeat(context.Background(), auditfix.Seat{Family: "claude"}); err != nil || seat != (auditfix.Seat{}) {
		t.Fatalf("ReviewSeat = %+v, %v, want no seat and no error", seat, err)
	}
	r.Profiles = []string{"missing"}
	if _, err := r.ReviewSeat(context.Background(), auditfix.Seat{Family: "claude"}); !errors.Is(err, errSeat) {
		t.Fatalf("ReviewSeat = %v, want the routing error when no profile routes", err)
	}
}

func reviewRequest(t *testing.T) auditfix.ReviewRequest {
	t.Helper()
	return auditfix.ReviewRequest{Workspace: t.TempDir(), Seat: auditfix.Seat{Profile: "go-reviewer", Family: "agy"},
		Patch: auditfix.Patch{Diff: "+count only json\n"}, Prescriptions: prescriptions()}
}

func TestReviewer_ReviewsOnlyThePatchAndThePrescriptionsAndReadsTheTypedVerdict(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		env  SeatResult
		err  error
		pass bool
	}{
		{"pass", SeatResult{Verdict: core.VerdictPASS}, nil, true},
		{"warn", SeatResult{Verdict: core.VerdictWARN}, nil, true},
		{"fail", SeatResult{Verdict: core.VerdictFAIL, RunLabel: "agent-go-reviewer-1", Family: "agy", High: 2}, errors.New("exit 1"), false},
		{"no verdict", SeatResult{}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := &fakeAgent{env: tc.env, err: tc.err}
			req := reviewRequest(t)
			got, err := (Reviewer{Run: agent.run, ProjectRoot: root}).Review(context.Background(), req)
			if err != nil || got.Pass != tc.pass || (!tc.pass && len(got.Reasons) != 1) {
				t.Fatalf("Review = %+v, %v, want pass %v with a reason on a refusal", got, err, tc.pass)
			}
			dir := filepath.Join(req.Workspace, reviewDir)
			want := []string{"go-reviewer", "--json", "--project-root", root, "--file", filepath.Join(dir, "patch.diff"), "--file", filepath.Join(dir, "prescriptions.md")}
			if !slices.Equal(agent.calls[0].args, want) || agent.calls[0].input != reviewBrief {
				t.Fatalf("args = %v, want %v", agent.calls[0].args, want)
			}
			if readFile(t, want[5]) != req.Patch.Diff || !strings.Contains(readFile(t, want[7]), "Prescription: Skip the lock file") {
				t.Fatalf("the review inputs do not hold the patch and the prescriptions")
			}
		})
	}
}

func TestReviewer_ADispatchErrorOrAnUnwritableWorkspaceIsLoud(t *testing.T) {
	t.Parallel()
	agent := &fakeAgent{env: SeatResult{Verdict: core.VerdictFAIL, Error: "no launch"}, err: errSeat}
	if _, err := (Reviewer{Run: agent.run}).Review(context.Background(), reviewRequest(t)); !errors.Is(err, errSeat) {
		t.Fatalf("Review = %v, want the dispatch error", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Reviewer{Run: agent.run}).Review(context.Background(), auditfix.ReviewRequest{Workspace: file}); err == nil {
		t.Fatal("a workspace that is a file passed")
	}
	req := reviewRequest(t)
	if err := os.MkdirAll(filepath.Join(req.Workspace, reviewDir, "patch.diff"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (Reviewer{Run: agent.run}).Review(context.Background(), req); err == nil {
		t.Fatal("an unwritable review input passed")
	}
}

func writeRunState(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	raw := `{"cycle_id":2035,"run_id":"run-2035","worktree_base_sha":"abc123"}`
	if err := os.WriteFile(filepath.Join(ws, core.RunStateFile), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestGates_RunTheFloorAndTheCycleACSOnThePatchedWorktree(t *testing.T) {
	t.Parallel()
	ws := writeRunState(t)
	var floorIn core.ReviewInput
	floor := core.BuildHandoffFloor{{Name: "fake", Run: func(_ context.Context, in core.ReviewInput) []string {
		floorIn = in
		return []string{"cover-changed 92%"}
	}}}
	var acsOpts acssuite.Options
	g := Gates{Floor: floor, Protected: func(string) bool { return false }, ProjectRoot: root,
		ACS: func(o acssuite.Options) (acssuite.Verdict, error) {
			acsOpts = o
			return acssuite.Verdict{RedIDs: []string{"AC-3"}}, nil
		}}

	got, err := g.Run(context.Background(), auditfix.GateRequest{Cycle: 2035, Workspace: ws, Worktree: "/wt/2035"})

	if err != nil || got.Pass || !slices.Equal(got.Reasons, []string{"cover-changed 92%", "acs red: AC-3"}) {
		t.Fatalf("Run = %+v, %v, want the floor and the ACS reasons", got, err)
	}
	if floorIn.Phase != string(core.PhaseBuild) || floorIn.Worktree != "/wt/2035" || floorIn.RunID != "run-2035" || floorIn.WorktreeBaseSHA != "abc123" || floorIn.ProjectRoot != root || floorIn.Protected == nil {
		t.Fatalf("floor input = %+v, want the build floor input of the cycle", floorIn)
	}
	if acsOpts.Root != "/wt/2035" || acsOpts.ProjectRoot != root || acsOpts.Cycle != 2035 {
		t.Fatalf("acs options = %+v", acsOpts)
	}
	g.Floor, g.ACS = nil, func(acssuite.Options) (acssuite.Verdict, error) { return acssuite.Verdict{}, nil }
	if got, err := g.Run(context.Background(), auditfix.GateRequest{Cycle: 2035, Workspace: ws, Worktree: "/wt/2035"}); err != nil || !got.Pass {
		t.Fatalf("Run = %+v, %v, want a pass with no failure", got, err)
	}
}

func TestGates_AMissingRunStateOrAnACSErrorIsLoud(t *testing.T) {
	t.Parallel()
	g := Gates{ACS: func(acssuite.Options) (acssuite.Verdict, error) { return acssuite.Verdict{}, errSeat }}
	if _, err := g.Run(context.Background(), auditfix.GateRequest{Workspace: t.TempDir()}); err == nil {
		t.Fatal("a missing run state passed")
	}
	if _, err := g.Run(context.Background(), auditfix.GateRequest{Workspace: writeRunState(t)}); !errors.Is(err, errSeat) {
		t.Fatalf("Run = %v, want the ACS error", err)
	}
}

func TestRouterRoute_NamesTheFirstFamilyOfTheChain(t *testing.T) {
	t.Parallel()
	var asked cliroute.Request
	route := RouterRoute(func(req cliroute.Request) (cliroute.Decision, error) {
		asked = req
		return cliroute.Decision{Plan: llmroute.Plan{Candidates: []string{"claude-tmux", "agy"}, Model: "deep"}}, nil
	}, root)
	seat, err := route("go-reviewer")
	if err != nil || seat != (auditfix.Seat{Family: llmroute.Family("claude-tmux"), Tier: "deep"}) || seat.Family == "" {
		t.Fatalf("route = %+v, %v", seat, err)
	}
	if asked.Agent != "go-reviewer" || asked.Phase != "go-reviewer" || asked.ProjectRoot != root || !asked.NeedsTools {
		t.Fatalf("request = %+v", asked)
	}
	empty := RouterRoute(func(cliroute.Request) (cliroute.Decision, error) { return cliroute.Decision{}, nil }, root)
	if _, err := empty("go-reviewer"); err == nil {
		t.Fatal("a plan with no candidate routed")
	}
	failing := RouterRoute(func(cliroute.Request) (cliroute.Decision, error) { return cliroute.Decision{}, errSeat }, root)
	if _, err := failing("go-reviewer"); !errors.Is(err, errSeat) {
		t.Fatalf("route = %v, want the router error", err)
	}
}
