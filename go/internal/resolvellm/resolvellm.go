// Package resolvellm resolves a phase role to {cli, model_tier} from its
// profile. Zero side effects.
//
// Step 9 (see docs/architecture/step9-llm-config-removal.md) removed the
// .evolve/llm_config.json layer: profiles (+ policy pins, applied upstream)
// own the CLI, and the live model-catalog overlay on the manifest ModelTierMap
// (Step 10c) translates the tier to a concrete model downstream. What remains
// here is the profile read:
//
//	profile cli + model_tier_default (default "balanced") → source="profile"
package resolvellm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Result is the resolved dispatch info for a role: the CLI + the abstract model
// tier. (Step 9 removed the exact-model path; the realizer's ModelTierMap —
// catalog-overlaid in Step 10c — translates ModelTier to a concrete model.)
type Result struct {
	CLI string
	// ModelTier is the canonical abstract tier ("fast" | "balanced" | "deep").
	// Legacy values ("haiku" | "sonnet" | "opus") are accepted for one release
	// via the realizer fallback ladder + parseManifest v1 shim; see ADR-0022.
	ModelTier string
	Source    string // always "profile" since Step 9
}

// ErrProfileNotFound signals the role's profile was not found. Matches bash
// exit code 1.
var ErrProfileNotFound = errors.New("resolvellm: profile not found")

// Options exposes seams for testing.
type Options struct {
	// PluginRoot maps to EVOLVE_PLUGIN_ROOT. Searched first for profile JSON.
	PluginRoot string
	// ProjectRoot maps to EVOLVE_PROJECT_ROOT. Searched second.
	ProjectRoot string
	// GitRoot is the git rev-parse --show-toplevel result. Searched last.
	GitRoot string
	// Env stubs os.Getenv (defaults to os.Getenv).
	Env func(string) string
}

// Resolve runs the precedence chain for role and returns a Result.
func Resolve(role string, opts Options) (Result, error) {
	if strings.TrimSpace(role) == "" {
		return Result{}, errors.New("resolvellm: role is required")
	}
	getEnv := opts.Env
	if getEnv == nil {
		getEnv = os.Getenv
	}

	// Profile resolution (Step 9: llm_config.json removed). CLI + model tier come
	// from the per-phase profile; the tier→concrete-model translation happens
	// downstream via the live model-catalog overlay on the manifest ModelTierMap
	// (Step 10c). See docs/architecture/step9-llm-config-removal.md.
	profilePath, err := findProfile(role, opts, getEnv)
	if err != nil {
		return Result{}, ErrProfileNotFound
	}
	raw, rerr := os.ReadFile(profilePath)
	if rerr != nil {
		return Result{}, fmt.Errorf("resolvellm: read profile: %w", rerr)
	}
	var prof profileDoc
	if perr := json.Unmarshal(raw, &prof); perr != nil {
		return Result{}, fmt.Errorf("resolvellm: parse %s: %w", profilePath, perr)
	}
	if prof.CLI == "" {
		return Result{}, fmt.Errorf("resolvellm: profile %s missing .cli field", profilePath)
	}
	tier := prof.ModelTierDefault
	if tier == "" {
		tier = "balanced"
	}
	return Result{CLI: prof.CLI, ModelTier: tier, Source: "profile"}, nil
}

// JSON returns the single wire shape resolvellm now emits (cli + model_tier +
// source). Manual formatting (not encoding/json) preserves key order.
func (r Result) JSON() string {
	return fmt.Sprintf(`{"cli":"%s","model_tier":"%s","source":"%s"}`, r.CLI, r.ModelTier, r.Source)
}

// findProfile mirrors resolve-llm.sh:_find_profile() — tries plugin root,
// project root, then git root, returning the first existing profile path.
func findProfile(role string, opts Options, getEnv func(string) string) (string, error) {
	candidates := []string{}
	if opts.PluginRoot != "" {
		candidates = append(candidates, filepath.Join(opts.PluginRoot, ".evolve", "profiles", role+".json"))
	}
	if v := getEnv("EVOLVE_PLUGIN_ROOT"); v != "" {
		candidates = append(candidates, filepath.Join(v, ".evolve", "profiles", role+".json"))
	}
	if opts.ProjectRoot != "" {
		candidates = append(candidates, filepath.Join(opts.ProjectRoot, ".evolve", "profiles", role+".json"))
	}
	if v := getEnv("EVOLVE_PROJECT_ROOT"); v != "" {
		candidates = append(candidates, filepath.Join(v, ".evolve", "profiles", role+".json"))
	}
	if opts.GitRoot != "" {
		candidates = append(candidates, filepath.Join(opts.GitRoot, ".evolve", "profiles", role+".json"))
	} else if gr := gitRoot(); gr != "" {
		candidates = append(candidates, filepath.Join(gr, ".evolve", "profiles", role+".json"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("not found")
}

func gitRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

type profileDoc struct {
	CLI              string `json:"cli"`
	ModelTierDefault string `json:"model_tier_default"`
}
