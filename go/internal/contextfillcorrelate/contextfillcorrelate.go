// Package contextfillcorrelate joins the per-phase context fill ratio with the final cycle verdict.
// See docs/architecture/packages/internal-contextfillcorrelate.md.
package contextfillcorrelate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

type CycleFill struct {
	Cycle   int       `json:"cycle"`
	Verdict string    `json:"verdict"`
	Fills   []float64 `json:"fills,omitempty"`
}

type Bucket struct {
	Label    string
	Min      float64
	Max      float64
	Cycles   int
	Fail     int
	Pass     int
	Warn     int
	FailRate float64
}

type bucketWire struct {
	Label    string   `json:"label"`
	Min      float64  `json:"min"`
	Max      *float64 `json:"max"`
	Cycles   int      `json:"cycles"`
	Fail     int      `json:"fail"`
	Pass     int      `json:"pass"`
	Warn     int      `json:"warn"`
	FailRate float64  `json:"fail_rate"`
}

func (b Bucket) MarshalJSON() ([]byte, error) {
	w := bucketWire{
		Label: b.Label, Min: b.Min,
		Cycles: b.Cycles, Fail: b.Fail, Pass: b.Pass, Warn: b.Warn,
		FailRate: b.FailRate,
	}
	if !math.IsInf(b.Max, 1) {
		max := b.Max
		w.Max = &max
	}
	return json.Marshal(w)
}

func (b *Bucket) UnmarshalJSON(data []byte) error {
	var w bucketWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*b = Bucket{
		Label: w.Label, Min: w.Min, Max: math.Inf(1),
		Cycles: w.Cycles, Fail: w.Fail, Pass: w.Pass, Warn: w.Warn,
		FailRate: w.FailRate,
	}
	if w.Max != nil {
		b.Max = *w.Max
	}
	return nil
}

type VerdictSplit struct {
	Cycles   int     `json:"cycles"`
	Fail     int     `json:"fail"`
	FailRate float64 `json:"fail_rate"`
}

type Report struct {
	Buckets      []Bucket     `json:"buckets"`
	Hot          VerdictSplit `json:"hot"`
	Cold         VerdictSplit `json:"cold"`
	NoData       []int        `json:"no_data"`
	CyclesJoined int          `json:"cycles_joined"`
}

var bandEdges = []float64{0.0, 0.50, 0.70, contextfill.HotThreshold}

func Correlate(rows []CycleFill) Report {
	rep := Report{Buckets: newBuckets(), NoData: []int{}}

	for _, row := range rows {
		peak, ok := peakFill(row.Fills)
		verdict, known := normalizeVerdict(row.Verdict)
		if !ok || !known {
			rep.NoData = append(rep.NoData, row.Cycle)
			continue
		}
		rep.CyclesJoined++

		b := &rep.Buckets[bucketIndex(peak)]
		b.Cycles++
		split := &rep.Cold
		if contextfill.IsHot(peak) {
			split = &rep.Hot
		}
		split.Cycles++
		switch verdict {
		case "FAIL":
			b.Fail++
			split.Fail++
		case "PASS":
			b.Pass++
		case "WARN":
			b.Warn++
		}
	}

	for i := range rep.Buckets {
		rep.Buckets[i].FailRate = rate(rep.Buckets[i].Fail, rep.Buckets[i].Cycles)
	}
	rep.Hot.FailRate = rate(rep.Hot.Fail, rep.Hot.Cycles)
	rep.Cold.FailRate = rate(rep.Cold.Fail, rep.Cold.Cycles)
	sort.Ints(rep.NoData)
	return rep
}

var recognisedVerdicts = map[string]bool{"FAIL": true, "PASS": true, "WARN": true}

func normalizeVerdict(raw string) (string, bool) {
	v := strings.ToUpper(strings.TrimSpace(raw))
	return v, recognisedVerdicts[v]
}

