package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclefacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavehealth"
)

const (
	checkinExitBrake     = 3
	checkinExitFixLocal  = 4
	waveCheckinOp        = "wave checkin"
	failureDigestFile    = "failure-digest.json"
	maxHostSampleLineLen = 1 << 16
)

func runWaveCheckin(env waveEnv, args []string, stdout, stderr io.Writer) int {
	plane, records, asJSON, rc := readWaveRecords(env, waveRead{op: waveCheckinOp, args: args}, stderr)
	if rc != 0 {
		return rc
	}
	snap, err := checkinGatherer{env: env, plane: plane, stderr: stderr}.gather(records)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	report := wavehealth.Evaluate(snap)
	if asJSON {
		if rc := writeWaveJSON(report, stdout, stderr); rc != 0 {
			return rc
		}
	} else {
		fmt.Fprint(stdout, report.Text())
	}
	return checkinExitCode(report.Outcome)
}

func checkinExitCode(outcome wavehealth.Outcome) int {
	switch outcome {
	case wavehealth.OutcomeBrake:
		return checkinExitBrake
	case wavehealth.OutcomeFixLocal:
		return checkinExitFixLocal
	}
	return 0
}

type checkinGatherer struct {
	env    waveEnv
	plane  wavePlaneDirs
	stderr io.Writer
}

func (g checkinGatherer) gather(records []wave.Record) (wavehealth.Snapshot, error) {
	pol := policy.LoadOrDefaults(filepath.Join(g.plane.evolveDir, "policy.json"), g.stderr)
	snap := wavehealth.Snapshot{Now: g.env.now(), Settings: pol.WaveCheckinSettings()}
	samples, err := readHostSamples(hostSamplesFilePath(g.plane.evolveDir, ""))
	if err != nil {
		return snap, err
	}
	snap.LoadSamples = samples
	if len(records) == 0 {
		snap.Loop = g.loopState(len(waveLiveRuns(g.plane.evolveDir, snap.Now)) > 0)
	} else if snap, err = g.withWave(snap, records[len(records)-1]); err != nil {
		return snap, err
	}
	if snap.Loop.Value.Running {
		snap.CI, snap.DiskFree = wavehealth.Have(g.env.mainCI(g.plane.root)), g.disk()
	}
	return snap, nil
}

func (g checkinGatherer) loopState(running bool) wavehealth.Known[wavehealth.LoopState] {
	return wavehealth.Have(wavehealth.LoopState{Running: running, BrakeHeld: loopchain.BrakeEngaged(g.plane.evolveDir)})
}

func (g checkinGatherer) withWave(snap wavehealth.Snapshot, cur wave.Record) (wavehealth.Snapshot, error) {
	live, startWarning := waveLoopLive(g.env, g.plane.evolveDir, cur)
	snap.Loop = g.loopState(live)
	read, warnings := readWaveOf(g.plane, cur)
	writeWaveWarnings(append(warnings, startWarning), g.stderr)
	facts := wavehealth.WaveFacts{Start: wave.RunFrom(cur), Started: len(read.Cycles), Shipped: len(read.Landed)}
	if n, ok := wave.PoolLive(cur, nil, signalsUpTo(read.Loop, snap.Now)); ok {
		facts.PoolLive = wavehealth.Have(n)
	}
	snap.Wave = wavehealth.Have(facts)
	snap.Stalls = wavehealth.Have(stallLinesOf(g.plane.root, read, snap.Now))
	var err error
	if snap.Abnormal, err = readCycleEvents(g.plane.root, read.Cycles); err != nil {
		return snap, err
	}
	snap.Failures = g.failures(cur)
	return snap, nil
}

func (g checkinGatherer) disk() wavehealth.Known[uint64] {
	free, err := g.env.diskFree(g.plane.root)
	if err != nil {
		fmt.Fprintf(g.stderr, "%sWARN: free disk of %s unreadable: %v\n", wavePrefix, g.plane.root, err)
		return wavehealth.Known[uint64]{}
	}
	return wavehealth.Have(free)
}

