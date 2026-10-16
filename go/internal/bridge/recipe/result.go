package recipe

import "strings"

type Status string

const (
	StatusComplete Status = "complete"
	StatusFailed   Status = "failed"
)

type StepStatus string

const (
	StepOK                StepStatus = "ok"
	StepTimedOut          StepStatus = "timed_out"
	StepTimedOutContinued StepStatus = "timed_out_continued"
	StepFailed            StepStatus = "failed"
)

type StepResult struct {
	Name     string     `json:"name"`
	Status   StepStatus `json:"status"`
	ElapsedS int        `json:"elapsed_s"`
	PaneTail string     `json:"pane_tail,omitempty"`
}

type Result struct {
	Recipe string       `json:"recipe"`
	CLI    string       `json:"cli"`
	Status Status       `json:"status"`
	Steps  []StepResult `json:"steps"`
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
