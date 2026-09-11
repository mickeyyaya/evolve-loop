package llmcalls

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Performance is one comparable model/outcome group. Token volumes include
// every valid observed count; the output rate uses only complete measurements
// with a positive attempt duration.
type Performance struct {
	CLI               string `json:"cli"`
	Model             string `json:"model"`
	RequestedModel    string `json:"requested_model,omitempty"`
	DispatchSource    string `json:"dispatch_source"`
	MeasurementSource string `json:"measurement_source"`
	TimingScope       string `json:"timing_scope"`
	Outcome           string `json:"outcome"`
	Attempts          int    `json:"attempts"`
	Successes         int    `json:"successes"`
	Failures          int    `json:"failures"`
	UnknownOutcomes   int    `json:"unknown_outcomes"`

	LatencySamples     int   `json:"latency_samples"`
	LatencyUnavailable int   `json:"latency_unavailable"`
	TotalLatencyMS     int64 `json:"total_latency_ms"`
	MinLatencyMS       int64 `json:"min_latency_ms"`
	P50LatencyMS       int64 `json:"p50_latency_ms"`
	P95LatencyMS       int64 `json:"p95_latency_ms"`
	MaxLatencyMS       int64 `json:"max_latency_ms"`

	UsageMeasured    int                   `json:"usage_measured"`
	UsagePartial     int                   `json:"usage_partial"`
	UsageUnavailable int                   `json:"usage_unavailable"`
	ResolverErrors   int                   `json:"resolver_errors"`
	Tokens           cyclestate.TokenUsage `json:"tokens"`

	ThroughputSamples     int      `json:"throughput_samples"`
	OutputTokensPerSecond *float64 `json:"amortized_output_tokens_per_second,omitempty"`
}

type performanceKey struct {
	cli, model, requested, dispatchSource, measurementSource, timingScope, outcome string
}

type performanceAccumulator struct {
	row                  Performance
	latencies            []int64
	throughputDurationMS int64
	throughputOutput     int
}

// Aggregate builds a deterministic performance index without persisting a
// second copy of the ledger. Legacy selector labels stay in RequestedModel and
// are never treated as verified concrete identity.
func Aggregate(records []Record) []Performance {
	groups := make(map[performanceKey]*performanceAccumulator)
	for _, rec := range records {
		model, dispatchSource := rec.ModelIdentity()
		requested := rec.RequestedModel
		if requested == "" {
			requested = rec.Model
		}
		outcome := outcomeOf(rec)
		measurementSource := defaultString(rec.Source, SourceUnknown)
		timingScope := defaultString(rec.TimingScope, TimingLegacyUnknown)
		key := performanceKey{rec.CLI, model, requested, dispatchSource, measurementSource, timingScope, outcome}
		acc := groups[key]
		if acc == nil {
			acc = &performanceAccumulator{row: Performance{
				CLI: rec.CLI, Model: model, RequestedModel: requested,
				DispatchSource: dispatchSource, MeasurementSource: measurementSource,
				TimingScope: timingScope, Outcome: outcome,
			}}
			groups[key] = acc
		}
		acc.add(rec)
	}

	out := make([]Performance, 0, len(groups))
	for _, acc := range groups {
		acc.finish()
		out = append(out, acc.row)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.CLI != b.CLI {
			return a.CLI < b.CLI
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		if a.RequestedModel != b.RequestedModel {
			return a.RequestedModel < b.RequestedModel
		}
		if a.DispatchSource != b.DispatchSource {
			return a.DispatchSource < b.DispatchSource
		}
		if a.MeasurementSource != b.MeasurementSource {
			return a.MeasurementSource < b.MeasurementSource
		}
		if a.TimingScope != b.TimingScope {
			return a.TimingScope < b.TimingScope
		}
		return a.Outcome < b.Outcome
	})
	return out
}

func (a *performanceAccumulator) add(rec Record) {
	a.row.Attempts++
	switch a.row.Outcome {
	case OutcomeSuccess:
		a.row.Successes++
	case OutcomeFailure:
		a.row.Failures++
	default:
		a.row.UnknownOutcomes++
	}
	if rec.DurationMS == nil || *rec.DurationMS < 0 {
		a.row.LatencyUnavailable++
	} else {
		duration := *rec.DurationMS
		a.latencies = append(a.latencies, duration)
		a.row.LatencySamples++
		a.row.TotalLatencyMS += duration
	}

	status := normalizedUsageStatus(rec)
	switch status {
	case UsageMeasured:
		a.row.UsageMeasured++
	case UsagePartial:
		a.row.UsagePartial++
	case UsageResolverError:
		a.row.ResolverErrors++
	default:
		a.row.UsageUnavailable++
	}
	if status == UsageMeasured || status == UsagePartial {
		a.row.Tokens = addTokens(a.row.Tokens, rec.Tokens)
	}
	if status == UsageMeasured && rec.DurationMS != nil && *rec.DurationMS > 0 {
		a.throughputDurationMS += *rec.DurationMS
		a.throughputOutput += rec.Tokens.Output
		a.row.ThroughputSamples++
	}
}

func (a *performanceAccumulator) finish() {
	if len(a.latencies) > 0 {
		sort.Slice(a.latencies, func(i, j int) bool { return a.latencies[i] < a.latencies[j] })
		a.row.MinLatencyMS = a.latencies[0]
		a.row.P50LatencyMS = percentile(a.latencies, 50)
		a.row.P95LatencyMS = percentile(a.latencies, 95)
		a.row.MaxLatencyMS = a.latencies[len(a.latencies)-1]
	}
	if a.throughputDurationMS > 0 && a.row.ThroughputSamples > 0 {
		rate := float64(a.throughputOutput) / (float64(a.throughputDurationMS) / 1000)
		a.row.OutputTokensPerSecond = &rate
	}
}

func percentile(sorted []int64, pct int) int64 {
	index := (len(sorted)*pct + 99) / 100
	if index < 1 {
		index = 1
	}
	return sorted[index-1]
}

func normalizedUsageStatus(rec Record) UsageStatus {
	if rec.Tokens.Input < 0 || rec.Tokens.Output < 0 || rec.Tokens.CacheRead < 0 || rec.Tokens.CacheWrite < 0 {
		return UsageUnavailable
	}
	switch rec.UsageStatus {
	case UsageMeasured:
		if rec.Source == "scrollback_peak" {
			return UsagePartial
		}
		return UsageMeasured
	case UsagePartial, UsageUnavailable, UsageResolverError:
		return rec.UsageStatus
	}
	if rec.Source == "scrollback_peak" {
		return UsagePartial
	}
	if rec.Source != "" && rec.Source != "none" {
		return UsageMeasured
	}
	return UsageUnavailable
}

func outcomeOf(rec Record) string {
	if rec.ExitCode == nil {
		return OutcomeUnknown
	}
	if *rec.ExitCode == 0 {
		return OutcomeSuccess
	}
	return OutcomeFailure
}

func addTokens(a, b cyclestate.TokenUsage) cyclestate.TokenUsage {
	a.Input += b.Input
	a.Output += b.Output
	a.CacheRead += b.CacheRead
	a.CacheWrite += b.CacheWrite
	return a
}
