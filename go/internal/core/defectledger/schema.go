package defectledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const (
	LedgerFile       = "defect-ledger.json"
	DispositionsFile = "defect-dispositions.json"
)

const (
	StatusOpen     = "OPEN"
	StatusFixed    = "FIXED"
	StatusDeferred = "DEFERRED"
)

const PrescriptionPrefix = "PRESCRIPTION: "

const (
	MaxEntries   = 64
	TextMaxRunes = 2000
)

const (
	PreflightMissingMarker    = "disposition-preflight: MISSING"
	PreflightIncompleteMarker = "disposition-preflight: INCOMPLETE"
)

const DispositionsSchemaExample = `{"dispositions": [
  {"id": "d0f3a7c1e59b246d8a0c4e6f13579bde2", "status": "FIXED",
   "evidence": "go/internal/phases/audit/defect_ledger.go:267-356"},
  {"id": "d9c8b7a6958473625140f3e2d1c0b9a87", "status": "DEFERRED",
   "reason": "out of this lane's scope; queued as disposition-evidence-tolerant-unmarshal"}
]}`

const evidenceSeparator = "; "

type Entry struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Status    string `json:"status"`
	Evidence  string `json:"evidence,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Source    string `json:"source,omitempty"`
	Round     int    `json:"round,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Dimension string `json:"dimension,omitempty"`
}

type Doc struct {
	OriginCycle int     `json:"origin_cycle"`
	Entries     []Entry `json:"entries"`
}

func (d Doc) OpenEntries() []Entry {
	var open []Entry
	for _, e := range d.Entries {
		if e.Status == StatusOpen {
			open = append(open, e)
		}
	}
	return open
}

type readFault struct {
	op  string
	err error
}

func (f *readFault) Error() string { return f.op + " " + LedgerFile + ": " + f.err.Error() }
func (f *readFault) Unwrap() error { return f.err }

func read(dir string) (Doc, bool, *readFault) {
	raw, err := os.ReadFile(filepath.Join(dir, LedgerFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Doc{}, false, nil
		}
		return Doc{}, false, &readFault{op: "read", err: err}
	}
	var doc Doc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Doc{}, false, &readFault{op: "parse", err: err}
	}
	return doc, true, nil
}

func Read(dir string) (Doc, bool, error) {
	doc, ok, fault := read(dir)
	if fault != nil {
		return doc, ok, fault
	}
	return doc, ok, nil
}

func Write(dir string, doc Doc) error {
	return atomicwrite.JSON(filepath.Join(dir, LedgerFile), doc)
}

func ID(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "d" + hex.EncodeToString(sum[:16])
}

func Truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…[truncated]"
}

type dispositionDoc struct {
	Dispositions []struct {
		ID       string              `json:"id"`
		Status   string              `json:"status"`
		Evidence dispositionEvidence `json:"evidence"`
		Reason   string              `json:"reason"`
	} `json:"dispositions"`
}

type dispositionEvidence struct {
	citations []string
}

func (e *dispositionEvidence) UnmarshalJSON(raw []byte) error {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		e.citations = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		e.citations = many
		return nil
	}
	return fmt.Errorf("`evidence` must be a citation string or an array of citation strings, got %s", Truncate(strings.TrimSpace(string(raw)), 120))
}

func (e dispositionEvidence) joined() string {
	return strings.Join(e.citations, evidenceSeparator)
}
