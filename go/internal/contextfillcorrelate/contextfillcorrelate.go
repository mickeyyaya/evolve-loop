// Package contextfillcorrelate joins the per-phase context-window fill ratio
// (internal/contextfill, persisted onto phasetiming.Entry.ContextFillRatio in
// cycle-1271) against the cycle's final verdict from its knowledge-base
// dossier. Parts (1) and (2) of the `context-fill-telemetry-and-cap` inbox item
// derive and persist the number; nothing before this package ASKED the question
// the number exists to answer — do cycles that ran hot fail more often?
//
// The package is a leaf: stdlib plus internal/contextfill (the one definition of
// the hot boundary) and internal/phasetiming (the one reader of the timing log).
// The derivation is not reimplemented here.
//
// Absent evidence is never reported as data. A cycle with no usable fill ratio,
// or with no final verdict, lands in Report.NoData — it is NEVER bucketed as a
// fabricated 0.0, which would manufacture exactly the correlation being
// measured. Conservation holds: CyclesJoined + len(NoData) equals the number of
// dossiers read.
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

// CycleFill is one cycle's joined row: its final verdict plus the per-phase
// context fill ratios actually recorded for it. Fills holds only the ratios
// PRESENT in the timing log — absent entries are omitted, never zero-filled, so
// an empty Fills means "unknown", not "ran empty".
type CycleFill struct {
	Cycle   int       `json:"cycle"`
	Verdict string    `json:"verdict"`
	Fills   []float64 `json:"fills,omitempty"`
}

// Bucket is one peak-fill band of the report. Min is inclusive, Max exclusive;
// the top bucket is open-ended (Max is +Inf, encoded as JSON null since JSON
// cannot represent infinity). FailRate is Fail/Cycles, and is a finite 0 for an
// empty bucket rather than a 0/0 NaN.
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

// bucketWire is Bucket's on-disk shape. Max is a pointer so an open-ended top
// bucket marshals as null instead of failing on +Inf.
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

// MarshalJSON encodes an open-ended bucket's Max as null.
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

// UnmarshalJSON decodes a null Max back into the open-ended +Inf it stands for,
// so a Report round-trips through the CLI's --json projection unchanged.
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

// VerdictSplit is the verdict tally for one side of the hot/cold partition.
type VerdictSplit struct {
	Cycles   int     `json:"cycles"`
	Fail     int     `json:"fail"`
	FailRate float64 `json:"fail_rate"`
}

// Report is the correlation evidence: fill bands with their FAIL rates, the
// coarse hot/cold split, and the cycles that could not be joined at all.
type Report struct {
	Buckets      []Bucket     `json:"buckets"`
	Hot          VerdictSplit `json:"hot"`
	Cold         VerdictSplit `json:"cold"`
	NoData       []int        `json:"no_data"`
	CyclesJoined int          `json:"cycles_joined"`
}

// bandEdges are the bucket lower bounds. The last edge is contextfill's one
// definition of the hot boundary — re-declaring 0.85 here would give the tree a
// second, drift-prone copy of it.
var bandEdges = []float64{0.0, 0.50, 0.70, contextfill.HotThreshold}

// Correlate is the pure join: it buckets each cycle by its PEAK per-phase fill
// ratio and tallies verdicts per bucket. Cycles without fill data or without a
// verdict are excluded into NoData and appear in no bucket.
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
		// Exhaustive by construction: normalizeVerdict already rejected
		// anything outside recognisedVerdicts into NoData above, so no arm of
		// this switch can fall through and leave a counted cycle untallied.
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

// recognisedVerdicts is the CLOSED set of dossier final_verdict values this
// join knows how to tally. Membership is checked BEFORE a row is counted as a
// joined cycle: an unrecognised verdict (a schema drift, a typo, a future
// "BLOCKED") that still incremented Bucket.Cycles would raise the denominator
// of FailRate without ever being able to raise its numerator, silently
// diluting the very correlation this package measures. Absent evidence is
// reported as absent — the same invariant that keeps a missing fill ratio out
// of the zero bucket.
var recognisedVerdicts = map[string]bool{"FAIL": true, "PASS": true, "WARN": true}

// normalizeVerdict canonicalises a raw dossier verdict and reports whether it
// is one this join can tally. An empty or whitespace-only verdict is simply the
// commonest unrecognised value, so it needs no separate check.
func normalizeVerdict(raw string) (string, bool) {
	v := strings.ToUpper(strings.TrimSpace(raw))
	return v, recognisedVerdicts[v]
}

// newBuckets materialises the ascending band table; the top band is open-ended.
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

// bucketIndex returns the band containing peak. Ratios are not clamped at 1.0
// (contextfill.FillRatio deliberately reports overruns), so anything at or above
// the last edge lands in the open-ended top band.
func bucketIndex(peak float64) int {
	idx := 0
	for i, min := range bandEdges {
		if peak >= min {
			idx = i
		}
	}
	return idx
}

// peakFill returns the largest recorded fill ratio and whether one exists. A
// non-positive peak means the timing log carried no usable ratio (the field is
// omitempty, so absent and 0.0 are indistinguishable on disk) — reported as
// absent rather than as a real zero.
func peakFill(fills []float64) (float64, bool) {
	peak := 0.0
	for _, f := range fills {
		if f > peak {
			peak = f
		}
	}
	return peak, peak > 0
}

// rate is Fail/Cycles with an empty denominator yielding a finite 0.
func rate(fail, cycles int) float64 {
	if cycles <= 0 {
		return 0
	}
	return float64(fail) / float64(cycles)
}

// Load reads the real corpus under projectRoot: every
// knowledge-base/cycles/cycle-*.json dossier for its final_verdict, joined by
// cycle id to .evolve/runs/cycle-<n>/phase-timing.json for the recorded
// ContextFillRatio values. Every dossier on disk yields exactly one CycleFill —
// an unreadable dossier or a missing timing log degrades to an empty
// verdict/fill row (which Correlate reports as no-data), never to a dropped
// cycle. A root with no dossiers is an error: an empty corpus must not be
// reported as a zero correlation.
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
		// The FILENAME is the authoritative identity: the glob makes it unique
		// on disk by construction, whereas the body's `cycle` field is free
		// text no writer validates. Trusting the body let two dossiers claim
		// one id and tally that cycle twice while row-based conservation still
		// passed. The body id is now a fallback used only when the filename
		// will not parse, and only for an id not already claimed.
		row := CycleFill{Cycle: cycleFromPath(p)}
		d, ok := readDossier(p) // one read per dossier — see readDossier
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

// dossier is the slice of the cycle dossier this join needs.
type dossier struct {
	Cycle        int    `json:"cycle"`
	FinalVerdict string `json:"final_verdict"`
}

// readDossier reads and decodes one dossier. Load calls it exactly ONCE per
// file: the earlier verdict-then-cycle accessor pair re-read and re-decoded
// every dossier twice (~1450 whole-file reads over the 726-dossier corpus) for
// two fields of the same struct.
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

// cycleFromPath parses the id out of a cycle-<n>.json filename. It is the
// fallback identity for a dossier whose body will not parse, so a corrupt file
// still occupies its slot in the conservation count.
func cycleFromPath(path string) int {
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	n, err := strconv.Atoi(strings.TrimPrefix(base, "cycle-"))
	if err != nil {
		return 0
	}
	return n
}

// fillsForCycle returns the per-phase ratios present in the cycle's timing log.
// A missing or unreadable log yields nil — unknown, not zero.
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

// Markdown renders the report as the bucket table the inbox item asked for.
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
