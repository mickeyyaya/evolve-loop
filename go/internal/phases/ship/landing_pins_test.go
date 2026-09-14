package ship

// landing_pins_test.go — ADR-0103 unit 07 (the ship landing): the pre-move
// pins over the ff-merge, the three push sites with their inline push-race
// repair, and the ship-binding writer. The goldens under landing/testdata
// were captured on 8e8f080f BEFORE any code moved (a throwaway recorder,
// deleted after the capture) and are replayed here through the host and in
// the leaf's own tests, so a transcription slip in the move is a byte diff,
// not a review opinion. Every test is untagged and fake-only: the recorder
// scripts git by its FULL argv (the package's scriptedRunner keys on the
// subcommand and cannot tell `rev-parse origin/main` from `rev-parse HEAD`).

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const (
	goldenCapturedAt = "8e8f080f"
	pinBranch        = "main"
	pinCycleBranch   = "cycle-7-branch"
	pinHead          = "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"
	pinOriginRef     = "0f1e2d3c4b5a69788796a5b4c3d2e1f0fedcba98"
	pinTree          = "77777777777777777777777777777777abcdef01"
)

// scriptedCall is one scripted git response; a queue per argv is consumed
// FIFO and the last entry is sticky, so the SAME argv (the retry push) can
// answer differently on its second call.
type scriptedCall struct {
	stdout string
	exit   int
	err    error
}

// argvCall is one recorded git call: the argv joined by spaces and which
// writers it streamed to — the merge and the pushes write to the operator
// streams, the probes to io.Discard, the captures to a builder.
type argvCall struct {
	Argv    string `json:"argv"`
	Streams string `json:"streams"`
}

type argvRecorder struct {
	scripts map[string][]scriptedCall
	calls   []argvCall
	opts    *Options
}

func newArgvRecorder(opts *Options) *argvRecorder {
	r := &argvRecorder{scripts: map[string][]scriptedCall{}, opts: opts}
	opts.Runner = r.runner()
	return r
}

func (r *argvRecorder) on(argv string, calls ...scriptedCall) *argvRecorder {
	r.scripts[argv] = append(r.scripts[argv], calls...)
	return r
}

// set replaces the queue for argv (a row that needs BOTH pushes rejected).
func (r *argvRecorder) set(argv string, calls ...scriptedCall) *argvRecorder {
	r.scripts[argv] = calls
	return r
}

func (r *argvRecorder) streamsOf(stdout, stderr io.Writer) string {
	switch {
	case stdout == io.Discard && stderr == io.Discard:
		return "discard"
	case r.opts != nil && stdout == r.opts.Stdout && stderr == r.opts.Stderr:
		return "streams"
	case stderr == io.Discard:
		return "capture"
	}
	return "other"
}

func (r *argvRecorder) runner() CmdRunner {
	return func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		if name != "git" {
			return 0, nil
		}
		argv := strings.Join(args, " ")
		r.calls = append(r.calls, argvCall{Argv: argv, Streams: r.streamsOf(stdout, stderr)})
		queue := r.scripts[argv]
		if len(queue) == 0 {
			return 0, nil
		}
		next := queue[0]
		if len(queue) > 1 {
			r.scripts[argv] = queue[1:]
		}
		if next.stdout != "" {
			_, _ = io.WriteString(stdout, next.stdout)
		}
		return next.exit, next.err
	}
}

// argvs renders the recorded calls with the project root templated as
// <root>, so the goldens are environment-free.
func (r *argvRecorder) argvs() []string {
	out := make([]string, 0, len(r.calls))
	for _, c := range r.calls {
		argv := c.Argv
		if r.opts != nil && r.opts.ProjectRoot != "" {
			argv = strings.ReplaceAll(argv, r.opts.ProjectRoot, "<root>")
		}
		out = append(out, argv+" ["+c.Streams+"]")
	}
	return out
}

// goldenError is the ShipError shape the goldens record.
type goldenError struct {
	Code    string            `json:"code"`
	Class   string            `json:"class"`
	Stage   string            `json:"stage"`
	Message string            `json:"message"`
	Debug   map[string]string `json:"debug"`
}

// goldenRow is one observed scenario: argv + streams, logs, the result's
// fields and the error.
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

func goldenErrorOf(t *testing.T, err error) *goldenError {
	t.Helper()
	if err == nil {
		return nil
	}
	se, ok := core.AsShipError(err)
	if !ok {
		t.Fatalf("not a ShipError: %v", err)
	}
	debug := map[string]string{}
	for k, v := range se.Debug {
		debug[k] = v
	}
	return &goldenError{Code: string(se.Code), Class: string(se.Class), Stage: string(se.Stage), Message: se.Message, Debug: debug}
}

