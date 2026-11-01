package agentrequest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
)

const (
	RecordSchema = "evolve.request.record/1"

	DecisionRun      = "run"
	DecisionApproved = "approved"
	DecisionRefused  = "refused"
	DecisionFailed   = "failed"
)

type Record struct {
	Schema       string    `json:"schema"`
	Request      string    `json:"request"`
	SHA256       string    `json:"sha256"`
	Summary      string    `json:"summary"`
	Cycle        int       `json:"cycle"`
	Phase        string    `json:"phase"`
	Depth        int       `json:"depth"`
	Kind         string    `json:"kind,omitempty"`
	Role         string    `json:"role,omitempty"`
	Decision     string    `json:"decision"`
	Precondition string    `json:"precondition,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	Stage        string    `json:"stage"`
	RunLabel     string    `json:"run_label,omitempty"`
	Report       string    `json:"report,omitempty"`
	At           time.Time `json:"at"`
}

func recordPath(cell Cell, name string) string {
	return filepath.Join(inputindex.RequestsDir(cell.ProjectRoot, cell.Cycle), name+".json")
}

func readRecord(cell Cell, name string) (Record, bool, error) {
	raw, err := os.ReadFile(recordPath(cell, name))
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	var r Record
	if err == nil {
		err = json.Unmarshal(raw, &r)
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("request %s: %w", name, err)
	}
	return r, true, nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func recordCount(cell Cell) int {
	records, _ := filepath.Glob(filepath.Join(inputindex.RequestsDir(cell.ProjectRoot, cell.Cycle), "*.json"))
	return len(records)
}

func writeRecord(cell Cell, r Record) error {
	r.Summary = fmt.Sprintf("%s to %s: %s", r.Request, r.Role, r.Decision)
	if r.Reason != "" {
		r.Summary += " (" + r.Reason + ")"
	}
	path := recordPath(cell, r.Request)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = atomicwrite.JSON(path, r)
	}
	if err != nil {
		return fmt.Errorf("request %s: %w", r.Request, err)
	}
	return nil
}
