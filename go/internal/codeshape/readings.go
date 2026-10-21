package codeshape

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const (
	treeWindow           = "tree"
	perThousand          = 1000
	treeConformanceFloor = 0.878
)

func census(index string, value float64, n int, window, source, origin string) qualitypanel.Reading {
	return qualitypanel.Reading{Index: index, Interval: qualitypanel.Interval{Value: value, Lo: value, Hi: value}, Method: qualitypanel.MethodCensus, N: n, Window: window, Source: source, Origin: origin}
}

func GoRoot(root string) string { return filepath.Join(root, "go") }

func TreeConformance(t Tree) qualitypanel.Reading {
	k, n := Conforming(t.Funcs)
	r := census("CQ1", ratio(k, n), n, treeWindow, "go/", "codeshape.TreeConformance")
	floor := treeConformanceFloor
	r.Threshold = &floor
	return r
}

func InterfaceDepth(t Tree) qualitypanel.Reading {
	d := t.Depths()
	return census("CQ3", qualitypanel.Median(d), len(d), treeWindow, "go/internal/", "codeshape.InterfaceDepth")
}

func TreeDuplication(t Tree) qualitypanel.Reading {
	return census("CQ4", perK(t.DuplicateWindows(), t.Lines), t.Lines, treeWindow, "go/", "codeshape.TreeDuplication")
}

func JudgmentSurface(lines, files int) qualitypanel.Reading {
	return census("CQ5", float64(lines), files, treeWindow, "agents/, skills/", "codeshape.JudgmentSurface")
}

func ratio(k, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(k) / float64(n)
}

func perK(count, lines int) float64 { return ratio(count*perThousand, lines) }

func closingShips(cycles []pipelinerates.Cycle) (int, []pipelinerates.Cycle) {
	ws := pipelinerates.Windows(cycles)
	if len(ws) == 0 {
		return 0, nil
	}
	var ships []pipelinerates.Cycle
	for _, c := range cycles {
		if c.Wave == ws[0].From && c.Shipped && c.Commit != "" {
			ships = append(ships, c)
		}
	}
	return ws[0].From, ships
}

func ChangedConformance(root string, cycles []pipelinerates.Cycle) ([]qualitypanel.Reading, error) {
	wave, ships := closingShips(cycles)
	var out []qualitypanel.Reading
	var all []Func
	for _, c := range ships {
		funcs, err := ChangedFuncs(root, c.Commit)
		if err != nil {
			return out, err
		}
		all = append(all, funcs...)
		out = append(out, changedReading(funcs, fmt.Sprintf("cycle:%d", c.ID), c.ID))
	}
	return append(out, changedReading(all, fmt.Sprintf("wave:%d", wave), 0)), nil
}

func changedReading(funcs []Func, window string, cycle int) qualitypanel.Reading {
	k, n := Conforming(funcs)
	return qualitypanel.Reading{Index: "CQ1", Interval: qualitypanel.Wilson(k, n), Method: qualitypanel.MethodWilson95, N: n, Window: window, Source: "git show -U0", Cycle: cycle, Origin: "codeshape.ChangedConformance"}
}

func DiffDuplication(root string, t Tree, cycles []pipelinerates.Cycle) (qualitypanel.Reading, error) {
	wave, ships := closingShips(cycles)
	var added [][]string
	for _, c := range ships {
		d, err := DiffOf(root, c.Commit)
		if err != nil {
			return qualitypanel.Reading{}, err
		}
		added = append(added, d.Added...)
	}
	dup, lines := t.DiffDuplicates(added)
	return census("CQ4", perK(dup, lines), lines, fmt.Sprintf("wave:%d/added", wave), "git show -U0", "codeshape.DiffDuplication"), nil
}
