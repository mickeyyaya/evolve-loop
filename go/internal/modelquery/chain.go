package modelquery

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type ChainClassifier struct {
	CLIs       []string
	Dispatcher PromptDispatcher
	Log        io.Writer
}

func (c ChainClassifier) Classify(ctx context.Context, targetCLI string, modelIDs []string) (map[string]string, error) {
	if len(c.CLIs) == 0 {
		return nil, fmt.Errorf("modelquery: classifier chain for %s has no CLIs", targetCLI)
	}
	log := c.Log
	if log == nil {
		log = io.Discard
	}
	failures := make([]string, 0, len(c.CLIs))
	for _, cli := range c.CLIs {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("modelquery: classifier chain for %s stopped: %w", targetCLI, err)
		}
		tiers, err := CLIClassifier{CLI: cli, Dispatcher: c.Dispatcher}.Classify(ctx, targetCLI, modelIDs)
		if err == nil {
			return tiers, nil
		}
		fmt.Fprintf(log, "[modelquery] WARN %s: classifier cli=%s failed: %v\n", targetCLI, cli, err)
		failures = append(failures, err.Error())
	}
	return nil, fmt.Errorf("every classifier CLI failed (%s): %s", strings.Join(c.CLIs, ", "), strings.Join(failures, "; "))
}
