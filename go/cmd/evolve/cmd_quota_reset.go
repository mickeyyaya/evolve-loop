package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
)

var computeQuotaResetFn = quotareset.Compute

func runQuotaReset(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var workspace string
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve estimate-quota-reset [WORKSPACE]")
			return 0
		default:
			if workspace == "" {
				workspace = a
			}
		}
	}
	pol := policy.LoadOrDefaults(filepath.Join(os.Getenv("EVOLVE_PROJECT_ROOT"), ".evolve", "policy.json"), stderr)
	qr := pol.QuotaResetConfig()
	r, err := computeQuotaResetFn(workspace, quotareset.Options{
		ResetAt:      qr.ResetAt,
		DefaultHours: qr.DefaultHours,
	})
	if err != nil {
		fmt.Fprintf(stderr, "[estimate-quota-reset] FATAL: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, r.Format())
	return 0
}
