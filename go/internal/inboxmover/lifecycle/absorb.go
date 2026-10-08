package lifecycle

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AbsorbOutcome string

const (
	AbsorbDropped AbsorbOutcome = "dropped"
	AbsorbParked  AbsorbOutcome = "parked"
)

type Absorbed struct {
	ID         string
	Cycle      int
	RootPath   string
	Outcome    AbsorbOutcome
	ParkedPath string
}

func (m *Mover) AbsorbRootCopies() ([]Absorbed, error) {
	list, err := ListClaims(m.inboxDir)
	if err != nil {
		return nil, err
	}
	absorbed := []Absorbed{}
	var errs []error
	for _, it := range list.Items {
		if !it.Duplicate {
			continue
		}
		a, err := m.absorbOne(it)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		absorbed = append(absorbed, a)
	}
	return absorbed, errors.Join(errs...)
}

func (m *Mover) absorbOne(it ClaimedItem) (Absorbed, error) {
	a := Absorbed{ID: it.ID, Cycle: it.Cycle, RootPath: it.RootPath, Outcome: AbsorbDropped}
	if provenEqual(it.Path, it.RootPath) {
		if err := os.Remove(it.RootPath); err != nil {
			return a, fmt.Errorf("absorb %s: remove the root copy: %w", it.ID, err)
		}
		m.linef("absorbed: %s — the root copy equals the claim of cycle-%d and is removed", filepath.Base(it.RootPath), it.Cycle)
		m.ledgerLine(m.absorbEntry("absorb", it, "the root copy equals the claim; the root copy is removed"))
		return a, nil
	}
	a.Outcome = AbsorbParked
	a.ParkedPath = filepath.Join(m.inboxDir, "origin-conflicts", "cycle-"+strconv.Itoa(it.Cycle), filepath.Base(it.RootPath))
	if err := m.park(it.RootPath, a.ParkedPath); err != nil {
		return a, fmt.Errorf("absorb %s: park origin's copy: %w", it.ID, err)
	}
	m.linef("WARN: absorb conflict: %s differs from the claim of cycle-%d; the claim stays, and origin's copy is parked at %s", filepath.Base(it.RootPath), it.Cycle, a.ParkedPath)
	m.ledgerLine(m.absorbEntry("absorb-conflict", it, "origin's copy differs from the claim; the claim stays, origin's copy is parked at "+relToInbox(a.ParkedPath, m.inboxDir)))
	return a, nil
}

func (m *Mover) park(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return moveExclusive(src, dest)
}

func (m *Mover) absorbEntry(action string, it ClaimedItem, reason string) ledgerEntry {
	return ledgerEntry{
		Action: action,
		TaskID: it.ID,
		From:   ".evolve/inbox/" + filepath.Base(it.RootPath),
		To:     relToInbox(it.Path, m.inboxDir),
		Cycle:  intPtr(strconv.Itoa(it.Cycle)),
		Reason: reason,
	}
}

func relToInbox(path, inboxDir string) string {
	return ".evolve/inbox/" + strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(path, inboxDir)), "/")
}

func provenEqual(a, b string) bool {
	bodyA, errA := os.ReadFile(a)
	bodyB, errB := os.ReadFile(b)
	return errA == nil && errB == nil && bytes.Equal(bodyA, bodyB)
}

func moveExclusive(src, dest string) error {
	if err := os.Link(src, dest); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return errors.Join(err, os.Remove(dest))
	}
	return nil
}
