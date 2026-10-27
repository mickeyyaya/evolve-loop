package agentcmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type Profile struct {
	Name         string `json:"name"`
	Archetype    string `json:"archetype"`
	Description  string `json:"description,omitempty"`
	WritesSource bool   `json:"writes_source,omitempty"`
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

func admit(o runOptions, root string) (phasespec.PhaseSpec, error) {
	name := o.profile
	if _, builtin := registry.For(name); builtin {
		return phasespec.PhaseSpec{}, refusal{exitRefused, fmt.Sprintf("%s is a built-in phase that reads and writes cycle state; run it with `evolve phase %s --cycle N`", name, name)}
	}
	cat, err := phasecmd.LoadCatalog(root)
	if err != nil {
		return phasespec.PhaseSpec{}, refusal{exitUsage, err.Error()}
	}
	spec, ok := cat.Get(name)
	if !ok || !phasecmd.SpecRunnable(spec) {
		return spec, refusal{exitUsage, fmt.Sprintf("unknown profile %q (see `evolve agent list`)", name)}
	}
	if err := checkWorktree(o.worktree); err != nil {
		return spec, err
	}
	if writesSource(spec, cat, root) {
		return spec, admitWriter(spec, root, o)
	}
	if len(o.writable) > 0 {
		return spec, refusal{exitUsage, fmt.Sprintf("%s is read-only: --writable is only for a profile that writes source", name)}
	}
	return spec, nil
}

func admitWriter(spec phasespec.PhaseSpec, root string, o runOptions) error {
	if !worktreeWriter(spec, root) {
		return refusal{exitRefused, fmt.Sprintf("%s writes source: it declares writes_source, or its profile sandbox can write outside the run workspace; a standalone writer needs a sandbox that writes only %s and the run workspace and denies %s", spec.Name, worktreeGrant, strings.Join(writerDenies, " and "))}
	}
	if o.worktree == "" || len(o.writable) == 0 {
		return refusal{exitUsage, fmt.Sprintf("%s writes source: give --worktree DIR (a linked git worktree) and one --writable PATH for each file that it can change", spec.Name)}
	}
	for _, rel := range o.writable {
		if !filepath.IsLocal(rel) || filepath.Clean(rel) != rel {
			return refusal{exitUsage, fmt.Sprintf("--writable %q is not a clean path inside the worktree", rel)}
		}
	}
	return nil
}

func checkWorktree(dir string) error {
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) {
		return refusal{exitUsage, fmt.Sprintf("--worktree %q is not an absolute path", dir)}
	}
	if info, err := plane.Classify(dir); err != nil || !info.IsLinkedWorktree {
		return refusal{exitRefused, fmt.Sprintf("--worktree %s is not a linked git worktree: a console seat never runs in the main checkout", dir)}
	}
	return nil
}

func listProfiles(root string) ([]Profile, error) {
	cat, err := phasecmd.LoadCatalog(root)
	if err != nil {
		return nil, err
	}
	out := []Profile{}
	for _, spec := range cat.All() {
		if !phasecmd.SpecRunnable(spec) {
			continue
		}
		writer := writesSource(spec, cat, root)
		if !writer || worktreeWriter(spec, root) {
			out = append(out, Profile{Name: spec.Name, Archetype: string(spec.RoleOrDefault()), Description: spec.Description, WritesSource: writer})
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

const (
	runDirGrant   = ".evolve/runs/cycle-*"
	worktreeGrant = "{worktree_path}"
)

var writerDenies = []string{".evolve/" + launchrecord.Subdir, commitgate.AttestDirName}

func profileOf(spec phasespec.PhaseSpec, root string) (profiles.Profile, error) {
	name := strings.TrimPrefix(spec.AgentName(), "evolve-")
	return profiles.NewFromDir(filepath.Join(paths.EvolveDirOf(root), "profiles")).Get(name)
}

func writesSource(spec phasespec.PhaseSpec, cat phasespec.Catalog, root string) bool {
	if core.PhaseWritesSource(core.Phase(spec.Name), cat) {
		return true
	}
	profile, err := profileOf(spec, root)
	return err != nil || !confinedToWorkspace(profile.Sandbox)
}

func worktreeWriter(spec phasespec.PhaseSpec, root string) bool {
	profile, err := profileOf(spec, root)
	return err == nil && confinedToWorktree(profile.Sandbox)
}

func confinedToWorkspace(s *profiles.SandboxConfig) bool {
	return confinedTo(s, runDirGrant)
}

func confinedToWorktree(s *profiles.SandboxConfig) bool {
	if !confinedTo(s, runDirGrant, worktreeGrant) {
		return false
	}
	for _, deny := range writerDenies {
		if !slices.Contains(s.DenySubpaths, deny) {
			return false
		}
	}
	return true
}

func confinedTo(s *profiles.SandboxConfig, grants ...string) bool {
	if s == nil || !s.Enabled || !s.ReadOnlyRepo {
		return false
	}
	for _, declared := range s.WriteSubpaths {
		if !slices.ContainsFunc(grants, func(g string) bool { return within(declared, g) }) {
			return false
		}
	}
	return true
}

func within(declared, grant string) bool {
	clean := path.Clean(declared)
	return clean == grant || strings.HasPrefix(clean, grant+"/")
}
