package phaseio

type Verdict string

const (
	VerdictPASS    Verdict = "PASS"
	VerdictFAIL    Verdict = "FAIL"
	VerdictWARN    Verdict = "WARN"
	VerdictSKIPPED Verdict = "SKIPPED"
)

func (v Verdict) IsValid() bool {
	switch v {
	case VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED:
		return true
	}
	return false
}

type Defect struct {
	Severity string
	Title    string
	Detail   string
}

type FailureBlock struct {
	Class   string
	Summary string
	Defects []Defect
}

type PhaseOutput struct {
	Phase           string
	Verdict         Verdict
	Failure         *FailureBlock
	Signals         map[string]any
	NextPhase       string
	CommitSHA       string
	WorktreeTreeSHA string
	Reconciled      bool
}

func (o PhaseOutput) IsPass() bool { return o.Verdict == VerdictPASS }

func (o PhaseOutput) HasFailure() bool { return o.Failure != nil }
