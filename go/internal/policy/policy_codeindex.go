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
)

type CodeIndexPolicy struct {
	Backends []string `json:"backends,omitempty"`
	IndexDir *string  `json:"index_dir,omitempty"`
	TrailBin *string  `json:"trail_bin,omitempty"`
}

type CodeIndexSettings struct {
	Backends []string
	IndexDir string
	TrailBin string
}

func (p Policy) CodeIndexConfig() (CodeIndexSettings, error) {
	out := CodeIndexSettings{Backends: []string{defaultCodeIndexBackend}, IndexDir: defaultCodeIndexDir, TrailBin: defaultCodeIndexTrail}
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
	return out, nil
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
