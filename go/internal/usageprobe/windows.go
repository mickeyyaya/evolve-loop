package usageprobe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

const WindowsFile = "usage-windows.json"

type Observation struct {
	CLI             string                   `json:"cli"`
	ObservedAt      time.Time                `json:"observed_at"`
	Windows         []quotastate.UsageWindow `json:"windows"`
	RegexWallFamily string                   `json:"regex_wall_family,omitempty"`
	Error           string                   `json:"error,omitempty"`
}

func WindowsPath(evolveDir string) string { return filepath.Join(evolveDir, WindowsFile) }

func RecordObservation(evolveDir string, obs Observation) error {
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		return fmt.Errorf("usage windows: %w", err)
	}
	path := WindowsPath(evolveDir)
	return flock.WithPathLock(path, func() error {
		all, err := LoadObservations(evolveDir)
		if err != nil {
			return err
		}
		all[obs.CLI] = obs
		return atomicwrite.JSON(path, all)
	})
}

func LoadObservations(evolveDir string) (map[string]Observation, error) {
	data, err := os.ReadFile(WindowsPath(evolveDir))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]Observation{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("usage windows: %w", err)
	}
	var all map[string]Observation
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("usage windows: %s: %w", WindowsPath(evolveDir), err)
	}
	if all == nil {
		all = map[string]Observation{}
	}
	return all, nil
}

type familyWall struct {
	family   string
	reset    time.Time
	evidence []string
}

type reading struct {
	pane      string
	windows   []quotastate.UsageWindow
	regexWall bool
}

func (p *Prober) read(family, pane string) reading {
	if windows := p.windowsOf(family, pane); len(windows) > 0 {
		return reading{pane: pane, windows: windows}
	}
	return reading{pane: pane, regexWall: p.Classify != nil && p.Classify(family, pane)}
}

func (p *Prober) act(family string, r reading) {
	switch {
	case len(r.windows) > 0:
		p.judgeWindows(family, r.windows)
	case r.regexWall:
		p.benchFamily(family, r.pane)
	}
}

func (p *Prober) benchFamily(family, pane string) {
	entry, err := p.Store.BenchWall(family, benchReason, pane)
	if err != nil {
		fmt.Fprintf(p.Log, "[usage-probe] WARN bench %s failed: %v\n", family, err)
		return
	}
	fmt.Fprintf(p.Log, "[usage-probe] %s capped — benched until %s (strikes=%d)\n",
		family, entry.BenchedUntil.Format(time.RFC3339), entry.Strikes)
}

func (p *Prober) windowsOf(family, pane string) []quotastate.UsageWindow {
	if p.Windows == nil {
		return nil
	}
	return p.Windows(family, pane)
}

func (p *Prober) record(probed string, r reading) {
	if p.Record == nil || len(r.windows) == 0 {
		return
	}
	if err := p.Record(probed, r.windows); err != nil {
		fmt.Fprintf(p.Log, "[usage-probe] WARN %s: recording its usage windows failed: %v\n", probed, err)
	}
}

func (p *Prober) judgeWindows(probed string, windows []quotastate.UsageWindow) {
	var walls []familyWall
	for _, w := range windows {
		switch {
		case !w.Exhausted:
		case w.Family == "":
			fmt.Fprintf(p.Log, "[usage-probe] WARN %s: the %s window is exhausted but its manifest maps it to no routing family, so nothing is benched: %s\n", probed, w.Scope, w.Evidence())
		case !w.ExhaustsFamily():
			fmt.Fprintf(p.Log, "[usage-probe] WARN %s: the %s model's window is exhausted (%s); benches are per family, so %s is not benched and the window is recorded for the router\n", probed, w.Model, w.Evidence(), w.Family)
		default:
			walls = withWall(walls, w)
		}
	}
	for _, wall := range walls {
		p.benchWall(probed, wall)
	}
}

func withWall(walls []familyWall, w quotastate.UsageWindow) []familyWall {
	i := len(walls)
	for j, wall := range walls {
		if wall.family == w.Family {
			i = j
		}
	}
	if i == len(walls) {
		walls = append(walls, familyWall{family: w.Family})
	}
	if w.ResetsAt != nil && w.ResetsAt.After(walls[i].reset) {
		walls[i].reset = *w.ResetsAt
	}
	walls[i].evidence = append(walls[i].evidence, w.Evidence())
	return walls
}

func (p *Prober) benchWall(probed string, wall familyWall) {
	evidence := strings.Join(wall.evidence, "; ")
	entry, err := p.Store.BenchWallUntil(wall.family, clihealth.Wall{Pattern: benchReason, Evidence: evidence, Reset: wall.reset})
	if err != nil {
		fmt.Fprintf(p.Log, "[usage-probe] WARN bench %s failed: %v\n", wall.family, err)
		return
	}
	fmt.Fprintf(p.Log, "[usage-probe] %s: benched %s until %s (strikes=%d): %s\n",
		probed, wall.family, entry.BenchedUntil.Format(time.RFC3339), entry.Strikes, evidence)
}
