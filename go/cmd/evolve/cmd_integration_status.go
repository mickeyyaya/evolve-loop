package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	verdictClean      = "clean"
	verdictNotClean   = "not clean"
	verdictNoEvidence = "no evidence"

	integrationStatusUsage = "usage: evolve integration status [--pool N] [--wave N] [--project-root P] [--json]\n  exit 0 = clean, 1 = not clean, no evidence or an unreadable record; 2 = an absent root, an unreadable wave store, or a wave number that the store does not know; 10 = usage"
	statusOrderNote        = "order: by delivery time; rank, risk and deps not supplied"
	statusEnforceNote      = "enforce: not ready (no enforce actor)"
)

type integrationStatusEvidence struct {
	Found        bool
	Record       integration.WaveRecord
	Shipped      []int
	ReadFailures []string
}

type integrationStatusMember struct {
	Cycle  int    `json:"cycle"`
	Status string `json:"status"`
	Landed bool   `json:"landed,omitempty"`
}

type integrationStatusReport struct {
	ProjectRoot string                         `json:"project_root"`
	Pool        int                            `json:"pool"`
	Wave        int                            `json:"wave,omitempty"`
	Stage       string                         `json:"stage"`
	Enforce     string                         `json:"enforce"`
	Order       string                         `json:"order"`
	Verdict     string                         `json:"verdict"`
	Failed      []string                       `json:"failed,omitempty"`
	Ended       bool                           `json:"ended"`
	Epoch       uint64                         `json:"epoch,omitempty"`
	ShadowBase  string                         `json:"shadow_base,omitempty"`
	LastCycle   int                            `json:"last_cycle,omitempty"`
	Counts      map[string]int                 `json:"counts"`
	Members     []integrationStatusMember      `json:"members"`
	Batches     []integration.IntegrationBatch `json:"batches,omitempty"`
	Decisions   []integration.FlushRecord      `json:"decisions,omitempty"`
	Failures    []integration.DriverFailure    `json:"failures,omitempty"`
}

type integrationStatusWaves struct {
	ProjectRoot string                    `json:"project_root"`
	Wave        int                       `json:"wave"`
	Pools       []integrationStatusReport `json:"pools"`
}

func composes(rec integration.WaveRecord) int {
	n := 0
	for _, m := range rec.Members {
		switch m.Status {
		case integration.MemberComposed, integration.MemberConflicted, integration.MemberConflictedBaseAhead:
			n++
		}
	}
	return n
}

func hasWaveEndDecision(rec integration.WaveRecord) bool {
	return slices.ContainsFunc(rec.Decisions, func(d integration.FlushRecord) bool { return d.Trigger == integration.TriggerWaveEnd })
}

func pendingAtEnd(rec integration.WaveRecord) []string {
	var out []string
	for _, c := range slices.Sorted(maps.Keys(rec.Members)) {
		if rec.Members[c].Status == integration.MemberPending {
			out = append(out, fmt.Sprintf("cycle %d is still pending at wave end", c))
		}
	}
	return out
}

func integrationVerdict(ev integrationStatusEvidence) (string, []string) {
	var failed []string
	for _, f := range ev.ReadFailures {
		failed = append(failed, "unreadable: "+f)
	}
	if !ev.Found {
		return verdictNotClean, append(failed, "no wave record")
	}
	rec := ev.Record
	if !rec.Ended {
		failed = append(failed, "the wave record has no Ended")
	}
	if !hasWaveEndDecision(rec) {
		failed = append(failed, "the wave record has no wave-end decision")
	}
	if rec.LastCycle < 1 {
		failed = append(failed, "the wave record has no last cycle, so the shipped cycles cannot be counted")
	}
	for _, c := range ev.Shipped {
		if _, ok := rec.Members[c]; !ok {
			failed = append(failed, fmt.Sprintf("missing record: cycle %d shipped and has no member", c))
		}
	}
	if rec.Ended {
		failed = append(failed, pendingAtEnd(rec)...)
	}
	for _, f := range rec.Failures {
		failed = append(failed, fmt.Sprintf("driver failure at %s: %s", f.Step, f.Error))
	}
	switch {
	case len(failed) > 0:
		return verdictNotClean, failed
	case composes(rec) == 0:
		return verdictNoEvidence, nil
	}
	return verdictClean, nil
}

