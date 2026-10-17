package looppreflight

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

const cliOnlyConvention = "docs/conventions/cli-only-no-mcp.md"

type MCPConfigSource struct {
	Binary string
	Files  []string
}

type MCPConfigPort struct {
	Sources  func() ([]MCPConfigSource, error)
	ReadFile func(path string) ([]byte, error)
	HomeDir  func() (string, error)
}

func defaultMCPConfigPort(p MCPConfigPort) MCPConfigPort {
	if p.Sources == nil {
		p.Sources = func() ([]MCPConfigSource, error) {
			return manifestMCPConfigSources(bridge.ManifestNames(), bridge.LoadManifest)
		}
	}
	if p.ReadFile == nil {
		p.ReadFile = os.ReadFile
	}
	if p.HomeDir == nil {
		p.HomeDir = os.UserHomeDir
	}
	return p
}

func manifestMCPConfigSources(names []string, load func(string) (bridge.Manifest, error)) ([]MCPConfigSource, error) {
	byBinary := map[string][]string{}
	var errs []error
	for _, name := range names {
		m, err := load(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, file := range m.MCPConfigFiles {
			if !slices.Contains(byBinary[m.Binary], file) {
				byBinary[m.Binary] = append(byBinary[m.Binary], file)
			}
		}
	}
	sources := make([]MCPConfigSource, 0, len(byBinary))
	for _, bin := range slices.Sorted(maps.Keys(byBinary)) {
		sources = append(sources, MCPConfigSource{Binary: bin, Files: byBinary[bin]})
	}
	return sources, errors.Join(errs...)
}

func checkMCPConfig(o resolved) CheckResult {
	const name = "mcp-config"
	warns := mcpConfigWarnings(o.mcpConfig, o.projectRoot)
	if len(warns) == 0 {
		return CheckResult{Name: name, Level: LevelPass, Message: "no MCP server configured for any CLI"}
	}
	return CheckResult{Name: name, Level: LevelWarn,
		Message: fmt.Sprintf("%d MCP config warning(s); the loop reaches outside services through a CLI", len(warns)),
		Detail:  strings.Join(warns, "\n"),
	}
}

func mcpConfigWarnings(port MCPConfigPort, projectRoot string) []string {
	var warns []string
	sources, err := port.Sources()
	if err != nil {
		warns = append(warns, fmt.Sprintf("WARN: cannot list the MCP config files of every CLI: %v", err))
	}
	for _, src := range sources {
		names, problems := configuredServers(port, projectRoot, src.Files)
		warns = append(warns, problems...)
		if len(names) > 0 {
			warns = append(warns, fmt.Sprintf("WARN: %s has MCP servers configured (%s); the repo rule is CLI only (%s)",
				src.Binary, strings.Join(names, ", "), cliOnlyConvention))
		}
	}
	return warns
}

func configuredServers(port MCPConfigPort, projectRoot string, files []string) (names, problems []string) {
	for _, file := range files {
		path, err := mcpConfigPath(port.HomeDir, projectRoot, file)
		if err != nil {
			problems = append(problems, fmt.Sprintf("WARN: cannot resolve MCP config %s: %v", file, err))
			continue
		}
		found, err := mcpServerNames(port.ReadFile, path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("WARN: cannot read MCP config %s: %v", path, err))
			continue
		}
		for _, n := range found {
			if !slices.Contains(names, n) {
				names = append(names, n)
			}
		}
	}
	slices.Sort(names)
	return names, problems
}

func mcpConfigPath(homeDir func() (string, error), projectRoot, file string) (string, error) {
	rel, fromHome := strings.CutPrefix(file, "~/")
	if !fromHome {
		return filepath.Join(projectRoot, file), nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, rel), nil
}

func mcpServerNames(readFile func(string) ([]byte, error), path string) ([]string, error) {
	data, err := readFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(doc.Servers)), nil
}
