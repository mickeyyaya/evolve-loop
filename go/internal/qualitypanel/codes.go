package qualitypanel

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const (
	CodeSourceAbsent       signalcenter.Code = "QUALITY_SOURCE_ABSENT"
	CodeSourceUnreadable   signalcenter.Code = "QUALITY_SOURCE_UNREADABLE"
	CodeVacuousDenominator signalcenter.Code = "QUALITY_VACUOUS_DENOMINATOR"
)

func BreachCode(id string) signalcenter.Code {
	return signalcenter.Code("QUALITY_" + id + "_BREACH")
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleQuality, CodeSourceAbsent, "a declared source of a quality index does not exist; fields.index and fields.source name it (ADR-0153)")
	signalcenter.RegisterCode(signalcenter.ModuleQuality, CodeSourceUnreadable, "a source of a quality index exists but does not parse; fields.source names it, the reason holds the error (ADR-0153)")
	signalcenter.RegisterCode(signalcenter.ModuleQuality, CodeVacuousDenominator, "a quality reading has n = 0 where the index expects data; a rule that holds on an empty set proves nothing (ADR-0153)")
	for _, ix := range registry {
		if ix.Breachable {
			signalcenter.RegisterCode(signalcenter.ModuleQuality, BreachCode(ix.ID), "the whole 95% interval of "+ix.ID+" ("+ix.Name+") is on the bad side of its threshold at stage report or gate (ADR-0153)")
		}
	}
}
