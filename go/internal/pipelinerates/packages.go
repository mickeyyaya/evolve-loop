package pipelinerates

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const (
	bootstrapResamples = 2000
	bootstrapSeed      = 20261021
)

type ShipPaths func(commit string) ([]string, error)

func GitShipPaths(root string) ShipPaths {
	return func(commit string) ([]string, error) {
		out, err := exec.Command("git", "-C", root, "show", "--numstat", "-z", "--no-renames", "--diff-merges=first-parent", "--format=", commit).Output()
		if err != nil {
			return nil, fmt.Errorf("git show %s: %w", commit, err)
		}
		var paths []string
		for _, record := range strings.Split(string(out), "\x00") {
			if f := strings.SplitN(strings.TrimLeft(record, "\n"), "\t", 3); len(f) == 3 {
				paths = append(paths, f[2])
			}
		}
		return paths, nil
	}
}

func PackageCount(paths []string) int {
	dirs := map[string]bool{}
	for _, p := range paths {
		if strings.HasSuffix(p, ".go") && !strings.HasPrefix(p, ".evolve/") {
			dirs[filepath.Dir(p)] = true
		}
	}
	return len(dirs)
}

func PackagesPerShip(cycles []Cycle, show ShipPaths) ([]qualitypanel.Reading, error) {
	counts := map[int]float64{}
	var unresolved []error
	for _, c := range cycles {
		if c.Shipped && c.Commit != "" {
			paths, err := show(c.Commit)
			if err != nil {
				unresolved = append(unresolved, err)
				continue
			}
			counts[c.ID] = float64(PackageCount(paths))
		}
	}
	var out []qualitypanel.Reading
	for _, w := range Windows(cycles) {
		clusters, n := clustersByWave(cycles, w, counts)
		out = append(out, qualitypanel.Reading{Index: "CQ2", Interval: qualitypanel.ClusterBootstrapMedian(clusters, bootstrapResamples, bootstrapSeed), Method: qualitypanel.MethodBootstrap95, N: n, Window: w.Name, Source: runsSource + " + git show", Origin: "pipelinerates.PackagesPerShip"})
	}
	return out, errors.Join(unresolved...)
}

func clustersByWave(cycles []Cycle, w Window, counts map[int]float64) ([][]float64, int) {
	byWave := map[int][]float64{}
	n := 0
	for _, c := range cycles {
		if v, ok := counts[c.ID]; ok && w.holds(c) {
			byWave[c.Wave] = append(byWave[c.Wave], v)
			n++
		}
	}
	var clusters [][]float64
	for wave := w.From; wave <= w.To; wave++ {
		clusters = append(clusters, byWave[wave])
	}
	return clusters, n
}
