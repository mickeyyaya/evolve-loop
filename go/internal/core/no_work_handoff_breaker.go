package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
)

const noWorkHandoffsRule = "no-work-handoffs"

func CollectBatchNoWorkHandoffs(evolveDir string, fromCycle int) []int {
	entries, err := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if err != nil {
		return nil
	}
	var cycles []int
	for _, e := range entries {
		n, inBatch := batchCycleNumber(e, fromCycle)
		if !inBatch {
			continue
		}
		if outcome, _ := cyclehealth.ClassifyOutcome(filepath.Join(evolveDir, "runs", e.Name())); outcome == cyclehealth.OutcomeNoWork {
			cycles = append(cycles, n)
		}
	}
	sort.Ints(cycles)
	return cycles
}

func EvaluateNoWorkHandoffs(cycles []int, ceiling int) BlockerVerdict {
	if ceiling <= 0 || len(cycles) < ceiling {
		return BlockerVerdict{}
	}
	names := make([]string, len(cycles))
	for i, c := range cycles {
		names[i] = strconv.Itoa(c)
	}
	return BlockerVerdict{
		Halt: true, Rule: noWorkHandoffsRule, Count: len(cycles),
		Reason: fmt.Sprintf("%d planned no-work lanes in this batch handed their items to the console (cycles %s; ceiling %d) — a triage that drops work it should commit drains the queue to the console while the other lanes ship",
			len(cycles), strings.Join(names, ", "), ceiling),
	}
}
