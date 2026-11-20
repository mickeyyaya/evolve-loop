package qualitypanel

import (
	"math"
	"math/rand"
	"sort"
)

const (
	z95          = 1.959964
	zPower80     = 0.84
	bootstrapLow = 0.025
	bootstrapTop = 0.975
)

type Interval struct{ Value, Lo, Hi float64 }

func Wilson(k, n int) Interval {
	if n <= 0 {
		return Interval{}
	}
	p, nn := float64(k)/float64(n), float64(n)
	d := 1 + z95*z95/nn
	centre := (p + z95*z95/(2*nn)) / d
	half := z95 / d * math.Sqrt(p*(1-p)/nn+z95*z95/(4*nn*nn))
	lo, hi := math.Max(0, centre-half), math.Min(1, centre+half)
	if k <= 0 {
		lo = 0
	}
	if k >= n {
		hi = 1
	}
	return Interval{Value: p, Lo: lo, Hi: hi}
}

func MDE(pBar float64, n int) float64 {
	if n <= 0 {
		return 1
	}
	return (z95 + zPower80) * math.Sqrt(2*pBar*(1-pBar)/float64(n))
}

func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

func ClusterBootstrapMedian(clusters [][]float64, resamples int, seed int64) Interval {
	clusters = nonEmpty(clusters)
	all := flatten(clusters)
	if len(all) == 0 || resamples <= 0 {
		return Interval{}
	}
	r := rand.New(rand.NewSource(seed))
	medians := make([]float64, resamples)
	for i := range medians {
		medians[i] = Median(resampleClusters(clusters, r))
	}
	sort.Float64s(medians)
	return Interval{Value: Median(all), Lo: percentile(medians, bootstrapLow), Hi: percentile(medians, bootstrapTop)}
}

func resampleClusters(clusters [][]float64, r *rand.Rand) []float64 {
	var out []float64
	for range clusters {
		out = append(out, clusters[r.Intn(len(clusters))]...)
	}
	return out
}

func nonEmpty(clusters [][]float64) [][]float64 {
	var out [][]float64
	for _, c := range clusters {
		if len(c) > 0 {
			out = append(out, c)
		}
	}
	return out
}

func flatten(clusters [][]float64) []float64 {
	var out []float64
	for _, c := range clusters {
		out = append(out, c...)
	}
	return out
}

func percentile(sorted []float64, q float64) float64 {
	return sorted[int(q*float64(len(sorted)-1))]
}

func ClusterBootstrapRatio(num, den []float64, resamples int, seed int64) Interval {
	if len(num) == 0 || resamples <= 0 {
		return Interval{}
	}
	r := rand.New(rand.NewSource(seed))
	ratios := make([]float64, resamples)
	for i := range ratios {
		var n, d float64
		for range num {
			j := r.Intn(len(num))
			n, d = n+num[j], d+den[j]
		}
		ratios[i] = safeRatio(n, d)
	}
	sort.Float64s(ratios)
	return Interval{Value: safeRatio(sum(num), sum(den)), Lo: percentile(ratios, bootstrapLow), Hi: percentile(ratios, bootstrapTop)}
}

func sum(xs []float64) float64 {
	var total float64
	for _, x := range xs {
		total += x
	}
	return total
}

func safeRatio(n, d float64) float64 {
	if d == 0 {
		return 0
	}
	return n / d
}
