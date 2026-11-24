package policy

import "github.com/mickeyyaya/evolve-loop/go/internal/events/publisher"

const (
	eventQoSLossless   = string(publisher.QoSLossless)
	eventQoSBestEffort = string(publisher.QoSBestEffort)
)

const DefaultChannelMaxTotalMB = 128

const ledgerSignalsMaxTotalMB = 512

type EventChannel struct {
	Name       string
	Route      string
	QoS        string
	SegmentMB  int
	TTLDays    int
	MaxTotalMB int
}

type EventsPolicy struct {
	Channels          []EventChannel
	SegmentMB         int
	LockDeadlineMS    int
	QueueEvents       int
	QueueBytes        int
	EnqueueDeadlineMS int
}

func EventsDefaults() EventsPolicy {
	return EventsPolicy{
		Channels: []EventChannel{
			{Name: "loop", Route: "module=loop", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "cycle", Route: "kind=phase.dispatched,phase.outcome,phase.aborted,cycle.sealed,ship.landed,system.failure", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "errors", Route: "severity>=WARN", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "ship", Route: "kind=ship.*", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "inbox", Route: "module=inbox", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "ledger", Route: "kind=ledger.appended", QoS: eventQoSBestEffort, MaxTotalMB: ledgerSignalsMaxTotalMB},
			{Name: "quality.breach", Route: "kind=quality.breach,quality.warning", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "quality.reading", Route: "kind=quality.reading", QoS: eventQoSBestEffort, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "isolation.violation", Route: "kind=isolation.violation", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "selfheal.incident", Route: "kind=selfheal.incident", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "selfheal.decision", Route: "kind=selfheal.decision", QoS: eventQoSLossless, MaxTotalMB: DefaultChannelMaxTotalMB},
			{Name: "signals", Route: "", QoS: eventQoSBestEffort, MaxTotalMB: ledgerSignalsMaxTotalMB},
		},
		SegmentMB:         16,
		LockDeadlineMS:    250,
		QueueEvents:       1024,
		QueueBytes:        4 << 20,
		EnqueueDeadlineMS: 0,
	}
}
