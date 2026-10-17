package opscmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

var (
	doctorInstalledVersions = defaultInstalledVersions
	doctorLatestRelease     = looppreflight.DefaultLatestRelease
)

func defaultInstalledVersions() map[string]string {
	candidates := []string{"agy", "claude", "codex", "gemini", "ollama"}
	res := make(map[string]string, len(candidates))
	for _, c := range candidates {
		if v, err := looppreflight.CLIVersion(c); err == nil && v != "" {
			res[c] = v
		}
	}
	return res
}

type doctorVersionRow struct {
	CLI       string `json:"cli"`
	Installed string `json:"installed"`
	Latest    string `json:"latest"`
	Lagging   bool   `json:"lagging"`
}

type doctorVersionsPayload struct {
	Versions []doctorVersionRow `json:"versions"`
}

func collectDoctorVersionRows(checkLatest bool) ([]doctorVersionRow, bool) {
	installed := doctorInstalledVersions()
	clis := make([]string, 0, len(installed))
	for cli := range installed {
		clis = append(clis, cli)
	}
	sort.Strings(clis)

	rows := make([]doctorVersionRow, 0, len(clis))
	anyLagging := false
	for _, cli := range clis {
		inst := installed[cli]
		var lat string
		var lag bool
		if checkLatest {
			lat, lag = resolveLatestAndLag(cli, inst)
			if lag {
				anyLagging = true
			}
		}
		rows = append(rows, doctorVersionRow{
			CLI:       cli,
			Installed: inst,
			Latest:    lat,
			Lagging:   lag,
		})
	}
	return rows, anyLagging
}

func resolveLatestAndLag(cli, inst string) (string, bool) {
	if doctorLatestRelease == nil {
		return "unavailable", false
	}
	l, err := doctorLatestRelease(cli)
	if err != nil {
		return "unavailable", false
	}
	return l, looppreflight.IsLagging(inst, l)
}

func printDoctorVersionsText(w io.Writer, rows []doctorVersionRow, checkLatest bool) {
	for _, r := range rows {
		if checkLatest {
			if r.Lagging {
				fmt.Fprintf(w, "%-10s %s (latest: %s) [lagging]\n", r.CLI, r.Installed, r.Latest)
			} else {
				fmt.Fprintf(w, "%-10s %s (latest: %s)\n", r.CLI, r.Installed, r.Latest)
			}
		} else {
			fmt.Fprintf(w, "%-10s %s\n", r.CLI, r.Installed)
		}
	}
}

func printDoctorVersionsJSON(w io.Writer, rows []doctorVersionRow) error {
	payload := doctorVersionsPayload{Versions: rows}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func runDoctorVersions(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve doctor versions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	var checkLatest bool
	fs.BoolVar(&asJSON, "json", false, "emit JSON payload")
	fs.BoolVar(&checkLatest, "latest", false, "compare against latest releases")
	if err := fs.Parse(cmdutil.ReorderArgs(args)); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve doctor versions: unexpected arguments: %v\n", fs.Args())
		return exitUsage
	}

	rows, anyLagging := collectDoctorVersionRows(checkLatest)
	if asJSON {
		if err := printDoctorVersionsJSON(stdout, rows); err != nil {
			fmt.Fprintf(stderr, "evolve doctor versions: %v\n", err)
			return exitFail
		}
	} else {
		printDoctorVersionsText(stdout, rows, checkLatest)
	}

	if checkLatest && anyLagging {
		return exitFail
	}
	return exitOK
}
