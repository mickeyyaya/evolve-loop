package codeindex

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

type TruthSite struct {
	File string
	Line int
	Func string
}

type BenchSymbol struct {
	Name      string
	DefFile   string
	DefLine   int
	Sites     []TruthSite
	GrepFiles []string
}

type BenchRun struct {
	Backends    []Backend
	Symbols     []BenchSymbol
	Rounds      int
	Now         func() time.Time
	LoadAvg     func() float64
	Root        string
	GrepSymbols int
}

type BenchRow struct {
	Backend     string
	Query       Query
	TP, FP, FN  int
	Precision   float64
	Recall      float64
	P50, P95    time.Duration
	MedianBytes int
	Failures    int
}

type BenchReport struct {
	Rows    []BenchRow
	LoadAvg float64
}

func RunBench(ctx context.Context, run BenchRun) (BenchReport, error) {
	if err := grepSample(run); err != nil {
		return BenchReport{}, err
	}
	report := BenchReport{LoadAvg: run.LoadAvg()}
	for _, backend := range run.Backends {
		if !backend.Capabilities().Serves(QueryCallers) {
			continue
		}
		row, err := benchCallers(ctx, backend, run)
		if err != nil {
			return BenchReport{}, err
		}
		report.Rows = append(report.Rows, row)
	}
	return report, nil
}

func grepSample(run BenchRun) error {
	checked := 0
	for _, sym := range run.Symbols {
		if run.Root == "" || checked >= run.GrepSymbols {
			return nil
		}
		if len(sym.GrepFiles) == 0 {
			continue
		}
		if err := GrepCheck(run.Root, sym); err != nil {
			return err
		}
		checked++
	}
	return nil
}

func benchCallers(ctx context.Context, backend Backend, run BenchRun) (BenchRow, error) {
	row := BenchRow{Backend: backend.Name(), Query: QueryCallers}
	var latencies []time.Duration
	var sizes []int
	answers := make([][]Hit, len(run.Symbols))
	for round := 0; round < run.Rounds; round++ {
		for i, sym := range run.Symbols {
			if err := ctx.Err(); err != nil {
				return BenchRow{}, fmt.Errorf("bench: %w", err)
			}
			start := run.Now()
			hits, err := Dispatch(ctx, backend, Request{Query: QueryCallers, Arg: sym.Name, Direction: DirectionIn})
			latencies = append(latencies, run.Now().Sub(start))
			if err != nil && !errors.Is(err, ErrNotFound) {
				row.Failures++
				continue
			}
			sizes = append(sizes, hitBytes(hits))
			if round == 0 {
				answers[i] = hits
			}
		}
	}
	for i, sym := range run.Symbols {
		tp, fp, fn := scoreCallers(sym.Sites, answers[i])
		row.TP, row.FP, row.FN = row.TP+tp, row.FP+fp, row.FN+fn
	}
	row.Precision, row.Recall = ratio(row.TP, row.TP+row.FP), ratio(row.TP, row.TP+row.FN)
	row.P50, row.P95 = percentile(latencies, 50), percentile(latencies, 95)
	row.MedianBytes = medianInt(sizes)
	return row, nil
}

func scoreCallers(truth []TruthSite, hits []Hit) (tp, fp, fn int) {
	want := map[string]bool{}
	for _, site := range truth {
		want[callerKey(site.File, site.Func)] = true
	}
	got := map[string]bool{}
	for _, hit := range hits {
		got[callerKey(hit.File, hit.Symbol)] = true
	}
	for key := range got {
		if want[key] {
			tp++
		} else {
			fp++
		}
	}
	return tp, fp, len(want) - tp
}

func callerKey(file, fn string) string { return file + "\x00" + fn }

func ratio(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

func hitBytes(hits []Hit) int {
	n := 0
	for _, h := range hits {
		n += len(h.File) + len(strconv.Itoa(h.Line)) + len(h.Symbol) + len(h.Text)
	}
	return n
}

func percentile(samples []time.Duration, pct int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(samples))
	rank := (pct*len(sorted) + 99) / 100
	return sorted[max(rank, 1)-1]
}

func medianInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(values))
	return sorted[(len(sorted)-1)/2]
}

func (r BenchReport) Table() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "backend\tquery\tprecision\trecall\tp50\tp95\tbytes\tfailures\ttp\tfp\tfn")
	for _, row := range r.Rows {
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
			row.Backend, row.Query, row.Precision, row.Recall,
			row.P50.Round(time.Microsecond), row.P95.Round(time.Microsecond),
			row.MedianBytes, row.Failures, row.TP, row.FP, row.FN)
	}
	w.Flush()
	fmt.Fprintf(&b, "host load average: %.2f\n", r.LoadAvg)
	return b.String()
}
