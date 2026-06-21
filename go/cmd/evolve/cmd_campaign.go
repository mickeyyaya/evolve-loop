package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseregistrar"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const campaignUsage = `Usage:
  evolve campaign study --workspace <cycle-workspace>
  evolve campaign replan --workspace <cycle-workspace> --feedback <text>
  evolve campaign run --plan <campaign-plan.json> [--simulate] [--concurrency <n>] [--project-root <path>] [--ignore-progress]
  evolve campaign status --plan <campaign-plan.json> [--project-root <path>]
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
	case "status":
		return runCampaignStatus(args[1:], stdout, stderr)
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
	ignoreProgress := fs.Bool("ignore-progress", false, "start fresh, ignoring saved progress (default: auto-resume completed waves when the plan is unchanged)")
	cycleTimeout := fs.Duration("cycle-timeout", 2*time.Hour, "per-cycle deadline; a cycle exceeding it is reaped so it can't hang the whole campaign (0 = no deadline)")
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
	goalHash := campaignGoalHash(plan)
	rawPlan, rerr := os.ReadFile(*planPath)
	if rerr != nil {
		fmt.Fprintf(stderr, "evolve campaign run: read plan for progress key: %v\n", rerr)
		return 1
	}
	progressPath := campaign.ProgressPath(campaignEvolveDir(*projectRoot), goalHash)
	for wi := range waves {
		for i := range waves[wi] {
			waves[wi][i].GoalHash = goalHash
		}
	}
	// Cross-session ownership lease (ADR-0059): a real run takes the exclusive
	// goal-hash lease in the git common dir (shared by every worktree) so a
	// second autonomous session on the SAME plan refuses-or-attaches instead of
	// clobbering the incumbent. --simulate is a dry plumbing check, not an owned
	// run, so it does not take ownership. The flock frees on our exit (defer) or
	// death, so a dead owner never blocks the next run.
	if !*simulate {
		lease, lerr := campaign.AcquireOwnership(campaignLeaseDir(*projectRoot), goalHash, campaignOwnerSelf(*projectRoot))
		if lerr != nil {
			var held *campaign.HeldError
			if errors.As(lerr, &held) {
				fmt.Fprintf(stderr, "evolve campaign run: %v\n", held)
				return 1
			}
			fmt.Fprintf(stderr, "evolve campaign run: ownership lease: %v\n", lerr)
			return 1
		}
		defer lease.Release()
	}
	// SIGINT/SIGTERM cancels in-flight cycles (exec.CommandContext reaps the
	// children); RunWaves then returns and the progress checkpoint up to the last
	// completed wave survives, so --resume picks up where the interrupt hit.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	supervisor := &fleet.Supervisor{
		Concurrency:  *concurrency,
		CycleTimeout: *cycleTimeout,
		Launch:       campaignLaunchFactory(binPath, *simulate, *projectRoot, stdout, stderr),
	}
	runner := func(rctx context.Context, wave []fleet.CycleSpec) []fleet.Result {
		fmt.Fprintf(stderr, "[campaign] running wave: %d cycle(s)\n", len(wave))
		return supervisor.Run(rctx, wave)
	}
	if err := campaign.RunWaves(ctx, waves, runner, campaign.RunOptions{
		ProgressPath: progressPath,
		PlanSHA:      campaign.HashPlan(rawPlan),
		Resume:       !*ignoreProgress,
		MaxRetries:   1, // one batched retry of a wave's failed cycles before abort
		Cooldown:     campaignQuotaCooldown(*projectRoot),
		BeforeWave: func() {
			// Clear recovered quota benches before each wave so a wave doesn't
			// re-hit a wall that already lifted (and re-bench ones still walled).
			runCLIHealthCanary(*projectRoot, nil, defaultLiveProbe(*projectRoot, stderr), stderr)
		},
	}); err != nil {
		fmt.Fprintf(stderr, "evolve campaign run: %v\n", err)
		return 1
	}
	return 0
}

// campaignGoalHash is the SSOT for the campaign goal hash — used both as each
// cycle's --goal-hash and as the progress-file key, so run and status agree.
func campaignGoalHash(plan *campaign.Plan) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(plan.Goal)))
}