func loadGolden(t *testing.T, name string) goldenFile {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("landing", "testdata", name))
	if err != nil {
		t.Fatalf("golden %s missing — captured on %s before the move, never regenerated silently: %v", name, goldenCapturedAt, err)
	}
	var g goldenFile
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatal(err)
	}
	if g.CapturedAt != goldenCapturedAt {
		t.Fatalf("golden %s captured at %q, want %s", name, g.CapturedAt, goldenCapturedAt)
	}
	return g
}

// pinOptions is the host fixture every pin drives: a temp project root, the
// operator streams as buffers, a fake integrator lock, cycle 7, no release.
func pinOptions(t *testing.T, class Class) (*Options, *strings.Builder, *strings.Builder) {
	t.Helper()
	var stdout, stderr strings.Builder
	opts := &Options{
		Class:         class,
		CommitMessage: "feat: pinned landing",
		ProjectRoot:   t.TempDir(),
		CycleID:       7,
		Env:           map[string]string{"EVOLVE_" + "SHIP_RELEASE_NOTES": ""},
		Stdout:        &stdout,
		Stderr:        &stderr,
		shipLock:      func(string) (func(), error) { return func() {}, nil },
	}
	opts.PluginRoot = opts.ProjectRoot
	return opts, &stdout, &stderr
}

// scriptGreenPost scripts the post-push reads every green landing performs.
func scriptGreenPost(r *argvRecorder) {
	r.on("rev-parse HEAD", scriptedCall{stdout: pinHead + "\n"})
	r.on("rev-parse HEAD^{tree}", scriptedCall{stdout: pinTree + "\n"})
}

// integrateRow drives worktreeShip.integrate() for one reset × merge × fleet
// combination and returns what it observed.
func integrateRow(t *testing.T, resetOK, mergeOK, fleet bool) (goldenRow, *strings.Builder) {
	t.Helper()
	opts, _, stderr := pinOptions(t, ClassCycle)
	if fleet {
		opts.Env["EVOLVE_FLEET"] = "1"
	}
	r := newArgvRecorder(opts)
	if !resetOK {
		r.on("checkout HEAD -- go/evolve", scriptedCall{exit: 1})
	}
	if !mergeOK {
		r.on("merge --ff-only "+pinCycleBranch, scriptedCall{exit: 128})
	}
	scriptGreenPost(r)
	res := &RunResult{}
	s := newWorktreeShip(context.Background(), opts, res, pinBranch, filepath.Join(opts.ProjectRoot, "wt"))
	s.cycleBranch = pinCycleBranch
	err := s.integrate()
	name := "reset-" + onOff(resetOK, "ok", "fail") + "/merge-" + onOff(mergeOK, "ok", "diverged") + "/fleet-" + onOff(fleet, "on", "off")
	return goldenRow{Name: name, Argv: r.argvs(), Logs: res.Logs, CommitSHA: res.CommitSHA,
		RepairAttempted: res.RepairAttempted, RepairOutcome: res.RepairOutcome, Error: goldenErrorOf(t, err)}, stderr
}

