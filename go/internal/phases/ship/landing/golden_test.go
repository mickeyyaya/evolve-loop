package landing

// golden_test.go — test 22: the goldens captured on the HOST before the move
// (testdata/*.golden.json, 8e8f080f) replayed through the leaf. A golden row
// is a whole host scenario; the leaf owns the window from the reset (or the
// push) up to its post-push head read, and the `[ship]   OK: ff-merged` and
// `[ship] REPAIR:` log lines — the host's prelude, its `rev-parse HEAD^{tree}`
// and its own OK lines are outside the leaf and are cut before comparing.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

type goldenError struct {
	Code    string            `json:"code"`
	Class   string            `json:"class"`
	Stage   string            `json:"stage"`
	Message string            `json:"message"`
	Debug   map[string]string `json:"debug"`
}

type goldenRow struct {
	Name            string       `json:"name"`
	Argv            []string     `json:"argv"`
	Logs            []string     `json:"logs"`
	CommitSHA       string       `json:"commit_sha"`
	RepairAttempted string       `json:"repair_attempted"`
	RepairOutcome   string       `json:"repair_outcome"`
	Error           *goldenError `json:"error"`
}

type goldenFile struct {
	CapturedAt string      `json:"captured_at"`
	Rows       []goldenRow `json:"rows"`
}

func loadGolden(t *testing.T, name string) goldenFile {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatal(err)
	}
	if g.CapturedAt != "8e8f080f" {
		t.Fatalf("%s: captured_at %q, want the pre-move commit 8e8f080f", name, g.CapturedAt)
	}
	return g
}

