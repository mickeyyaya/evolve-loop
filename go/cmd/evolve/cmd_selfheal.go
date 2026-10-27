package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	selfhealUsage    = "usage: evolve selfheal incidents|propose [--json] [--project-root P]"
	selfhealExitIn   = 11
	selfhealCIFields = "databaseId,headBranch,status,conclusion,attempt,createdAt,updatedAt"
	selfhealCILimit  = "200"
	selfhealGHWait   = 60 * time.Second
)

const (
	selfhealVerbIncidents = "incidents"
	selfhealVerbPropose   = "propose"
)

var selfhealSchemas = map[string]string{selfhealVerbIncidents: "evolve.selfheal.incidents/1", selfhealVerbPropose: "evolve.selfheal.propose/1"}

var selfhealChannels = []string{"errors", "loop"}

type selfhealEnv struct {
	agent     agentSeat
	now       func() time.Time
	gh        func(ctx context.Context, root string, args ...string) (string, error)
	declared  func(family string) bool
	families  func() []string
	newCenter func(root, evolveDir string, console io.Writer) (*signalcenter.Center, func())
}

type selfhealTransition struct {
	Kind     string `json:"kind"`
	Incident string `json:"incident"`
}

type selfhealReport struct {
	Stage       string               `json:"stage"`
	Incidents   []selfheal.Incident  `json:"incidents"`
	Transitions []selfhealTransition `json:"transitions"`
}

func runSelfheal(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runSelfhealWith(selfhealEnv{agent: runAgentCommand, now: time.Now, gh: ghOutput, declared: bridge.UsageWindowsDeclared, families: bridge.InteractiveFamilies, newCenter: newSelfhealCenter}, args, stdout, stderr)
}

func newSelfhealCenter(root, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	return newRoleSignalCenter(roleSelfheal, root, evolveDir, console)
}

type selfhealArgs struct {
	verb, root string
	json       bool
}

func runSelfhealWith(env selfhealEnv, args []string, stdout, stderr io.Writer) int {
	a, err := parseSelfhealArgs(args)
	w := codeEnvelopeWriter{schema: selfhealSchemas[cmp.Or(a.verb, selfhealVerbIncidents)], json: a.json, stdout: stdout, stderr: stderr}
	if err != nil {
		return w.fail(exitUsage, "usage", "evolve selfheal: "+err.Error()+"\n"+selfhealUsage)
	}
	evolveDir := paths.EvolveDirOf(a.root)
	pol, err := policy.Load(filepath.Join(evolveDir, paths.PolicyFile))
	if err != nil {
		return w.fail(selfhealExitIn, "input", "evolve selfheal: "+err.Error())
	}
	if a.verb == selfhealVerbPropose {
		return runSelfhealPropose(env, w, a.root, pol)
	}
	cfg := pol.HealingDetector()
	existing, err := selfheal.LoadIncidents(selfheal.Dir(evolveDir))
	if err != nil {
		return w.fail(exitIO, "io", "evolve selfheal: "+err.Error())
	}
	report := selfhealReport{Stage: cfg.Stage, Incidents: existing, Transitions: []selfhealTransition{}}
	if cfg.Stage != policy.HealingStageOff {
		report, w.warnings, err = detectIncidents(env, a.root, cfg, existing, stderr)
		if err != nil {
			return w.fail(exitIO, "io", "evolve selfheal: "+err.Error())
		}
	}
	printSelfheal(stdout, a.json, report)
	return w.ok(report)
}

func parseSelfhealArgs(args []string) (selfhealArgs, error) {
	a := selfhealArgs{json: slices.Contains(args, "--json")}
	if len(args) == 0 || selfhealSchemas[args[0]] == "" {
		return a, errors.New("want the incidents or the propose verb")
	}
	a.verb = args[0]
	operands, err := (cliFlags{bools: map[string]*bool{"--json": &a.json}, values: map[string]*string{"--project-root": &a.root}}).parse(args[1:])
	switch {
	case err != nil:
		return a, err
	case len(operands) > 0:
		return a, fmt.Errorf("unexpected argument %q", operands[0])
	}
	a.root = paths.AbsoluteRoot("--project-root", a.root, nil)
	return a, nil
}

func detectIncidents(env selfhealEnv, root string, cfg policy.HealingDetectorPolicy, existing []selfheal.Incident, stderr io.Writer) (selfhealReport, []string, error) {
	now := env.now()
	obs, verifier, warnings, err := gatherIncidentEvidence(env, root, cfg, now)
	if err != nil {
		return selfhealReport{}, nil, err
	}
	for _, warning := range warnings {
		fmt.Fprintln(stderr, "evolve selfheal: WARN: "+warning)
	}
	next, transitions := selfheal.Detect(existing, obs, verifier, now)
	if err := writeChangedIncidents(selfheal.Dir(paths.EvolveDirOf(root)), existing, next); err != nil {
		return selfhealReport{}, nil, err
	}
	report := selfhealReport{Stage: cfg.Stage, Incidents: next, Transitions: []selfhealTransition{}}
	for _, t := range transitions {
		report.Transitions = append(report.Transitions, selfhealTransition{Kind: t.Kind, Incident: t.Incident.ID})
	}
	publishTransitions(env, root, cfg.Stage, transitions, stderr)
	return report, warnings, nil
}