func onOff(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// pushSiteDriver drives one of the three push sites to its push with the
// stage/commit prelude scripted green.
type pushSiteDriver struct {
	name  string
	drive func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error
}

var pushSites = []pushSiteDriver{
	{name: "direct", drive: func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error {
		opts.Class = ClassManual
		r.on("diff --cached --quiet", scriptedCall{exit: 1}) // staged changes exist
		return shipDirect(context.Background(), opts, res, pinBranch)
	}},
	{name: "worktree", drive: func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error {
		s := newWorktreeShip(context.Background(), opts, res, pinBranch, filepath.Join(opts.ProjectRoot, "wt"))
		s.cycleBranch = pinCycleBranch
		return s.integrate()
	}},
	{name: "pushonly", drive: func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error {
		opts.PushOnly = true
		mustWrite(t, shipJournalPath(opts.ProjectRoot), `{"sha":"`+pinHead+`","class":"cycle","ts":"2026-09-14T00:00:00Z"}`+"\n")
		r.on("rev-parse --abbrev-ref HEAD", scriptedCall{stdout: pinBranch + "\n"})
		r.on("rev-list origin/"+pinBranch+"..HEAD", scriptedCall{stdout: pinHead + "\n"})
		return runPushOnly(context.Background(), opts, res)
	}},
}

// repairRow scripts the rejected push and one repair scenario.
type repairRow struct {
	name   string
	script func(opts *Options, r *argvRecorder)
}

var repairRows = []repairRow{
	{name: "push-ok", script: func(_ *Options, r *argvRecorder) {}},
	{name: "fetch-rc1", script: func(_ *Options, r *argvRecorder) {
		r.on("fetch origin "+pinBranch, scriptedCall{exit: 1})
	}},
	{name: "fetch-spawn-error", script: func(_ *Options, r *argvRecorder) {
		r.on("fetch origin "+pinBranch, scriptedCall{err: errors.New("spawn: no git")})
	}},
	{name: "origin-ref-error", script: func(_ *Options, r *argvRecorder) {
		r.on("rev-parse origin/"+pinBranch, scriptedCall{exit: 128})
	}},
	{name: "head-error", script: func(_ *Options, r *argvRecorder) {
		r.on("rev-parse origin/"+pinBranch, scriptedCall{stdout: pinOriginRef + "\n"})
		r.on("rev-parse HEAD", scriptedCall{err: errors.New("spawn: no git")})
	}},
	{name: "already-pushed", script: func(_ *Options, r *argvRecorder) {
		r.on("rev-parse origin/"+pinBranch, scriptedCall{stdout: pinHead + "\n"})
	}},
	{name: "ancestor-retry-ok", script: func(_ *Options, r *argvRecorder) {
		r.on("rev-parse origin/"+pinBranch, scriptedCall{stdout: pinOriginRef + "\n"})
		r.on("merge-base --is-ancestor "+pinOriginRef+" HEAD", scriptedCall{exit: 0})
	}},
	{name: "ancestor-retry-rc1", script: func(_ *Options, r *argvRecorder) {
		r.on("rev-parse origin/"+pinBranch, scriptedCall{stdout: pinOriginRef + "\n"})
		r.on("merge-base --is-ancestor "+pinOriginRef+" HEAD", scriptedCall{exit: 0})
		r.set("push origin "+pinBranch, scriptedCall{exit: 1}, scriptedCall{exit: 1}) // the retry is rejected too
	}},
	{name: "diverged", script: func(_ *Options, r *argvRecorder) {
		r.on("rev-parse origin/"+pinBranch, scriptedCall{stdout: pinOriginRef + "\n"})
		r.on("merge-base --is-ancestor "+pinOriginRef+" HEAD", scriptedCall{exit: 1})
	}},
	{name: "once-guard-preset", script: func(opts *Options, _ *argvRecorder) {
		opts.repairAttempted = map[core.ShipErrorCode]bool{core.CodeGitPushRejected: true}
	}},
	{name: "dry-run", script: func(opts *Options, _ *argvRecorder) {
		opts.DryRun = true
	}},
}

// pushRow drives one site through one repair scenario. Every row but
// push-ok rejects the first push (rc=1); the worktree site's own wording
// probe (`rev-parse HEAD`) answers the head so its message can name it.
func pushRow(t *testing.T, site pushSiteDriver, row repairRow) goldenRow {
	t.Helper()
	opts, _, _ := pinOptions(t, ClassCycle)
	r := newArgvRecorder(opts)
	if row.name != "push-ok" {
		r.on("push origin "+pinBranch, scriptedCall{exit: 1}, scriptedCall{exit: 0})
	}
	scriptGreenPost(r)
	row.script(opts, r)
	res := &RunResult{}
	err := site.drive(t, opts, r, res)
	return goldenRow{Name: site.name + "/" + row.name, Argv: r.argvs(), Logs: res.Logs, CommitSHA: res.CommitSHA,
		RepairAttempted: res.RepairAttempted, RepairOutcome: res.RepairOutcome, Error: goldenErrorOf(t, err)}
}

// dryRunPushSites are the sites a --dry-run row reaches the push on: the
// direct path returns before its push under DryRun (gitops.go), so only the
// worktree integrate (called after run()'s own DryRun return) and push-only
// (native.go runs it before any DryRun gate) carry the row.
func dryRunApplies(site pushSiteDriver) bool { return site.name != "direct" }

func assertRowsMatchGolden(t *testing.T, got []goldenRow, golden goldenFile) {
	t.Helper()
	if len(got) != len(golden.Rows) {
		t.Fatalf("%d rows observed, golden has %d", len(got), len(golden.Rows))
	}
	for i, want := range golden.Rows {
		g := got[i]
		if g.Name != want.Name {
			t.Fatalf("row %d: %s observed, golden %s", i, g.Name, want.Name)
		}
		if strings.Join(g.Argv, "\n") != strings.Join(want.Argv, "\n") {
			t.Errorf("%s: argv\n got %q\nwant %q", g.Name, g.Argv, want.Argv)
		}
		if strings.Join(g.Logs, "\n") != strings.Join(want.Logs, "\n") {
			t.Errorf("%s: logs\n got %q\nwant %q", g.Name, g.Logs, want.Logs)
		}
		if g.CommitSHA != want.CommitSHA || g.RepairAttempted != want.RepairAttempted || g.RepairOutcome != want.RepairOutcome {
			t.Errorf("%s: result {sha=%q repair=%q/%q}, golden {sha=%q repair=%q/%q}", g.Name,
				g.CommitSHA, g.RepairAttempted, g.RepairOutcome, want.CommitSHA, want.RepairAttempted, want.RepairOutcome)
		}
		assertGoldenError(t, g.Name, g.Error, want.Error)
	}
}

// assertGoldenError compares a ShipError with the golden's; the `step` Debug
// key is the unit's ONE declared addition to the moved errors (07 doc §8 c)
// and is compared by the leaf's own tests, so it is excluded here.
func assertGoldenError(t *testing.T, name string, got, want *goldenError) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("%s: error %+v, golden %+v", name, got, want)
		return
	}
	if got == nil {
		return
	}
	if got.Code != want.Code || got.Class != want.Class || got.Stage != want.Stage || got.Message != want.Message {
		t.Errorf("%s: error %+v, golden %+v", name, got, want)
	}
	for k, v := range want.Debug {
		if got.Debug[k] != v {
			t.Errorf("%s: Debug[%s]=%q, golden %q", name, k, got.Debug[k], v)
		}
	}
	for k := range got.Debug {
		if _, ok := want.Debug[k]; !ok && k != "step" {
			t.Errorf("%s: Debug gained %s=%q", name, k, got.Debug[k])
		}
	}
}

