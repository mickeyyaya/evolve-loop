package modelcatalog_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRefreshDegradedEventAmplify_ReasonsKeepTheFieldsVerbatim(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"unicode":   "列表超时 ✗",
		"multiline": "line one\nline two",
		"long":      strings.Repeat("x", 4096),
		"quotes":    `say "no" and 'yes'`,
	}
	for name, reason := range cases {
		reason := reason
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", "agy", reason)
			if e.Fields["fallback_reason"] != reason {
				t.Errorf("fallback_reason changed: got %d bytes, want %d", len(e.Fields["fallback_reason"]), len(reason))
			}
			if e.Reason == "" || e.Severity != signalcenter.SeverityWarn {
				t.Errorf("event = %+v, want a reason and WARN severity", e)
			}
		})
	}
}

func TestRefreshDegradedEventAmplify_EachCallOwnsItsFieldsMap(t *testing.T) {
	t.Parallel()
	first := modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", "agy", "a")
	first.Fields["cli"] = "mutated"

	second := modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", "agy", "a")

	if second.Fields["cli"] != "agy" {
		t.Errorf("second event cli = %q, want agy: fields map is shared between calls", second.Fields["cli"])
	}
}

func TestRefreshDegradedEventAmplify_EmptyFamilyIsKeptNotInvented(t *testing.T) {
	t.Parallel()

	e := modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", "", "boom")

	if v, ok := e.Fields["cli"]; ok && v != "" {
		t.Errorf("cli field = %q, want empty or absent for an empty family", v)
	}
	if e.Code != modelcatalog.CodeRefreshDegraded {
		t.Errorf("code = %q, want %q", e.Code, modelcatalog.CodeRefreshDegraded)
	}
}

func TestRefreshDegradedEventAmplify_ManyEventsAllReachTheCenter(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	families := []string{"claude", "codex", "agy", "ollama"}

	for _, f := range families {
		center.Emit(modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", f, "down"))
	}
	center.Flush()

	got := center.Recent()
	seen := map[string]bool{}
	for _, e := range got {
		if e.Code == modelcatalog.CodeRefreshDegraded {
			seen[e.Fields["cli"]] = true
		}
	}
	for _, f := range families {
		if !seen[f] {
			t.Errorf("family %q missing from the center: %+v", f, got)
		}
	}
}
