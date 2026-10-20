package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

const laneTimesFile = "lane-time.json"

func recordLaneTimes(evolveDir string, stderr io.Writer) {
	cycles, _, err := lanetime.ReadCycles(evolveDir, 0, 0)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] lane-time: WARN the run dirs do not read (%v); the build seat keeps the last measure\n", err)
		return
	}
	lanes := lanetime.LaneTimeToPass(cycles, lanetime.DefaultBuilds)
	if err := atomicwrite.JSON(filepath.Join(evolveDir, laneTimesFile), lanes); err != nil {
		fmt.Fprintf(stderr, "[loop] lane-time: WARN %v\n", err)
		return
	}
	fmt.Fprintf(stderr, "[loop] lane-time: %d families measured over the last %d builds of each\n", len(lanes), lanetime.DefaultBuilds)
}

func loadLaneTimes(evolveDir string) ([]lanetime.FamilyLane, error) {
	path := filepath.Join(evolveDir, laneTimesFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lanes []lanetime.FamilyLane
	if err := json.Unmarshal(data, &lanes); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return lanes, nil
}

func projectLaneTimes(projectRoot string) ([]lanetime.FamilyLane, error) {
	return loadLaneTimes(filepath.Join(projectRoot, ".evolve"))
}

func projectWeekUsed(now func() time.Time) func(string) (map[string]float64, error) {
	return func(projectRoot string) (map[string]float64, error) {
		return usageprobe.WeekUsed(filepath.Join(projectRoot, ".evolve"), now())
	}
}
