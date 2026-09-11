package llmcalls

import (
	"math"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestAggregate_SeparatesIdentityOutcomeAndMeasurementCoverage(t *testing.T) {
	measuredA := testRecord("a")
	measuredA.DurationMS = int64Ptr(1000)
	measuredA.Tokens = cyclestate.TokenUsage{Input: 100, Output: 10, CacheRead: 20}
	measuredB := testRecord("b")
	measuredB.DurationMS = int64Ptr(3000)
	measuredB.Tokens = cyclestate.TokenUsage{Input: 300, Output: 30, CacheRead: 60}

	partial := testRecord("c")
	partial.UsageStatus = UsagePartial
	partial.Source = "scrollback_peak"
	partial.DurationMS = int64Ptr(1000)
	partial.Tokens = cyclestate.TokenUsage{Output: 999}

	failed := testRecord("d")
	failed.ExitCode = intPtr(81)
	failed.CauseCode = "completion_detector_error"
	failed.UsageStatus = UsageResolverError
	failed.Source = "none"
	failed.Tokens = cyclestate.TokenUsage{}
	failed.DurationMS = nil

	rows := Aggregate([]Record{measuredA, measuredB, partial, failed})
	if len(rows) != 3 {
		t.Fatalf("Aggregate rows = %d, want measured, partial, and failure groups: %+v", len(rows), rows)
	}
	var measured, partialRow, failure Performance
	for _, row := range rows {
		switch {
		case row.Outcome == OutcomeSuccess && row.MeasurementSource == "events_result":
			measured = row
		case row.Outcome == OutcomeSuccess && row.MeasurementSource == "scrollback_peak":
			partialRow = row
		case row.Outcome == OutcomeFailure:
			failure = row
		}
	}
	if measured.Attempts != 2 || measured.Successes != 2 || measured.LatencySamples != 2 {
		t.Fatalf("measured counts = %+v", measured)
	}
	if measured.UsageMeasured != 2 || measured.UsagePartial != 0 || measured.ThroughputSamples != 2 {
		t.Fatalf("measured coverage = %+v", measured)
	}
	if measured.Tokens != (cyclestate.TokenUsage{Input: 400, Output: 40, CacheRead: 80}) {
		t.Fatalf("measured tokens = %+v", measured.Tokens)
	}
	if measured.P50LatencyMS != 1000 || measured.P95LatencyMS != 3000 || measured.TotalLatencyMS != 4000 {
		t.Fatalf("latency distribution = %+v", measured)
	}
	if measured.OutputTokensPerSecond == nil || math.Abs(*measured.OutputTokensPerSecond-10) > 0.0001 {
		t.Fatalf("amortized measured output rate = %v, want 10", measured.OutputTokensPerSecond)
	}
	if partialRow.Attempts != 1 || partialRow.UsagePartial != 1 || partialRow.Tokens.Output != 999 || partialRow.OutputTokensPerSecond != nil {
		t.Fatalf("partial group = %+v", partialRow)
	}
	if failure.Attempts != 1 || failure.Failures != 1 || failure.ResolverErrors != 1 || failure.LatencyUnavailable != 1 {
		t.Fatalf("failure coverage = %+v", failure)
	}
	if failure.OutputTokensPerSecond != nil {
		t.Fatalf("resolver-error group fabricated output rate %v", *failure.OutputTokensPerSecond)
	}
}

func TestAggregate_SeparatesTimingScopes(t *testing.T) {
	bridge := testRecord("bridge")
	other := testRecord("other")
	other.TimingScope = "provider_request"

	rows := Aggregate([]Record{bridge, other})
	if len(rows) != 2 {
		t.Fatalf("incompatible timing scopes merged: %+v", rows)
	}
	if rows[0].TimingScope == rows[1].TimingScope {
		t.Fatalf("timing scope dimension missing: %+v", rows)
	}
}

func TestAggregate_ZeroDurationFloorAndNegativeCountsNeverProduceRate(t *testing.T) {
	zero := testRecord("zero")
	zero.DurationMS = int64Ptr(0)
	floor := testRecord("floor")
	floor.UsageStatus = UsagePartial
	floor.Source = "scrollback_peak"
	floor.DurationMS = int64Ptr(1000)
	negative := testRecord("negative")
	negative.Tokens.Input = -1
	negative.DurationMS = int64Ptr(1000)

	rows := Aggregate([]Record{zero, floor, negative})
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	for _, row := range rows {
		if row.OutputTokensPerSecond != nil || row.ThroughputSamples != 0 {
			t.Fatalf("invalid measurements produced throughput: %+v", row)
		}
		if row.MeasurementSource == "events_result" && row.UsageUnavailable != 1 {
			t.Fatalf("negative token record must be unavailable: %+v", row)
		}
	}
}

func TestAggregate_LegacyRequestedModelIsNeverConcreteIdentity(t *testing.T) {
	legacy := Record{
		CLI: "claude-tmux", Model: "deep", Phase: "audit",
		DurationMS: int64Ptr(100), ExitCode: intPtr(0),
	}
	rows := Aggregate([]Record{legacy})
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Model != UnknownModel || rows[0].DispatchSource != DispatchLegacyUnverified || rows[0].RequestedModel != "deep" {
		t.Fatalf("legacy model attribution fabricated concrete identity: %+v", rows[0])
	}
}
