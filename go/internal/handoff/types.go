package handoff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const SchemaVersion = 1

type Status string

const (
	StatusMet           Status = "met"
	StatusUnmet         Status = "unmet"
	StatusBlocked       Status = "blocked"
	StatusNotApplicable Status = "not_applicable"
)

type Exit string

const (
	ExitPass    Exit = "pass"
	ExitCap     Exit = "cap"
	ExitBlocked Exit = "blocked"
)

type Ask struct {
	ID           string `json:"id"`
	Text         string `json:"text"`
	Source       string `json:"source"`
	EvidenceKind string `json:"evidence_kind"`
	Required     bool   `json:"required"`
}

type Request struct {
	Pair string `json:"pair"`
	Asks []Ask  `json:"asks"`
}

func (r Request) Digest() string {
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type Evidence struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type Entry struct {
	Ask      string     `json:"ask"`
	Status   Status     `json:"status"`
	Evidence []Evidence `json:"evidence"`
	Note     string     `json:"note"`
}

type Author struct {
	CLI   string `json:"cli"`
	Model string `json:"model"`
	Tier  string `json:"tier"`
}

type Ledger struct {
	SchemaVersion int     `json:"schema_version"`
	Pair          string  `json:"pair"`
	RequestDigest string  `json:"request_digest"`
	OutputDigest  string  `json:"output_digest"`
	Author        Author  `json:"author"`
	Round         int     `json:"round"`
	Entries       []Entry `json:"entries"`
	Check         *Check  `json:"check,omitempty"`
}

type Check struct {
	Stage          string   `json:"stage"`
	Outcome        string   `json:"outcome"`
	Unmet          []string `json:"unmet"`
	Blocked        []string `json:"blocked"`
	Paths          []string `json:"paths"`
	FailedEvidence []string `json:"failed_evidence"`
	Errors         []string `json:"errors"`
}
