package phasecmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseorder"
)

// RunPhaseOrder prints the registry's phase order, or the hardcoded order when the registry is off or missing.
func RunPhaseOrder(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve phase-order")
			fmt.Fprintln(stdout, "Env: EVOLVE_USE_PHASE_REGISTRY=0 forces hardcoded order")
			return 0
		}
	}
	useRegistry := os.Getenv("EVOLVE_USE_PHASE_REGISTRY") != "0"

	projectRoot := os.Getenv("EVOLVE_PROJECT_ROOT")
	if projectRoot != "" {
		// Only the env root can be relative; the git and cwd fallbacks are absolute.
		projectRoot = paths.AbsoluteRoot("EVOLVE_PROJECT_ROOT", projectRoot, func(m string) {
			fmt.Fprintf(stderr, "[phase-order] WARN: %s\n", m)
		})
	} else {
		out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err == nil {
			projectRoot = strings.TrimSpace(string(out))
		} else {
			projectRoot, _ = os.Getwd()
		}
	}
	registryPath := config.RegistryPath(projectRoot)

	phases, err := phaseorder.List(registryPath, useRegistry)
	if err != nil {
		fmt.Fprintf(stderr, "[phase-order] ERROR: %v\n", err)
		return 1
	}
	for _, p := range phases {
		fmt.Fprintln(stdout, p)
	}
	return 0
}
