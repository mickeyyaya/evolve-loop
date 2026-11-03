package bridge

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed cellview/tool-view.json
var cellToolViewRaw []byte

const cellDirToken = "{cell}"

type cellToolView struct {
	Dir   string            `json:"dir"`
	Env   map[string]string `json:"env"`
	Dirs  []string          `json:"dirs"`
	Files map[string]string `json:"files"`
}

var defaultCellToolView = mustCellToolView(cellToolViewRaw)

func parseCellToolView(raw []byte) (cellToolView, error) {
	var v cellToolView
	if err := json.Unmarshal(raw, &v); err != nil {
		return cellToolView{}, fmt.Errorf("bridge:cell tool view: invalid JSON: %w", err)
	}
	if !filepath.IsLocal(v.Dir) {
		return cellToolView{}, fmt.Errorf("bridge:cell tool view: dir %q is not a local path of the workspace", v.Dir)
	}
	var errs []error
	for key, value := range v.Env {
		if !isShellIdentifier(key) || isReservedEnvKey(key) || hasControlByte(value) {
			errs = append(errs, fmt.Errorf("bridge:cell tool view: env %q is not a shell identifier, is reserved, or its value carries a control byte", key))
		}
	}
	for _, name := range slices.Concat(v.Dirs, slices.Sorted(maps.Keys(v.Files))) {
		if !filepath.IsLocal(name) {
			errs = append(errs, fmt.Errorf("bridge:cell tool view: %q is not a local path of the cell dir", name))
		}
	}
	return v, errors.Join(errs...)
}

func mustCellToolView(raw []byte) cellToolView {
	v, err := parseCellToolView(raw)
	if err != nil {
		panic(err)
	}
	return v
}

func (v cellToolView) envFor(workspace string) map[string]string {
	cell := filepath.Join(workspace, v.Dir)
	env := make(map[string]string, len(v.Env))
	for key, value := range v.Env {
		env[key] = strings.ReplaceAll(value, cellDirToken, cell)
	}
	return env
}

func (v cellToolView) write(workspace string) error {
	cell := filepath.Join(workspace, v.Dir)
	for _, dir := range v.Dirs {
		if err := os.MkdirAll(filepath.Join(cell, dir), 0o700); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(v.Files)) {
		if err := os.WriteFile(filepath.Join(cell, name), []byte(v.Files[name]), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (deps Deps) cellToolEnv(cfg *Config) map[string]string {
	if cfg.Workspace == "" {
		return cfg.Realization.Env
	}
	workspace := cfg.Workspace
	if abs, err := filepath.Abs(workspace); err == nil {
		workspace = abs
	}
	if err := defaultCellToolView.write(workspace); err != nil && deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN cell tool view: %v (the cell keeps the tool view env, so each tool finds no host config)\n", err)
	}
	env := map[string]string{}
	maps.Copy(env, cfg.Realization.Env)
	maps.Copy(env, defaultCellToolView.envFor(workspace))
	return env
}
