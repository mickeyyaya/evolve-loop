package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/publisher"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func parseEvents(t *testing.T, events string) (Policy, error) {
	t.Helper()
	return Parse([]byte(`{"events":` + events + `}`))
}

func mustParseEvents(t *testing.T, events string) Policy {
	t.Helper()
	p, err := parseEvents(t, events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func channelOf(t *testing.T, ev EventsPolicy, name string) EventChannel {
	t.Helper()
	for _, c := range ev.Channels {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("channel %q is not in %v", name, channelNamesOf(ev))
	return EventChannel{}
}

func TestEventsConfig_CompiledDefaultsMatchTheChannelTable(t *testing.T) {
	t.Parallel()

	if got := (Policy{}).EventsConfig(); !reflect.DeepEqual(got, EventsDefaults()) {
		t.Errorf("a policy with no events block = %+v, want EventsDefaults()", got)
	}
	got := mustParseEvents(t, `{"channels":{"ship":{"segment_mb":4}},"queue_events":7}`).EventsConfig()
	want := EventsDefaults()
	want.QueueEvents = 7
	for i := range want.Channels {
		if want.Channels[i].Name == "ship" {
			want.Channels[i].SegmentMB = 4
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a partial override =\n%+v\nwant only ship.segment_mb and queue_events changed:\n%+v", got, want)
	}
}

func TestEventsConfig_ANewChannelFollowsTheDefaultsInFileOrderAndTakesTheCapDefault(t *testing.T) {
	t.Parallel()

	got := mustParseEvents(t, `{"channels":{"zeta":{"route":"module=loop"},"alpha":{"route":"module=inbox","qos":"lossless","max_total_mb":9,"ttl_days":3}}}`).EventsConfig()

	names := channelNamesOf(got)
	if tail := names[len(names)-2:]; !reflect.DeepEqual(tail, []string{"zeta", "alpha"}) {
		t.Errorf("channel order ends %v, want the new channels in file order: zeta, alpha", tail)
	}
	zeta, alpha := channelOf(t, got, "zeta"), channelOf(t, got, "alpha")
	if zeta.MaxTotalMB != DefaultChannelMaxTotalMB || zeta.QoS != "best_effort" {
		t.Errorf("zeta = %+v, want the %d MB cap default and qos best_effort", zeta, DefaultChannelMaxTotalMB)
	}
	if alpha.MaxTotalMB != 9 || alpha.TTLDays != 3 || alpha.QoS != "lossless" {
		t.Errorf("alpha = %+v, want its own cap 9, ttl 3 and qos lossless", alpha)
	}
}

func TestEventsDefaults_HoldTheCapsOfTheSpec(t *testing.T) {
	t.Parallel()
	for _, c := range EventsDefaults().Channels {
		want := DefaultChannelMaxTotalMB
		if c.Name == "ledger" || c.Name == "signals" {
			want = 512
		}
		if c.MaxTotalMB != want {
			t.Errorf("channel %s cap = %d MB, want %d (spec §2)", c.Name, c.MaxTotalMB, want)
		}
	}
}

func TestEventsConfig_AnUnknownKeyFails(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"top level": `{"queue_event":5}`,
		"channel":   `{"channels":{"ship":{"segment":4}}}`,
	}
	for name, events := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseEvents(t, events); !errors.Is(err, ErrUnknownKey) {
				t.Errorf("Parse error = %v, want ErrUnknownKey", err)
			}
		})
	}
}

func TestEventsBlock_ADirectDecodeRefusesAnUnknownKeyToo(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string]string{"top level": `{"queue_event":5}`, "channel": `{"channels":{"ship":{"segment":4}}}`} {
		var b EventsBlock

		if err := json.Unmarshal([]byte(raw), &b); !errors.Is(err, ErrUnknownKey) {
			t.Errorf("%s: Unmarshal error = %v, want ErrUnknownKey without the generic pass", name, err)
		}
	}
}

