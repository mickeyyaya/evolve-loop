package opscmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

var (
	doctorInstalledVersions = defaultInstalledVersions
	doctorLatestRelease     = defaultLatestRelease
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

func defaultLatestRelease(bin string) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	var url string
	switch bin {
	case "claude":
		url = "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"
	case "codex":
		url = "https://registry.npmjs.org/@openai/codex/latest"
	default:
		return "", fmt.Errorf("no release source configured for %s", bin)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Version == "" {
		return "", errors.New("empty version in release payload")
	}
	return payload.Version, nil
}

func parseVersionParts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	tokens := strings.Split(v, ".")
	parts := make([]int, 0, len(tokens))
	for _, tok := range tokens {
		var n int
		for i := 0; i < len(tok); i++ {
			if tok[i] >= '0' && tok[i] <= '9' {
				n = n*10 + int(tok[i]-'0')
			} else {
				break
			}
		}
		parts = append(parts, n)
	}
	return parts
}

func isLagging(installed, latest string) bool {
	if latest == "" || latest == "unavailable" {
		return false
	}
	inst := parseVersionParts(installed)
	lat := parseVersionParts(latest)
	maxLen := len(inst)
	if len(lat) > maxLen {
		maxLen = len(lat)
	}
	for i := 0; i < maxLen; i++ {
		var pInst, pLat int
		if i < len(inst) {
			pInst = inst[i]
		}
		if i < len(lat) {
			pLat = lat[i]
		}
		if pLat > pInst {
			return true
		}
		if pLat < pInst {
			return false
		}
	}
	return false
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
	return l, isLagging(inst, l)
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
