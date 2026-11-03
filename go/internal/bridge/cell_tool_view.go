package bridge

import (
	"cmp"
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
	errs := []error{validateEnvEntries("bridge:cell tool view: env", "the embedded data", v.Env)}
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

func cellWorkspace(cfg *Config) string {
	abs, _ := filepath.Abs(cfg.Workspace)
	return cmp.Or(abs, cfg.Workspace)
}

func cellToolEnv(cfg *Config) map[string]string {
	if cfg.Workspace == "" {
		return cfg.Realization.Env
	}
	env := map[string]string{}
	maps.Copy(env, cfg.Realization.Env)
	maps.Copy(env, defaultCellToolView.envFor(cellWorkspace(cfg)))
	return env
}

func (deps Deps) prepareCellToolView(cfg *Config) {
	if cfg.Workspace == "" || deps.envStage() != envStageEnforce {
		return
	}
	if err := defaultCellToolView.write(cellWorkspace(cfg)); err != nil {
		w := deps.Stderr
		if w == nil {
			w = os.Stderr
		}
		fmt.Fprintf(w, "[bridge] WARN cell tool view: %v (the cell keeps the tool view env, so each tool finds no host config)\n", err)
	}
}