func buildIntegrationStatus(pool int, stage string, ev integrationStatusEvidence) integrationStatusReport {
	verdict, failed := integrationVerdict(ev)
	rep := integrationStatusReport{
		Pool: pool, Stage: stage, Enforce: statusEnforceNote, Order: statusOrderNote, Verdict: verdict, Failed: failed,
		Counts: map[string]int{"shipped": len(ev.Shipped)}, Members: []integrationStatusMember{},
	}
	rec := ev.Record
	rep.Ended, rep.Epoch, rep.ShadowBase, rep.LastCycle = rec.Ended, rec.Epoch, rec.ShadowBase, rec.LastCycle
	rep.Batches, rep.Decisions, rep.Failures = rec.Batches, rec.Decisions, rec.Failures
	rep.Counts["members"] = len(rec.Members)
	for _, m := range rec.Members {
		rep.Counts[string(m.Status)]++
		if m.Landed {
			rep.Counts["landed"]++
		}
	}
	for _, c := range slices.Sorted(maps.Keys(rec.Members)) {
		m := rec.Members[c]
		rep.Members = append(rep.Members, integrationStatusMember{Cycle: c, Status: string(m.Status), Landed: m.Landed})
	}
	return rep
}

func gatherEvidence(root string, store integration.FileWaveStore, pool int) integrationStatusEvidence {
	var ev integrationStatusEvidence
	rec, found, err := store.Read(pool)
	if err != nil {
		ev.ReadFailures = append(ev.ReadFailures, err.Error())
		return ev
	}
	ev.Found, ev.Record = found, rec
	if !found || rec.LastCycle < 1 {
		return ev
	}
	rng := wave.ReadRunRange(root, pool, rec.LastCycle)
	for _, f := range rng.Failures {
		ev.ReadFailures = append(ev.ReadFailures, f.Error())
	}
	for _, c := range rng.Cycles {
		if c.Shipped {
			ev.Shipped = append(ev.Shipped, c.ID)
		}
	}
	return ev
}

func stageOf(root string) string {
	stage, err := policy.LoadOrDefaults(paths.EvolveDirOf(root)+"/policy.json", io.Discard).FleetIntegrationStage()
	if err != nil {
		return "invalid: " + err.Error()
	}
	return stage
}

type integrationWaveBounds struct {
	wave.Bounds
	Number int
}

func waveBoundsList(root string) ([]integrationWaveBounds, error) {
	recs, err := wave.NewStore(paths.EvolveDirOf(root)).Records()
	if err != nil {
		return nil, err
	}
	out := make([]integrationWaveBounds, len(recs))
	for i, r := range recs {
		out[i] = integrationWaveBounds{Bounds: wave.BoundsOf(recs, i), Number: r.Number}
	}
	return out, nil
}

func waveBoundsOf(root string, number int) (integrationWaveBounds, error) {
	list, err := waveBoundsList(root)
	if err != nil {
		return integrationWaveBounds{}, err
	}
	for _, b := range list {
		if b.Number == number {
			return b, nil
		}
	}
	return integrationWaveBounds{}, fmt.Errorf("wave %d has no record", number)
}

func waveOfPool(root string, pool int) int {
	list, err := waveBoundsList(root)
	if err != nil {
		return 0
	}
	for _, b := range list {
		if b.Holds(pool) {
			return b.Number
		}
	}
	return 0
}

func statusVerb(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	var pool, waveN, rootFlag string
	var asJSON bool
	operands, err := cliFlags{bools: map[string]*bool{"--json": &asJSON}, values: map[string]*string{"--pool": &pool, "--wave": &waveN, "--project-root": &rootFlag}}.parse(args)
	poolN, waveNum, err := statusSelectors(operands, pool, waveN, err)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", integrationPrefix, err, integrationStatusUsage)
		return exitUsage
	}
	root := paths.AbsoluteRoot("--project-root", chooseRoot(rootFlag, env.ProjectRootEnv, env.Root), nil)
	if rc := checkRootDir("integration status", root, stderr); rc != 0 {
		return rc
	}
	store := integration.FileWaveStore{Root: root}
	if waveNum > 0 {
		return statusOfWave(root, store, waveNum, asJSON, stdout, stderr)
	}
	if poolN == 0 {
		pools := store.Pools()
		if len(pools) == 0 {
			fmt.Fprintf(stderr, "%sno wave record exists under %s\n", integrationPrefix, root)
			return exitRefused
		}
		poolN = pools[len(pools)-1]
	}
	rep := reportOf(root, store, poolN)
	return printStatus(rep, asJSON, stdout, stderr)
}

