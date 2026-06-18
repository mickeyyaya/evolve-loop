package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseregistrar"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const campaignUsage = `Usage:
  evolve campaign study --workspace <cycle-workspace>
  evolve campaign replan --workspace <cycle-workspace> --feedback <text>
  evolve campaign run --plan <campaign-plan.json> [--simulate] [--concurrency <n>] [--project-root <path>]
`

var campaignLaunchFactory = execCycleLaunch

func runCampaign(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, campaignUsage)
		return 2
	}
	switch args[0] {
	case "study":
		return runCampaignStudy(args[1:], stdout, stderr)
	case "replan":
		return runCampaignReplan(args[1:], stdout, stderr)
	case "run":
		return runCampaignRun(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve campaign: unknown subcommand %q\n%s", args[0], campaignUsage)
		return 2
	}
}

func runCampaignStudy(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve campaign study", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "cycle workspace that receives campaign-plan.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" {
		fmt.Fprintln(stderr, "evolve campaign study: --workspace is required")
		return 2
	}
	if err := runPreliminaryStudy(*workspace, ""); err != nil {
		fmt.Fprintf(stderr, "evolve campaign study: %v\n", err)
		return 1
	}
	return renderCampaignPlan(filepath.Join(*workspace, "campaign-plan.json"), stdout, stderr)
}

func runCampaignReplan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve campaign replan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "cycle workspace containing campaign-plan.json")
	feedback := fs.String("feedback", "", "operator feedback for the revised study")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || strings.TrimSpace(*feedback) == "" {
		fmt.Fprintln(stderr, "evolve campaign replan: --workspace and --feedback are required")
		return 2
	}
	planPath := filepath.Join(*workspace, "campaign-plan.json")
	previous, err := campaign.LoadFile(planPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign replan: load prior plan: %v\n", err)
		return 1
	}
	if err := previous.Verify(); err != nil {
		fmt.Fprintf(stderr, "evolve campaign replan: verify prior plan: %v\n", err)
		return 1
	}
	if err := runPreliminaryStudy(*workspace, *feedback); err != nil {
		fmt.Fprintf(stderr, "evolve campaign replan: %v\n", err)
		return 1
	}
	updated, err := loadVerifiedCampaignPlan(planPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign replan: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, campaign.Diff(previous, updated))
	rendered, err := updated.Render()
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign replan: render: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, rendered)
	return 0
}

func runCampaignRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve campaign run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	planPath := fs.String("plan", "", "campaign-plan.json to execute")
	simulate := fs.Bool("simulate", false, "exercise wave execution without LLM calls")
	concurrency := fs.Int("concurrency", campaign.MaxWaveWidth, "max concurrent cycles")
	projectRoot := fs.String("project-root", "", "project root passed to each cycle")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *planPath == "" {
		fmt.Fprintln(stderr, "evolve campaign run: --plan is required")
		return 2
	}
	if *projectRoot != "" {
		absoluteRoot, err := filepath.Abs(*projectRoot)
		if err != nil {
			fmt.Fprintf(stderr, "evolve campaign run: resolve project root: %v\n", err)
			return 1
		}
		*projectRoot = absoluteRoot
	}
	plan, err := loadVerifiedCampaignPlan(*planPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign run: %v\n", err)
		return 1
	}
	waves, err := plan.Waves()
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign run: plan waves: %v\n", err)
		return 1
	}
	binPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign run: resolve binary: %v\n", err)
		return 1
	}
	goalHash := fmt.Sprintf("%x", sha256.Sum256([]byte(plan.Goal)))
	supervisor := &fleet.Supervisor{
		Concurrency: *concurrency,
		Launch:      campaignLaunchFactory(binPath, *simulate, *projectRoot, stdout, stderr),
	}
	for waveIndex, wave := range waves {
		for i := range wave {
			wave[i].GoalHash = goalHash
		}
		fmt.Fprintf(stderr, "[campaign] wave %d/%d: %d cycle(s)\n", waveIndex+1, len(waves), len(wave))
		results := supervisor.Run(context.Background(), wave)
		for _, result := range results {
			if result.Err == nil && result.ExitCode == 0 {
				continue
			}
			fmt.Fprintf(stderr, "[campaign] wave %d cycle %d failed; retrying only that cycle\n", waveIndex+1, result.Index)
			retry := supervisor.Run(context.Background(), []fleet.CycleSpec{wave[result.Index]})
			if len(retry) != 1 || retry[0].Err != nil || retry[0].ExitCode != 0 {
				fmt.Fprintf(stderr, "evolve campaign run: wave %d cycle %d failed after localized retry\n", waveIndex+1, result.Index)
				return 1
			}
		}
	}
	return 0
}

func loadVerifiedCampaignPlan(path string) (*campaign.Plan, error) {
	plan, err := campaign.LoadFile(path)
	if err != nil {
		return nil, err
	}
	if err := plan.Verify(); err != nil {
		return nil, err
	}
	return plan, nil
}

func renderCampaignPlan(path string, stdout, stderr io.Writer) int {
	plan, err := loadVerifiedCampaignPlan(path)
	if err != nil {
		fmt.Fprintf(stderr, "campaign: %v\n", err)
		return 1
	}
	rendered, err := plan.Render()
	if err != nil {
		fmt.Fprintf(stderr, "campaign: render: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, rendered)
	return 0
}

func runPreliminaryStudy(workspace, feedback string) error {
	workspace, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	worktree := sourceRoot()
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(workspace)))
	cfgPath := filepath.Join(worktree, ".evolve", "phases", "preliminary-study", "phase.json")
	cfg, err := phaseconfig.Load(cfgPath)
	if err != nil {
		return err
	}
	prompt, err := os.ReadFile(filepath.Join(filepath.Dir(cfgPath), "agent.md"))
	if err != nil {
		return fmt.Errorf("read preliminary-study prompt: %w", err)
	}
	cfg.Prompt = string(prompt)
	registered, err := (phaseregistrar.Registrar{
		Bridge:  bridge.NewDefault(projectRoot),
		Prompts: prompts.NewForProject(worktree),
	}).Register(cfg)
	if err != nil {
		return err
	}
	req := core.PhaseRequest{
		Cycle:       cycleFromWorkspace(workspace),
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Worktree:    worktree,
		Context:     map[string]string{"campaign_feedback": feedback},
	}
	resp, err := registered.Runner.Run(context.Background(), req)
	if err != nil {
		return err
	}
	if resp.Verdict != "" && resp.Verdict != core.VerdictPASS {
		return fmt.Errorf("preliminary-study verdict=%s", resp.Verdict)
	}
	return nil
}

func cycleFromWorkspace(workspace string) int {
	base := filepath.Base(filepath.Clean(workspace))
	n, _ := strconv.Atoi(strings.TrimPrefix(base, "cycle-"))
	return n
}