func TestEventsConfig_ABadValueFails(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"events.channels.ship.qos":          `{"channels":{"ship":{"qos":"maybe"}}}`,
		"events.channels.ship.segment_mb":   `{"channels":{"ship":{"segment_mb":0}}}`,
		"events.channels.ship.ttl_days":     `{"channels":{"ship":{"ttl_days":-1}}}`,
		"events.channels.ship.max_total_mb": `{"channels":{"ship":{"max_total_mb":"big"}}}`,
		"events.channels.Bad Name":          `{"channels":{"Bad Name":{}}}`,
		"events.segment_mb":                 `{"segment_mb":0}`,
		"events.enqueue_deadline_ms":        `{"enqueue_deadline_ms":-5}`,
		"events.channels":                   `{"channels":[]}`,
		"events.channels.ship":              `{"channels":{"ship":5}}`,
		"events.channels.ship.route":        `{"channels":{"ship":{"route":5}}}`,
		"events.channels.cycle.qos":         `{"channels":{"cycle":{"qos":5}}}`,
		"events":                            `[1]`,
	}
	for path, events := range cases {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			_, err := parseEvents(t, events)

			var bad *ValueError
			if !errors.As(err, &bad) || bad.Path != path {
				t.Errorf("Parse error = %v, want *ValueError at %q", err, path)
			}
		})
	}
}

func TestEventsConfig_ARouteMustParse(t *testing.T) {
	t.Parallel()
	p, err := parseEvents(t, `{"channels":{"ship":{"route":"kind=no.such.kind"},"fresh":{"route":"nokey=1"},"cycle":{"qos":"best_effort"}}}`)
	if err != nil {
		t.Fatalf("a bad route must not fail the load: %v", err)
	}

	got := p.EventsConfig()

	if c := channelOf(t, got, "ship"); c.Route != "kind=ship.*" {
		t.Errorf("ship route = %q, want the compiled route kept when the override is refused", c.Route)
	}
	if c := channelOf(t, got, "cycle"); c.QoS != "best_effort" {
		t.Errorf("cycle qos = %q, want a good override of the same block still applied", c.QoS)
	}
	for _, name := range channelNamesOf(got) {
		if name == "fresh" {
			t.Error("channel fresh has a refused route and must be left out")
		}
	}
	routeErrors := map[string]bool{}
	for _, d := range p.EventsDiagnostics() {
		var re *RouteError
		if errors.As(d, &re) {
			routeErrors[re.Channel] = true
		}
	}
	if !routeErrors["ship"] || !routeErrors["fresh"] || len(routeErrors) != 2 {
		t.Errorf("route errors for %v, want exactly ship and fresh", routeErrors)
	}
}

func TestEventsConfig_AKindGlobThatMatchesNoRegisteredKindIsRefused(t *testing.T) {
	t.Parallel()
	p := mustParseEvents(t, `{"channels":{"ghost":{"route":"kind=ghost.*","qos":"lossless"}}}`)

	for _, name := range channelNamesOf(p.EventsConfig()) {
		if name == "ghost" {
			t.Error("the ghost channel must be refused: no registered kind matches its glob (spec §8)")
		}
	}
	if len(p.EventsDiagnostics()) != 1 {
		t.Errorf("diagnostics = %v, want one route error", p.EventsDiagnostics())
	}
}

func TestEventsConfig_ABadRouteStillLoadsEveryGCKnobAndSafetyFloor(t *testing.T) {
	t.Parallel()
	raw := `{"gc":{"mode":"enforce","logs_ttl_days":9},"docs_floor":{"stage":"enforce"},"events":{"channels":{"ship":{"route":"kind=no.such.kind"}}}}`

	p, err := Parse([]byte(raw))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.GC == nil || p.GC.Mode != "enforce" || p.GC.LogsTTLDays != 9 {
		t.Errorf("gc block = %+v, want mode enforce and logs_ttl_days 9 from the same file", p.GC)
	}
	if p.DocsFloor == nil || p.DocsFloor.Stage != "enforce" {
		t.Errorf("docs_floor = %+v, want stage enforce", p.DocsFloor)
	}
}

