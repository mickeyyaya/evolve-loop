package agentcmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

const (
	inputFile   = "agent-input.md"
	ledgerKind  = "agent_run"
	ledgerSrc   = "evolve agent run"
	runsSubpath = "ephemeral/agent-runs"
)

type Envelope struct {
	Profile      string                   `json:"profile"`
	RunLabel     string                   `json:"run_label"`
	Verdict      string                   `json:"verdict"`
	Findings     reportdoc.SeverityCounts `json:"findings"`
	Report       string                   `json:"report"`
	Model        string                   `json:"model"`
	ModelSource  string                   `json:"model_source,omitempty"`
	CostUSD      float64                  `json:"cost_usd"`
	Tokens       core.TokenUsage          `json:"tokens"`
	DurationMS   int64                    `json:"duration_ms"`
	Diagnostics  []core.Diagnostic        `json:"diagnostics,omitempty"`
	CLI          string                   `json:"cli,omitempty"`
	Family       string                   `json:"family,omitempty"`
	LaunchRecord string                   `json:"launch_record,omitempty"`
	Worktree     string                   `json:"worktree,omitempty"`
	Workspace    string                   `json:"workspace,omitempty"`
	Error        string                   `json:"error,omitempty"`
}

type runOptions struct {
	profile  string
	input    string
	files    fileList
	worktree string
	writable fileList
	asJSON   bool
	keep     bool
	root     string
}

type standaloneRun struct {
	spec      phasespec.PhaseSpec
	root      string
	label     string
	workspace string
	input     string
	worktree  string
	writable  []string
}

type launchOutcome struct {
	resp       core.PhaseResponse
	runErr     error
	report     string
	hasReport  bool
	cli        string
	family     string
	model      string
	verdict    string
	findings   reportdoc.SeverityCounts
	writer     bool
	treeBefore string
	treeAfter  string
}

type launchSpy struct {
	core.Bridge
	signals *signalcenter.Center
	cli     string
	model   string
}

func (s *launchSpy) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	s.cli, s.model = req.CLI, req.Model
	return s.Bridge.Launch(ctx, req)
}

func (s *launchSpy) Signals() *signalcenter.Center { return s.signals }

func parseRun(args []string, stderr io.Writer) (runOptions, error) {
	var o runOptions
	if len(args) == 0 || args[0] == "" || args[0][0] == '-' {
		return o, refusal{exitUsage, "missing profile: the first argument of `evolve agent run` names the profile, before any flag (see `evolve agent list`)"}
	}
	o.profile = args[0]
	fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.input, "input", "", "read the input from FILE (- is stdin)")
	fs.Var(&o.files, "file", "attach PATH as one fenced section (repeatable)")
	fs.StringVar(&o.worktree, "worktree", "", "run the agent in the linked git worktree DIR")
	fs.Var(&o.writable, "writable", "let a writer change PATH of the worktree (repeatable)")
	fs.BoolVar(&o.asJSON, "json", false, "print one JSON envelope instead of the report")
	fs.BoolVar(&o.keep, "keep", false, "keep the run workspace")
	fs.StringVar(&o.root, "project-root", "", "project root (default EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args[1:]); err != nil {
		return o, refusal{exitUsage, err.Error()}
	}
	if fs.NArg() > 0 {
		return o, refusal{exitUsage, fmt.Sprintf("unexpected argument %q", fs.Arg(0))}
	}
	return o, nil
}

func (c command) prepare(args []string, stdin io.Reader, stderr io.Writer) (runOptions, standaloneRun, error) {
	o, err := parseRun(args, stderr)
	if err != nil {
		return o, standaloneRun{}, err
	}
	if subagent.ReadDispatchDepth(c.deps.Getenv) > 0 {
		return o, standaloneRun{}, refusal{exitRefused, "refused inside a phase: a phase agent must not start another agent run"}
	}
	r := standaloneRun{root: projectRoot(o.root), worktree: o.worktree, writable: o.writable}
	if r.spec, err = admit(o, r.root); err != nil {
		return o, r, err
	}
	if r.input, err = composeInput(o.input, o.files, stdin); err != nil {
		return o, r, err
	}
	if err := c.deps.InstallRouter(r.root); err != nil {
		return o, r, refusal{exitRefused, fmt.Sprintf("install the root router: %v", err)}
	}
	return o, r, nil
}

