// Package resolvellm ports legacy/scripts/dispatch/resolve-llm.sh.
//
// Pure-function LLM router. Given a role name and optional llm_config path,
// returns {cli, model|model_tier, source} describing which CLI+model the
// phase agent should use. Zero side effects.
//
// Resolution precedence (per ADR-1):
//  1. llm_config.phases.<role>        → source="llm_config"
//  2. llm_config._fallback            → source="llm_config_fallback"
//  3. profile cli + model_tier_default → source="profile"
//  4. llm_config absent               → source="profile"
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

// Result mirrors the JSON shape emitted by resolve-llm.sh. Exactly one of
// Model / ModelTier is non-empty in any single emission.
type Result struct {
	CLI   string
	Model string // exact model name (resolved from llm_config.phases.<role>.model)
	// ModelTier is the canonical abstract tier ("fast" | "balanced" | "deep").
	// Translated to CLI-native model by the realizer's ModelTierMap lookup.
	// Legacy values ("haiku" | "sonnet" | "opus") are accepted for one
	// release via the realizer fallback ladder + parseManifest v1 shim;
	// see ADR-0022 PR 2 addendum for the deprecation timeline.
	ModelTier string
	Source    string // "llm_config" | "llm_config_fallback" | "profile"
}

// ErrProfileNotFound signals neither llm_config nor a profile contained
// usable routing data. Matches bash exit code 1.
var ErrProfileNotFound = errors.New("resolvellm: profile not found")

// Options exposes seams for testing.
type Options struct {
	// ConfigPath optionally overrides the llm_config.json path. When empty,
	// the package derives it from EVOLVE_PROJECT_ROOT or git rev-parse.
	ConfigPath string
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

	// Step 1 / 2 — read llm_config.json
	configPath := opts.ConfigPath
	if configPath == "" {
		root := getEnv("EVOLVE_PROJECT_ROOT")
		if root == "" {
			root = gitRoot()
			if root == "" {
				root, _ = os.Getwd()
			}
		}
		configPath = filepath.Join(root, ".evolve", "llm_config.json")
	}

	if raw, err := os.ReadFile(configPath); err == nil {
		// File exists. Try to parse — invalid JSON logs warning + falls through.
		var cfg llmConfig
		if perr := json.Unmarshal(raw, &cfg); perr == nil {
			if entry, ok := cfg.Phases[role]; ok && entry.CLI != "" {
				if entry.Model != "" {
					return Result{CLI: entry.CLI, Model: entry.Model, Source: "llm_config"}, nil
				}
				if entry.ModelTier != "" {
					return Result{CLI: entry.CLI, ModelTier: entry.ModelTier, Source: "llm_config"}, nil
				}
				// cli only, no model/tier — bash emits empty model
				return Result{CLI: entry.CLI, Model: "", Source: "llm_config"}, nil
			}
			if cfg.Fallback != nil && cfg.Fallback.CLI != "" {
				tier := cfg.Fallback.ModelTier
				if tier == "" {
					tier = "balanced"
				}
				return Result{CLI: cfg.Fallback.CLI, ModelTier: tier, Source: "llm_config_fallback"}, nil
			}
		}
		// else: silently fall through (matches bash WARNING behaviour for callers)
	}

	// Step 3 — profile fallback
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

// JSON returns the byte-identical wire shape of resolve-llm.sh. Manual
// formatting (not encoding/json) preserves key order across all four shapes.
func (r Result) JSON() string {
	if r.Model != "" || (r.Source == "llm_config" && r.ModelTier == "") {
		return fmt.Sprintf(`{"cli":"%s","model":"%s","source":"%s"}`, r.CLI, r.Model, r.Source)
	}
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

// llmConfig models the subset of .evolve/llm_config.json we consume.
type llmConfig struct {
	Phases   map[string]phaseEntry `json:"phases"`
	Fallback *phaseEntry           `json:"_fallback,omitempty"`
}

type phaseEntry struct {
	CLI       string `json:"cli"`
	Model     string `json:"model"`
	ModelTier string `json:"model_tier"`
}

type profileDoc struct {
	CLI              string `json:"cli"`
	ModelTierDefault string `json:"model_tier_default"`
}
