package agentcmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type Profile struct {
	Name        string `json:"name"`
	Archetype   string `json:"archetype"`
	Description string `json:"description,omitempty"`
}

type refusal struct {
	code   int
	reason string
}

func (r refusal) Error() string { return r.reason }

func projectRoot(flagValue string) string {
	if flagValue == "" {
		flagValue = cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	}
	return paths.AbsoluteRoot("--project-root", flagValue, nil)
}

func admit(name, root string) (phasespec.PhaseSpec, error) {
	if _, builtin := registry.For(name); builtin {
		return phasespec.PhaseSpec{}, refusal{exitRefused, fmt.Sprintf("%s is a built-in phase that reads and writes cycle state; run it with `evolve phase %s --cycle N`", name, name)}
	}
	cat, err := phasecmd.LoadCatalog(root)
	if err != nil {
		return phasespec.PhaseSpec{}, refusal{exitUsage, err.Error()}
	}
	spec, ok := cat.Get(name)
	switch {
	case !ok || !phasecmd.SpecRunnable(spec):
		return spec, refusal{exitUsage, fmt.Sprintf("unknown profile %q (see `evolve agent list`)", name)}
	case writesSource(spec, cat, root):
		return spec, refusal{exitRefused, fmt.Sprintf("%s writes source: it declares writes_source, or its profile sandbox can write outside the run workspace; a standalone run accepts read-only profiles only", name)}
	}
	return spec, nil
}

func listProfiles(root string) ([]Profile, error) {
	cat, err := phasecmd.LoadCatalog(root)
	if err != nil {
		return nil, err
	}
	out := []Profile{}
	for _, spec := range cat.All() {
		if phasecmd.SpecRunnable(spec) && !writesSource(spec, cat, root) {
			out = append(out, Profile{Name: spec.Name, Archetype: string(spec.RoleOrDefault()), Description: spec.Description})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func printJSON(w io.Writer, v any) {
	buf, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(w, string(buf))
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agent list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print a JSON array")
	root := fs.String("project-root", "", "project root (default EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	profiles, err := listProfiles(projectRoot(*root))
	if err != nil {
		fmt.Fprintf(stderr, "evolve agent list: %v\n", err)
		return exitFail
	}
	if *asJSON {
		printJSON(stdout, profiles)
		return 0
	}
	for _, p := range profiles {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", p.Name, p.Archetype, p.Description)
	}
	return 0
}

const runDirGrant = ".evolve/runs/cycle-*"

func writesSource(spec phasespec.PhaseSpec, cat phasespec.Catalog, root string) bool {
	if core.PhaseWritesSource(core.Phase(spec.Name), cat) {
		return true
	}
	name := strings.TrimPrefix(spec.AgentName(), "evolve-")
	profile, err := profiles.NewFromDir(filepath.Join(paths.EvolveDirOf(root), "profiles")).Get(name)
	return err != nil || !confinedToWorkspace(profile.Sandbox)
}

func confinedToWorkspace(s *profiles.SandboxConfig) bool {
	if s == nil || !s.Enabled || !s.ReadOnlyRepo {
		return false
	}
	for _, declared := range s.WriteSubpaths {
		grant := path.Clean(declared)
		if grant != runDirGrant && !strings.HasPrefix(grant, runDirGrant+"/") {
			return false
		}
	}
	return true
}
