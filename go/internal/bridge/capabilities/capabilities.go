// Package capabilities is the per-CLI capability catalog: the written-down,
// cross-referenced record of each LLM CLI's interactive control surface
// (slash commands, key bindings, extension mechanism, headless entrypoint).
//
// The catalog has two complementary sources:
//
//   - Static: hand-authored capabilities/<cli>.json, grounded in the official
//     CLI docs (cited in each file's "sources"). This is the "write it down,
//     cross-compare with online research" deliverable.
//   - Live: parseHelpOutput parses a captured `/help` pane; Diff reconciles it
//     against the static catalog so drift between docs and the installed
//     binary is surfaced (the `evolve bridge introspect` command).
//
// Mirrors the manifest loader's embed+override Repository pattern so a project
// can ship or correct a catalog without a rebuild.
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

// catalogSource is the test-swappable embedded source (cf. manifestSource).
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

// catalogDir is the writable override directory consulted before the embedded
// set.
func catalogDir() string {
	return catalogDirFn()
}

// SlashCommand is one interactive `/command`.
type SlashCommand struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose,omitempty"`
	Args    string `json:"args,omitempty"`
}

// KeyBinding is one keyboard control.
type KeyBinding struct {
	Keys   string `json:"keys"`
	Action string `json:"action"`
}

// Extension describes how a user adds capabilities to the CLI.
type Extension struct {
	// Kind is "plugin_marketplace" | "skills_mcp" | "none".
	Kind        string   `json:"kind"`
	Summary     string   `json:"summary,omitempty"`
	InstallFlow []string `json:"install_flow,omitempty"` // exact command sequence
}

// Headless describes the non-interactive entrypoint.
type Headless struct {
	Entrypoint string   `json:"entrypoint,omitempty"`
	Available  []string `json:"available,omitempty"`
}

// Catalog is the full capability record for one CLI.
type Catalog struct {
	CLI           string         `json:"cli"`
	DisplayName   string         `json:"display_name,omitempty"`
	SlashCommands []SlashCommand `json:"slash_commands"`
	KeyBindings   []KeyBinding   `json:"key_bindings,omitempty"`
	Extension     Extension      `json:"extension"`
	Headless      Headless       `json:"headless,omitempty"`
	Sources       []string       `json:"sources,omitempty"`
}

// LoadCatalog reads the catalog for cli (override dir wins over embedded).
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

// CatalogNames returns the sorted set of embedded catalog CLI names.
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

// slashLineRE captures the leading /command token on a line. Tolerant of the
// varied `/help` layouts across CLIs: "/cmd  desc", "/cmd — desc",
// "/cmd: desc", "  /cmd". A command token is a slash + (letter or '?') + word
// chars or hyphens — the leading-'?' arm captures ollama's "/?" help alias.
var slashLineRE = regexp.MustCompile(`(^|\s)(/[a-zA-Z?][a-zA-Z0-9_-]*)`)

// parseHelpOutput extracts the unique, sorted set of slash-command names from
// a captured `/help` pane. Pure and tolerant: it ignores prose and only keeps
// the first /token per line, so it works across the differing help layouts of
// claude / codex / agy / ollama.
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

// ParseHelp is the exported wrapper over the pure parser.
func ParseHelp(pane string) []SlashCommand { return parseHelpOutput(pane) }

// Drift is the reconciliation between the static catalog and a live `/help`.
type Drift struct {
	CLI              string   `json:"cli"`
	CatalogCount     int      `json:"catalog_count"`
	LiveCount        int      `json:"live_count"`
	InCatalogNotLive []string `json:"in_catalog_not_live"` // documented but absent from live /help
	InLiveNotCatalog []string `json:"in_live_not_catalog"` // present live but undocumented in catalog
}

// Clean reports whether the live surface matches the catalog exactly.
func (d Drift) Clean() bool {
	return len(d.InCatalogNotLive) == 0 && len(d.InLiveNotCatalog) == 0
}

// Diff reconciles a parsed live slash-command set against the static catalog.
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
