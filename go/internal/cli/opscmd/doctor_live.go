package opscmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

var doctorUsageEvidenceFn = productionDoctorUsageEvidence

func productionDoctorUsageEvidence(projectRoot string, deps bridge.Deps) (usageevidence.Explain, func()) {
	ws, err := os.MkdirTemp("", "evolve-doctor-usage-*")
	if err != nil {
		return func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
			return usageprobe.Evidence{Family: driver, Verdict: usageprobe.VerdictUnavailable, Detail: "usage workspace: " + err.Error()}
		}, func() {}
	}
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		pol = policy.Policy{}
	}
	factory := bridge.NewControllerFactory(projectRoot, ws, "doctor-usage", deps)
	explain := usageevidence.New(usageevidence.Options{ProjectRoot: projectRoot, Config: pol.CLIHealthConfig(), Probe: usageevidence.UsagePane(factory), Log: deps.Stderr})
	return explain, func() { _ = os.RemoveAll(ws) }
}

type doctorRun struct {
	projectRoot, driver string
	deps                bridge.Deps
}

func doctorUsage(run doctorRun, rc int) *usageprobe.Evidence {
	if rc == bridge.ExitOK || rc == bridge.ExitBadFlags {
		return nil
	}
	explain, cleanup := doctorUsageEvidenceFn(run.projectRoot, run.deps)
	defer cleanup()
	if explain == nil {
		return nil
	}
	ev := explain(context.Background(), run.driver, time.Now())
	return &ev
}

func printUsage(stderr io.Writer, usage *usageprobe.Evidence) {
	if usage == nil {
		return
	}
	fmt.Fprintf(stderr, "[doctor] usage: %s\n", usage.Summary())
	for _, w := range usage.Windows {
		state := "ok"
		if w.Exhausted {
			state = "exhausted"
		}
		fmt.Fprintf(stderr, "[doctor]   %s (%s)\n", w.Evidence(), state)
	}
}

var trustDialogTail = regexp.MustCompile(`(?i)\btrust (the files in |the contents of )?this (folder|project|directory)\b`)

func liveFailingStage(rc int, scrollback string) string {
	switch {
	case rc == bridge.ExitMissingBinary, rc == bridge.ExitSafetyGate, rc == bridge.ExitCostLeak:
		return "launch"
	case trustDialogTail.MatchString(bridge.ScrollbackTail(scrollback, 12)):
		return "trust prompt"
	case rc == bridge.ExitREPLBootTimeout, rc == bridge.ExitModelMismatch:
		return "REPL prompt"
	case rc == bridge.ExitArtifactTimeout, rc == bridge.ExitRequireFullUnmet:
		return "artifact"
	default:
		return "delivery"
	}
}

func printFailingStage(stderr io.Writer, rc int, scrollback string) {
	fmt.Fprintf(stderr, "[doctor] failing stage: %s\n", liveFailingStage(rc, scrollback))
}

type liveProbeTarget struct {
	driver string
	model  string
}

func (p liveProbeTarget) String() string {
	if p.model == "" {
		return p.driver
	}
	return fmt.Sprintf("%s --model %q", p.driver, p.model)
}

type liveOutcome struct {
	target              liveProbeTarget
	asJSON              bool
	rc                  int
	pattern, scrollback string
	servedModel         string
	usage               *usageprobe.Evidence
}

const servedModelUnreported = "unreported"

func servedModelLabel(rc int, model string) string {
	switch {
	case rc == bridge.ExitBadFlags:
		return ""
	case model == "":
		return servedModelUnreported
	default:
		return model
	}
}

func writeLiveJSON(stdout io.Writer, o liveOutcome, served string) {
	buf, _ := json.MarshalIndent(struct {
		Driver      string               `json:"driver"`
		Model       string               `json:"model,omitempty"`
		ServedModel string               `json:"served_model,omitempty"`
		ExitCode    int                  `json:"exit_code"`
		Healthy     bool                 `json:"healthy"`
		Pattern     string               `json:"pattern,omitempty"`
		Usage       *usageprobe.Evidence `json:"usage,omitempty"`
	}{o.target.driver, o.target.model, served, o.rc, o.rc == bridge.ExitOK, o.pattern, o.usage}, "", "  ")
	fmt.Fprintf(stdout, "%s\n", buf)
}

