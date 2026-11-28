package cycleidentity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type Part string

const (
	PartGraph    Part = "graph"
	PartContract Part = "contract"
	PartPolicy   Part = "policy"
)

var (
	ErrNoIdentity = errors.New("cycle identity: no identity recorded")
	ErrMismatch   = errors.New("cycle identity: the pipeline changed since the cycle started")
)

type Identity struct {
	Graph    string `json:"graph_hash"`
	Contract string `json:"contract_hash"`
	Policy   string `json:"policy_hash"`
}

type Sources struct {
	Root      string
	Contracts []phasecontract.Contract
}

func Live(root string) Sources {
	return Sources{Root: root, Contracts: phasecontract.Contracts()}
}

func Compute(s Sources) (Identity, error) {
	graph, err := digestFile(config.RegistryPath(s.Root))
	if err != nil {
		return Identity{}, fmt.Errorf("cycle identity: phase registry: %w", err)
	}
	policy, err := digestFile(filepath.Join(s.Root, ".evolve", "policy.json"), policyKeysOutsideIdentity...)
	if err != nil {
		return Identity{}, fmt.Errorf("cycle identity: policy: %w", err)
	}
	return Identity{Graph: graph, Contract: digestContracts(s.Contracts), Policy: policy}, nil
}

var policyKeysOutsideIdentity = []string{"resume"}

func digestFile(path string, omit ...string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return digest(normalize(raw, omit)), nil
}

func normalize(raw []byte, omit []string) []byte {
	var doc any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &doc); err != nil {
		return raw
	}
	if object, ok := doc.(map[string]any); ok {
		for _, key := range omit {
			delete(object, key)
		}
	}
	canonical, _ := json.Marshal(doc)
	return canonical
}

func digestContracts(contracts []phasecontract.Contract) string {
	sorted := append([]phasecontract.Contract(nil), contracts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Phase < sorted[j].Phase })
	canonical, _ := json.Marshal(sorted)
	return digest(canonical)
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func Diff(recorded, current Identity) []Part {
	var parts []Part
	if recorded.Graph != current.Graph {
		parts = append(parts, PartGraph)
	}
	if recorded.Contract != current.Contract {
		parts = append(parts, PartContract)
	}
	if recorded.Policy != current.Policy {
		parts = append(parts, PartPolicy)
	}
	return parts
}

type MismatchError struct {
	Parts []Part
}

func (e *MismatchError) Error() string {
	return fmt.Sprintf("%v: changed: %s", ErrMismatch, JoinParts(e.Parts))
}

func (e *MismatchError) Unwrap() error { return ErrMismatch }

func JoinParts(parts []Part) string {
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = string(p)
	}
	return strings.Join(names, ", ")
}

func Check(recorded, current Identity) error {
	if parts := Diff(recorded, current); len(parts) > 0 {
		return &MismatchError{Parts: parts}
	}
	return nil
}

func Encode(id Identity) string {
	raw, _ := json.Marshal(id)
	return string(raw)
}

func Decode(message string) (Identity, error) {
	if message == "" {
		return Identity{}, ErrNoIdentity
	}
	var id Identity
	if err := json.Unmarshal([]byte(message), &id); err != nil {
		return Identity{}, fmt.Errorf("cycle identity: decode: %w", err)
	}
	return id, nil
}