// Test 1 — the ff-merge over reset ok/fail × merge ok/diverged × fleet on/off:
// argv order and streams, the OK line, the class-by-fleet error and its
// Debug keys, byte-identical to the capture. Kills: merge before reset, the
// fleet class flipped, cycle_branch dropped, the OK line moved after the push.
func TestWorktreeShipIntegrate_GoldenArgvLogsAndErrors(t *testing.T) {
	golden := loadGolden(t, "landing_integrate.golden.json")
	var got []goldenRow
	for _, resetOK := range []bool{true, false} {
		for _, mergeOK := range []bool{true, false} {
			for _, fleet := range []bool{false, true} {
				row, _ := integrateRow(t, resetOK, mergeOK, fleet)
				got = append(got, row)
			}
		}
	}
	assertRowsMatchGolden(t, got, golden)
}

// Test 2 — the three push sites × the repair matrix, byte-identical to the
// capture: argv (the probes, the retry, no rebase/force), the REPAIR lines,
// RepairAttempted/RepairOutcome, CommitSHA, and the error per site. Kills:
// the once-guard dropped, declined building a new error, needs-reaudit made
// transient, the retry push streamed to io.Discard, the worktree head omitted.
func TestPushSites_GoldenRejectionAndRepairMatrix(t *testing.T) {
	for _, site := range pushSites {
		t.Run(site.name, func(t *testing.T) {
			golden := loadGolden(t, "push_"+site.name+".golden.json")
			var got []goldenRow
			for _, row := range repairRows {
				if row.name == "dry-run" && !dryRunApplies(site) {
					continue
				}
				got = append(got, pushRow(t, site, row))
			}
			assertRowsMatchGolden(t, got, golden)
			for _, r := range got {
				for _, a := range r.Argv {
					if strings.Contains(a, "rebase") || strings.Contains(a, "--force") || strings.Contains(a, " -f") {
						t.Errorf("%s: the landing never rebases or force-pushes: %s", r.Name, a)
					}
				}
			}
		})
	}
}

