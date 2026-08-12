package phaseoutputs

// signal_test.go — the survey's projection into the unified signal stream
// (abnormal-events.jsonl). One line per cycle, and the abnormality decision
// lives HERE, in the pure layer, so the loop adapter and any future emitter
// cannot disagree about what counts as a problem.

import (
	"strings"
	"testing"
)

func TestSignal_CompleteCycleWithHealthyChainIsNotAbnormal(t *testing.T) {
	t.Parallel()
	res := Survey([]string{"build"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	))
	details, abnormal := Signal(res, ChainPresent)
	if abnormal {
		t.Error("a fully-instrumented cycle with a present chain is the healthy case — flagging it abnormal drowns the stream")
	}
	if !strings.Contains(details, "1/1 complete") || !strings.Contains(details, string(ChainPresent)) {
		t.Errorf("details must carry the summary and the chain state: %q", details)
	}
}

func TestSignal_AuditNotRunAloneIsNotAbnormal(t *testing.T) {
	t.Parallel()
	res := Survey([]string{"scout"}, files(
		"scout-report.md", "scout-prompt.txt", "scout-events.ndjson", "scout-usage.json",
	))
	if _, abnormal := Signal(res, ChainAuditNotRun); abnormal {
		t.Error("a cycle whose audit never ran has nothing to comply with — not a chain abnormality")
	}
}

func TestSignal_GapsAndChainAnomaliesAreAbnormal(t *testing.T) {
	t.Parallel()
	gapped := Survey([]string{"build"}, files("build-report.md"))
	if _, abnormal := Signal(gapped, ChainPresent); !abnormal {
		t.Error("a cycle with missing review data must signal WARN — that silence is the defect this tool exists to end")
	}
	clean := Survey([]string{"build"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	))
	for _, chain := range []ChainStatus{ChainAbsent, ChainRecordMissing, ChainRecordCorrupt, ChainInconsistent} {
		if details, abnormal := Signal(clean, chain); !abnormal {
			t.Errorf("chain state %s demands operator attention; details=%q", chain, details)
		}
	}
}
