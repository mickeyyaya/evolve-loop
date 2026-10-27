package agentcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/rootwire"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type treeStates struct {
	values []string
	err    error
	dirs   []string
}

func (s *treeStates) state(_ context.Context, dir string) (string, error) {
	s.dirs = append(s.dirs, dir)
	if s.err != nil {
		return "", s.err
	}
	v := s.values[0]
	if len(s.values) > 1 {
		s.values = s.values[1:]
	}
	return v, nil
}

func seatDeps(b *fakeBridge, trees *treeStates) Deps {
	d := depsWith(b)
	d.TreeState = trees.state
	return d
}

func readRecord(t *testing.T, root, runID string) launchrecord.Record {
	t.Helper()
	r, err := launchrecord.Read(paths.EvolveDirOf(root), runID)
	if err != nil {
		t.Fatalf("read the launch record: %v", err)
	}
	return r
}

func runJSON(t *testing.T, d Deps, args []string, stdin string) (int, Envelope, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := New(d)(args, strings.NewReader(stdin), &stdout, &stderr)
	var env Envelope
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("stdout is not one envelope: %v\n%s", err, stdout.String())
		}
	}
	return code, env, stderr.String()
}

func linkedWorktree(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+filepath.Join(t.TempDir(), "wt")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return wt
}

func TestAgentRun_EachRunWritesABridgeLaunchRecordThatNamesTheDriverFamilyModelAndTree(t *testing.T) {
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	trees := &treeStates{values: []string{"tree-1"}}

	code, env, stderr := runJSON(t, seatDeps(fb, trees), runArgs(root, "--json"), diff)

	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr=%s", code, stderr)
	}
	rec := readRecord(t, root, env.RunLabel)
	launched := fb.requests[0]
	want := launchrecord.Record{
		RunID: env.RunLabel, Profile: "adversarial-review", CLI: launched.CLI, Family: env.Family,
		Model: env.Model, Verdict: core.VerdictPASS, TreeBefore: "tree-1", TreeAfter: "tree-1",
		ArtifactSHA256: rec.ArtifactSHA256, TS: "2026-10-21T09:30:00Z",
	}
	if rec != want || rec.ArtifactSHA256 == "" {
		t.Errorf("record = %+v, want %+v with a report hash", rec, want)
	}
	if launched.CLI == "" || env.CLI != launched.CLI || env.Family == "" {
		t.Errorf("envelope cli=%q family=%q, want the launched driver %q and its family", env.CLI, env.Family, launched.CLI)
	}
	if !reflect.DeepEqual(trees.dirs, []string{root, root}) {
		t.Errorf("tree states read in %v, want the project root before and after the run", trees.dirs)
	}
	if want := filepath.Join(root, ".evolve", launchrecord.Subdir, env.RunLabel+".json"); env.LaunchRecord != want {
		t.Errorf("envelope launch_record = %q, want %q", env.LaunchRecord, want)
	}
}

func TestAgentRun_ATreeThatChangedDuringTheRunRecordsBothStates(t *testing.T) {
	root := project(t)
	trees := &treeStates{values: []string{"tree-1", "tree-2"}}

	code, env, _ := runJSON(t, seatDeps(&fakeBridge{artifact: passReport}, trees), runArgs(root, "--json"), diff)

	rec := readRecord(t, root, env.RunLabel)
	if code != 0 || rec.TreeBefore != "tree-1" || rec.TreeAfter != "tree-2" || rec.BoundTo("tree-2") {
		t.Errorf("exit %d record trees %q -> %q, want both states and no binding to either", code, rec.TreeBefore, rec.TreeAfter)
	}
}

func TestAgentRun_ATreeStateErrorIsNamedOnStderrAndLeavesTheTreeEmpty(t *testing.T) {
	root := project(t)
	trees := &treeStates{err: errors.New("not a git checkout")}

	code, env, stderr := runJSON(t, seatDeps(&fakeBridge{artifact: passReport}, trees), runArgs(root, "--json"), diff)

	rec := readRecord(t, root, env.RunLabel)
	if code != 0 || rec.TreeBefore != "" || rec.TreeAfter != "" {
		t.Errorf("exit %d record trees %q %q, want 0 and no tree state", code, rec.TreeBefore, rec.TreeAfter)
	}
	if strings.Count(stderr, "not a git checkout") != 2 {
		t.Errorf("stderr = %q, want the tree-state error named before and after the run", stderr)
	}
}

