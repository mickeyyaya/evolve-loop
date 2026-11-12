package testdup

import (
	"strings"
	"testing"
	"time"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	day := time.Date(2026, time.November, 9, 0, 0, 0, 0, time.UTC)
	baseline, err := ParseBaseline(FormatBaseline(Baseline{Count: 1, Date: day}))
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scan(t.TempDir(), day)
	if err != nil {
		t.Fatal(err)
	}
	var group Group
	report = Report{Copies: report.Copies, Groups: append(report.Groups, group)}
	if err := Check(report, baseline); err == nil || !strings.Contains(err.Error(), "lower go/"+BaselineRelPath+" to 0") {
		t.Errorf("Check at 0 copies against a baseline of 1 = %v, want an error that names go/%s", err, BaselineRelPath)
	}
}
