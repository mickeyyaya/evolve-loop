package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var launchGoldenCodes = []int{0, 2, 3, 10, 80, 81, 85, 86, 99, 124, 127, -1, 42}

var launchGoldenFixtures = []string{"empty", "first-bridge-line", "last-line-only", "marker-submit-wedged", "marker-prose", "long-line", "long-marker"}

type launchOutcomeRow struct {
	Code      int    `json:"code"`
	Ctx       string `json:"ctx"`
	Stderr    string `json:"stderr"`
	Err       string `json:"err"`
	Sentinel  string `json:"sentinel"`
	CauseCode string `json:"cause_code"`
}

func launchSentinelOf(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, core.ErrArtifactTimeout):
		return "artifact_timeout"
	case errors.Is(err, core.ErrTransientBridgeFailure):
		return "transient"
	}
	return "plain"
}

func launchCtx(state string) context.Context {
	if state == "live" {
		return context.Background()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func launchThroughFixture(t *testing.T, ctx context.Context, deps Deps, script []string, req core.BridgeRequest) (core.BridgeResponse, string, error) {
	t.Helper()
	ensureLaunchFixtureDriver(t)
	ws := req.Workspace
	if ws == "" {
		ws = t.TempDir()
	}
	if deps.LookupEnv == nil {
		deps.LookupEnv = mapLookup(nil)
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC) }
	}
	req.CLI, req.Profile, req.Model, req.Workspace = launchOutcomeFixtureCLI, writeProfile(t, ws, "unit10", ""), "auto", ws
	req.Prompt, req.ArtifactPath, req.ExtraFlags = "x", filepath.Join(ws, "a.md"), script
	if req.Agent == "" {
		req.Agent = "build"
	}
	resp, err := NewEngine(deps).Launch(ctx, req)
	return resp, ws, err
}

func lastLedgerRecord(t *testing.T, ws string) llmcalls.Record {
	t.Helper()
	res, err := llmcalls.ReadWorkspace(ws)
	if err != nil || len(res.Records) == 0 {
		t.Fatalf("llm-calls.ndjson: %v (%d records)", err, len(res.Records))
	}
	return res.Records[len(res.Records)-1]
}

func goldenJSON(t *testing.T, path string, got any) {
	t.Helper()
	want, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data, merr := json.MarshalIndent(got, "", "  ")
		if merr != nil {
			t.Fatal(merr)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("golden %s captured on this tree — re-run to replay it", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	gotData, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(want)) != strings.TrimSpace(string(gotData)) {
		t.Fatalf("%s drifted from the golden captured on the pre-extraction code:\n--- want\n%s\n--- got\n%s", path, want, gotData)
	}
}

func TestLaunchOutcome_Golden_Capture(t *testing.T) {
	var rows []launchOutcomeRow
	for _, code := range launchGoldenCodes {
		for _, ctxState := range []string{"live", "cancelled"} {
			for _, fixture := range launchGoldenFixtures {
				_, ws, err := launchThroughFixture(t, launchCtx(ctxState), Deps{}, launchFixtureScript(code, fixture), core.BridgeRequest{})
				row := launchOutcomeRow{Code: code, Ctx: ctxState, Stderr: fixture, Sentinel: launchSentinelOf(err), CauseCode: lastLedgerRecord(t, ws).CauseCode}
				if err != nil {
					row.Err = strings.ReplaceAll(err.Error(), ws, "<ws>")
				}
				rows = append(rows, row)
			}
		}
	}
	goldenJSON(t, filepath.Join("testdata", "launch-outcome.golden.json"), rows)
}

func TestLaunchArgs_Golden_FullArgvVector(t *testing.T) {
	t.Parallel()
	req := core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p/profile.json", Model: "opus", Workspace: "/ws",
		ArtifactPath: "/ws/report.md", SecondaryArtifacts: []string{"/ws/second.md", "/ws/third.json"},
		Cycle: 1700, Agent: "build", Worktree: "/wt", RunID: "run-1700", ProjectRoot: "/root",
		RequireSandbox: true, Completion: "stdout", PermissionMode: "plan", SessionName: "evolve-build",
		ExtraFlags: []string{"--bare", "--strict-mcp-config"}, BudgetScale: 1.5,
	}
	deps := Deps{PhaseArtifactTimeoutS: map[string]int{"build": 600}}
	args := launchArgs(req, "/ws/build-prompt.txt", "/ws/build-stdout.log", "/ws/build-stderr.log", deps)
	path := filepath.Join("testdata", "launch-args.golden.txt")
	got := strings.Join(args, "\n") + "\n"
	want, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("golden %s captured on this tree — re-run to replay it", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Fatalf("launchArgs drifted from the golden argv vector:\n--- want\n%s\n--- got\n%s", want, got)
	}
}

