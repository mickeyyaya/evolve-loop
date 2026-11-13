package mainstop

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	var (
		_ Parked
		_ Proof
		_ Admission
	)
	var s Stop
	now := time.Time{}
	s = s.Opened(baseattr.Verdict{}, FixItemID(""), now).WithParked(Parked{}).Released(nil, "", now).Escalate(now)
	if s.Holds() || s.Overdue(now, 0) || !s.Resumes("") || s.Admit("", Proof{}) != AdmitNoStop || s.Cause() == "" || s.ParkedCycles() != "0" || s.ResumeCondition() == "" {
		t.Errorf("a stop with no key = %+v, want no hold, no admission refusal, a cause and a resume condition", s)
	}
	if AdmitFix == RefuseRebase || RefuseRebase == RefuseNotAFix || Schema == "" || Path("") == "" {
		t.Error("two admissions are equal, or the schema or the record path is empty")
	}
	if _, _, err := Read(t.TempDir()); err != nil {
		t.Error(err)
	}
	if _, err := Update(t.TempDir(), func(s Stop, _ bool) (Stop, error) { return s, nil }); err != nil {
		t.Error(err)
	}
}
