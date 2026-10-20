package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSchemaVersion      = errors.New("schema version must be 1")
	ErrUnknownStatus      = errors.New("unknown status")
	ErrWrongPair          = errors.New("wrong pair")
	ErrStaleRequestDigest = errors.New("stale request digest")
	ErrUnknownAsk         = errors.New("unknown ask")
	ErrMissingEntry       = errors.New("missing entry")
	ErrDuplicateEntry     = errors.New("duplicate entry")
	ErrNoteRequired       = errors.New("note required")
)

func Parse(data []byte) (Ledger, error) {
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return Ledger{}, err
	}
	if l.SchemaVersion != SchemaVersion {
		return Ledger{}, fmt.Errorf("%w: got %d", ErrSchemaVersion, l.SchemaVersion)
	}
	for _, entry := range l.Entries {
		switch entry.Status {
		case StatusMet, StatusUnmet, StatusBlocked, StatusNotApplicable:
		default:
			return Ledger{}, fmt.Errorf("%w: %q", ErrUnknownStatus, entry.Status)
		}
	}
	return l, nil
}

func Validate(l Ledger, req Request) error {
	var errs []error
	if l.Pair != req.Pair {
		errs = append(errs, fmt.Errorf("%w: %s", ErrWrongPair, l.Pair))
	}
	if l.RequestDigest != req.Digest() {
		errs = append(errs, fmt.Errorf("%w: %s", ErrStaleRequestDigest, l.RequestDigest))
	}
	knownAsks := make(map[string]bool, len(req.Asks))
	for _, ask := range req.Asks {
		knownAsks[ask.ID] = true
	}
	seenUnknown := make(map[string]bool)
	entryCounts := make(map[string]int, len(l.Entries))
	for _, entry := range l.Entries {
		if !knownAsks[entry.Ask] {
			if !seenUnknown[entry.Ask] {
				seenUnknown[entry.Ask] = true
				errs = append(errs, fmt.Errorf("%w: %s", ErrUnknownAsk, entry.Ask))
			}
		}
		entryCounts[entry.Ask]++
		if entry.Status == StatusNotApplicable && strings.TrimSpace(entry.Note) == "" {
			errs = append(errs, fmt.Errorf("%w: %s", ErrNoteRequired, entry.Ask))
		}
	}
	for _, ask := range req.Asks {
		count := entryCounts[ask.ID]
		if count == 0 {
			errs = append(errs, fmt.Errorf("%w: %s", ErrMissingEntry, ask.ID))
		} else if count > 1 {
			errs = append(errs, fmt.Errorf("%w: %s", ErrDuplicateEntry, ask.ID))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func IsCheckOnly(raw []byte) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	if _, ok := fields["check"]; !ok {
		return false
	}
	for key := range fields {
		if key != "check" && key != "schema_version" && key != "pair" {
			return false
		}
	}
	return true
}