func TestAgentRun_ALaunchRecordWriteFailureExitsThree(t *testing.T) {
	root := project(t)
	if err := os.WriteFile(filepath.Join(root, ".evolve", launchrecord.Subdir), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	code, env, stderr := runJSON(t, seatDeps(&fakeBridge{artifact: passReport}, &treeStates{values: []string{"t"}}), runArgs(root, "--json"), diff)

	if code != 3 || !strings.Contains(stderr, "launch record") || !strings.Contains(env.Error, "launch record") {
		t.Errorf("exit %d stderr %q envelope error %q, want 3 and the launch record error", code, stderr, env.Error)
	}
}

func TestAgentRun_AnIsolationSignalOfTheBridgeReachesTheIsolationChannel(t *testing.T) {
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	d := Deps{Getenv: noEnv, Now: fixedNow, TreeState: (&treeStates{values: []string{"t"}}).state}
	d.Signals = func(projectRoot string, stderr io.Writer) (*signalcenter.Center, func()) {
		c := signalcenter.New()
		c.Subscribe(signalcenter.ConsoleSink(stderr))
		return c, rootwire.Attach(c, "agent", paths.EvolveDirOf(projectRoot), stderr)
	}
	d.Bridge = func(_ string, signals *signalcenter.Center) core.Bridge {
		fb.onLaunch = func(req core.BridgeRequest) {
			signals.Emit(signalcenter.Event{
				RunID: req.RunID, Phase: "adversarial-review", Module: signalcenter.ModuleIsolation, Origin: "Test.emit",
				Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(2), Reason: "a dropped variable",
			})
		}
		return fb
	}
	var stdout, stderr bytes.Buffer

	code := New(d)(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	log, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "isolation.violation", channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := log.Read(0)
	if err != nil {
		t.Fatalf("read the isolation channel: %v", err)
	}
	var codes []string
	for _, r := range batch.Records {
		if r.Signal != nil {
			codes = append(codes, string(r.Signal.Code)+"@"+r.Source)
		}
	}
	if !reflect.DeepEqual(codes, []string{"ISOLATION_I2_VIOLATION@agent"}) {
		t.Errorf("isolation channel holds %v, want the one violation from role agent", codes)
	}
	if !strings.Contains(stderr.String(), "ISOLATION_I2_VIOLATION") {
		t.Errorf("stderr = %q, want the console line of the warning", stderr.String())
	}
}

func builderArgs(root string, extra ...string) []string {
	return append([]string{"run", "console-builder", "--project-root", root, "--json"}, extra...)
}

func TestAgentRun_TheBuildSeatWritesOnlyInAGivenLinkedWorktreeWithAFence(t *testing.T) {
	root := project(t)
	wt := linkedWorktree(t)
	fb := &fakeBridge{artifact: "## Changes\nx.go\n## Tests\n1/1 PASS\n## Verdict\nVerdict: PASS\n"}
	trees := &treeStates{values: []string{"before", "after"}}

	code, env, stderr := runJSON(t, seatDeps(fb, trees), builderArgs(root, "--worktree", wt, "--writable", "go/x.go", "--writable", "go/x_test.go"), "build item X")

	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr=%s", code, stderr)
	}
	if got := fb.requests[0].Worktree; got != wt {
		t.Errorf("bridge request worktree = %q, want %q", got, wt)
	}
	rec := readRecord(t, root, env.RunLabel)
	if rec.Profile != "console-builder" || rec.Worktree != wt || rec.TreeBefore != "before" || rec.TreeAfter != "after" {
		t.Errorf("record = %+v, want the build seat, its worktree and both tree states", rec)
	}
	if !reflect.DeepEqual(trees.dirs, []string{wt, wt}) {
		t.Errorf("tree states read in %v, want the worktree", trees.dirs)
	}
}

func TestStandaloneRun_RequestFencesTheWorktreeToTheWritablePaths(t *testing.T) {
	cases := []struct {
		name     string
		run      standaloneRun
		wantNil  bool
		writable []string
	}{
		{"no worktree", standaloneRun{}, true, nil},
		{"a reader in a worktree", standaloneRun{worktree: "/wt"}, false, nil},
		{"the build seat", standaloneRun{worktree: "/wt", writable: []string{"go/x.go"}}, false, []string{"go/x.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.run.request()
			if req.Worktree != tc.run.worktree || (req.Fence == nil) != tc.wantNil || !reflect.DeepEqual(req.Fence.WritablePaths(), tc.writable) {
				t.Errorf("request worktree=%q fence=%+v, want worktree %q, nil fence %v and writable %v", req.Worktree, req.Fence, tc.run.worktree, tc.wantNil, tc.writable)
			}
		})
	}
}

func TestAgentRun_TheBuildSeatRefusals(t *testing.T) {
	root := project(t)
	wt := linkedWorktree(t)
	plain := t.TempDir()
	cases := []struct {
		name   string
		args   []string
		code   int
		reason string
	}{
		{"no worktree", builderArgs(root, "--writable", "go/x.go"), 10, "--worktree"},
		{"no writable path", builderArgs(root, "--worktree", wt), 10, "--writable"},
		{"a relative worktree", builderArgs(root, "--worktree", "dev/x", "--writable", "go/x.go"), 10, "absolute"},
		{"not a linked worktree", builderArgs(root, "--worktree", plain, "--writable", "go/x.go"), 2, "linked git worktree"},
		{"an absolute writable path", builderArgs(root, "--worktree", wt, "--writable", "/etc/passwd"), 10, "/etc/passwd"},
		{"a writable path that escapes", builderArgs(root, "--worktree", wt, "--writable", "../x.go"), 10, "../x.go"},
		{"a writable path that is not clean", builderArgs(root, "--worktree", wt, "--writable", "go/./x.go"), 10, "go/./x.go"},
		{"a reader with a writable path", []string{"run", "adversarial-review", "--project-root", root, "--worktree", wt, "--writable", "go/x.go"}, 10, "read-only"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &fakeBridge{artifact: passReport}
			var stdout, stderr bytes.Buffer
			code := New(depsWith(fb))(tc.args, strings.NewReader("x"), &stdout, &stderr)
			if code != tc.code || !strings.Contains(stderr.String(), tc.reason) || len(fb.requests) != 0 {
				t.Errorf("exit %d stderr %q launches %d, want %d, %q and no launch", code, stderr.String(), len(fb.requests), tc.code, tc.reason)
			}
		})
	}
}