// leafWindow cuts a host argv sequence down to the calls the leaf owns:
// from the first reset/push through the last call before the host's
// `rev-parse HEAD^{tree}` (the verification read that stays with the host).
func leafWindow(argv []string) []string {
	start := -1
	for i, a := range argv {
		if strings.HasPrefix(a, "checkout HEAD -- ") || strings.HasPrefix(a, "push origin ") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	var out []string
	for _, a := range argv[start:] {
		if strings.HasPrefix(a, "rev-parse HEAD^{tree}") {
			break
		}
		out = append(out, a)
	}
	return out
}

// leafLogs keeps the log lines the leaf writes (the ff-merge OK line and the
// REPAIR lines); the host's own lines are dropped.
func leafLogs(logs []string) []string {
	var out []string
	for _, l := range logs {
		if strings.HasPrefix(l, "[ship]   OK: ff-merged ") || strings.HasPrefix(l, "[ship] REPAIR: ") {
			out = append(out, l)
		}
	}
	return out
}

// scriptScenario scripts the fake git for a golden row by its name — the
// same responses the host recorder gave when the golden was captured.
func scriptScenario(f *fakeGit, name string) {
	scenario := name[strings.LastIndex(name, "/")+1:]
	if strings.HasPrefix(name, "reset-") {
		if strings.Contains(name, "reset-fail") {
			f.on("checkout HEAD -- go/evolve", scripted{exit: 1})
		}
		if strings.Contains(name, "merge-diverged") {
			f.on("merge --ff-only "+testCycleBr, scripted{exit: 128})
		}
		f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
		return
	}
	if scenario != "push-ok" {
		f.on("push origin "+testBranch, scripted{exit: 1}, scripted{exit: 0})
	}
	f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
	switch scenario {
	case "fetch-rc1":
		f.on("fetch origin "+testBranch, scripted{exit: 1})
	case "fetch-spawn-error":
		f.on("fetch origin "+testBranch, scripted{err: errors.New("spawn: no git")})
	case "origin-ref-error":
		f.on("rev-parse origin/"+testBranch, scripted{exit: 128})
	case "head-error":
		// The capture answered `rev-parse HEAD` [ok, error] at every site: on
		// the worktree site the wording probe takes the ok and the REPAIR's
		// head probe declines; on the direct and push-only sites the repair
		// probe takes the ok, the retry lands, and the POST-PUSH head read
		// fails (Head empty, no error) — two scenarios under one name.
		f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
		f.set("rev-parse HEAD", scripted{stdout: testHead + "\n"}, scripted{err: errors.New("spawn: no git")})
	case "already-pushed":
		f.on("rev-parse origin/"+testBranch, scripted{stdout: testHead + "\n"})
	case "ancestor-retry-ok":
		f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
		f.on("merge-base --is-ancestor "+testOriginRef+" HEAD", scripted{exit: 0})
	case "ancestor-retry-rc1":
		f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
		f.on("merge-base --is-ancestor "+testOriginRef+" HEAD", scripted{exit: 0})
		f.set("push origin "+testBranch, scripted{exit: 1}, scripted{exit: 1})
	case "diverged":
		f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
		f.on("merge-base --is-ancestor "+testOriginRef+" HEAD", scripted{exit: 1})
	}
}

func pushRequestFor(name string, site PushSite, sink func(string)) PushRequest {
	req := PushRequest{Branch: testBranch, Site: site, Log: sink}
	switch name[strings.LastIndex(name, "/")+1:] {
	case "once-guard-preset":
		req.RepairAttempted = true
	case "dry-run":
		req.DryRun = true
	}
	return req
}

func assertErrorMatchesGolden(t *testing.T, name string, err error, want *goldenError, step string) {
	t.Helper()
	if (err == nil) != (want == nil) {
		t.Errorf("%s: error %v, golden %+v", name, err, want)
		return
	}
	if err == nil {
		return
	}
	se, ok := shiperr.AsShipError(err)
	if !ok {
		t.Fatalf("%s: %T", name, err)
	}
	if string(se.Code) != want.Code || string(se.Class) != want.Class || string(se.Stage) != want.Stage || se.Message != want.Message {
		t.Errorf("%s: [%s/%s @%s] %q, golden %+v", name, se.Code, se.Class, se.Stage, se.Message, want)
	}
	if se.Debug[shiperr.StepKey] != step {
		t.Errorf("%s: Debug[step]=%q, want %s (the unit's declared addition)", name, se.Debug[shiperr.StepKey], step)
	}
	if len(se.Debug) != len(want.Debug)+1 {
		t.Errorf("%s: Debug %v, golden %v plus step", name, se.Debug, want.Debug)
	}
	for k, v := range want.Debug {
		if se.Debug[k] != v {
			t.Errorf("%s: Debug[%s]=%q, golden %q", name, k, se.Debug[k], v)
		}
	}
}

// Test 22 (integrate rows) — every landing_integrate golden row replays
// through Integrate + Push: the leaf window's argv and streams, the leaf's
// log lines, the head and the error are the host's bytes.
func TestIntegrate_ArgvAndLogSequencesMatchTheGoldens(t *testing.T) {
	for _, row := range loadGolden(t, "landing_integrate.golden.json").Rows {
		f := newFakeGit()
		scriptScenario(f, row.Name)
		l, _ := newLanding(f)
		sink, lines := logSink()
		req := Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve", Fleet: strings.HasSuffix(row.Name, "fleet-on"), Log: sink}
		err := l.Integrate(context.Background(), req)
		var out PushResult
		if err == nil {
			out, err = l.Push(context.Background(), PushRequest{Branch: testBranch, Site: SiteWorktree, Log: sink})
		}
		assertReplay(t, row, f.calls, *lines, out, err, "integrate")
	}
}

// Test 22 (push rows) — every push_<site> golden row replays through Push.
func TestPush_ArgvAndLogSequencesMatchTheGoldens(t *testing.T) {
	for name, site := range map[string]PushSite{"direct": SiteDirect, "worktree": SiteWorktree, "pushonly": SitePushOnly} {
		for _, row := range loadGolden(t, "push_"+name+".golden.json").Rows {
			f := newFakeGit()
			scriptScenario(f, row.Name)
			l, _ := newLanding(f)
			sink, lines := logSink()
			if site == SiteWorktree { // the worktree rows start at the integrate
				if err := l.Integrate(context.Background(), Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve", Log: sink}); err != nil {
					t.Fatal(err)
				}
			}
			out, err := l.Push(context.Background(), pushRequestFor(row.Name, site, sink))
			assertReplay(t, row, f.calls, *lines, out, err, "push")
		}
	}
}

func assertReplay(t *testing.T, row goldenRow, argv, logs []string, out PushResult, err error, step string) {
	t.Helper()
	if want := leafWindow(row.Argv); strings.Join(argv, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s: argv\n got %q\nwant %q", row.Name, argv, want)
	}
	if want := leafLogs(row.Logs); strings.Join(logs, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s: logs\n got %q\nwant %q", row.Name, logs, want)
	}
	if out.Head != row.CommitSHA || out.RepairAttempted != (row.RepairAttempted != "") || string(out.RepairOutcome) != row.RepairOutcome {
		t.Errorf("%s: result %+v, golden {sha=%q repair=%q/%q}", row.Name, out, row.CommitSHA, row.RepairAttempted, row.RepairOutcome)
	}
	assertErrorMatchesGolden(t, row.Name, err, row.Error, step)
}
