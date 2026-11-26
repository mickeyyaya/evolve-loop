package cycleblocker

import (
	"encoding/json"
	"errors"
	"fmt"
)

const SchemaV1 = "evolve.blocker/1"

var ErrSchema = errors.New("cycleblocker: unknown record schema")

type Record struct {
	Schema  string  `json:"schema"`
	Cycle   int     `json:"cycle"`
	Blocker string  `json:"blocker"`
	Minutes float64 `json:"minutes"`
}

func DecodeRecord(raw []byte) (Record, error) {
	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		return Record{}, fmt.Errorf("cycleblocker: decode record: %w", err)
	}
	if r.Schema != SchemaV1 {
		return Record{}, fmt.Errorf("%w: %q is not %q", ErrSchema, r.Schema, SchemaV1)
	}
	return r, nil
}
