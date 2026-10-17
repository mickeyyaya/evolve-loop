package usagelog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const (
	FileName            = "code-calls.ndjson"
	userDirName         = "evolve"
	MaxUserBytes  int64 = 5 << 20
	rotatedSuffix       = ".1"
	dirMode             = 0o700
	fileMode            = 0o600
	cliKey              = "EVOLVE_CLI"
)

type Record struct {
	TS         time.Time `json:"ts"`
	Command    string    `json:"command"`
	ArgLen     int       `json:"arg_len"`
	Exit       int       `json:"exit"`
	Hits       int       `json:"hits"`
	DurationMS int64     `json:"duration_ms"`
	Phase      string    `json:"phase"`
	Cycle      int       `json:"cycle"`
	CLI        string    `json:"cli"`
	Cwd        string    `json:"cwd"`
}

type Row struct {
	Phase    string `json:"phase"`
	Command  string `json:"command"`
	Calls    int    `json:"calls"`
	Hits     int    `json:"hits"`
	Failures int    `json:"failures"`
}

func Append(path string, rec Record, rotateAt int64) error {
	rec.TS = rec.TS.UTC()
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	if err := rotate(path, rotateAt); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	return errors.Join(werr, f.Close())
}

func rotate(path string, rotateAt int64) error {
	if rotateAt <= 0 {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() <= rotateAt {
		return nil
	}
	return os.Rename(path, path+rotatedSuffix)
}

func UserPath(cacheDir func() (string, error)) (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, userDirName, FileName), nil
}

func Target(env func(string) string, projectRoot string, cacheDir func() (string, error)) (path string, rotateAt int64, err error) {
	if id, ok := proctree.ParseDispatchID(env(ipcenv.DispatchIDKey)); ok {
		return filepath.Join(paths.RunWorkspace(projectRoot, id.Cycle), FileName), 0, nil
	}
	path, err = UserPath(cacheDir)
	return path, MaxUserBytes, err
}

func Identity(env func(string) string) (phase string, cycle int, cli string) {
	cli = env(cliKey)
	if id, ok := proctree.ParseDispatchID(env(ipcenv.DispatchIDKey)); ok {
		return id.Agent, id.Cycle, cli
	}
	return "", 0, cli
}

func Read(path string, since time.Time) (records []Record, skipped int, err error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()
	reader := bufio.NewReader(f)
	for {
		line, rerr := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			var rec Record
			if json.Unmarshal(trimmed, &rec) != nil {
				skipped++
			} else if !rec.TS.Before(since) {
				records = append(records, rec)
			}
		}
		if rerr == io.EOF {
			return records, skipped, nil
		}
		if rerr != nil {
			return nil, 0, rerr
		}
	}
}

func ReadAll(userPath, projectRoot string, since time.Time) (records []Record, skipped int, err error) {
	workspaces, err := workspaceLogs(projectRoot)
	if err != nil {
		return nil, 0, err
	}
	for _, file := range append([]string{userPath + rotatedSuffix, userPath}, workspaces...) {
		got, bad, rerr := Read(file, since)
		if rerr != nil {
			return nil, 0, rerr
		}
		records = append(records, got...)
		skipped += bad
	}
	return records, skipped, nil
}

func workspaceLogs(projectRoot string) ([]string, error) {
	runs := filepath.Join(paths.EvolveDirOf(projectRoot), "runs")
	entries, err := os.ReadDir(runs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var logs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "cycle-") {
			logs = append(logs, filepath.Join(runs, entry.Name(), FileName))
		}
	}
	return logs, nil
}

func Summarize(records []Record) []Row {
	type key struct{ phase, command string }
	byKey := map[key]*Row{}
	for _, rec := range records {
		k := key{rec.Phase, rec.Command}
		row, ok := byKey[k]
		if !ok {
			row = &Row{Phase: rec.Phase, Command: rec.Command}
			byKey[k] = row
		}
		row.Calls++
		row.Hits += rec.Hits
		if rec.Exit != 0 {
			row.Failures++
		}
	}
	rows := make([]Row, 0, len(byKey))
	for _, row := range byKey {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Phase != rows[j].Phase {
			return rows[i].Phase < rows[j].Phase
		}
		return rows[i].Command < rows[j].Command
	})
	return rows
}
