package selfheal

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const rerunGreenSubject = "rerun-green"

type CIFailure struct {
	Jobs    []string
	Package string
	Test    string
	Step    string
	Repro   string
}

func (f CIFailure) subject() string {
	return rerunGreenSubject + ":" + f.name()
}

func (f CIFailure) name() string {
	switch {
	case f.Step != "":
		return f.Step
	case f.Test != "" && f.Package != "":
		return f.Package + "." + f.Test
	case f.Test != "":
		return f.Test
	case f.Package != "":
		return f.Package
	}
	return strings.Join(f.Jobs, ",")
}

func (f CIFailure) summary(base string) string {
	out := base + ": " + f.name()
	if len(f.Jobs) > 0 {
		out += " (" + strings.Join(f.Jobs, ", ") + ")"
	}
	return out
}

func (r CIRun) IsRerunGreen() bool {
	return r.Attempt > 1 && r.Status == completed && r.Conclusion == "success"
}

func (r CIRun) flakyObservations() []Observation {
	summary := "a CI rerun of the same code turned green"
	if r.Unread {
		return nil
	}
	if len(r.Failures) == 0 {
		return []Observation{{Class: ClassFlakyTest, Subject: rerunGreenSubject + ":run-" + strconv.FormatInt(r.ID, 10), Severity: signalcenter.SeverityWarn, Summary: summary}}
	}
	out := make([]Observation, 0, len(r.Failures))
	for _, f := range r.Failures {
		out = append(out, Observation{Class: ClassFlakyTest, Subject: f.subject(), Severity: signalcenter.SeverityWarn, ReproTest: f.Repro, Summary: f.summary(summary)})
	}
	return out
}
