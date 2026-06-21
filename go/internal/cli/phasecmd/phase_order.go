package phasecmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseorder"
)

// runPhaseOrder is the `evolve phase-order` subcommand. Ports
// legacy/scripts/dispatch/list-phase-order.sh — emits the phase names
// in order from phase-registry.json, falling back to a hardcoded order
// when EVOLVE_USE_PHASE_REGISTRY=0 or registry is missing.
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
		// Absolutize a relative env root (cycle-119 class). The git/cwd
		// fallbacks below already yield absolute paths.
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
	registryPath := filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")

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
