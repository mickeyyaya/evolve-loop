package codeindex

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
)

//go:embed testdata/independent/go.mod.txt
var independentGoMod []byte

//go:embed testdata/independent/work/work.go.txt
var independentWorkGo []byte

//go:embed testdata/independent/truth.json
var independentTruthJSON []byte

func IndependentTruth() ([]BenchSymbol, error) {
	return parseIndependentTruth(independentTruthJSON)
}

func parseIndependentTruth(raw []byte) ([]BenchSymbol, error) {
	var symbols []BenchSymbol
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, err
	}
	return symbols, nil
}

func WriteIndependentFixture(repoDir string) error {
	files := []struct {
		rel  string
		body []byte
	}{
		{rel: filepath.Join("go", "go.mod"), body: independentGoMod},
		{rel: filepath.Join("go", "work", "work.go"), body: independentWorkGo},
	}
	for _, f := range files {
		dst := filepath.Join(repoDir, f.rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, f.body, 0o644); err != nil {
			return err
		}
	}
	return nil
}
