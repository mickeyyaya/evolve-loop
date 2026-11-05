package modelcatalog

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeRefreshDegraded signalcenter.Code = "CONFIG_CATALOG_REFRESH_DEGRADED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeRefreshDegraded, "a catalog refresh could not list the live models of one CLI family, so the prior live pick of that family stays in the committed catalog; the fields name the family and the fallback reason, and the origin names the composition step")
}

func RefreshDegradedEvent(origin, cli, reason string) signalcenter.Event {
	detail := reason
	if detail == "" {
		detail = "no fallback reason recorded"
	}
	return signalcenter.Event{
		Module:   signalcenter.ModuleConfig,
		Origin:   origin,
		Kind:     signalcenter.KindConfigWarning,
		Code:     CodeRefreshDegraded,
		Severity: signalcenter.SeverityWarn,
		Reason:   fmt.Sprintf("catalog_refresh_degraded %s: the live list failed (%s); the prior live pick stays", cli, detail),
		Fields:   map[string]string{"cli": cli, "fallback_reason": reason},
	}
}
