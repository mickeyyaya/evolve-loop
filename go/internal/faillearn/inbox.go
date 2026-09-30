package faillearn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InboxItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Weight     float64  `json:"weight"`
	Kind       string   `json:"kind"`
	Priority   string   `json:"priority"`
	Files      []string `json:"files"`
	InjectedBy string   `json:"injected_by"`
}

type Option func(*writeConfig)

type writeConfig struct {
	inboxDir         string
	inboxItems       []InboxItem
	noveltyThreshold float64
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
	for _, it := range c.inboxItems {
		path, err := c.itemPath(it)
		if err != nil {
			return err
		}
		body, err := json.MarshalIndent(it, "", "  ")
		if err != nil {
			return fmt.Errorf("faillearn: encode inbox item %s: %w", it.ID, err)
		}
		skipped, err := writeIfAbsent(path, body)
		if err != nil {
			return fmt.Errorf("faillearn: write inbox item %s: %w", it.ID, err)
		}
		if !skipped {
			continue
		}
		existing, rerr := os.ReadFile(path)
		if rerr != nil {
			return fmt.Errorf("faillearn: inbox item %s already exists but cannot be read to confirm it matches: %w", it.ID, rerr)
		}
		if !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(body)) {
			return fmt.Errorf("faillearn: inbox item %s already exists at %s with DIFFERENT content — refusing to drop the remediation item %q; resolve the id collision", it.ID, path, it.Title)
		}
	}
	return nil
}

func (c writeConfig) itemPath(it InboxItem) (string, error) {
	if strings.TrimSpace(it.ID) == "" {
		return "", fmt.Errorf("faillearn: inbox item %q has no id — an unaddressable remediation item cannot be reconciled later", it.Title)
	}
	if it.ID != filepath.Base(it.ID) || it.ID == "." || it.ID == ".." || strings.ContainsRune(it.ID, filepath.Separator) {
		return "", fmt.Errorf("faillearn: inbox item id %q is not a bare filename — an id that resolves to a path would write the item outside the inbox", it.ID)
	}
	return filepath.Join(c.inboxDir, it.ID+".json"), nil
}

func (c writeConfig) unqueuedItems() []InboxItem {
	unqueued := make([]InboxItem, 0, len(c.inboxItems))
	for _, it := range c.inboxItems {
		if !c.itemReachedInbox(it) {
			unqueued = append(unqueued, it)
		}
	}
	return unqueued
}

func (c writeConfig) itemReachedInbox(it InboxItem) bool {
	if c.inboxDir == "" {
		return false
	}
	path, err := c.itemPath(it)
	if err != nil {
		return false
	}
	body, err := json.MarshalIndent(it, "", "  ")
	if err != nil {
		return false
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(body))
}
