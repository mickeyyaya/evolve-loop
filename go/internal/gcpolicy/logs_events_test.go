package gcpolicy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestLogCatalog_HasOneCategoryForEachChannel(t *testing.T) {
	t.Parallel()
	channels := []gcpolicy.ChannelRetention{
		{Name: "loop", Dir: "events/ch/loop", TTLDays: 0, MaxTotalMB: 128},
		{Name: "quality.breach", Dir: "events/ch/quality.breach", TTLDays: 4, MaxTotalMB: 512},
		{Name: "custom", Dir: "events/ch/custom", MaxTotalMB: 9},
	}

	got := gcpolicy.ChannelCategories(channels, 17)

	want := []gcpolicy.LogCategory{
		{Name: "events.loop", TTLDays: 17, MaxTotalBytes: 128 * megabyte, Pin: gcpolicy.PinTailSegment, RulePrefix: `events.channels["loop"]`,
			Homes: []gcpolicy.LogHome{{Dir: "events/ch/loop", Glob: gcpolicy.ChannelSegmentGlob}}},
		{Name: "events.quality.breach", TTLDays: 4, MaxTotalBytes: 512 * megabyte, Pin: gcpolicy.PinTailSegment, RulePrefix: `events.channels["quality.breach"]`,
			Homes: []gcpolicy.LogHome{{Dir: "events/ch/quality.breach", Glob: gcpolicy.ChannelSegmentGlob}}},
		{Name: "events.custom", TTLDays: 17, MaxTotalBytes: 9 * megabyte, Pin: gcpolicy.PinTailSegment, RulePrefix: `events.channels["custom"]`,
			Homes: []gcpolicy.LogHome{{Dir: "events/ch/custom", Glob: gcpolicy.ChannelSegmentGlob}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ChannelCategories =\n%+v\nwant one category for each channel, a zero ttl inheriting the default 17:\n%+v", got, want)
	}
}

func TestChannelCategories_ANoChannelListGivesNoCategory(t *testing.T) {
	t.Parallel()
	if got := gcpolicy.ChannelCategories(nil, 30); len(got) != 0 {
		t.Errorf("ChannelCategories(nil) = %+v, want none", got)
	}
}

func TestLogCatalog_TheCompiledCategoriesKeepTheCurrentOrLivePinAndNoPrefix(t *testing.T) {
	t.Parallel()
	var zeroPin gcpolicy.PinRule = gcpolicy.PinCurrentOrLive
	for _, c := range (gcpolicy.Policy{}).LogCatalog() {
		if c.Pin != zeroPin || c.RulePrefix != "" {
			t.Errorf("category %q pin %d prefix %q, want the zero values", c.Name, c.Pin, c.RulePrefix)
		}
	}
}
