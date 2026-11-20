package orchestrator

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type HostPressureEvent struct {
	Old, New, Sized, CPUs                                  int
	Cause, Stage                                           string
	Applied                                                bool
	Load1, LoadPerCore, MemAvailablePct, SwapUsedMB, Score float64
}

func (p Publish) HostPressureChange(e HostPressureEvent) bool {
	if e.New == e.Old {
		return false
	}
	p.HostPressure(e)
	return true
}

func (p Publish) HostPressure(e HostPressureEvent) {
	p.emit("hostpressure.Decide", CodeHostPressure, signalcenter.SeverityInfo,
		"host pressure: cap "+strconv.Itoa(e.Old)+" to "+strconv.Itoa(e.New)+": "+e.Cause,
		map[string]string{
			"old": strconv.Itoa(e.Old), "new": strconv.Itoa(e.New), "sized": strconv.Itoa(e.Sized), "cause": e.Cause,
			"stage": e.Stage, "applied": strconv.FormatBool(e.Applied), "cpus": strconv.Itoa(e.CPUs),
			"load1": formatFloat(e.Load1), "load_per_core": formatFloat(e.LoadPerCore),
			"mem_available_pct": formatFloat(e.MemAvailablePct), "swap_used_mb": formatFloat(e.SwapUsedMB), "score": formatFloat(e.Score),
		})
}

func (p Publish) HostBlind(err error, goos string) {
	p.emit("hostpressure.Read", CodeHostBlind, signalcenter.SeverityWarn,
		"host pressure reader failed: "+err.Error()+"; the cap is held",
		map[string]string{"error": err.Error(), "os": goos})
}

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