func newBuckets() []Bucket {
	out := make([]Bucket, len(bandEdges))
	for i, min := range bandEdges {
		max := math.Inf(1)
		if i+1 < len(bandEdges) {
			max = bandEdges[i+1]
		}
		out[i] = Bucket{Label: label(min, max), Min: min, Max: max}
	}
	return out
}

func label(min, max float64) string {
	if math.IsInf(max, 1) {
		return fmt.Sprintf(">=%.2f (hot)", min)
	}
	return fmt.Sprintf("%.2f-%.2f", min, max)
}

func bucketIndex(peak float64) int {
	idx := 0
	for i, min := range bandEdges {
		if peak >= min {
			idx = i
		}
	}
	return idx
}

func peakFill(fills []float64) (float64, bool) {
	peak := 0.0
	for _, f := range fills {
		if f > peak {
			peak = f
		}
	}
	return peak, peak > 0
}

func rate(fail, cycles int) float64 {
	if cycles <= 0 {
		return 0
	}
	return float64(fail) / float64(cycles)
}

func Load(projectRoot string) ([]CycleFill, error) {
	pattern := filepath.Join(projectRoot, "knowledge-base", "cycles", "cycle-*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("contextfillcorrelate: no cycle dossiers under %s", filepath.Dir(pattern))
	}

	sort.Strings(paths)
	rows := make([]CycleFill, 0, len(paths))
	seen := make(map[int]bool, len(paths))
	for _, p := range paths {
		row := CycleFill{Cycle: cycleFromPath(p)}
		d, ok := readDossier(p)
		if ok {
			row.Verdict = d.FinalVerdict
			if row.Cycle == 0 && d.Cycle > 0 && !seen[d.Cycle] {
				row.Cycle = d.Cycle
			}
		}
		if row.Cycle > 0 {
			seen[row.Cycle] = true
		}
		row.Fills = fillsForCycle(projectRoot, row.Cycle)
		rows = append(rows, row)
	}
	return rows, nil
}

type dossier struct {
	Cycle        int    `json:"cycle"`
	FinalVerdict string `json:"final_verdict"`
}

func readDossier(path string) (dossier, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dossier{}, false
	}
	var d dossier
	if err := json.Unmarshal(data, &d); err != nil {
		return dossier{}, false
	}
	return d, true
}

func cycleFromPath(path string) int {
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	n, err := strconv.Atoi(strings.TrimPrefix(base, "cycle-"))
	if err != nil {
		return 0
	}
	return n
}

func fillsForCycle(projectRoot string, cycle int) []float64 {
	workspace := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	entries, err := phasetiming.Read(workspace)
	if err != nil {
		return nil
	}
	var fills []float64
	for _, e := range entries {
		if e.ContextFillRatio > 0 {
			fills = append(fills, e.ContextFillRatio)
		}
	}
	return fills
}

func Markdown(rep Report) string {
	var b strings.Builder
	b.WriteString("# Context-fill vs final-verdict correlation\n\n")
	fmt.Fprintf(&b, "Cycles joined: %d · no data: %d\n\n", rep.CyclesJoined, len(rep.NoData))
	b.WriteString("| peak fill band | cycles | fail | pass | warn | fail rate |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|\n")
	for _, bk := range rep.Buckets {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %.3f |\n",
			bk.Label, bk.Cycles, bk.Fail, bk.Pass, bk.Warn, bk.FailRate)
	}
	b.WriteString("\n| split | cycles | fail | fail rate |\n|---|---:|---:|---:|\n")
	fmt.Fprintf(&b, "| hot (>=%.2f) | %d | %d | %.3f |\n", contextfill.HotThreshold, rep.Hot.Cycles, rep.Hot.Fail, rep.Hot.FailRate)
	fmt.Fprintf(&b, "| cold | %d | %d | %.3f |\n", rep.Cold.Cycles, rep.Cold.Fail, rep.Cold.FailRate)
	if len(rep.NoData) > 0 {
		fmt.Fprintf(&b, "\nNo usable fill ratio or verdict (reported, never bucketed as 0.0): %d cycles.\n", len(rep.NoData))
	}
	return b.String()
}