func (c command) run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, r, err := c.prepare(args, stdin, stderr)
	if err != nil {
		return refuse(err, stderr)
	}
	now := c.deps.Now()
	r.label = runLabel(r.spec.Name, now)
	r.workspace = filepath.Join(paths.EvolveDirOf(r.root), runsSubpath, r.label)
	if err := stageWorkspace(r, r.input); err != nil {
		fmt.Fprintf(stderr, "evolve agent run: create the run workspace: %v\n", err)
		return exitDispatch
	}
	if !o.keep {
		defer removeWorkspace(r.workspace, stderr)
	}
	signals, closeSignals := c.deps.Signals(r.root, stderr)
	defer closeSignals()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	out := c.launch(ctx, r, signals, stderr)
	recordPath, recordErr := writeLaunchRecord(r, out, now)
	recordErr = errors.Join(appendLedger(r, out, now), recordErr)
	emit(o, r, out, recordPath, errors.Join(out.runErr, recordErr), stdout)
	return finish(out.resp, out.runErr, recordErr, stderr)
}

func (c command) launch(ctx context.Context, r standaloneRun, signals *signalcenter.Center, stderr io.Writer) launchOutcome {
	spy := &launchSpy{Bridge: c.deps.Bridge(r.root, signals), signals: signals}
	out := launchOutcome{treeBefore: c.treeState(ctx, r.treeDir(), stderr)}
	out.resp, out.runErr = phasecmd.NewSpecRunner(r.spec, r.root, spy).Run(ctx, r.request())
	out.treeAfter = c.treeState(context.WithoutCancel(ctx), r.treeDir(), stderr)
	out.report, out.hasReport = readReport(r)
	out.cli, out.model, out.family = spy.cli, spy.model, llmroute.Family(spy.cli)
	out.writer = len(r.writable) > 0
	if out.runErr == nil {
		out = out.withReportVerdict(r, signals)
	}
	return out
}

func (c command) treeState(ctx context.Context, dir string, stderr io.Writer) string {
	sum, err := c.deps.TreeState(ctx, dir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve agent run: no tree state for the launch record: %v\n", err)
		return ""
	}
	return sum
}

func (r standaloneRun) treeDir() string {
	if r.worktree != "" {
		return r.worktree
	}
	return r.root
}

func refuse(err error, stderr io.Writer) int {
	code := exitDispatch
	var r refusal
	if errors.As(err, &r) {
		code = r.code
	}
	fmt.Fprintf(stderr, "evolve agent run: %v\n", err)
	return code
}

func runLabel(profile string, now time.Time) string {
	suffix := (now.Nanosecond() ^ os.Getpid()) & 0xffff
	return fmt.Sprintf("agent-%s-%s-%04x", profile, now.UTC().Format("20060102T150405Z"), suffix)
}

func stageWorkspace(r standaloneRun, input string) error {
	record, err := launchrecord.Path(paths.EvolveDirOf(r.root), r.label)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(record); err == nil {
		return fmt.Errorf("the run label %s is taken: its launch record exists", r.label)
	}
	workspace := r.workspace
	if err := os.MkdirAll(filepath.Dir(workspace), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(workspace, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspace, inputFile), []byte(input), 0o644)
}

func removeWorkspace(workspace string, stderr io.Writer) {
	if err := os.RemoveAll(workspace); err != nil {
		fmt.Fprintf(stderr, "evolve agent run: remove the run workspace: %v\n", err)
	}
}

