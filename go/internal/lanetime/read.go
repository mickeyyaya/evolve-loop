package lanetime

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	signalsFile  = "signals.ndjson"
	callsFile    = "llm-calls.ndjson"
	maxLineBytes = 16 << 20
)

var runDirName = regexp.MustCompile(`^cycle-(\d+)$`)

type signalLine struct {
	TS     time.Time `json:"ts"`
	Phase  string    `json:"phase"`
	Kind   string    `json:"kind"`
	PID    int       `json:"pid"`
	Code   string    `json:"code"`
	Fields struct {
		Verdict string `json:"verdict"`
		Live    string `json:"live"`
		Target  string `json:"target"`
	} `json:"fields"`
}

type callLine struct {
	Phase     string    `json:"phase"`
	CLI       string    `json:"cli"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

func forEachLine[T any](path string, visit func(T)) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, maxLineBytes)
	malformed := 0
	for scanner.Scan() {
		var line T
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			malformed++
			continue
		}
		visit(line)
	}
	return malformed, scanner.Err()
}

type spanBuilder struct {
	open     map[string]time.Time
	spans    []Span
	unpaired int
}

func (b *spanBuilder) add(l signalLine) {
	switch l.Kind {
	case "phase.dispatched":
		if _, held := b.open[l.Phase]; !held {
			b.open[l.Phase] = l.TS
		}
	case "phase.outcome", "phase.aborted":
		if l.Kind == "phase.outcome" && l.Fields.Verdict == "" {
			return
		}
		start, held := b.open[l.Phase]
		if !held {
			b.unpaired++
			return
		}
		delete(b.open, l.Phase)
		b.spans = append(b.spans, Span{Phase: l.Phase, Start: start, End: l.TS, Verdict: l.Fields.Verdict})
	}
}

func readCycle(dir string, id int) (Cycle, int, error) {
	b := &spanBuilder{open: map[string]time.Time{}}
	malformed, err := forEachLine(filepath.Join(dir, signalsFile), b.add)
	if err != nil {
		return Cycle{}, 0, err
	}
	cycle := Cycle{ID: id, Spans: b.spans, Unpaired: b.unpaired, Open: len(b.open)}
	bad, err := forEachLine(filepath.Join(dir, callsFile), func(l callLine) {
		cycle.Calls = append(cycle.Calls, Call{Phase: l.Phase, CLI: l.CLI, Start: l.StartedAt, End: l.EndedAt})
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Cycle{}, 0, err
	}
	return cycle, malformed + bad, nil
}

func cycleID(name string, floor, ceiling int) (int, bool) {
	m := runDirName.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	id, err := strconv.Atoi(m[1])
	return id, err == nil && id > floor && (ceiling == 0 || id <= ceiling)
}

func ReadCycles(evolveDir string, floor, ceiling int) ([]Cycle, []string, error) {
	runs := filepath.Join(evolveDir, "runs")
	entries, err := os.ReadDir(runs)
	if err != nil {
		return nil, nil, err
	}
	var cycles []Cycle
	missing, malformed, unpaired := 0, 0, 0
	for _, e := range entries {
		id, ok := cycleID(e.Name(), floor, ceiling)
		if !ok {
			continue
		}
		c, bad, err := readCycle(filepath.Join(runs, e.Name()), id)
		if errors.Is(err, fs.ErrNotExist) {
			missing++
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		malformed += bad
		unpaired += c.Unpaired
		cycles = append(cycles, c)
	}
	sort.Slice(cycles, func(i, j int) bool { return cycles[i].ID < cycles[j].ID })
	return cycles, readWarnings(missing, malformed, unpaired), nil
}

func readWarnings(missing, malformed, unpaired int) []string {
	var warnings []string
	for _, w := range []struct {
		count int
		text  string
	}{
		{missing, "run dir(s) had no " + signalsFile},
		{malformed, "malformed line(s) were skipped"},
		{unpaired, "phase outcome(s) had no dispatch signal (older run dirs) and were skipped"},
	} {
		if w.count > 0 {
			warnings = append(warnings, fmt.Sprintf("%d %s", w.count, w.text))
		}
	}
	return warnings
}

type Sample struct {
	At     time.Time
	Live   int
	Target int
	PID    int
}

func ReadSamples(path string) ([]Sample, int, error) {
	var samples []Sample
	bad := 0
	malformed, err := forEachLine(path, func(l signalLine) {
		if !strings.HasPrefix(l.Code, "LOOP_ORCH_") || l.Fields.Live == "" {
			return
		}
		live, liveErr := strconv.Atoi(l.Fields.Live)
		target, targetErr := strconv.Atoi(l.Fields.Target)
		if liveErr != nil || targetErr != nil {
			bad++
			return
		}
		samples = append(samples, Sample{At: l.TS, Live: live, Target: target, PID: l.PID})
	})
	return samples, malformed + bad, err
}
