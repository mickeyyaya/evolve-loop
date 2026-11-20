// Package testaudit finds weak tests with go/ast rules and ratchets the findings against a baseline.
package testaudit

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Guard struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Reason  string `json:"reason"`
}

type Fake struct {
	Tool     string `json:"tool"`
	SuiteDir string `json:"suite_dir"`
	RealSide string `json:"real_side"`
	Reason   string `json:"reason"`
}

type Config struct {
	Guards       []Guard
	Fakes        []Fake
	MaxGuardRows int
}

//go:embed fakes.json
var fakesJSON []byte

//go:embed guards.json
var guardsJSON []byte

const (
	realSideNone = "none"
	fakesFile    = "fakes.json"
	guardsFile   = "guards.json"
)

func DefaultConfig() (Config, error) {
	return parseConfig(fakesJSON, guardsJSON)
}

func LoadConfig(moduleRoot string) (Config, error) {
	fakes, err := os.ReadFile(filepath.Join(moduleRoot, filepath.FromSlash(auditDir), fakesFile))
	if err != nil {
		return Config{}, err
	}
	guards, err := os.ReadFile(filepath.Join(moduleRoot, filepath.FromSlash(auditDir), guardsFile))
	if err != nil {
		return Config{}, err
	}
	return parseConfig(fakes, guards)
}

func parseConfig(fakesRaw, guardsRaw []byte) (Config, error) {
	var fakes struct {
		Rows []Fake `json:"rows"`
	}
	if err := json.Unmarshal(fakesRaw, &fakes); err != nil {
		return Config{}, fmt.Errorf("testaudit: %s: %w", fakesFile, err)
	}
	var guards struct {
		MaxRows int     `json:"max_rows"`
		Rows    []Guard `json:"rows"`
	}
	if err := json.Unmarshal(guardsRaw, &guards); err != nil {
		return Config{}, fmt.Errorf("testaudit: %s: %w", guardsFile, err)
	}
	cfg := Config{Guards: guards.Rows, Fakes: fakes.Rows, MaxGuardRows: guards.MaxRows}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.MaxGuardRows <= 0 {
		return fmt.Errorf("testaudit: guards.json max_rows must be positive, got %d", c.MaxGuardRows)
	}
	if len(c.Guards) > c.MaxGuardRows {
		return fmt.Errorf("testaudit: guards.json holds %d rows, above max_rows %d", len(c.Guards), c.MaxGuardRows)
	}
	for _, g := range c.Guards {
		if g.Package == "" || strings.TrimSpace(g.Reason) == "" {
			return fmt.Errorf("testaudit: guard row %+v needs a package and a reason", g)
		}
	}
	for _, f := range c.Fakes {
		if f.Tool == "" {
			return fmt.Errorf("testaudit: fakes.json row %+v has no tool", f)
		}
		if f.RealSide == realSideNone && strings.TrimSpace(f.Reason) == "" {
			return fmt.Errorf("testaudit: fakes.json row %q has real side none and no reason", f.Tool)
		}
		if f.RealSide != realSideNone && f.SuiteDir == "" {
			return fmt.Errorf("testaudit: fakes.json row %q has a real side and no suite_dir", f.Tool)
		}
	}
	return nil
}

func (c Config) guarded(f Finding) bool {
	for _, g := range c.Guards {
		if cleanPath(g.Package) == f.Package && (g.Test == "" || g.Test == f.Test) {
			return true
		}
	}
	return false
}

func (c Config) fakeListed(tool string) bool {
	for _, f := range c.Fakes {
		if f.Tool == tool {
			return true
		}
	}
	return false
}
