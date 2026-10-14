package ship

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
	pinLaneTree      = "66666666666666666666666666666666abcdef02"
	pinItemBlob      = "55555555555555555555555555555555abcdef03"
)

type scriptedCall struct {
	stdout string
	exit   int
	err    error
}

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

func (r *argvRecorder) set(argv string, calls ...scriptedCall) *argvRecorder {
	r.scripts[argv] = calls
	return r
}

func (r *argvRecorder) streamsOf(stdout, stderr io.Writer) string {
	switch {
	case stdout == io.Discard && stderr == io.Discard:
		return "discard"
	case r.opts != nil && stdout == r.opts.Stdout:
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

func scriptGreenPost(r *argvRecorder) {
	r.on("rev-parse HEAD", scriptedCall{stdout: pinHead + "\n"})
	r.on("rev-parse HEAD^{tree}", scriptedCall{stdout: pinTree + "\n"})
}

func scriptGreenLanding(r *argvRecorder, wt string) {
	scriptGreenPost(r)
	r.on("-C "+wt+" write-tree", scriptedCall{stdout: pinTree + "\n"})
	r.on("-C "+wt+" -c commit.gpgsign=false commit-tree "+pinTree+" -p HEAD -m feat: pinned landing", scriptedCall{stdout: pinHead + "\n"})
	r.on("ls-tree "+pinHead+"^ -- .evolve/inbox/item.json", scriptedCall{stdout: "100644 blob " + pinItemBlob + "\t.evolve/inbox/item.json\n"})
	r.on("write-tree", scriptedCall{stdout: pinLaneTree + "\n"})
	r.on("-C "+wt+" rev-parse HEAD", scriptedCall{stdout: pinHead + "\n"})
	r.on("rev-parse "+pinHead+"^{tree}", scriptedCall{stdout: pinTree + "\n"})
	r.on("rev-parse "+pinBranch, scriptedCall{stdout: pinOriginRef + "\n"})
}

type pushSiteDriver struct {
	name  string
	drive func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error
}

var pushSites = []pushSiteDriver{
	{name: "direct", drive: func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error {
		opts.Class = ClassManual
		r.on("diff --cached --quiet", scriptedCall{exit: 1})
		return shipDirect(context.Background(), opts, res, pinBranch)
	}},
	{name: "pushonly", drive: func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error {
		opts.PushOnly = true
		mustWrite(t, shipJournalPath(opts.ProjectRoot), `{"sha":"`+pinHead+`","class":"cycle","ts":"2026-09-14T00:00:00Z"}`+"\n")
		r.on("rev-parse --abbrev-ref HEAD", scriptedCall{stdout: pinBranch + "\n"})
		r.on("rev-list origin/"+pinBranch+"..HEAD", scriptedCall{stdout: pinHead + "\n"})
		return runPushOnly(context.Background(), opts, res)
	}},
}

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
		r.set("push origin "+pinBranch, scriptedCall{exit: 1}, scriptedCall{exit: 1})
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

func TestPushSites_GoldenRejectionAndRepairMatrix(t *testing.T) {
	t.Parallel()
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

func TestWriteShipBinding_GoldenBytesAndAtomicity(t *testing.T) {
	t.Parallel()
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

func TestShipBinding_WriterAndIdempotencyReaderAgreeOnThePath(t *testing.T) {
	t.Parallel()
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

func TestCaptureGitOutput_ExitRuleAndErrorTexts(t *testing.T) {
	t.Parallel()
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
