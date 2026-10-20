package retryladder

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

type ReportRow struct {
	Rung   Rung                  `json:"rung"`
	Class  string                `json:"class"`
	Tries  int                   `json:"tries"`
	Fixes  int                   `json:"fixes"`
	Tokens cyclestate.TokenUsage `json:"tokens"`
}

type reportKey struct {
	rung  Rung
	class string
}

func BuildReport(runs [][]llmcalls.Record) []ReportRow {
	acc := map[reportKey]*ReportRow{}
	for _, run := range runs {
		for _, rec := range run {
			if rec.Rung == "" {
				continue
			}
			key := reportKey{Rung(rec.Rung), rec.FailureClass}
			row := acc[key]
			if row == nil {
				row = &ReportRow{Rung: key.rung, Class: key.class}
				acc[key] = row
			}
			row.Tries++
			if isFix(rec) {
				row.Fixes++
			}
			row.Tokens.Input += rec.Tokens.Input
			row.Tokens.Output += rec.Tokens.Output
			row.Tokens.CacheRead += rec.Tokens.CacheRead
			row.Tokens.CacheWrite += rec.Tokens.CacheWrite
		}
	}
	rows := make([]ReportRow, 0, len(acc))
	for _, row := range acc {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Rung != rows[j].Rung {
			return rows[i].Rung < rows[j].Rung
		}
		return rows[i].Class < rows[j].Class
	})
	return rows
}

func isFix(rec llmcalls.Record) bool {
	return Rung(rec.Rung) != R0 && rec.ExitCode != nil && *rec.ExitCode == 0
}

func LoadRuns(runsDir string) ([][]llmcalls.Record, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, err
	}
	var runs [][]llmcalls.Record
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		read, err := llmcalls.Read(filepath.Join(runsDir, entry.Name(), llmcalls.Filename))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		runs = append(runs, read.Records)
	}
	return runs, nil
}
