// Package resolvellm resolves a phase role to {cli, model_tier} from its
// profile.
package resolvellm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Result struct {
	CLI       string
	ModelTier string
	Source    string
}

var ErrProfileNotFound = errors.New("resolvellm: profile not found")

type Options struct {
	PluginRoot  string
	ProjectRoot string
	GitRoot     string
	Env         func(string) string
}

func Resolve(role string, opts Options) (Result, error) {
	if strings.TrimSpace(role) == "" {
		return Result{}, errors.New("resolvellm: role is required")
	}
	getEnv := opts.Env
	if getEnv == nil {
		getEnv = os.Getenv
	}

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

func (r Result) JSON() string {
	return fmt.Sprintf(`{"cli":"%s","model_tier":"%s","source":"%s"}`, r.CLI, r.ModelTier, r.Source)
}

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
	out, err := sysexec.Command(context.Background(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

type profileDoc struct {
	CLI              string `json:"cli"`
	ModelTierDefault string `json:"model_tier_default"`
}
