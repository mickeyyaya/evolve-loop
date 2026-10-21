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
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

const (
	inputFile   = "agent-input.md"
	ledgerKind  = "agent_run"
	ledgerSrc   = "evolve agent run"
	runsSubpath = "ephemeral/agent-runs"
)

type Envelope struct {
	Profile     string            `json:"profile"`
	RunLabel    string            `json:"run_label"`
	Verdict     string            `json:"verdict"`
	Report      string            `json:"report"`
	Model       string            `json:"model"`
	ModelSource string            `json:"model_source,omitempty"`
	CostUSD     float64           `json:"cost_usd"`
	Tokens      core.TokenUsage   `json:"tokens"`
	DurationMS  int64             `json:"duration_ms"`
	Diagnostics []core.Diagnostic `json:"diagnostics,omitempty"`
	Workspace   string            `json:"workspace,omitempty"`
	Error       string            `json:"error,omitempty"`
}

type runOptions struct {
	profile string
	input   string
	files   fileList
	asJSON  bool
	keep    bool
	root    string
}

type standaloneRun struct {
	spec      phasespec.PhaseSpec
	root      string
	label     string
	workspace string
	input     string
}

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
	r := standaloneRun{root: projectRoot(o.root)}
	if r.spec, err = admit(o.profile, r.root); err != nil {
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
	if err := stageWorkspace(r.workspace, r.input); err != nil {
		fmt.Fprintf(stderr, "evolve agent run: create the run workspace: %v\n", err)
		return exitDispatch
	}
	if !o.keep {
		defer removeWorkspace(r.workspace, stderr)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	resp, runErr := phasecmd.NewSpecRunner(r.spec, r.root, c.deps.Bridge(r.root)).Run(ctx, r.request())
	report, hasReport := readReport(r)
	ledgerErr := appendLedger(r, resp, report, hasReport, runErr, now)
	emit(o, r, resp, report, errors.Join(runErr, ledgerErr), stdout)
	return finish(resp, runErr, ledgerErr, stderr)
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

func stageWorkspace(workspace, input string) error {
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
		RunID:       r.label,
		Context:     map[string]string{specrunner.AgentInputKey: r.input},
	}
}

func readReport(r standaloneRun) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(r.workspace, specrunner.ArtifactName(r.spec)))
	return string(raw), err == nil
}

func appendLedger(r standaloneRun, resp core.PhaseResponse, report string, hasReport bool, runErr error, now time.Time) error {
	entry := core.LedgerEntry{
		TS: now.UTC().Format(time.RFC3339), CycleLabel: r.label, Role: r.spec.Name, Kind: ledgerKind,
		Model: resp.ResolvedModel, ExitCode: exitCode(resp, runErr), DurationS: strconv.FormatInt(resp.DurationMS/1000, 10),
		Source: ledgerSrc, RunID: r.label,
	}
	if hasReport {
		sum := sha256.Sum256([]byte(report))
		entry.ArtifactSHA256 = hex.EncodeToString(sum[:])
	}
	return ledger.New(paths.EvolveDirOf(r.root)).Append(context.Background(), entry)
}

func emit(o runOptions, r standaloneRun, resp core.PhaseResponse, report string, failure error, stdout io.Writer) {
	if !o.asJSON {
		fmt.Fprint(stdout, report)
		return
	}
	env := Envelope{
		Profile: r.spec.Name, RunLabel: r.label, Verdict: resp.Verdict, Report: report,
		Model: resp.ResolvedModel, ModelSource: resp.ModelSource, CostUSD: resp.CostUSD, Tokens: resp.Tokens,
		DurationMS: resp.DurationMS, Diagnostics: resp.Diagnostics,
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

func finish(resp core.PhaseResponse, runErr, ledgerErr error, stderr io.Writer) int {
	if runErr != nil {
		fmt.Fprintf(stderr, "evolve agent run: %s: %v\n", resp.Phase, runErr)
	}
	if ledgerErr != nil {
		fmt.Fprintf(stderr, "evolve agent run: append the ledger entry: %v\n", ledgerErr)
		return exitDispatch
	}
	return exitCode(resp, runErr)
}
