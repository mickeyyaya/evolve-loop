package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func bridgeCommandIntent(cli, permission string, efforts policy.EffortTable) bridge.LaunchIntent {
	return bridge.LaunchIntent{Permission: permission, Effort: bridge.LaunchEffort(cli, "", efforts)}
}

func bypassIf(allowBypass bool) string {
	if allowBypass {
		return "bypass"
	}
	return ""
}

func commandEfforts(stderr io.Writer) policy.EffortTable {
	root, err := routingProjectRoot("", os.Getwd)
	if err != nil {
		fmt.Fprintf(stderr, "evolve bridge: WARN %v; the effort is the compiled default\n", err)
		return policy.EffortTable{}
	}
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve bridge: WARN %v; the effort is the compiled default\n", err)
		return policy.EffortTable{}
	}
	return pol.Efforts()
}