func gatherIncidentEvidence(env selfhealEnv, root string, cfg policy.HealingDetectorPolicy, now time.Time) ([]selfheal.Observation, selfheal.TreeVerifier, []string, error) {
	evolveDir := paths.EvolveDirOf(root)
	since := now.Add(-hours(cfg.WindowHours))
	events, err := selfheal.LoadSignals(paths.EventChannelsRoot(evolveDir), selfhealChannels, since)
	if err != nil {
		return nil, selfheal.TreeVerifier{}, nil, err
	}
	obs := selfheal.FromSignals(events)
	var warnings []string
	runs, err := selfhealCIRuns(env, root, since)
	if err != nil {
		warnings = append(warnings, "the CI rules did not run: "+err.Error())
	}
	obs = append(obs, selfheal.FromCIRuns(runs, now)...)
	cycleObs, cycleWarnings := cycleObservations(evolveDir, cfg.Cycles)
	obs, warnings = append(obs, cycleObs...), append(warnings, cycleWarnings...)
	usage, err := selfheal.LoadUsage(evolveDir, env.declared)
	if err != nil {
		warnings = append(warnings, "the usage rule did not run: "+err.Error())
	}
	obs = append(obs, selfheal.FromUsage(usage, now)...)
	waves, err := wave.NewStore(evolveDir).Records()
	if err != nil {
		warnings = append(warnings, "no incident can close, the wave records are unreadable: "+err.Error())
	}
	return obs, selfheal.TreeVerifier{Root: root, MainGreen: selfheal.MainGreen(runs), Waves: waves}, warnings, nil
}

func cycleObservations(evolveDir string, limit int) ([]selfheal.Observation, []string) {
	shipped, err := selfheal.LoadShipped(shipjournal.Path(evolveDir))
	if err != nil {
		return nil, []string{"the cycle rules did not run: " + err.Error()}
	}
	cycles, warnings, err := selfheal.LoadCycles(filepath.Join(evolveDir, "runs"), limit, shipped)
	if err != nil {
		return nil, []string{"the cycle rules did not run: " + err.Error()}
	}
	return selfheal.FromCycles(cycles), warnings
}

type ghCIRun struct {
	DatabaseID int64     `json:"databaseId"`
	HeadBranch string    `json:"headBranch"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	Attempt    int       `json:"attempt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func selfhealCIRuns(env selfhealEnv, root string, since time.Time) ([]selfheal.CIRun, error) {
	ctx, cancel := context.WithTimeout(context.Background(), selfhealGHWait)
	defer cancel()
	out, err := env.gh(ctx, root, "run", "list", "--workflow", ciparity.RequiredWorkflow, "--limit", selfhealCILimit, "--json", selfhealCIFields)
	if err != nil {
		return nil, err
	}
	var raw []ghCIRun
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("gh run list: decode: %w", err)
	}
	var runs []selfheal.CIRun
	for _, r := range raw {
		if !r.CreatedAt.Before(since) {
			runs = append(runs, selfheal.CIRun{ID: r.DatabaseID, Branch: r.HeadBranch, Status: r.Status, Conclusion: r.Conclusion, Attempt: r.Attempt, Created: r.CreatedAt, Updated: r.UpdatedAt})
		}
	}
	return runs, nil
}

func writeChangedIncidents(dir string, before, after []selfheal.Incident) error {
	old := map[string]selfheal.Incident{}
	for _, inc := range before {
		old[inc.ID] = inc
	}
	for _, inc := range after {
		if prev, ok := old[inc.ID]; ok && reflect.DeepEqual(prev, inc) {
			continue
		}
		if err := selfheal.WriteIncident(dir, inc); err != nil {
			return err
		}
	}
	return nil
}

func publishTransitions(env selfhealEnv, root, stage string, transitions []selfheal.Transition, stderr io.Writer) {
	if len(transitions) == 0 {
		return
	}
	center, closeCenter := env.newCenter(root, paths.EvolveDirOf(root), stderr)
	defer closeCenter()
	for _, t := range transitions {
		center.Emit(t.Event(stage))
	}
}

func printSelfheal(stdout io.Writer, asJSON bool, r selfhealReport) {
	if asJSON {
		return
	}
	open := 0
	for _, inc := range r.Incidents {
		if inc.State == selfheal.StateOpen {
			open++
		}
	}
	fmt.Fprintf(stdout, "selfheal incidents: stage %s, %d record(s), %d open, %d transition(s)\n", r.Stage, len(r.Incidents), open, len(r.Transitions))
	for _, inc := range r.Incidents {
		fmt.Fprintf(stdout, "%s %s %s %s %s recurrence=%d: %s\n", inc.ID, inc.State, inc.Severity, inc.Tier, inc.Fingerprint, inc.Recurrence, inc.Summary)
	}
}
