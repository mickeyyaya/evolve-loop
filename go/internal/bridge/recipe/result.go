package recipe

import "strings"

// Status is the overall recipe outcome.
type Status string

const (
	StatusComplete Status = "complete"
	StatusFailed   Status = "failed"
)

// StepStatus is one step's outcome.
type StepStatus string

const (
	StepOK                StepStatus = "ok"
	StepTimedOut          StepStatus = "timed_out"           // OnTimeoutAbort → recipe fails
	StepTimedOutContinued StepStatus = "timed_out_continued" // OnTimeoutContinue → proceed
	StepFailed            StepStatus = "failed"              // send/capture/fail_regex/escalation
)

// StepResult records the outcome of one step.
type StepResult struct {
	Name     string     `json:"name"`
	Status   StepStatus `json:"status"`
	ElapsedS int        `json:"elapsed_s"`
	PaneTail string     `json:"pane_tail,omitempty"`
}

// Result is the structured outcome of a recipe run. It is always returned
// (even on error) so a caller can inspect which step failed and the pane that
// was visible when it did.
type Result struct {
	Recipe string       `json:"recipe"`
	CLI    string       `json:"cli"`
	Status Status       `json:"status"`
	Steps  []StepResult `json:"steps"`
}

// lastLines returns the last n lines of s (recipe-local copy of the bridge
// idiom — the recipe package imports nothing from bridge).
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
