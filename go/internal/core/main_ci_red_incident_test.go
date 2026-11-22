package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestMainCIRedIncident_NamesTheJobsTheTipAndTheFixItem(t *testing.T) {
	t.Parallel()
	r := MainCIRed{Main: originMainTip, Failed: originMainFailedJobs}

	e := r.Incident("test", nil)

	if e.Code != CodeMainCIRed || e.Severity != signalcenter.SeverityIncident || e.Module != signalcenter.ModuleGateRepo {
		t.Errorf("event = %+v, want a gate-repo INCIDENT with the CI code", e)
	}
	if e.Fields["main"] != originMainTip || e.Fields["fix_item"] != mainstop.FixItemID(originMainTip) || e.Fields["fix_item_error"] != "" {
		t.Errorf("fields = %v, want the tip and the fix item with no error", e.Fields)
	}
	if !strings.Contains(e.Fields["failed"], "build+test=failure") || !strings.Contains(e.Fields["failed"], "lint=timed_out") {
		t.Errorf("fields.failed = %q, want each failed job", e.Fields["failed"])
	}
}

func TestMainCIRedIncident_AFailedFixItemWriteIsInTheFieldAndTheReason(t *testing.T) {
	t.Parallel()
	e := MainCIRed{Main: originMainTip, Failed: originMainFailedJobs[:1]}.Incident("test", errors.New("disk full"))

	if e.Fields["fix_item_error"] != "disk full" || !strings.Contains(e.Reason, "disk full") {
		t.Errorf("event = %+v, want the file error in the field and the reason", e)
	}
}
