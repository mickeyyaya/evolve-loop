package bridge

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func clearAuthBench(projectRoot, cli string, source clihealth.ClearSource, deps Deps) {
	if projectRoot == "" {
		return
	}
	family, log := llmroute.Family(cli), deps.Stderr
	if log == nil {
		log = io.Discard
	}
	cleared, err := clihealth.NewStore(projectRoot, deps.Now).ClearAuthBench(family, source, deps.Signals)
	if err != nil {
		fmt.Fprintf(log, "[bridge] WARN the %s of %s succeeded, but its auth bench was not cleared: %v\n", source, family, err)
	}
	for _, e := range cleared {
		fmt.Fprintf(log, "[bridge] the %s of %s succeeded: the %s bench %s is cleared\n", source, family, e.Reason, e.Key())
	}
}
