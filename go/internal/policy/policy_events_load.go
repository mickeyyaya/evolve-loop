package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/publisher"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type ValueError struct {
	Path string
	Err  error
}

func (e *ValueError) Error() string { return fmt.Sprintf("policy: bad value at %s: %v", e.Path, e.Err) }

func (e *ValueError) Unwrap() error { return e.Err }

type RouteError struct {
	Channel string
	Err     error
}

func (e *RouteError) Error() string {
	return fmt.Sprintf("events.channels[%q].route is refused and the channel keeps its compiled route or is left out: %v", e.Channel, e.Err)
}

func (e *RouteError) Unwrap() error { return e.Err }

type RouteWarning struct {
	Channel string
	Warning filter.Warning
}

func (w *RouteWarning) Error() string {
	return fmt.Sprintf("events.channels[%q].route: %s", w.Channel, w.Warning)
}

type channelOverride struct {
	name         string
	route        *string
	qos          *string
	ttlDays      *int
	maxTotalMB   *int
	segmentMB    *int
	routeRefused bool
}

type EventsBlock struct {
	segmentMB         *int
	lockDeadlineMS    *int
	queueEvents       *int
	queueBytes        *int
	enqueueDeadlineMS *int
	channels          []channelOverride
	diagnostics       []error
}

type namedRaw struct {
	key string
	raw json.RawMessage
}

type topLevelInt struct {
	field func(*EventsBlock) **int
	min   int
}

var eventsTopLevelInts = map[string]topLevelInt{
	"segment_mb":          {func(b *EventsBlock) **int { return &b.segmentMB }, 1},
	"lock_deadline_ms":    {func(b *EventsBlock) **int { return &b.lockDeadlineMS }, 1},
	"queue_events":        {func(b *EventsBlock) **int { return &b.queueEvents }, 1},
	"queue_bytes":         {func(b *EventsBlock) **int { return &b.queueBytes }, 1},
	"enqueue_deadline_ms": {func(b *EventsBlock) **int { return &b.enqueueDeadlineMS }, 0},
}

type eventsChannelShape struct {
	Route      string `json:"route"`
	QoS        string `json:"qos"`
	TTLDays    int    `json:"ttl_days"`
	MaxTotalMB int    `json:"max_total_mb"`
	SegmentMB  int    `json:"segment_mb"`
}

type eventsShape struct {
	Channels          map[string]eventsChannelShape `json:"channels"`
	SegmentMB         int                           `json:"segment_mb"`
	LockDeadlineMS    int                           `json:"lock_deadline_ms"`
	QueueEvents       int                           `json:"queue_events"`
	QueueBytes        int                           `json:"queue_bytes"`
	EnqueueDeadlineMS int                           `json:"enqueue_deadline_ms"`
}

func (b *EventsBlock) UnmarshalJSON(raw []byte) error {
	pairs, err := orderedObject(raw, "events")
	if err != nil {
		return err
	}
	var parsed EventsBlock
	for _, kv := range pairs {
		if err := parsed.takeKey(kv); err != nil {
			return err
		}
	}
	*b = parsed
	return nil
}

func (b *EventsBlock) takeKey(kv namedRaw) error {
	keyPath := "events." + kv.key
	if kv.key == "channels" {
		return b.takeChannels(kv.raw)
	}
	spec, ok := eventsTopLevelInts[kv.key]
	if !ok {
		return fmt.Errorf("%w %q", ErrUnknownKey, keyPath)
	}
	value, err := intAt(kv.raw, keyPath, spec.min)
	if err != nil {
		return err
	}
	*spec.field(b) = &value
	return nil
}

func (b *EventsBlock) takeChannels(raw json.RawMessage) error {
	pairs, err := orderedObject(raw, "events.channels")
	if err != nil {
		return err
	}
	for _, kv := range pairs {
		override, diagnostics, err := parseChannelOverride(kv)
		if err != nil {
			return err
		}
		b.channels = append(b.channels, override)
		b.diagnostics = append(b.diagnostics, diagnostics...)
	}
	return nil
}

func parseChannelOverride(kv namedRaw) (channelOverride, []error, error) {
	base := "events.channels." + kv.key
	if !channel.ValidName(kv.key) {
		return channelOverride{}, nil, &ValueError{Path: "events.channels." + kv.key, Err: fmt.Errorf("channel refuses the name")}
	}
	fields, err := orderedObject(kv.raw, base)
	if err != nil {
		return channelOverride{}, nil, err
	}
	o := channelOverride{name: kv.key}
	for _, f := range fields {
		if err := o.takeField(base, f); err != nil {
			return channelOverride{}, nil, err
		}
	}
	diagnostics := o.checkRoute()
	return o, diagnostics, nil
}

