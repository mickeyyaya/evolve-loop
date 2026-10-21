package pipelinerates

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

type Rate struct{ K, N int }

func (r Rate) P() float64 {
	if r.N == 0 {
		return 0
	}
	return float64(r.K) / float64(r.N)
}

type Comparison struct {
	A, B      Rate
	Diff, MDE float64
	Status    qualitypanel.Status
}

func Compare(a, b Rate) Comparison {
	pooled := Rate{K: a.K + b.K, N: a.N + b.N}
	mde := qualitypanel.MDE(pooled.P(), min(a.N, b.N))
	c := Comparison{A: a, B: b, Diff: b.P() - a.P(), MDE: mde, Status: qualitypanel.StatusOK}
	if min(a.N, b.N) == 0 || math.Abs(c.Diff) <= mde {
		c.Status = qualitypanel.StatusInconclusive
	}
	return c
}

func (c Comparison) Reading(window string) qualitypanel.Reading {
	return qualitypanel.Reading{Index: "PQ3", Interval: qualitypanel.Interval{Value: c.Diff, Lo: c.Diff - c.MDE, Hi: c.Diff + c.MDE}, Method: qualitypanel.MethodMDE, N: min(c.A.N, c.B.N), Window: window, Source: runsSource, Origin: "pipelinerates.Compare", Status: c.Status}
}

func McNemarExact(onlyA, onlyB int) float64 {
	n, k := onlyA+onlyB, min(onlyA, onlyB)
	if n == 0 {
		return 1
	}
	tail := 0.0
	for i := 0; i <= k; i++ {
		tail += math.Exp(logChoose(n, i) - float64(n)*math.Ln2)
	}
	return math.Min(1, 2*tail)
}

func logChoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

func Discordant(cycles []Cycle, a, b Window) (onlyA, onlyB int) {
	inA, inB := itemOutcomes(cycles, a), itemOutcomes(cycles, b)
	for item, shippedA := range inA {
		shippedB, both := inB[item]
		switch {
		case !both:
		case shippedA && !shippedB:
			onlyA++
		case shippedB && !shippedA:
			onlyB++
		}
	}
	return onlyA, onlyB
}

func itemOutcomes(cycles []Cycle, w Window) map[string]bool {
	out := map[string]bool{}
	for _, c := range cycles {
		if c.Item != "" && w.holds(c) {
			out[c.Item] = out[c.Item] || c.Shipped
		}
	}
	return out
}

func ParseWindow(s string) (Window, error) {
	kind, span, ok := strings.Cut(s, ":")
	lo, hi, ranged := strings.Cut(span, "-")
	from, errFrom := strconv.Atoi(lo)
	to, errTo := from, error(nil)
	if ranged {
		to, errTo = strconv.Atoi(hi)
	}
	valid := ok && errFrom == nil && errTo == nil && from > 0 && to >= from && (kind == "wave" && !ranged || kind == "waves" && ranged)
	if !valid {
		return Window{}, fmt.Errorf("window %q is not wave:N or waves:A-B", s)
	}
	return Window{Name: s, From: from, To: to}, nil
}
