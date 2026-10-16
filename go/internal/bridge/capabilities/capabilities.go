// Package capabilities is the per-CLI capability catalog: the written-down,
// cross-referenced record of each LLM CLI's interactive control surface
// (slash commands, key bindings, extension mechanism, headless entrypoint).
package capabilities

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

//go:embed catalogs/*.json
var embeddedCatalogs embed.FS

type catalogSource interface {
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

var catalogFS catalogSource = embeddedCatalogs

var catalogDirFn = func() string {
	layout := paths.ResolveFromEnv()
	pol, err := policy.Load(filepath.Join(layout.EvolveDir, "policy.json"))
	if err == nil {
		if dir := pol.BridgeConfig().CatalogDir; dir != "" {
			return dir
		}
	}
	return filepath.Join(layout.EvolveDir, "bridge-catalogs")
}

func catalogDir() string {
	return catalogDirFn()
}

type SlashCommand struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose,omitempty"`
	Args    string `json:"args,omitempty"`
}

type KeyBinding struct {
	Keys   string `json:"keys"`
	Action string `json:"action"`
}

type Extension struct {
	Kind        string   `json:"kind"`
	Summary     string   `json:"summary,omitempty"`
	InstallFlow []string `json:"install_flow,omitempty"`
}

type Headless struct {
	Entrypoint string   `json:"entrypoint,omitempty"`
	Available  []string `json:"available,omitempty"`
}

type Catalog struct {
	CLI           string         `json:"cli"`
	DisplayName   string         `json:"display_name,omitempty"`
	SlashCommands []SlashCommand `json:"slash_commands"`
	KeyBindings   []KeyBinding   `json:"key_bindings,omitempty"`
	Extension     Extension      `json:"extension"`
	Headless      Headless       `json:"headless,omitempty"`
	Sources       []string       `json:"sources,omitempty"`
}

func LoadCatalog(cli string) (Catalog, error) {
	if cli == "" {
		return Catalog{}, fmt.Errorf("capabilities: empty cli name")
	}
	if data, err := os.ReadFile(filepath.Join(catalogDir(), cli+".json")); err == nil {
		return parseCatalog(cli, data)
	}
	data, err := catalogFS.ReadFile("catalogs/" + cli + ".json")
	if err != nil {
		return Catalog{}, fmt.Errorf("capabilities: no catalog for cli=%s", cli)
	}
	return parseCatalog(cli, data)
}

func CatalogNames() []string {
	entries, err := catalogFS.ReadDir("catalogs")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if n := e.Name(); strings.HasSuffix(n, ".json") {
			out = append(out, strings.TrimSuffix(n, ".json"))
		}
	}
	sort.Strings(out)
	return out
}

func parseCatalog(cli string, data []byte) (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return Catalog{}, fmt.Errorf("capabilities: invalid JSON for %s: %w", cli, err)
	}
	if c.CLI == "" {
		c.CLI = cli
	}
	return c, nil
}

var slashLineRE = regexp.MustCompile(`(^|\s)(/[a-zA-Z?][a-zA-Z0-9_-]*)`)

func parseHelpOutput(pane string) []SlashCommand {
	seen := map[string]bool{}
	for _, line := range strings.Split(pane, "\n") {
		if m := slashLineRE.FindStringSubmatch(line); m != nil {
			seen[m[2]] = true
		}
	}
	out := make([]SlashCommand, 0, len(seen))
	for n := range seen {
		out = append(out, SlashCommand{Name: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ParseHelp(pane string) []SlashCommand { return parseHelpOutput(pane) }

type Drift struct {
	CLI              string   `json:"cli"`
	CatalogCount     int      `json:"catalog_count"`
	LiveCount        int      `json:"live_count"`
	InCatalogNotLive []string `json:"in_catalog_not_live"`
	InLiveNotCatalog []string `json:"in_live_not_catalog"`
}

func (d Drift) Clean() bool {
	return len(d.InCatalogNotLive) == 0 && len(d.InLiveNotCatalog) == 0
}

func Diff(cat Catalog, live []SlashCommand) Drift {
	catSet := map[string]bool{}
	for _, c := range cat.SlashCommands {
		catSet[c.Name] = true
	}
	liveSet := map[string]bool{}
	for _, c := range live {
		liveSet[c.Name] = true
	}
	d := Drift{CLI: cat.CLI, CatalogCount: len(catSet), LiveCount: len(liveSet)}
	for n := range catSet {
		if !liveSet[n] {
			d.InCatalogNotLive = append(d.InCatalogNotLive, n)
		}
	}
	for n := range liveSet {
		if !catSet[n] {
			d.InLiveNotCatalog = append(d.InLiveNotCatalog, n)
		}
	}
	sort.Strings(d.InCatalogNotLive)
	sort.Strings(d.InLiveNotCatalog)
	return d
}