func reportOf(root string, store integration.FileWaveStore, pool int) integrationStatusReport {
	rep := buildIntegrationStatus(pool, stageOf(root), gatherEvidence(root, store, pool))
	rep.Wave = waveOfPool(root, pool)
	rep.ProjectRoot = root
	return rep
}

func statusSelectors(operands []string, pool, waveN string, err error) (int, int, error) {
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected operand %q", operands[0])
	}
	if err == nil && pool != "" && waveN != "" {
		err = errors.New("--pool and --wave exclude each other")
	}
	poolN, waveNum := 0, 0
	if err == nil && pool != "" {
		if poolN, err = strconv.Atoi(pool); err == nil && poolN < 1 {
			err = fmt.Errorf("--pool %q must be a number above 0", pool)
		}
	}
	if err == nil && waveN != "" {
		if waveNum, err = strconv.Atoi(waveN); err == nil && waveNum < 1 {
			err = fmt.Errorf("--wave %q must be a number above 0", waveN)
		}
	}
	return poolN, waveNum, err
}

func statusOfWave(root string, store integration.FileWaveStore, number int, asJSON bool, stdout, stderr io.Writer) int {
	bounds, err := waveBoundsOf(root, number)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", integrationPrefix, err)
		return exitIO
	}
	set := integrationStatusWaves{ProjectRoot: root, Wave: number, Pools: []integrationStatusReport{}}
	for _, p := range store.Pools() {
		if bounds.Holds(p) {
			set.Pools = append(set.Pools, reportOf(root, store, p))
		}
	}
	if len(set.Pools) == 0 {
		fmt.Fprintf(stderr, "%swave %d has no pool with a wave record\n", integrationPrefix, number)
		return exitRefused
	}
	if asJSON {
		return printIntegrationJSON(set, worstExit(set.Pools), stdout, stderr)
	}
	code := 0
	for _, rep := range set.Pools {
		code = max(code, printStatus(rep, false, stdout, stderr))
	}
	return code
}

func worstExit(reps []integrationStatusReport) int {
	for _, r := range reps {
		if r.Verdict != verdictClean {
			return exitRefused
		}
	}
	return 0
}

func printIntegrationJSON(v any, code int, stdout, stderr io.Writer) int {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%sencode the status: %v\n", integrationPrefix, err)
		return exitIO
	}
	fmt.Fprintln(stdout, string(raw))
	return code
}

func printStatus(rep integrationStatusReport, asJSON bool, stdout, stderr io.Writer) int {
	code := 0
	if rep.Verdict != verdictClean {
		code = exitRefused
	}
	if asJSON {
		return printIntegrationJSON(rep, code, stdout, stderr)
	}
	fmt.Fprintf(stdout, "pool %d", rep.Pool)
	if rep.Wave > 0 {
		fmt.Fprintf(stdout, " (wave %d)", rep.Wave)
	}
	fmt.Fprintf(stdout, ": stage %s, epoch %d, shadow base %s, ended %v, last cycle %d\n", rep.Stage, rep.Epoch, integration.ShortSHA(rep.ShadowBase), rep.Ended, rep.LastCycle)
	fmt.Fprintf(stdout, "verdict: %s\n", rep.Verdict)
	for _, f := range rep.Failed {
		fmt.Fprintf(stdout, "  failed: %s\n", f)
	}
	fmt.Fprintln(stdout, rep.Order)
	fmt.Fprintln(stdout, rep.Enforce)
	fmt.Fprintf(stdout, "counts: %s\n", countsLine(rep.Counts))
	for _, m := range rep.Members {
		fmt.Fprintf(stdout, "  cycle %d: %s%s\n", m.Cycle, m.Status, map[bool]string{true: ", landed", false: ""}[m.Landed])
	}
	for _, b := range rep.Batches {
		fmt.Fprintf(stdout, "  batch %d: %s base %s tip %s members %v\n", b.Index, b.Branch, integration.ShortSHA(b.Base), integration.ShortSHA(b.Tip), b.Members)
	}
	for _, d := range rep.Decisions {
		fmt.Fprintf(stdout, "  decision %s: flush %v hold %v\n", d.Trigger, d.Flush, d.Hold)
	}
	return code
}

func countsLine(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + strconv.Itoa(counts[k])
	}
	return strings.Join(parts, " ")
}
