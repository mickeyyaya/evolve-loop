package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func parseWaveNextArgs(args []string) (waveNextArgs, error) {
	a := waveNextArgs{}
	var merge, number string
	operands, err := cliFlags{
		bools:  map[string]*bool{"--dry-run": &a.dryRun, "--json": &a.json},
		values: map[string]*string{"--merge": &merge, "--max-cycles": &a.boundary.maxCycles, "--number": &number, "--project-root": &a.boundary.projectRoot},
		lists:  map[string]*[]string{"--note": &a.notes},
	}.parse(args)
	switch {
	case err != nil:
		return a, err
	case len(operands) > 0:
		return a, fmt.Errorf("unexpected argument %q", operands[0])
	case slices.ContainsFunc(a.notes, func(n string) bool { return strings.TrimSpace(n) == "" }):
		return a, errors.New("--note needs a text")
	}
	if a.boundary.maxCycles != "" {
		if n, err := strconv.Atoi(a.boundary.maxCycles); err != nil || n <= 0 {
			return a, fmt.Errorf("--max-cycles %q must be a positive number", a.boundary.maxCycles)
		}
	}
	if number != "" {
		if a.number, err = strconv.Atoi(number); err != nil || a.number <= 0 {
			return a, fmt.Errorf("--number %q must be a positive number", number)
		}
	}
	a.boundary.prs, err = boundaryPRs(merge)
	return a, err
}

func loadWaveInputs(plane wavePlaneDirs) (policy.WaveConfig, string, error) {
	pol, err := policy.Load(filepath.Join(plane.evolveDir, "policy.json"))
	if err != nil {
		return policy.WaveConfig{}, "", err
	}
	cfg := pol.WaveSettings()
	path := cfg.StandingGoal
	if !filepath.IsAbs(path) {
		path = filepath.Join(plane.root, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, "", fmt.Errorf("read the standing goal (policy wave.standing_goal): %w", err)
	}
	if strings.TrimSpace(string(b)) == "" {
		return cfg, "", fmt.Errorf("the standing goal %s is empty", path)
	}
	return cfg, string(b), nil
}

func composeWaveGoal(env waveEnv, plan *wavePlan, cfg policy.WaveConfig, standing string, records []wave.Record) error {
	queued, err := plan.plane.store.Notes()
	if err != nil {
		return err
	}
	plan.notes = queued
	notes := slices.Clone(queued)
	for _, text := range plan.args.notes {
		notes = append(notes, wave.Note{Text: strings.TrimSpace(text)})
	}
	in := wave.GoalInput{Next: plan.number, Standing: standing, Notes: notes}
	if len(records) == 0 {
		plan.warnings = append(plan.warnings, "no earlier wave is recorded; the goal has no facts section")
	} else {
		last := records[len(records)-1]
		plan.last = &last
		plan.facts = lastWaveFacts(env, plan, last)
		plan.facts.Limit = cfg.FactsMaxCycles
		in.Last = &plan.facts
		in.History = records[max(0, len(records)-1-cfg.HistoryK) : len(records)-1]
	}
	plan.goal = wave.Compose(in)
	return nil
}

func lastWaveFacts(env waveEnv, plan *wavePlan, last wave.Record) wave.Facts {
	cycles, warnings := wave.ReadCycles(plan.plane.root, last.CycleFloor, 0)
	plan.warnings = append(plan.warnings, warnings...)
	prs := slices.Clone(plan.args.boundary.prs)
	if last.MainSHA != "" {
		merged, err := env.mergedPRs(plan.plane.root, last.MainSHA)
		if err != nil {
			plan.warnings = append(plan.warnings, err.Error())
		}
		prs = append(prs, merged...)
	}
	slices.SortFunc(prs, func(x, y string) int {
		a, _ := strconv.Atoi(x)
		b, _ := strconv.Atoi(y)
		return a - b
	})
	return wave.Facts{Wave: last.Number, RunID: last.RunID, Cycles: cycles, MergedPRs: slices.Compact(prs)}
}

func waveBoundaryArgs(env waveEnv, plan wavePlan, cfg policy.WaveConfig) boundaryArgs {
	b := plan.args.boundary
	b.goalText = strings.TrimSpace(plan.goal)
	b.runID = gcpolicy.LogRunID(env.now())
	if b.maxCycles == "" {
		b.maxCycles = strconv.Itoa(cfg.MaxCycles)
	}
	return b
}

func waveSteps(a boundaryArgs, root string, drivers []string) []boundaryStep {
	at := []string{"--project-root", root}
	steps := []boundaryStep{{verb: "checkpoint", args: append([]string{"save", "--all"}, at...)}}
	steps = append(steps, boundaryHaltSteps(a, root)...)
	steps = append(steps,
		boundaryStep{verb: waveBuildVerb, args: at, desc: "make -C " + filepath.Join(root, "go") + " build"},
		boundaryStep{verb: "reset-sha", args: append([]string{"--operator"}, at...)})
	for _, d := range drivers {
		steps = append(steps, boundaryStep{verb: "doctor", args: []string{"live", d}})
	}
	launch := boundaryLaunchSteps(a, root)
	last := &launch[len(launch)-1]
	last.desc = waveLaunchDesc(*last, a.goalText)
	return append(steps, launch...)
}

func waveLaunchDesc(s boundaryStep, goal string) string {
	args := slices.Clone(s.args)
	if i := slices.Index(args, "--goal-text"); i >= 0 && i+1 < len(args) {
		args[i+1] = fmt.Sprintf("<goal: %d bytes>", len(goal))
	}
	return "evolve " + s.verb + " " + strings.Join(args, " ")
}
