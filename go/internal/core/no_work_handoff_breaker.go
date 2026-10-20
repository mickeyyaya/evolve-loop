package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

const (
	noWorkHandoffsRule             = "no-work-handoffs"
	noWorkHandoffFingerprintPrefix = noWorkHandoffsRule + "|cycle|"
)

func NoWorkHandoffFingerprint(cycle int) string {
	return noWorkHandoffFingerprintPrefix + strconv.Itoa(cycle)
}

func NoWorkHandoffsResolvedThrough(acked map[string]bool) int {
	through := 0
	for fp := range acked {
		rest, ours := strings.CutPrefix(fp, noWorkHandoffFingerprintPrefix)
		if n, err := strconv.Atoi(rest); ours && err == nil {
			through = max(through, n)
		}
	}
	return through
}

func CollectNoWorkHandoffs(evolveDir string, resolvedThrough, window int) []int {
	entries, err := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if err != nil {
		return nil
	}
	var cycles []int
	for _, e := range entries {
		if n, ok := batchCycleNumber(e, resolvedThrough+1); ok {
			cycles = append(cycles, n)
		}
	}
	sort.Ints(cycles)
	cycles = cycles[max(0, len(cycles)-window):]
	var handoffs []int
	for _, n := range cycles {
		if committedset.HandedOff(filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(n))) {
			handoffs = append(handoffs, n)
		}
	}
	return handoffs
}

func EvaluateNoWorkHandoffs(cycles []int, ceiling int) BlockerVerdict {
	if ceiling <= 0 || len(cycles) < ceiling {
		return BlockerVerdict{}
	}
	names := make([]string, len(cycles))
	for i, c := range cycles {
		names[i] = strconv.Itoa(c)
	}
	last := cycles[len(cycles)-1]
	return BlockerVerdict{
		Halt: true, Rule: noWorkHandoffsRule, Count: len(cycles), Fingerprint: NoWorkHandoffFingerprint(last),
		Reason: fmt.Sprintf("%d no-work lanes in the recent window handed their items to the console (cycles %s; ceiling %d) — a triage that drops work it should commit drains the queue to the console while the other lanes ship; consume this item or run evolve loop --reset --fingerprint %s to acknowledge them",
			len(cycles), strings.Join(names, ", "), ceiling, NoWorkHandoffFingerprint(last)),
	}
}