func TestAgentRun_AWriterWhoseSandboxCanWriteTheReviewRecordsIsRefused(t *testing.T) {
	root := project(t)
	path := filepath.Join(root, ".evolve", "profiles", "console-builder.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(raw, []byte(`".evolve/agent-runs",`), nil, 1), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{}))(builderArgs(root, "--worktree", linkedWorktree(t), "--writable", "go/x.go"), strings.NewReader("x"), &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), ".evolve/agent-runs") {
		t.Errorf("exit %d stderr %q, want 2 and a refusal that names the record directory", code, stderr.String())
	}
}

func TestConfinedToWorktree(t *testing.T) {
	cases := []struct {
		name string
		box  string
		want bool
	}{
		{"the build seat", `{"enabled":true,"read_only_repo":true,"write_subpaths":["{worktree_path}",".evolve/runs/cycle-*"],"deny_subpaths":[".commit-gate",".evolve/agent-runs"]}`, true},
		{"a sub path of the worktree", `{"enabled":true,"read_only_repo":true,"write_subpaths":["{worktree_path}/go"],"deny_subpaths":[".commit-gate",".evolve/agent-runs"]}`, true},
		{"no sandbox", `null`, false},
		{"disabled", `{"enabled":false,"read_only_repo":true,"deny_subpaths":[".commit-gate",".evolve/agent-runs"]}`, false},
		{"a writable repo", `{"enabled":true,"read_only_repo":false,"deny_subpaths":[".commit-gate",".evolve/agent-runs"]}`, false},
		{"a grant outside the worktree", `{"enabled":true,"read_only_repo":true,"write_subpaths":["docs"],"deny_subpaths":[".commit-gate",".evolve/agent-runs"]}`, false},
		{"a grant that escapes the worktree", `{"enabled":true,"read_only_repo":true,"write_subpaths":["{worktree_path}/../x"],"deny_subpaths":[".commit-gate",".evolve/agent-runs"]}`, false},
		{"no attestation deny", `{"enabled":true,"read_only_repo":true,"write_subpaths":["{worktree_path}"],"deny_subpaths":[".evolve/agent-runs"]}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := confinedToWorktree(sandboxOf(t, tc.box)); got != tc.want {
				t.Errorf("confinedToWorktree(%s) = %v, want %v", tc.box, got, tc.want)
			}
		})
	}
}

func sandboxOf(t *testing.T, raw string) *profiles.SandboxConfig {
	t.Helper()
	var s *profiles.SandboxConfig
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAgentList_NamesTheBuildSeatAsAWriter(t *testing.T) {
	root := project(t)
	var stdout, stderr bytes.Buffer

	code := New(depsWith(nil))([]string{"list", "--json", "--project-root", root}, nil, &stdout, &stderr)

	var got []Profile
	if code != 0 || json.Unmarshal(stdout.Bytes(), &got) != nil {
		t.Fatalf("exit %d stdout %q", code, stdout.String())
	}
	writers := map[string]bool{}
	for _, p := range got {
		writers[p.Name] = p.WritesSource
	}
	if w, listed := writers["console-builder"]; !listed || !w {
		t.Errorf("list = %+v, want console-builder as a writer", got)
	}
	if writers["adversarial-review"] {
		t.Errorf("list marks adversarial-review as a writer")
	}
}

func TestNew_TheDefaultSignalsAreNoCenter(t *testing.T) {
	signals, closeSignals := noSignals(t.TempDir(), io.Discard)
	closeSignals()
	if signals != nil {
		t.Errorf("the default signals = %v, want no Center: only the cmd/evolve root builds one", signals)
	}
}

func TestDefaultBridge_IsTheCenterLessRegistryBridge(t *testing.T) {
	b, ok := defaultBridge(project(t), signalcenter.New()).(interface{ SignalsWired() bool })
	if !ok || b.SignalsWired() {
		t.Errorf("the default bridge carries a Signal Center; only the cmd/evolve root wires one")
	}
}

func TestDefaultTreeState_OutsideAGitCheckoutIsAnError(t *testing.T) {
	sum, err := defaultTreeState(context.Background(), t.TempDir())
	if err == nil || sum != "" {
		t.Errorf("defaultTreeState outside a checkout = %q, %v; want no state and an error", sum, err)
	}
}

func TestAgentRun_ALabelWhoseLaunchRecordExistsIsNotReusedAndDispatchesNothing(t *testing.T) {
	root := project(t)
	label := runLabel("adversarial-review", fixedNow())
	earlier := launchrecord.Record{RunID: label, Profile: "adversarial-review", CLI: "agy-tmux", Verdict: core.VerdictPASS}
	if _, err := launchrecord.Write(paths.EvolveDirOf(root), earlier); err != nil {
		t.Fatal(err)
	}
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(fb))(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != 3 || !strings.Contains(stderr.String(), "is taken") || len(fb.requests) != 0 {
		t.Errorf("exit %d stderr %q launches %d, want 3, a taken label and no dispatch", code, stderr.String(), len(fb.requests))
	}
	if rec := readRecord(t, root, label); rec != earlier {
		t.Errorf("the earlier record changed to %+v", rec)
	}
}

func TestAgentRun_ADispatchErrorRecordsNoVerdictAndAWriterIsMarked(t *testing.T) {
	root := project(t)
	code, env, _ := runJSON(t, seatDeps(&fakeBridge{err: errBridgeDown}, &treeStates{values: []string{"t"}}), runArgs(root, "--json"), diff)
	if rec := readRecord(t, root, env.RunLabel); code != 3 || rec.Verdict != "" || rec.WritesSource {
		t.Errorf("exit %d record %+v, want 3, no verdict and no writer mark", code, rec)
	}

	wt := linkedWorktree(t)
	build := &fakeBridge{artifact: "## Changes\nx\n## Tests\n1/1 PASS\n## Verdict\nVerdict: PASS\n"}
	_, env, stderr := runJSON(t, seatDeps(build, &treeStates{values: []string{"t"}}), builderArgs(root, "--worktree", wt, "--writable", "go/x.go"), "x")
	if rec := readRecord(t, root, env.RunLabel); !rec.WritesSource {
		t.Errorf("build record %+v, want the writer mark; stderr=%s", rec, stderr)
	}
}

func TestAgentRun_AProfileNameThatMakesNoRunIDIsADispatchError(t *testing.T) {
	root := project(t)
	writeSpec(t, root, "odd review", "evolve-adversarial-review")
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(fb))([]string{"run", "odd review", "--project-root", root}, strings.NewReader(diff), &stdout, &stderr)

	if code != 3 || !strings.Contains(stderr.String(), "run id") || len(fb.requests) != 0 {
		t.Errorf("exit %d stderr %q launches %d, want 3, a run id refusal and no dispatch", code, stderr.String(), len(fb.requests))
	}
}

func TestLaunchOutcome_TheModelIsTheResolvedOneElseTheModelOfTheLaunch(t *testing.T) {
	cases := []struct{ resolved, launched, want string }{
		{"gemini-3-pro", "balanced", "gemini-3-pro"},
		{"", "balanced", "balanced"},
	}
	for _, tc := range cases {
		out := launchOutcome{resp: core.PhaseResponse{ResolvedModel: tc.resolved}, model: tc.launched}
		if got := out.launchedModel(); got != tc.want {
			t.Errorf("launchedModel(resolved %q, launched %q) = %q, want %q", tc.resolved, tc.launched, got, tc.want)
		}
	}
}
