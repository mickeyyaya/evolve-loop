package modelcatalog_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRefreshDegradedEvent_NamesTheFamilyAndItsFallbackReasonAsARegisteredConfigWarning(t *testing.T) {
	t.Parallel()

	e := modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", "agy", "list timed out")

	if e.Module != signalcenter.ModuleConfig || e.Kind != signalcenter.KindConfigWarning || e.Severity != signalcenter.SeverityWarn {
		t.Errorf("module/kind/severity = %s/%s/%s, want config/config.warning/WARN", e.Module, e.Kind, e.Severity)
	}
	if e.Code != modelcatalog.CodeRefreshDegraded || string(e.Code) != "CONFIG_CATALOG_REFRESH_DEGRADED" {
		t.Errorf("code = %q, want CONFIG_CATALOG_REFRESH_DEGRADED", e.Code)
	}
	if owner, ok := signalcenter.IsRegistered(e.Code); !ok || owner != signalcenter.ModuleConfig {
		t.Errorf("code registered = %v for module %q, want the config module to own it", ok, owner)
	}
	if !e.Code.BelongsTo(signalcenter.ModuleConfig) || !signalcenter.ValidOrigin(e.Origin) || e.Origin != "commitKeepingLivePicks" {
		t.Errorf("code/origin = %q/%q, want a code of the config module and the origin given", e.Code, e.Origin)
	}
	if e.Fields["cli"] != "agy" || e.Fields["fallback_reason"] != "list timed out" {
		t.Errorf("fields = %v, want cli=agy and fallback_reason=list timed out", e.Fields)
	}
	if e.Reason == "" {
		t.Error("reason is empty: a WARN event without a reason is rewritten by the Center")
	}
}

func TestRefreshDegradedEvent_PassesTheCenterWithoutBeingRewritten(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()

	center.Emit(modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", "codex", "no models listed"))
	center.Flush()

	got := center.Recent()
	if len(got) != 1 {
		t.Fatalf("center holds %d events, want exactly the degraded event and no registry-drift companion: %+v", len(got), got)
	}
	if got[0].Code != modelcatalog.CodeRefreshDegraded || got[0].Kind != signalcenter.KindConfigWarning || got[0].Fields["cli"] != "codex" {
		t.Errorf("event = %+v, want it delivered as built", got[0])
	}
}

func TestRefreshDegradedEvent_AnEmptyFallbackReasonStillGivesAReasonAndOmitsNothingElse(t *testing.T) {
	t.Parallel()

	e := modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", "claude", "")

	if e.Reason == "" || e.Fields["cli"] != "claude" {
		t.Errorf("event = %+v, want a non-empty reason and the family kept when the fallback reason is unknown", e)
	}
}
