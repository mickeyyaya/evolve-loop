package policy

import (
	"reflect"
	"regexp"
	"testing"
)

func TestEventsDefaults_AreTheChannelTableOfTheSpec(t *testing.T) {
	t.Parallel()
	want := []EventChannel{
		{Name: "loop", Route: "module=loop", QoS: "lossless"},
		{Name: "cycle", Route: "kind=phase.dispatched,phase.outcome,phase.aborted,cycle.sealed,ship.landed,system.failure", QoS: "lossless"},
		{Name: "errors", Route: "severity>=WARN", QoS: "lossless"},
		{Name: "ship", Route: "kind=ship.*", QoS: "lossless"},
		{Name: "inbox", Route: "module=inbox", QoS: "lossless"},
		{Name: "ledger", Route: "kind=ledger.appended", QoS: "best_effort"},
		{Name: "quality.breach", Route: "kind=quality.breach,quality.warning", QoS: "lossless"},
		{Name: "quality.reading", Route: "kind=quality.reading", QoS: "best_effort"},
		{Name: "signals", Route: "", QoS: "best_effort"},
	}

	got := EventsDefaults()

	if !reflect.DeepEqual(got.Channels, want) {
		t.Errorf("channels = %+v\nwant %+v", got.Channels, want)
	}
}

func TestEventsDefaults_HoldTheQueueAndLockKeysOfTheSpec(t *testing.T) {
	t.Parallel()

	got := EventsDefaults()

	if got.SegmentMB != 16 || got.LockDeadlineMS != 250 || got.QueueEvents != 1024 || got.QueueBytes != 4<<20 || got.EnqueueDeadlineMS != 0 {
		t.Errorf("segment %d MB, lock %d ms, queue %d events, %d bytes, enqueue %d ms; want 16, 250, 1024, %d, 0",
			got.SegmentMB, got.LockDeadlineMS, got.QueueEvents, got.QueueBytes, 4<<20, got.EnqueueDeadlineMS)
	}
}

func TestEventsDefaults_EachCallReturnsItsOwnChannelList(t *testing.T) {
	t.Parallel()
	first := EventsDefaults()
	first.Channels[0].Name = "changed"

	second := EventsDefaults()

	if second.Channels[0].Name != "loop" {
		t.Errorf("first channel = %q, want loop: a caller that edits its list must not edit the next caller's", second.Channels[0].Name)
	}
}

func channelNamesOf(p EventsPolicy) []string {
	names := make([]string, len(p.Channels))
	for i, channel := range p.Channels {
		names[i] = channel.Name
	}
	return names
}

func TestEventsDefaults_NameEachChannelOnceWithAValidName(t *testing.T) {
	t.Parallel()
	valid := regexp.MustCompile(`^[a-z0-9_-]+(\.[a-z0-9_-]+)*$`)
	seen := map[string]bool{}

	for _, name := range channelNamesOf(EventsDefaults()) {
		if !valid.MatchString(name) {
			t.Errorf("channel name %q is not dot-separated tokens of [a-z0-9_-] (spec §2)", name)
		}
		if seen[name] {
			t.Errorf("channel %q is in the defaults twice, and the Publisher refuses such a config", name)
		}
		seen[name] = true
	}
	if len(seen) != 9 {
		t.Errorf("defaults name %d channels, want the 7 of the spec table without ci plus the two quality channels of ADR-0153", len(seen))
	}
}
