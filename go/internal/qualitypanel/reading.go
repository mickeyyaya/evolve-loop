package qualitypanel

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Method string

const (
	MethodWilson95    Method = "wilson95"
	MethodBootstrap95 Method = "bootstrap95"
	MethodCensus      Method = "census"
	MethodPassK       Method = "pass_k_unbiased"
	MethodMDE         Method = "mde"
)

type Status string

const (
	StatusOK           Status = "ok"
	StatusInconclusive Status = "inconclusive"
	StatusBreach       Status = "breach"
	StatusNoData       Status = "no_data"
)

const defaultOrigin = "qualitypanel.Events"

type Reading struct {
	Index     string
	Interval  Interval
	Method    Method
	N         int
	Window    string
	Source    string
	Cycle     int
	Origin    string
	Threshold *float64
	Status    Status
	Extra     map[string]string
}

func Judge(ix Index, r Reading) Status {
	ix = r.applyThreshold(ix)
	switch {
	case r.N == 0:
		return StatusNoData
	case !ix.HasThreshold:
		return StatusOK
	}
	status := sideOf(ix, r.Interval)
	if status == StatusBreach && r.N < ix.NMin {
		return StatusInconclusive
	}
	return status
}

func sideOf(ix Index, iv Interval) Status {
	bad, good := iv.Hi < ix.Threshold, iv.Lo >= ix.Threshold
	if ix.Direction == LowerIsBetter {
		bad, good = iv.Lo > ix.Threshold, iv.Hi <= ix.Threshold
	}
	switch {
	case bad:
		return StatusBreach
	case good:
		return StatusOK
	}
	return StatusInconclusive
}

func Events(r Reading, stage Stage) []signalcenter.Event {
	ix, ok := Lookup(r.Index)
	if !ok {
		return []signalcenter.Event{SourceWarning(r.Index, CodeSourceUnreadable, r.Source, "a reading names the unknown index "+strconv.Quote(r.Index))}
	}
	ix = r.applyThreshold(ix)
	status := r.Status
	if status == "" {
		status = Judge(ix, r)
	}
	out := []signalcenter.Event{readingEvent(ix, r, stage, status)}
	if status == StatusBreach && stage != StageShadow && ix.Breachable {
		out = append(out, r.event(signalcenter.KindQualityBreach, BreachCode(ix.ID), "the whole interval of "+ix.ID+" is on the bad side of its threshold", readingFields(ix, r, stage, status)))
	}
	if status == StatusNoData {
		out = append(out, r.event(signalcenter.KindQualityWarning, CodeVacuousDenominator, ix.ID+" has no data in window "+r.Window, map[string]string{"index": ix.ID, "window": r.Window, "source": r.Source}))
	}
	return out
}

func readingEvent(ix Index, r Reading, stage Stage, status Status) signalcenter.Event {
	return r.event(signalcenter.KindQualityReading, "", ix.ID+" "+ix.Name+" reading", readingFields(ix, r, stage, status))
}

func (r Reading) event(kind signalcenter.Kind, code signalcenter.Code, reason string, fields map[string]string) signalcenter.Event {
	severity := signalcenter.SeverityWarn
	if kind == signalcenter.KindQualityReading {
		severity = signalcenter.SeverityInfo
	}
	origin := r.Origin
	if origin == "" {
		origin = defaultOrigin
	}
	return signalcenter.Event{Cycle: r.Cycle, Module: signalcenter.ModuleQuality, Origin: origin, Kind: kind, Code: code, Severity: severity, Reason: reason, Fields: fields}
}

func readingFields(ix Index, r Reading, stage Stage, status Status) map[string]string {
	f := map[string]string{
		"index": ix.ID, "value": num(r.Interval.Value), "method": string(r.Method), "n": strconv.Itoa(r.N),
		"window": r.Window, "stage": string(stage), "status": string(status), "source": r.Source, "threshold": "",
		"lo": "", "hi": "",
	}
	if ix.HasThreshold {
		f["threshold"] = num(ix.Threshold)
	}
	if r.Method != MethodCensus {
		f["lo"], f["hi"] = num(r.Interval.Lo), num(r.Interval.Hi)
	}
	for k, v := range r.Extra {
		f[k] = v
	}
	return f
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

func SourceWarning(index string, code signalcenter.Code, source, reason string) signalcenter.Event {
	return Reading{Index: index, Source: source}.event(signalcenter.KindQualityWarning, code, reason, map[string]string{"index": index, "source": source})
}

func (r Reading) applyThreshold(ix Index) Index {
	if r.Threshold != nil {
		ix.Threshold, ix.HasThreshold = *r.Threshold, true
	}
	return ix
}
