package scopedset

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const ArtifactName = "scoped-recheck-shadow.ndjson"

type Record struct {
	Stage       string   `json:"stage"`
	Cycle       int      `json:"cycle"`
	AuditedBase string   `json:"audited_base"`
	Lane        []string `json:"lane,omitempty"`
	Main        []string `json:"main,omitempty"`
	All         bool     `json:"all,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Packages    []string `json:"packages,omitempty"`
	Escapes     []Escape `json:"escapes,omitempty"`
}

func Append(plane string, r Record) (string, error) {
	if plane == "" {
		return "", errors.New("scopedset: no plane to record under")
	}
	if err := os.MkdirAll(plane, 0o755); err != nil {
		return "", err
	}
	file := filepath.Join(plane, ArtifactName)
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	return file, errors.Join(json.NewEncoder(f).Encode(r), f.Close())
}