func (g checkinGatherer) failures(cur wave.Record) wavehealth.Known[wavehealth.Failures] {
	from := cur.CycleFloor + 1
	fp := breakerFailurePolicy(g.plane.evolveDir, g.stderr)
	cfg := blockerBreakerConfig(fp.Thresholds, breakerAckedFingerprints(g.plane.evolveDir, g.stderr), core.CollectBatchLaneDeferrals(g.plane.evolveDir, from))
	failures := wavehealth.Failures{Breaker: cfg}
	for _, d := range core.CollectBatchFailureDigests(g.plane.evolveDir, from) {
		workspace := paths.RunWorkspace(g.plane.root, d.Cycle)
		failures.Digests = append(failures.Digests, wavehealth.DigestAt{Digest: d, Class: cyclefacts.ReadDecision(workspace).Value.Category, At: fileModTime(filepath.Join(workspace, failureDigestFile))})
	}
	return wavehealth.Have(failures)
}

func signalsUpTo(events []signalcenter.Event, now time.Time) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if at, err := time.Parse(time.RFC3339Nano, e.TS); err == nil && at.After(now) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func readCycleEvents(root string, cycles []wave.Cycle) (wavehealth.Known[[]wavehealth.CycleEvent], error) {
	out := []wavehealth.CycleEvent{}
	for _, c := range cycles {
		events, err := dispatchevents.ReadEvents(paths.RunWorkspace(root, c.ID))
		if err != nil {
			return wavehealth.Known[[]wavehealth.CycleEvent]{}, err
		}
		for _, e := range events {
			out = append(out, wavehealth.CycleEvent{Cycle: c.ID, Event: e})
		}
	}
	return wavehealth.Have(out), nil
}

func readHostSamples(file string) (wavehealth.Known[[]wavehealth.LoadSample], error) {
	f, err := os.Open(file)
	if errors.Is(err, fs.ErrNotExist) {
		return wavehealth.Known[[]wavehealth.LoadSample]{}, nil
	}
	if err != nil {
		return wavehealth.Known[[]wavehealth.LoadSample]{}, fmt.Errorf("host samples: %w", err)
	}
	defer func() { _ = f.Close() }()
	samples := []wavehealth.LoadSample{}
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, maxHostSampleLineLen)
	for line := 1; sc.Scan(); line++ {
		sample, err := parseHostSample(sc.Bytes())
		if err != nil {
			return wavehealth.Known[[]wavehealth.LoadSample]{}, fmt.Errorf("%s line %d: %w", file, line, err)
		}
		samples = append(samples, sample)
	}
	if err := sc.Err(); err != nil {
		return wavehealth.Known[[]wavehealth.LoadSample]{}, fmt.Errorf("%s: %w", file, err)
	}
	return wavehealth.Have(samples), nil
}

func parseHostSample(raw []byte) (wavehealth.LoadSample, error) {
	var rec hostSampleLine
	if err := json.Unmarshal(raw, &rec); err != nil {
		return wavehealth.LoadSample{}, err
	}
	at, err := time.Parse(time.RFC3339, rec.TS)
	return wavehealth.LoadSample{At: at, PerCore: rec.LoadPerCore}, err
}

func fileModTime(file string) time.Time {
	info, err := os.Stat(file)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func mainCIFrom(fetch func(context.Context, string) statusRemote) func(string) wavehealth.CIState {
	return func(root string) wavehealth.CIState {
		ctx, cancel := context.WithTimeout(context.Background(), statusGHTimeout)
		defer cancel()
		return ciStateOf(fetch(ctx, root))
	}
}

func runIDOf(url string) string {
	if url == "" {
		return ""
	}
	return path.Base(url)
}

func ciStateOf(r statusRemote) wavehealth.CIState {
	runs, ok := r.Items.([]statusCIRun)
	if !r.Available || !ok || len(runs) == 0 {
		return wavehealth.CIState{}
	}
	run := runs[0]
	return wavehealth.CIState{Available: true, Status: run.Status, Conclusion: run.Conclusion, RunID: runIDOf(run.URL)}
}
