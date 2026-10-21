package regressiontia

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
)

type Miss struct {
	Cycle     int    `json:"cycle"`
	Package   string `json:"package"`
	Predicate string `json:"predicate"`
}

type Summary struct {
	Cycles     int      `json:"cycles"`
	Selected   int      `json:"selected"`
	WouldSkip  int      `json:"would_skip"`
	TotalMS    int64    `json:"total_ms"`
	MaxMS      int64    `json:"max_ms"`
	Misses     []Miss   `json:"misses,omitempty"`
	NoVerdict  []int    `json:"no_verdict,omitempty"`
	Unreadable []string `json:"unreadable,omitempty"`
}

type cycleVerdict struct {
	Results []struct {
		Predicate string `json:"predicate"`
		Result    string `json:"result"`
	} `json:"results"`
}

func Rollup(runsDir string) (Summary, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return Summary{}, err
	}
	var s Summary
	for _, cycle := range cycleNumbers(entries) {
		s.add(runsDir, cycle)
	}
	return s, nil
}

func cycleNumbers(entries []fs.DirEntry) []int {
	var cycles []int
	for _, e := range entries {
		number, isCycle := strings.CutPrefix(e.Name(), "cycle-")
		if !e.IsDir() || !isCycle {
			continue
		}
		if n, err := strconv.Atoi(number); err == nil {
			cycles = append(cycles, n)
		}
	}
	slices.Sort(cycles)
	return cycles
}

func (s *Summary) add(runsDir string, cycle int) {
	dir := "cycle-" + strconv.Itoa(cycle)
	var d Decision
	if !s.read(runsDir, dir, ArtifactName, &d) {
		return
	}
	s.Cycles++
	s.Selected += len(d.Selected)
	s.WouldSkip += len(d.WouldSkip)
	s.TotalMS += d.DurationMS
	s.MaxMS = max(s.MaxMS, d.DurationMS)
	var v cycleVerdict
	if _, err := os.Stat(filepath.Join(runsDir, dir, acsverdict.Filename)); errors.Is(err, fs.ErrNotExist) {
		s.NoVerdict = append(s.NoVerdict, cycle)
		return
	}
	if !s.read(runsDir, dir, acsverdict.Filename, &v) {
		return
	}
	for _, r := range v.Results {
		pkg := regressionPackage(r.Predicate)
		if r.Result == "red" && slices.Contains(d.WouldSkip, pkg) {
			s.Misses = append(s.Misses, Miss{Cycle: cycle, Package: pkg, Predicate: r.Predicate})
		}
	}
}

func (s *Summary) read(runsDir, dir, name string, into any) bool {
	raw, err := os.ReadFile(filepath.Join(runsDir, dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil || json.Unmarshal(raw, into) != nil {
		s.Unreadable = append(s.Unreadable, dir+"/"+name)
		return false
	}
	return true
}

func regressionPackage(predicate string) string {
	pkg, _, _ := strings.Cut(predicate, ":")
	pkg, _, _ = strings.Cut(pkg, "/...")
	return "./" + strings.TrimPrefix(pkg, "go/")
}
