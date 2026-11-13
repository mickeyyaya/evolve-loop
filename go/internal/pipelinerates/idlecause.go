package pipelinerates

import (
	"sort"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const idleIndex = "PQ8"

var idleBase = censusBase{index: idleIndex, source: signalsStream, origin: "pipelinerates.IdleCauseShare"}

func IdleCauseShare(flows []WaveFlow) []qualitypanel.Reading {
	var out []qualitypanel.Reading
	for _, f := range flows {
		var total float64
		causes := make([]string, 0, len(f.IdleHours))
		for cause, h := range f.IdleHours {
			total += h
			causes = append(causes, cause)
		}
		if total <= 0 {
			continue
		}
		sort.Strings(causes)
		for _, cause := range causes {
			r := idleBase.reading(waveWindow(f.Wave)+"/cause:"+cause, f.IdleHours[cause]/total, 1)
			r.Extra = map[string]string{"idle_slot_hours": strconv.FormatFloat(f.IdleHours[cause], 'f', 2, 64)}
			out = append(out, r)
		}
	}
	return out
}