func TestEventsConfig_AnUnknownCodeLoadsAndIsReportedOncePerRoute(t *testing.T) {
	t.Parallel()
	p := mustParseEvents(t, `{"channels":{"ship":{"route":"code=NO_SUCH_CODE_X"},"cycle":{"route":"code=NO_SUCH_CODE_Y"}}}`)

	var warned []string
	for _, d := range p.EventsDiagnostics() {
		var rw *RouteWarning
		if errors.As(d, &rw) {
			warned = append(warned, rw.Channel)
		}
	}

	if !reflect.DeepEqual(warned, []string{"ship", "cycle"}) {
		t.Errorf("warned channels %v, want one RouteWarning for each of ship and cycle in file order", warned)
	}
	if c := channelOf(t, p.EventsConfig(), "ship"); c.Route != "code=NO_SUCH_CODE_X" {
		t.Errorf("ship route = %q, want a warned route to stay in force", c.Route)
	}
}

func TestEventsConfig_AGoodBlockHasNoDiagnostics(t *testing.T) {
	t.Parallel()
	if d := mustParseEvents(t, `{"channels":{"ship":{"segment_mb":2}}}`).EventsDiagnostics(); len(d) != 0 {
		t.Errorf("diagnostics = %v, want none", d)
	}
	if d := (Policy{}).EventsDiagnostics(); len(d) != 0 {
		t.Errorf("diagnostics of an empty policy = %v, want none", d)
	}
}

func TestEventsDiagnostics_PrintOneLineForEachDiagnosticWithItsChannel(t *testing.T) {
	t.Parallel()
	p := mustParseEvents(t, `{"channels":{"ship":{"route":"kind=no.such.kind"},"cycle":{"route":"code=NO_SUCH_CODE_Y"}}}`)
	var out bytes.Buffer

	p.PrintEventsDiagnostics(&out)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "ship") || !strings.Contains(lines[1], "cycle") {
		t.Errorf("printed %q, want two lines that name ship and cycle", lines)
	}
}

func TestEventsConfig_GCChannelsGiveTheResolvedCapsAndTheChannelHome(t *testing.T) {
	t.Parallel()
	ev := mustParseEvents(t, `{"channels":{"quality.breach":{"ttl_days":4},"fresh":{"route":"module=loop"}}}`).EventsConfig()

	got := ev.GCChannels()

	if len(got) != len(ev.Channels) {
		t.Fatalf("GCChannels() = %d entries, want one for each of the %d channels", len(got), len(ev.Channels))
	}
	byName := map[string]int{}
	for i, c := range got {
		byName[c.Name] = i
		if c.Dir != path.Join(paths.EventChannelsDir, c.Name) {
			t.Errorf("channel %s home = %q, want %s joined with the name", c.Name, c.Dir, paths.EventChannelsDir)
		}
	}
	if c := got[byName["quality.breach"]]; c.TTLDays != 4 || c.MaxTotalMB != DefaultChannelMaxTotalMB {
		t.Errorf("quality.breach = %+v, want ttl 4 and the default cap", c)
	}
	if c := got[byName["fresh"]]; c.MaxTotalMB != DefaultChannelMaxTotalMB || c.TTLDays != 0 {
		t.Errorf("fresh = %+v, want the default cap and ttl 0 (inherit gc.logs_ttl_days)", c)
	}
	if c := got[byName["ledger"]]; c.MaxTotalMB != 512 {
		t.Errorf("ledger cap = %d, want 512", c.MaxTotalMB)
	}
}

