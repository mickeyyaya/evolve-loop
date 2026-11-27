package wavehealth

import "sort"

const (
	TriggerNoShip     = "no-ship"
	TriggerAbnormal   = "abnormal-kind"
	TriggerLaneStalls = "lane-stalls"
)

type StallTrigger struct {
	Trigger string
	Kind    string
	Detail  string
	Lanes   []string
	Cycles  []int
}

func StallFindings(s Snapshot, r Report) []StallTrigger {
	var out []StallTrigger
	for _, st := range r.Steps {
		if st.Result != ResultBrake {
			continue
		}
		switch st.ID {
		case "A2":
			out = append(out, StallTrigger{Trigger: TriggerNoShip, Detail: st.Reading})
		case "A4":
			if f, ok := abnormalTrigger(s, st); ok {
				out = append(out, f)
			}
		case "A5":
			out = append(out, StallTrigger{Trigger: TriggerLaneStalls, Detail: st.Reading, Lanes: silentStallLaneNames(s)})
		}
	}
	return out
}

func abnormalTrigger(s Snapshot, st Step) (StallTrigger, bool) {
	if !s.Abnormal.Known {
		return StallTrigger{}, false
	}
	tallies := tallyKinds(s.Abnormal.Value, s.Now)
	if len(tallies) == 0 {
		return StallTrigger{}, false
	}
	top := tallies[0]
	cycles := make([]int, 0, len(top.cycles))
	for id := range top.cycles {
		cycles = append(cycles, id)
	}
	sort.Ints(cycles)
	return StallTrigger{Trigger: TriggerAbnormal, Kind: top.kind, Detail: st.Reading, Cycles: cycles}, true
}

func silentStallLaneNames(s Snapshot) []string {
	if !s.Stalls.Known {
		return nil
	}
	seen := map[string]bool{}
	var lanes []string
	for _, l := range stillSilent(stallsInWave(s)) {
		if l.Lane != "" && !seen[l.Lane] {
			seen[l.Lane] = true
			lanes = append(lanes, l.Lane)
		}
	}
	sort.Strings(lanes)
	return lanes
}
