package policy

type EventChannel struct {
	Name      string
	Route     string
	QoS       string
	SegmentMB int
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
			{Name: "loop", Route: "module=loop", QoS: "lossless"},
			{Name: "cycle", Route: "kind=phase.dispatched,phase.outcome,phase.aborted,cycle.sealed,ship.landed,system.failure", QoS: "lossless"},
			{Name: "errors", Route: "severity>=WARN", QoS: "lossless"},
			{Name: "ship", Route: "kind=ship.*", QoS: "lossless"},
			{Name: "inbox", Route: "module=inbox", QoS: "lossless"},
			{Name: "ledger", Route: "kind=ledger.appended", QoS: "best_effort"},
			{Name: "quality.breach", Route: "kind=quality.breach,quality.warning", QoS: "lossless"},
			{Name: "quality.reading", Route: "kind=quality.reading", QoS: "best_effort"},
			{Name: "signals", Route: "", QoS: "best_effort"},
		},
		SegmentMB:         16,
		LockDeadlineMS:    250,
		QueueEvents:       1024,
		QueueBytes:        4 << 20,
		EnqueueDeadlineMS: 0,
	}
}