// campaignQuotaCooldown returns a Cooldown hook reporting how long to wait before
// a retry for the longest-active CLI quota bench to expire (capped at MaxCooldown),
// so a walled wave backs off instead of retrying straight into the wall. 0 when no
// family is benched (the common, non-quota failure path retries immediately).
func campaignQuotaCooldown(projectRoot string) func() time.Duration {
	return func() time.Duration {
		store := clihealth.NewStore(projectRoot, nil)
		var wait time.Duration
		for _, e := range store.Active() {
			if d := time.Until(e.BenchedUntil); d > wait {
				wait = d
			}
		}
		if wait > clihealth.MaxCooldown {
			wait = clihealth.MaxCooldown
		}
		return wait
	}
}

// campaignEvolveDir resolves the writable .evolve directory that holds campaign
// progress state, mirroring the projectRoot/.evolve convention used elsewhere.
// The root is always absolutized so a relative --project-root on the `status`
// path resolves the SAME progress file the `run` path wrote (run absolutizes its
// root before this; status calls in with the raw flag) — otherwise --resume reads
// a different (empty) checkpoint and never sees completed waves.
func campaignEvolveDir(projectRoot string) string {
	root := projectRoot
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	} else {
		// Abs only fails if Getwd fails (cwd deleted mid-run); a relative root here
		// would silently reproduce the resume/status mismatch this guards against.
		fmt.Fprintf(os.Stderr, "[campaign] WARN: could not absolutize progress root %q: %v\n", root, err)
	}
	return filepath.Join(root, ".evolve")
}

// campaignLeaseDir resolves the directory that holds the cross-session ownership
// lease (ADR-0059). It uses the git COMMON dir — shared by every linked worktree
// of a repo — so two sessions running the same plan from different worktrees
// contend on the SAME lease file. Off a git repo (tests, non-repo roots) it
// falls back to the worktree-local .evolve so each isolated root self-contains.
func campaignLeaseDir(projectRoot string) string {
	root := projectRoot
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out, err := gitexec.Default(root).Output(ctx, "rev-parse", "--git-common-dir"); err == nil {
		if cd := strings.TrimSpace(out); cd != "" {
			if !filepath.IsAbs(cd) {
				cd = filepath.Join(root, cd)
			}
			if abs, aerr := filepath.Abs(cd); aerr == nil {
				cd = abs
			}
			return filepath.Join(cd, "evolve", "campaign-leases")
		}
	}
	return filepath.Join(campaignEvolveDir(projectRoot), "campaign-leases")
}

// campaignOwnerSelf builds this process's ownership record for the lease — PID,
// worktree, host, and start time are informational (the flock is the liveness
// signal), surfaced in the refuse message and `campaign status`.
func campaignOwnerSelf(projectRoot string) campaign.Owner {
	worktree := projectRoot
	if worktree == "" {
		if wd, err := os.Getwd(); err == nil {
			worktree = wd
		}
	}
	host, _ := os.Hostname()
	return campaign.Owner{
		PID:       os.Getpid(),
		Worktree:  worktree,
		Host:      host,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// runCampaignStatus reports a campaign's wave/cycle progress from the durable
// checkpoint — the single queryable view an operator needs for a multi-hour run.
func runCampaignStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve campaign status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	planPath := fs.String("plan", "", "campaign-plan.json to report on")
	projectRoot := fs.String("project-root", "", "project root (locates .evolve progress)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *planPath == "" {
		fmt.Fprintln(stderr, "evolve campaign status: --plan is required")
		return 2
	}
	plan, err := loadVerifiedCampaignPlan(*planPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign status: %v\n", err)
		return 1
	}
	waves, err := plan.Waves()
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign status: %v\n", err)
		return 1
	}
	goalHash := campaignGoalHash(plan)
	prog, err := campaign.LoadProgress(campaign.ProgressPath(campaignEvolveDir(*projectRoot), goalHash))
	if err != nil {
		fmt.Fprintf(stderr, "evolve campaign status: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "campaign %s: %d/%d waves complete, %d cycles shipped, %d quarantined\n",
		goalHash[:8], len(prog.CompletedWaves), len(waves), len(prog.CompletedCycleIDs), len(prog.FailedCycleIDs))
	for i, wave := range waves {
		state := "pending"
		if prog.IsWaveComplete(i) {
			state = "done"
		}
		fmt.Fprintf(stdout, "  wave %d/%d [%s]: %d cycle(s)\n", i+1, len(waves), state, len(wave))
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