func TestLoadForEvents_AFailedLoadWritesOneLineAndGivesTheDefaults(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(file, []byte(`{"events":{"nope":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer

	got := LoadForEvents(file, &warn).EventsConfig()

	if !reflect.DeepEqual(got, EventsDefaults()) || strings.Count(warn.String(), "\n") != 1 || !strings.Contains(warn.String(), "compiled defaults apply") {
		t.Errorf("config equal to the defaults = %v, warning %q, want the defaults and one line", reflect.DeepEqual(got, EventsDefaults()), warn.String())
	}
	var quiet bytes.Buffer
	if p := LoadForEvents(filepath.Join(t.TempDir(), "absent.json"), &quiet); p.Events != nil || quiet.Len() != 0 {
		t.Errorf("an absent file gave %+v and wrote %q, want an empty policy and no line", p.Events, quiet.String())
	}
}

func TestEventsChannel_AZeroCapResolvesToTheDefaultInGCChannels(t *testing.T) {
	t.Parallel()
	got := EventsPolicy{Channels: []EventChannel{{Name: "x"}}}.GCChannels()

	if len(got) != 1 || got[0].MaxTotalMB != DefaultChannelMaxTotalMB {
		t.Errorf("GCChannels() = %+v, want the default cap for an unset cap", got)
	}
}

func TestOrderedObject_ATruncatedDocumentIsAValueError(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string]string{"cut after a key": `{"a"`, "cut after a pair": `{"a":1,`, "not an object": `[1]`} {
		_, err := orderedObject([]byte(raw), "events")

		var bad *ValueError
		if !errors.As(err, &bad) || bad.Path != "events" {
			t.Errorf("%s: orderedObject error = %v, want *ValueError at events", name, err)
		}
	}
}

func TestEventsErrors_KeepTheirCauseAndNameTheirPath(t *testing.T) {
	t.Parallel()
	cause := errors.New("cause")

	value := &ValueError{Path: "events.x", Err: cause}
	if err := error(value); !errors.Is(err, cause) || value.Unwrap() != cause || fmt.Sprint(err) == "" {
		t.Errorf("ValueError = %q, want it to wrap its cause and print a message", err)
	}
	route := &RouteError{Channel: "ship", Err: cause}
	if err := error(route); !errors.Is(err, cause) || route.Unwrap() != cause {
		t.Errorf("RouteError = %v, want it to wrap the parse error", err)
	}
}

func TestEventsConfig_ARouteRefusalWrapsTheFilterError(t *testing.T) {
	t.Parallel()
	p := mustParseEvents(t, `{"channels":{"ship":{"route":"kind=no.such.kind"}}}`)

	diagnostics := p.EventsDiagnostics()

	if len(diagnostics) != 1 || !errors.Is(diagnostics[0], filter.ErrRefused) {
		t.Errorf("diagnostics = %v, want one error that wraps filter.ErrRefused", diagnostics)
	}
}

func jsonTagsOf(t reflect.Type) []string {
	var tags []string
	for i := range t.NumField() {
		tags = append(tags, strings.Split(t.Field(i).Tag.Get("json"), ",")[0])
	}
	return tags
}

func TestEventsShape_NamesExactlyTheKeysThatTheBlockDecodes(t *testing.T) {
	t.Parallel()
	var topKeys []string
	for _, tag := range jsonTagsOf(reflect.TypeFor[eventsShape]()) {
		if tag != "channels" {
			topKeys = append(topKeys, tag)
		}
	}
	decoded := make([]string, 0, len(eventsTopLevelInts))
	for key := range eventsTopLevelInts {
		decoded = append(decoded, key)
	}
	slices.Sort(topKeys)
	slices.Sort(decoded)
	if !reflect.DeepEqual(topKeys, decoded) {
		t.Errorf("eventsShape top-level keys %v, decoder keys %v, want them equal", topKeys, decoded)
	}
	for _, key := range jsonTagsOf(reflect.TypeFor[eventsChannelShape]()) {
		var b EventsBlock
		raw := `{"channels":{"ship":{"` + key + `":1}}}`

		if err := json.Unmarshal([]byte(raw), &b); errors.Is(err, ErrUnknownKey) {
			t.Errorf("channel key %q is in eventsShape but the decoder calls it unknown", key)
		}
	}
}

func TestEventsConfig_ANameOrQoSTheOwnerRefusesIsRefusedAtLoad(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ok", "a.b", "a_b-c", "Upper", "a..b", "a b", "", ".a", "a."} {
		_, err := parseEvents(t, `{"channels":{"`+name+`":{"route":"module=loop"}}}`)

		if loaded := err == nil; loaded != channel.ValidName(name) {
			t.Errorf("name %q: loaded=%v but channel.ValidName=%v, want the load to follow the channel package", name, loaded, channel.ValidName(name))
		}
	}
	for _, qos := range []string{"lossless", "best_effort", "Lossless", "at_most_once", ""} {
		_, err := parseEvents(t, `{"channels":{"ship":{"qos":"`+qos+`"}}}`)

		if loaded := err == nil; loaded != publisher.ValidQoS(publisher.QoS(qos)) {
			t.Errorf("qos %q: loaded=%v but publisher.ValidQoS=%v, want the load to follow the publisher", qos, loaded, publisher.ValidQoS(publisher.QoS(qos)))
		}
	}
}
