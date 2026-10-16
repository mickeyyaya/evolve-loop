package cyclestate

type CycleResult struct {
	Cycle              int
	FinalVerdict       string
	PhasesRun          []Phase
	TerminationReason  string
	RetroDecision      string
	SkippedPhases      []SkippedPhase
	VerdictsNotAdopted []VerdictNotAdopted
	SystemFailure      *SystemFailureSignal
	Remediations       []string
	SpineFailOpens     []SpineFailOpen
	FailReasons        []string
}

type SystemFailureSignal struct {
	Category string `json:"category"`
	Level    string `json:"level"`
	Evidence string `json:"evidence"`
	Halt     bool   `json:"halt"`
}

type SkippedPhase struct {
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
}

type VerdictNotAdopted struct {
	Phase   string `json:"phase"`
	Verdict string `json:"verdict"`
}

type SpineFailOpen struct {
	Phase           string `json:"phase"`
	MissingArtifact string `json:"missing_artifact"`
	Reason          string `json:"reason,omitempty"`
}

type TokenUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
	Subject  string `json:"subject,omitempty"`
}

const (
	DiagCodeTriageProtectedSurface  = "TRIAGE_PROTECTED_SURFACE"
	DiagCodeTriageTopNEmpty         = "TRIAGE_TOPN_EMPTY"
	DiagCodeTriageCommitmentInvalid = "TRIAGE_COMMITMENT_INVALID"
	DiagCodeTriageScopeUnanswered   = "TRIAGE_SCOPE_UNANSWERED"
)

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

func ErrorMessages(diags []Diagnostic) []string {
	var msgs []string
	for _, d := range diags {
		if d.Severity == SeverityError {
			msgs = append(msgs, d.Message)
		}
	}
	return msgs
}

func ErrorCodes(diags []Diagnostic) []string {
	var codes []string
	seen := map[string]bool{}
	for _, d := range diags {
		if d.Severity != SeverityError || d.Code == "" || seen[d.Code] {
			continue
		}
		seen[d.Code] = true
		codes = append(codes, d.Code)
	}
	return codes
}

type Disposition struct {
	TaskLevel    bool
	RouteConsole bool
}

func RefusalDisposition(code string) Disposition {
	switch code {
	case DiagCodeTriageProtectedSurface:
		return Disposition{TaskLevel: true, RouteConsole: true}
	case DiagCodeTriageTopNEmpty, DiagCodeTriageScopeUnanswered:
		return Disposition{TaskLevel: true}
	default:
		return Disposition{}
	}
}