// Test 3 — the binding writer's bytes (2-space indent, trailing newline, an
// empty commit_sha still present), the 0o755 dir, no temp file left, the
// no-cycle_id error and the MkdirAll error. Kills: the newline dropped,
// MarshalIndent → Marshal, Rename skipped.
func TestWriteShipBinding_GoldenBytesAndAtomicity(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("landing", "testdata", "ship-binding.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	opts := &Options{ProjectRoot: t.TempDir(), CycleID: 42, internalAuditBoundTreeSHA: "bound-tree"}
	if err := writeShipBinding(opts, "committed-tree", " "+pinHead+"\n"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(opts.ProjectRoot, ".evolve", "runs", "cycle-42")
	got, err := os.ReadFile(filepath.Join(dir, "ship-binding.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("ship-binding.json bytes\n got %q\nwant %q", got, want)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm()&0o700 != 0o700 {
		t.Errorf("run dir: %v %v", info, err)
	}
	if tmps, _ := filepath.Glob(filepath.Join(dir, "ship-binding.*.tmp")); len(tmps) != 0 {
		t.Errorf("temp files left behind: %v", tmps)
	}
	empty := &Options{ProjectRoot: t.TempDir(), CycleID: 43}
	if err := writeShipBinding(empty, "", ""); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(filepath.Join(empty.ProjectRoot, ".evolve", "runs", "cycle-43", "ship-binding.json")); !strings.Contains(string(body), `"commit_sha": ""`) {
		t.Errorf("an empty commit_sha is still written (the delivery identity has no omitempty): %s", body)
	}
	noCycle := &Options{ProjectRoot: t.TempDir()}
	if err := writeShipBinding(noCycle, "t", "c"); err == nil || err.Error() != "no cycle_id in cycle-state.json" {
		t.Errorf("no cycle_id: %v", err)
	}
	blocked := &Options{ProjectRoot: t.TempDir(), CycleID: 44}
	mustWrite(t, filepath.Join(blocked.ProjectRoot, ".evolve", "runs"), "a file\n")
	if err := writeShipBinding(blocked, "t", "c"); err == nil {
		t.Error("a file where the runs dir should be must fail the write")
	}
}

// Test 4 — the writer and the idempotency reader agree on the path: the
// consumer pin on the run-workspace layout (core.RunWorkspacePath +
// dossier.ShipBindingFile), green before and after the projection. Kills:
// cycle-%d → cycle_%d at either site.
func TestShipBinding_WriterAndIdempotencyReaderAgreeOnThePath(t *testing.T) {
	opts, _, _ := pinOptions(t, ClassCycle)
	opts.internalAuditBoundTreeSHA = pinTree
	r := newArgvRecorder(opts)
	scriptGreenPost(r)
	if err := writeShipBinding(opts, pinTree, pinHead); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(core.RunWorkspacePath(opts.ProjectRoot, 7), dossier.ShipBindingFile)
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the binding must land at the run-workspace SSOT path: %v", err)
	}
	sha, idempotent, err := checkPostPushIdempotency(context.Background(), opts)
	if err != nil || !idempotent || sha != pinHead {
		t.Errorf("reader: sha=%q idempotent=%v err=%v", sha, idempotent, err)
	}
}

// Test 6 — the exit>1 rule and the two message texts of captureGitOutput:
// a spawn error and rc=2 are GIT_IO with git_args + git_err / git_rc; rc=1
// is success (git diff's "differences exist"). The intent moves to the leaf's
// Capture test with the move; the host keeps its spelling.
func TestCaptureGitOutput_ExitRuleAndErrorTexts(t *testing.T) {
	opts, _, _ := pinOptions(t, ClassCycle)
	r := newArgvRecorder(opts)
	r.on("rev-parse HEAD", scriptedCall{err: errors.New("boom")}, scriptedCall{exit: 2}, scriptedCall{stdout: "out\n", exit: 1})
	_, err := captureGitOutput(context.Background(), opts, "rev-parse", "HEAD")
	se := wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "ship: git [rev-parse HEAD]: boom")
	if se.Debug["git_args"] != "[rev-parse HEAD]" || se.Debug["git_err"] != "boom" {
		t.Errorf("spawn error Debug: %v", se.Debug)
	}
	_, err = captureGitOutput(context.Background(), opts, "rev-parse", "HEAD")
	se = wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "ship: git [rev-parse HEAD] exited 2")
	if se.Debug["git_args"] != "[rev-parse HEAD]" || se.Debug["git_rc"] != "2" {
		t.Errorf("rc=2 Debug: %v", se.Debug)
	}
	out, err := captureGitOutput(context.Background(), opts, "rev-parse", "HEAD")
	if err != nil || out != "out\n" {
		t.Errorf("rc=1 is success: %q %v", out, err)
	}
}