func (o *channelOverride) takeField(base string, f namedRaw) error {
	fieldPath := base + "." + f.key
	switch f.key {
	case "route":
		return textAt(f.raw, fieldPath, &o.route)
	case "qos":
		if err := textAt(f.raw, fieldPath, &o.qos); err != nil {
			return err
		}
		if !publisher.ValidQoS(publisher.QoS(*o.qos)) {
			return &ValueError{Path: fieldPath, Err: fmt.Errorf("publisher refuses qos %q", *o.qos)}
		}
		return nil
	case "ttl_days":
		return o.takeInt(&o.ttlDays, f.raw, fieldPath)
	case "max_total_mb":
		return o.takeInt(&o.maxTotalMB, f.raw, fieldPath)
	case "segment_mb":
		return o.takeInt(&o.segmentMB, f.raw, fieldPath)
	}
	return fmt.Errorf("%w %q", ErrUnknownKey, fieldPath)
}

func (o *channelOverride) takeInt(dst **int, raw json.RawMessage, keyPath string) error {
	value, err := intAt(raw, keyPath, 1)
	if err != nil {
		return err
	}
	*dst = &value
	return nil
}

func (o *channelOverride) checkRoute() []error {
	if o.route == nil {
		return nil
	}
	_, warnings, err := filter.Parse(*o.route, filter.RegisteredCatalog())
	if err != nil {
		o.routeRefused = true
		return []error{&RouteError{Channel: o.name, Err: err}}
	}
	out := make([]error, len(warnings))
	for i, w := range warnings {
		out[i] = &RouteWarning{Channel: o.name, Warning: w}
	}
	return out
}

func orderedObject(raw []byte, keyPath string) ([]namedRaw, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, &ValueError{Path: keyPath, Err: fmt.Errorf("want an object")}
	}
	var out []namedRaw
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, &ValueError{Path: keyPath, Err: err}
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, &ValueError{Path: keyPath, Err: err}
		}
		out = append(out, namedRaw{key: keyTok.(string), raw: value})
	}
	return out, nil
}

func intAt(raw json.RawMessage, keyPath string, min int) (int, error) {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, &ValueError{Path: keyPath, Err: err}
	}
	if value < min {
		return 0, &ValueError{Path: keyPath, Err: fmt.Errorf("want at least %d, got %d", min, value)}
	}
	return value, nil
}

func textAt(raw json.RawMessage, keyPath string, dst **string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return &ValueError{Path: keyPath, Err: err}
	}
	*dst = &value
	return nil
}

func (p Policy) EventsConfig() EventsPolicy {
	out := EventsDefaults()
	if p.Events == nil {
		return out
	}
	return p.Events.apply(out)
}

func (b *EventsBlock) apply(base EventsPolicy) EventsPolicy {
	setInt(&base.SegmentMB, b.segmentMB)
	setInt(&base.LockDeadlineMS, b.lockDeadlineMS)
	setInt(&base.QueueEvents, b.queueEvents)
	setInt(&base.QueueBytes, b.queueBytes)
	setInt(&base.EnqueueDeadlineMS, b.enqueueDeadlineMS)
	for _, o := range b.channels {
		base.Channels = o.overlay(base.Channels)
	}
	return base
}

func (o channelOverride) overlay(channels []EventChannel) []EventChannel {
	i := slices.IndexFunc(channels, func(c EventChannel) bool { return c.Name == o.name })
	if i < 0 && o.routeRefused {
		return channels
	}
	if i < 0 {
		channels = append(channels, EventChannel{Name: o.name, QoS: eventQoSBestEffort, MaxTotalMB: DefaultChannelMaxTotalMB})
		i = len(channels) - 1
	}
	c := channels[i]
	if o.route != nil && !o.routeRefused {
		c.Route = *o.route
	}
	if o.qos != nil {
		c.QoS = *o.qos
	}
	setInt(&c.TTLDays, o.ttlDays)
	setInt(&c.MaxTotalMB, o.maxTotalMB)
	setInt(&c.SegmentMB, o.segmentMB)
	channels[i] = c
	return channels
}

func setInt(dst *int, src *int) {
	if src != nil {
		*dst = *src
	}
}

func (p Policy) EventsDiagnostics() []error {
	if p.Events == nil {
		return nil
	}
	return slices.Clone(p.Events.diagnostics)
}

func (p Policy) PrintEventsDiagnostics(w io.Writer) {
	for _, d := range p.EventsDiagnostics() {
		fmt.Fprintf(w, "[policy] WARN %v\n", d)
	}
}

func LoadForEvents(policyPath string, warn io.Writer) Policy {
	p, err := Load(policyPath)
	if err != nil {
		fmt.Fprintf(warn, "[events] WARN: %v; the compiled defaults apply\n", err)
		return Policy{}
	}
	return p
}

func (e EventsPolicy) GCChannels() []gcpolicy.ChannelRetention {
	out := make([]gcpolicy.ChannelRetention, len(e.Channels))
	for i, c := range e.Channels {
		capMB := c.MaxTotalMB
		if capMB <= 0 {
			capMB = DefaultChannelMaxTotalMB
		}
		out[i] = gcpolicy.ChannelRetention{Name: c.Name, Dir: path.Join(paths.EventChannelsDir, c.Name), TTLDays: c.TTLDays, MaxTotalMB: capMB}
	}
	return out
}