func reportDoctorLiveResult(o liveOutcome, stdout, stderr io.Writer) int {
	target, rc, pattern, scrollback, usage := o.target, o.rc, o.pattern, o.scrollback, o.usage
	served := servedModelLabel(rc, o.servedModel)
	if o.asJSON {
		writeLiveJSON(stdout, o, served)
	}
	if served != "" {
		fmt.Fprintf(stderr, "[doctor] served model: %s\n", served)
	}

	switch {
	case rc == bridge.ExitOK:
		fmt.Fprintf(stderr, "[doctor] LIVE OK: %s answered the probe\n", target)
		return exitOK
	case rc == bridge.ExitBadFlags:
		fmt.Fprintf(stderr, "[doctor] live: %q is not a known *-tmux driver\n", target.driver)
		return exitUsage
	case rc == bridge.ExitModelMismatch:
		fmt.Fprintf(stderr, "[doctor] LIVE WRONG MODEL: %s rc=%d — the REPL booted a model outside the target's model family, or showed no readable model label, so the prompt was never sent\n", target, rc)
		if tail := bridge.ScrollbackTail(scrollback, 6); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		printUsage(stderr, usage)
		printFailingStage(stderr, rc, scrollback)
		return exitFail
	case pattern != "":
		fmt.Fprintf(stderr, "[doctor] LIVE WALLED: %s rc=%d pattern=%s\n", target, rc, pattern)
		if tail := bridge.ScrollbackTail(scrollback, 6); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		printUsage(stderr, usage)
		printFailingStage(stderr, rc, scrollback)
		return exitFail
	default:
		fmt.Fprintf(stderr, "[doctor] LIVE FAILED: %s rc=%d\n", target, rc)
		if tail := bridge.ScrollbackTail(scrollback, 12); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		printUsage(stderr, usage)
		printFailingStage(stderr, rc, scrollback)
		return exitFail
	}
}

func runDoctorLive(args []string, stdout, stderr io.Writer) int {
	return runDoctorLiveWith(args, stdout, bridge.Deps{Stderr: stderr, TokenResolver: tokenusage.DefaultResolver(tokenusage.ClaudeConfigRoot(nil))})
}

func runDoctorLiveWith(args []string, stdout io.Writer, deps bridge.Deps) int {
	stderr := deps.Stderr
	fs := flag.NewFlagSet("evolve doctor live", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	target := liveProbeTarget{}
	fs.BoolVar(&asJSON, "json", false, "emit JSON payload")
	fs.StringVar(&target.model, "model", "", "launch this model: a tier (fast|balanced|deep|top) or a model name the CLI accepts")
	positional, err := cmdutil.ParseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "evolve doctor live: usage: evolve doctor live <driver> [--model <model>] [--json]")
		return exitUsage
	}
	target.driver = positional[0]

	ws, err := os.MkdirTemp("", "evolve-doctorlive-*")
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor live: temp workspace: %v\n", err)
		return exitFail
	}
	defer func() { _ = os.RemoveAll(ws) }()
	cwd, _ := os.Getwd()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	probe := bridge.LiveSmokeTest(ctx, target.driver, &bridge.Config{Workspace: ws, ProjectRoot: cwd, Model: target.model}, deps)

	usage := doctorUsage(doctorRun{projectRoot: cwd, driver: target.driver, deps: deps}, probe.RC)
	return reportDoctorLiveResult(liveOutcome{target: target, asJSON: asJSON, rc: probe.RC, pattern: probe.Pattern, scrollback: probe.Scrollback, servedModel: probe.ServedModel, usage: usage}, stdout, stderr)
}
