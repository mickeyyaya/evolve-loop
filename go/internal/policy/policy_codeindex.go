package policy

import (
	"fmt"
	"slices"
	"strings"
)

const (
	defaultCodeIndexBackend = "trail"
	defaultCodeIndexDir     = "dev/code-index"
	defaultCodeIndexTrail   = "dev/.tools/trail/node_modules/.bin/trail"

	CodeIndexStaleWarn   = "warn"
	CodeIndexStaleRefuse = "refuse"
)

type CodeIndexPolicy struct {
	Backends []string `json:"backends,omitempty"`
	IndexDir *string  `json:"index_dir,omitempty"`
	TrailBin *string  `json:"trail_bin,omitempty"`
	Stale    *string  `json:"stale,omitempty"`
}

type CodeIndexSettings struct {
	Backends []string
	IndexDir string
	TrailBin string
	Stale    string
}

func (p Policy) CodeIndexConfig() (CodeIndexSettings, error) {
	out := CodeIndexSettings{Backends: []string{defaultCodeIndexBackend}, IndexDir: defaultCodeIndexDir, TrailBin: defaultCodeIndexTrail, Stale: CodeIndexStaleWarn}
	c := p.CodeIndex
	if c == nil {
		return out, nil
	}
	if c.Backends != nil {
		if err := validateCodeIndexBackends(c.Backends); err != nil {
			return CodeIndexSettings{}, err
		}
		out.Backends = slices.Clone(c.Backends)
	}
	var err error
	if out.IndexDir, err = codeIndexPath("index_dir", c.IndexDir, out.IndexDir); err != nil {
		return CodeIndexSettings{}, err
	}
	if out.TrailBin, err = codeIndexPath("trail_bin", c.TrailBin, out.TrailBin); err != nil {
		return CodeIndexSettings{}, err
	}
	if out.Stale, err = codeIndexStale(c.Stale); err != nil {
		return CodeIndexSettings{}, err
	}
	return out, nil
}

func codeIndexStale(set *string) (string, error) {
	if set == nil {
		return CodeIndexStaleWarn, nil
	}
	if *set != CodeIndexStaleWarn && *set != CodeIndexStaleRefuse {
		return "", fmt.Errorf("policy: code_index.stale %q: want %s|%s", *set, CodeIndexStaleWarn, CodeIndexStaleRefuse)
	}
	return *set, nil
}

func validateCodeIndexBackends(names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("policy: code_index.backends must name at least one backend")
	}
	seen := map[string]bool{}
	for _, name := range names {
		switch {
		case strings.TrimSpace(name) == "":
			return fmt.Errorf("policy: code_index.backends: name must not be blank")
		case seen[name]:
			return fmt.Errorf("policy: code_index.backends: duplicate backend %q", name)
		}
		seen[name] = true
	}
	return nil
}

func codeIndexPath(key string, set *string, fallback string) (string, error) {
	if set == nil {
		return fallback, nil
	}
	if strings.TrimSpace(*set) == "" {
		return "", fmt.Errorf("policy: code_index.%s must not be blank", key)
	}
	return *set, nil
}

func (p *CodeIndexPolicy) UnmarshalJSON(raw []byte) error {
	type block CodeIndexPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("code_index: %w", err)
	}
	*p = CodeIndexPolicy(decoded)
	return nil
}
