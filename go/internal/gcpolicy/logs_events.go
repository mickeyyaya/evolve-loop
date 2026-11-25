package gcpolicy

import "fmt"

type PinRule int

const (
	PinCurrentOrLive PinRule = iota
	PinTailSegment
)

const (
	ChannelSegmentGlob    = "seg-*.ndjson"
	channelCategoryPrefix = "events."
)

type ChannelRetention struct {
	Name       string
	Dir        string
	TTLDays    int
	MaxTotalMB int
}

func ChannelCategories(channels []ChannelRetention, defaultTTLDays int) []LogCategory {
	out := make([]LogCategory, len(channels))
	for i, ch := range channels {
		cat := resolveCategory(channelCategoryPrefix+ch.Name, LogRetention{TTLDays: ch.TTLDays}, defaultTTLDays, ch.MaxTotalMB,
			LogHome{Dir: ch.Dir, Glob: ChannelSegmentGlob})
		cat.Pin = PinTailSegment
		cat.RulePrefix = fmt.Sprintf("events.channels[%q]", ch.Name)
		out[i] = cat
	}
	return out
}
