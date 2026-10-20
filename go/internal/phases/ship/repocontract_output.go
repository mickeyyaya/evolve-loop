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
}

func (l packLog) Write(p []byte) (int, error) { return l.notes.Write(p) }

func repoContractSummary(err error, flaky []string, scanPath string) string {
	verdict := "green"
	if len(flaky) > 0 {
		verdict = fmt.Sprintf("green with %s for %s", CodeBackstopFlake, strings.Join(flaky, ", "))
	}
	if err != nil {
		verdict = "error"
		if se, ok := shiperr.AsShipError(err); ok {
			verdict = string(se.Code)
		}
	}
	return "[ship] repo-contract gate: " + verdict + "; full output: " + scanPath
}
