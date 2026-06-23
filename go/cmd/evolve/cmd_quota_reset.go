package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
	"github.com/mickeyyaya/evolveloop/go/internal/quotareset"
)

// runQuotaReset is the `evolve estimate-quota-reset [WORKSPACE]` subcommand.
// Ports legacy/scripts/dispatch/estimate-quota-reset.sh. Prints the
// 2-line stdout shape: ISO timestamp + "source=...".
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
	pol, _ := policy.Load(filepath.Join(os.Getenv("EVOLVE_PROJECT_ROOT"), ".evolve", "policy.json"))
	qr := pol.QuotaResetConfig()
	r, err := quotareset.Compute(workspace, quotareset.Options{
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
