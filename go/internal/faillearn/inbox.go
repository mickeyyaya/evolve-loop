package faillearn

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
)

type InboxItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Weight        float64  `json:"weight"`
	Kind          string   `json:"kind"`
	Priority      string   `json:"priority"`
	PriorityClass string   `json:"priority_class"`
	Files         []string `json:"files,omitempty"`
	InjectedBy    string   `json:"injected_by"`
	Summary       string   `json:"summary"`
	Fix           string   `json:"fix"`
	Acceptance    []string `json:"acceptance"`
}

type Option func(*writeConfig)

type writeConfig struct {
	inboxDir         string
	inboxItems       []InboxItem
	noveltyThreshold float64
	now              func() time.Time
}

func WithInbox(dir string, items []InboxItem) Option {
	return func(c *writeConfig) {
		c.inboxDir = dir
		c.inboxItems = items
	}
}

func (c writeConfig) writeInboxItems() error {
	if c.inboxDir == "" || len(c.inboxItems) == 0 {
		return nil
	}
	items := slices.Clone(c.inboxItems)
	for i := range items {
		it := &items[i]
		if strings.TrimSpace(it.Title) == "" {
			it.Title = it.ID
		}
		title := strings.TrimSpace(it.Title)
		if strings.TrimSpace(it.Summary) == "" {
			it.Summary = fmt.Sprintf("Floor remediation item for %s: %s", it.ID, title)
		}
		if strings.TrimSpace(it.Fix) == "" {
			it.Fix = fmt.Sprintf("Find where the phase emits this defect (%s), fix it at the source, and re-run the failed phase.", title)
		}
		if len(it.Acceptance) == 0 {
			it.Acceptance = []string{
				fmt.Sprintf("The defect %q no longer appears in the phase's failure block on a re-run", title),
				"A regression test fails before the fix and passes after it",
			}
		}
	}
	for _, it := range items {
		if err := inboxbatch.RequirePriorityClass(it.PriorityClass); err != nil {
			return fmt.Errorf("faillearn: inbox item %s: %w", it.ID, err)
		}
	}
	for _, it := range items {
		if err := checkItemID(it); err != nil {
			return err
		}
		body, err := json.MarshalIndent(it, "", "  ")
		if err != nil {
			return fmt.Errorf("faillearn: encode inbox item %s: %w", it.ID, err)
		}
		if _, err := inboxfile.Mint(c.inboxDir, body, c.now); err != nil {
			return fmt.Errorf("faillearn: write inbox item %s: %w", it.ID, err)
		}
	}
	return nil
}

func checkItemID(it InboxItem) error {
	if strings.TrimSpace(it.ID) == "" {
		return fmt.Errorf("faillearn: inbox item %q has no id — an unaddressable remediation item cannot be reconciled later", it.Title)
	}
	if it.ID != filepath.Base(it.ID) || it.ID == "." || it.ID == ".." || strings.ContainsRune(it.ID, filepath.Separator) {
		return fmt.Errorf("faillearn: inbox item id %q is not a bare filename — an id that resolves to a path would write the item outside the inbox", it.ID)
	}
	return nil
}

func (c writeConfig) unqueuedItems() []InboxItem {
	filed, err := inboxfile.PendingIDs(c.inboxDir)
	if err != nil {
		filed = nil
	}
	unqueued := make([]InboxItem, 0, len(c.inboxItems))
	for _, it := range c.inboxItems {
		if !filed[it.ID] {
			unqueued = append(unqueued, it)
		}
	}
	return unqueued
}
