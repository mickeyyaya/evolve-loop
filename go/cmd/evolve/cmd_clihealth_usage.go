package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

type usageReadout struct {
	usageprobe.Observation
	Note string `json:"note,omitempty"`
}

type usageView struct {
	projectRoot string
	families    []string
	asJSON      bool
}

var clihealthUsagePaneFn = productionUsagePanes

func productionUsagePanes(projectRoot string) (func(context.Context, string) (string, error), func()) {
	ws, err := os.MkdirTemp("", "evolve-clihealth-usage-*")
	if err != nil {
		return func(context.Context, string) (string, error) { return "", fmt.Errorf("usage workspace: %w", err) }, func() {}
	}
	factory := bridge.NewControllerFactory(projectRoot, ws, "clihealth-usage", bridge.Deps{})
	return bridgeUsageProbe(factory), func() { _ = os.RemoveAll(ws) }
}

func clihealthUsage(v usageView, stdout, stderr io.Writer) int {
	families := v.families
	if len(families) == 0 {
		families = bridge.InteractiveFamilies()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	probe, cleanup := clihealthUsagePaneFn(v.projectRoot)
	defer cleanup()
	readouts := make([]usageReadout, 0, len(families))
	for _, family := range families {
		readouts = append(readouts, readUsage(ctx, probe, family))
	}
	printUsageReadouts(readouts, v.asJSON, stdout)
	failed := 0
	for _, r := range readouts {
		if r.Error != "" {
			fmt.Fprintf(stderr, "evolve clihealth usage: %s: %s\n", r.CLI, r.Error)
			failed++
		}
	}
	return min(failed, 1)
}

func readUsage(ctx context.Context, probe func(context.Context, string) (string, error), family string) usageReadout {
	now := time.Now()
	r := usageReadout{Observation: usageprobe.Observation{CLI: family, ObservedAt: now.UTC()}}
	pane, err := probe(ctx, family)
	switch {
	case errors.Is(err, clicontrol.ErrUnsupported):
		r.Note = "its manifest maps no usage command"
	case err != nil:
		r.Error = err.Error()
	default:
		r.Windows = bridge.UsageWindows(family, pane, now)
		if len(r.Windows) == 0 {
			r.Note = "no usage window read: its manifest declares no controls.usage.windows, or the screen did not match them"
		}
	}
	return r
}

func printUsageReadouts(readouts []usageReadout, asJSON bool, stdout io.Writer) {
	if asJSON {
		body, _ := json.MarshalIndent(readouts, "", "  ")
		fmt.Fprintln(stdout, string(body))
		return
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CLI\tSCOPE\tKIND\tUSED\tRESETS\tTARGET\tEXHAUSTED\tBENCHES FAMILY")
	for _, r := range readouts {
		for _, w := range r.Windows {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%g%%\t%s\t%s\t%t\t%t\n", r.CLI, w.Scope, w.Kind, w.PercentUsed, w.ResetsText, windowTarget(w), w.Exhausted, w.ExhaustsFamily())
		}
		if r.Note != "" {
			fmt.Fprintf(tw, "%s\t(%s)\n", r.CLI, r.Note)
		}
	}
	_ = tw.Flush()
}

func windowTarget(w quotastate.UsageWindow) string {
	switch {
	case w.Family == "":
		return "(none)"
	case w.Model != "":
		return w.Family + "/" + w.Model
	}
	return w.Family
}
