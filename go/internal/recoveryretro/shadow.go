package recoveryretro

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

const (
	DirName       = "retrospectives"
	HistoryWindow = 400
)

type document struct {
	base   string
	doc    string
	lesson []byte
}

func WriteShadow(projectRoot, workspace string, cycle int, todos []cyclestate.CarryoverTodo) ([]string, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, errors.New("recoveryretro: the workspace path is empty")
	}
	docs, err := shadowDocuments(projectRoot, workspace, cycle, todos)
	if len(docs) == 0 {
		return nil, err
	}
	dir := filepath.Join(workspace, DirName)
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return nil, errors.Join(err, fmt.Errorf("recoveryretro: make %s: %w", dir, mkErr))
	}
	var written []string
	for _, d := range docs {
		for _, f := range []struct {
			ext  string
			data []byte
		}{{".md", []byte(d.doc)}, {".yaml", d.lesson}} {
			path := filepath.Join(dir, d.base+f.ext)
			if exists(path) {
				continue
			}
			if werr := os.WriteFile(path, f.data, 0o644); werr != nil {
				err = errors.Join(err, fmt.Errorf("recoveryretro: write %s: %w", path, werr))
				continue
			}
			written = append(written, path)
		}
	}
	return written, err
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func shadowDocuments(projectRoot, workspace string, cycle int, todos []cyclestate.CarryoverTodo) ([]document, error) {
	recoveries, err := recoveriesAt(projectRoot, workspace, cycle)
	var docs []document
	add := func(d document) {
		if slices.ContainsFunc(docs, func(o document) bool { return o.base == d.base }) {
			err = errors.Join(err, fmt.Errorf("recoveryretro: two documents have the name %s; the second is not written", d.base))
			return
		}
		docs = append(docs, d)
	}
	for _, r := range recoveries {
		base := RecoveryName(r)
		add(document{base: base, doc: RenderRecovery(r), lesson: recoveryLessonYAML(r, docPath(base))})
	}
	for _, l := range JudgmentLessons(todos) {
		add(document{base: l.ID, doc: RenderJudgmentLesson(l), lesson: judgmentLessonYAML(l, docPath(l.ID))})
	}
	return docs, err
}

func docPath(base string) string { return DocsHome + "/" + base + ".md" }

func recoveriesAt(projectRoot, workspace string, cycle int) ([]Recovery, error) {
	pass, err := cycleDossier(projectRoot, cycle)
	if err != nil || !shippedWithTasks(pass) {
		return nil, err
	}
	minCycle := cycle - HistoryWindow
	history := append(dossier.ReadDir(dossier.CyclesDir(projectRoot), minCycle), dossier.ReadDir(dossier.PendingDir(projectRoot), minCycle)...)
	recoveries := Recoveries(history, pass)
	if len(recoveries) == 0 {
		return nil, nil
	}
	rungs, err := laddersAbove(workspace)
	if err != nil {
		return nil, err
	}
	for i := range recoveries {
		recoveries[i].Rungs = rungs
	}
	return recoveries, nil
}

func cycleDossier(projectRoot string, cycle int) (*dossier.Dossier, error) {
	name := "cycle-" + strconv.Itoa(cycle) + ".json"
	for _, dir := range []string{dossier.CyclesDir(projectRoot), dossier.PendingDir(projectRoot)} {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("recoveryretro: read the dossier %s: %w", path, err)
		}
		d, err := dossier.ParseJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("recoveryretro: the dossier %s: %w", path, err)
		}
		return d, nil
	}
	return nil, nil
}

func laddersAbove(workspace string) ([]string, error) {
	read, err := llmcalls.ReadWorkspace(workspace)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recoveryretro: read the call log: %w", err)
	}
	var rungs []string
	for _, rec := range read.Records {
		if entry := rec.Phase + " " + rec.Rung; rec.Rung != "" && rec.Rung != "R0" && !slices.Contains(rungs, entry) {
			rungs = append(rungs, entry)
		}
	}
	return rungs, nil
}