type launchStreamPath struct {
	name, ctx    string
	code         int
	artifact     bool
	storeBroken  bool
	launchErrDir bool
}

var launchStreamPaths = []launchStreamPath{
	{name: "0-readable", ctx: "live", code: 0, artifact: true},
	{name: "0-unreadable", ctx: "live", code: 0},
	{name: "10", ctx: "live", code: 10},
	{name: "10-store-unwritable", ctx: "live", code: 10, storeBroken: true},
	{name: "10-launch-error-unwritable", ctx: "live", code: 10, launchErrDir: true},
	{name: "80-store-ok", ctx: "live", code: 80},
	{name: "80-store-unwritable", ctx: "live", code: 80, storeBroken: true},
	{name: "81", ctx: "live", code: 81},
	{name: "124", ctx: "live", code: 124},
	{name: "-1-cancelled", ctx: "cancelled", code: -1},
	{name: "-1-live", ctx: "live", code: -1},
}

func brokenBootStrikeStore(t *testing.T) *clihealth.Store {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	return clihealth.NewStore(root, nil)
}

func signalSequence(events []signalcenter.Event) []string {
	out := []string{}
	for _, e := range events {
		out = append(out, string(e.Module)+"/"+string(e.Kind)+"/"+string(e.Code))
	}
	return out
}

func runLaunchStreamPath(t *testing.T, p launchStreamPath) []string {
	t.Helper()
	c, got := recordingSignals()
	deps := Deps{Signals: c, Stderr: new(strings.Builder)}
	if p.storeBroken {
		deps.BootTimeoutStore = brokenBootStrikeStore(t)
	} else {
		deps.BootTimeoutStore = clihealth.NewStore(t.TempDir(), nil)
	}
	script := launchFixtureScript(p.code, "first-bridge-line")
	if p.artifact {
		script = append(script, "artifact=both")
	}
	req := core.BridgeRequest{Agent: "build", Cycle: 3, RunID: "run-3"}
	if p.launchErrDir {
		req.Workspace = t.TempDir()
		if err := os.MkdirAll(filepath.Join(req.Workspace, "build-launch-error.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_, _, _ = launchThroughFixture(t, launchCtx(p.ctx), deps, script, req)
	return signalSequence(*got)
}

func TestEngineLaunch_SignalStream_Golden(t *testing.T) {
	got := map[string][]string{}
	for _, p := range launchStreamPaths {
		got[p.name] = runLaunchStreamPath(t, p)
	}
	goldenJSON(t, filepath.Join("testdata", "launch-signals.golden.json"), got)
}

func TestLaunchOutcome_GoldenRowsCoverEveryCodeCtxAndFixture(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("testdata", "launch-outcome.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []launchOutcomeRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if want := len(launchGoldenCodes) * 2 * len(launchGoldenFixtures); len(rows) != want {
		t.Fatalf("golden rows = %d, want %d", len(rows), want)
	}
	seen := map[launchOutcomeRow]bool{}
	for _, r := range rows {
		key := launchOutcomeRow{Code: r.Code, Ctx: r.Ctx, Stderr: r.Stderr}
		if seen[key] {
			t.Errorf("duplicate golden row %+v", key)
		}
		seen[key] = true
	}
	if !reflect.DeepEqual(rows[0], launchOutcomeRow{Code: 0, Ctx: "live", Stderr: "empty", Sentinel: "none"}) {
		t.Errorf("the first row is the ExitOK zero outcome: %+v", rows[0])
	}
}
