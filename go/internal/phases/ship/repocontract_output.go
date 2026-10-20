package ship

import (
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

type packLog struct {
	notes io.Writer
	raw   io.Writer
	tally *gateTally
}

type gateTally struct {
	flaky        []string
	infraRetried []string
}

type infraRetryRecorder interface {
	recordInfraRetry(name string)
}

func (l packLog) Write(p []byte) (int, error) { return l.notes.Write(p) }

func (l packLog) recordInfraRetry(name string) {
	if l.tally != nil {
		l.tally.infraRetried = append(l.tally.infraRetried, name)
	}
}

func repoContractSummary(err error, tally gateTally, scanPath string) string {
	verdict := "green"
	if len(tally.infraRetried) > 0 {
		verdict += " after an infra retry of the " + strings.Join(tally.infraRetried, ", ")
	}
	if len(tally.flaky) > 0 {
		verdict += fmt.Sprintf(" with %s for %s", CodeBackstopFlake, strings.Join(tally.flaky, ", "))
	}
	if err != nil {
		verdict = "error"
		if se, ok := shiperr.AsShipError(err); ok {
			verdict = string(se.Code)
		}
	}
	return "[ship] repo-contract gate: " + verdict + "; full output: " + scanPath
}
