package policy

import (
	_ "embed"
	"fmt"
	"maps"
	"slices"
	"sync"
)

const (
	SeatPhaseAdvisor       = "phase-advisor"
	SeatFailureAdvisor     = "failure-advisor"
	SeatPlanJudge          = "plan-judge"
	SeatRouter             = "router"
	SeatContractEscalation = "contract-escalation"
	SeatPostmortem         = "postmortem"
	SeatObserver           = "observer"
	SeatSubagent           = "subagent"
	SeatNoProfile          = "no-profile"
	SeatWaveHealth         = "wave-health"
)

type Floors map[string][]string

func (f Floors) Families() []string {
	return slices.Sorted(maps.Keys(f))
}

func (f Floors) FamilyOf(agent string) string {
	for _, family := range f.Families() {
		if slices.Contains(f[family], agent) {
			return family
		}
	}
	return ""
}

func (f Floors) clone() Floors {
	out := make(Floors, len(f))
	for family, agents := range f {
		out[family] = slices.Clone(agents)
	}
	return out
}

type Seat struct {
	CLI   string `json:"cli,omitempty"`
	Model string `json:"model,omitempty"`
}

type routingDefaults struct {
	Floors                   Floors          `json:"floors"`
	Seats                    map[string]Seat `json:"seats"`
	ClassifierFamilies       []string        `json:"classifier_families"`
	UnmeasuredOrder          []string        `json:"unmeasured_order"`
	UniversalFallbackExclude []string        `json:"universal_fallback_exclude"`
}

//go:embed routing_defaults.json
var routingDefaultsJSON []byte

var compiledRoutingDefaults = sync.OnceValue(func() routingDefaults { return mustRoutingDefaults(routingDefaultsJSON) })

func mustRoutingDefaults(raw []byte) routingDefaults {
	var d routingDefaults
	if err := decodeStrict(raw, &d); err != nil {
		panic(fmt.Errorf("policy: the embedded routing_defaults.json does not decode: %w", err))
	}
	return d
}

func DefaultFloors() Floors { return compiledRoutingDefaults().Floors.clone() }

func (p Policy) Floors() Floors {
	if p.CLIRouting != nil && p.CLIRouting.Floors != nil {
		return p.CLIRouting.Floors.clone()
	}
	return DefaultFloors()
}

func SeatDefault(name string) Seat { return compiledRoutingDefaults().Seats[name] }

func ClassifierFamilies() []string { return slices.Clone(compiledRoutingDefaults().ClassifierFamilies) }

func UnmeasuredOrder() []string { return slices.Clone(compiledRoutingDefaults().UnmeasuredOrder) }

func defaultUniversalFallbackExclude() []string {
	return slices.Clone(compiledRoutingDefaults().UniversalFallbackExclude)
}
