package phasecmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseorder"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var absoluteRoot = paths.AbsoluteRoot

func RunPhaseOrder(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve phase-order")
			return 0
		}
	}
	projectRoot := os.Getenv("EVOLVE_PROJECT_ROOT")
	if projectRoot != "" {
		projectRoot = absoluteRoot("EVOLVE_PROJECT_ROOT", projectRoot, func(m string) {
			fmt.Fprintf(stderr, "[phase-order] WARN: %s\n", m)
		})
	} else {
		out, err := sysexec.Command(context.Background(), "git", "rev-parse", "--show-toplevel").Output()
		if err == nil {
			projectRoot = strings.TrimSpace(string(out))
		} else {
			projectRoot, _ = os.Getwd()
		}
	}
	registryPath := config.RegistryPath(projectRoot)

	phases, err := phaseorder.List(registryPath)
	if err != nil {
		fmt.Fprintf(stderr, "[phase-order] ERROR: %v\n", err)
		return 1
	}
	for _, p := range phases {
		fmt.Fprintln(stdout, p)
	}
	return 0
}