func (r standaloneRun) request() core.PhaseRequest {
	return core.PhaseRequest{
		ProjectRoot: r.root,
		Workspace:   r.workspace,
		Worktree:    r.worktree,
		Fence:       core.FenceFor(r.worktree == "", r.writable),
		RunID:       r.label,
		Context:     map[string]string{specrunner.AgentInputKey: r.input},
	}
}

func readReport(r standaloneRun) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(r.workspace, specrunner.ArtifactName(r.spec)))
	return string(raw), err == nil
}

func (out launchOutcome) reportSHA() string {
	if !out.hasReport {
		return ""
	}
	sum := sha256.Sum256([]byte(out.report))
	return hex.EncodeToString(sum[:])
}

func (out launchOutcome) launchedModel() string {
	if out.resp.ResolvedModel != "" {
		return out.resp.ResolvedModel
	}
	return out.model
}

func appendLedger(r standaloneRun, out launchOutcome, now time.Time) error {
	entry := core.LedgerEntry{
		TS: now.UTC().Format(time.RFC3339), CycleLabel: r.label, Role: r.spec.Name, Kind: ledgerKind,
		Model: out.launchedModel(), ExitCode: exitCode(out.resp, out.runErr), DurationS: strconv.FormatInt(out.resp.DurationMS/1000, 10),
		Source: ledgerSrc, RunID: r.label, ArtifactSHA256: out.reportSHA(),
	}
	if err := ledger.New(paths.EvolveDirOf(r.root)).Append(context.Background(), entry); err != nil {
		return fmt.Errorf("append the ledger entry: %w", err)
	}
	return nil
}

func writeLaunchRecord(r standaloneRun, out launchOutcome, now time.Time) (string, error) {
	rec := launchrecord.Record{
		RunID: r.label, Profile: r.spec.Name, CLI: out.cli, Family: out.family, Model: out.launchedModel(),
		Verdict: out.verdict, WritesSource: out.writer, TreeBefore: out.treeBefore, TreeAfter: out.treeAfter, ArtifactSHA256: out.reportSHA(),
		Worktree: r.worktree, TS: now.UTC().Format(time.RFC3339),
	}
	path, err := launchrecord.Write(paths.EvolveDirOf(r.root), rec)
	if err != nil {
		return "", fmt.Errorf("write the launch record: %w", err)
	}
	return path, nil
}

func emit(o runOptions, r standaloneRun, out launchOutcome, recordPath string, failure error, stdout io.Writer) {
	if !o.asJSON {
		fmt.Fprint(stdout, out.report)
		return
	}
	resp := out.resp
	env := Envelope{
		Profile: r.spec.Name, RunLabel: r.label, Verdict: resp.Verdict, Findings: out.findings, Report: out.report,
		Model: out.launchedModel(), ModelSource: resp.ModelSource, CostUSD: resp.CostUSD, Tokens: resp.Tokens,
		DurationMS: resp.DurationMS, Diagnostics: resp.Diagnostics,
		CLI: out.cli, Family: out.family, LaunchRecord: recordPath, Worktree: r.worktree,
	}
	if o.keep {
		env.Workspace = r.workspace
	}
	if failure != nil {
		env.Error = failure.Error()
	}
	printJSON(stdout, env)
}

func exitCode(resp core.PhaseResponse, runErr error) int {
	switch {
	case runErr != nil:
		return exitDispatch
	case resp.Verdict == core.VerdictFAIL:
		return exitFail
	}
	return 0
}

func finish(resp core.PhaseResponse, runErr, recordErr error, stderr io.Writer) int {
	if runErr != nil {
		fmt.Fprintf(stderr, "evolve agent run: %s: %v\n", resp.Phase, runErr)
	}
	if recordErr != nil {
		fmt.Fprintf(stderr, "evolve agent run: %v\n", recordErr)
		return exitDispatch
	}
	return exitCode(resp, runErr)
}
