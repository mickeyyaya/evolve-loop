package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func (m *Mover) Withdraw(taskID, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", fmt.Errorf("%w: withdraw requires a reason", ErrBadArgs)
	}
	loc, err := m.locatePending("withdraw", taskID)
	if err != nil {
		return "", err
	}
	body, err := m.checkWithdrawable(taskID, loc.Path)
	if err != nil {
		return "", err
	}
	from := ".evolve/inbox/" + filepath.Base(loc.Path)
	digest := sha256.Sum256(body)
	if err := m.appendRecord(ledgerEntry{Action: "withdraw", TaskID: taskID, From: from, To: "withdrawn",
		Reason: fmt.Sprintf("%s; sha256=%s; body=%s", reason, hex.EncodeToString(digest[:]), body)}); err != nil {
		return "", fmt.Errorf("withdraw: record %s before removing it: %w", from, err)
	}
	if err := os.Remove(loc.Path); err != nil {
		m.ledgerLine(ledgerEntry{Action: "withdraw-aborted", TaskID: taskID, From: from, Reason: err.Error()})
		return "", fmt.Errorf("withdraw: remove %s: %w", loc.Path, err)
	}
	m.linef("withdrew %s: %s", taskID, filepath.Base(loc.Path))
	return loc.Path, nil
}

func (m *Mover) checkWithdrawable(taskID, path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	var fields map[string]json.RawMessage
	if err == nil {
		err = json.Unmarshal(body, &fields)
	}
	if err == nil {
		err = refuseLifecycleStamps(taskID, fields)
	}
	if err == nil {
		err = m.refuseBinding(taskID)
	}
	if err == nil {
		err = m.refuseDependents(taskID)
	}
	return body, err
}

func (m *Mover) refuseBinding(taskID string) error {
	bound, err := m.bound(taskID)
	switch {
	case err != nil:
		return fmt.Errorf("withdraw: read the continuation binding of %s: %w", taskID, err)
	case bound:
		return fmt.Errorf("%w: a continuation binds %s; release it first with `evolve continuation release %s`", ErrNotWithdrawable, taskID, taskID)
	}
	return nil
}

func refuseLifecycleStamps(taskID string, fields map[string]json.RawMessage) error {
	if owned := slices.DeleteFunc(slices.Sorted(maps.Keys(fields)), notLifecycleOwned); len(owned) > 0 {
		return fmt.Errorf("%w: %s carries %s, which the lifecycle verbs wrote", ErrNotWithdrawable, taskID, strings.Join(owned, ", "))
	}
	return nil
}

func (m *Mover) refuseDependents(taskID string) error {
	dependents, err := m.pendingDependents(taskID)
	if err == nil && len(dependents) > 0 {
		err = fmt.Errorf("%w: pending item(s) %s name %s in deps; edit their deps first", ErrNotWithdrawable, strings.Join(dependents, ", "), taskID)
	}
	return err
}

func notLifecycleOwned(key string) bool { return !isLifecycleOwned(key) }

func (m *Mover) pendingDependents(taskID string) ([]string, error) {
	var dependents []string
	for _, dir := range append([]string{m.inboxDir}, inboxbatch.ProcessingCycleDirs(m.inboxDir)...) {
		entries, err := jsonEntries(dir)
		if err != nil {
			return nil, fmt.Errorf("withdraw: scan %s: %w", dir, err)
		}
		for _, e := range entries {
			if id, deps := readIDAndDeps(filepath.Join(dir, e.Name())); slices.Contains(deps, taskID) {
				dependents = append(dependents, id)
			}
		}
	}
	return dependents, nil
}

func readIDAndDeps(path string) (string, []string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	var doc struct {
		ID   string   `json:"id"`
		Deps []string `json:"deps"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return "", nil
	}
	return doc.ID, doc.Deps
}
