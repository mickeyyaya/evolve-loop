package reviewcheck

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
)

type InboxChange struct {
	BasePaths []string
	HeadPaths []string
	Renamed   map[string]string
	Bodies    []string
	ReadBase  func(path string) ([]byte, error)
}

func InboxIntegrity(c InboxChange) ([]Refusal, error) {
	copies := map[string]bool{}
	for _, p := range c.HeadPaths {
		copies[path.Base(p)] = true
	}
	var closed []string
	for _, body := range c.Bodies {
		closed = append(closed, inboxmover.ClosesInboxIDs([]byte(body))...)
	}
	var refusals []Refusal
	for _, p := range c.BasePaths {
		if !isPending(p) || copies[path.Base(p)] || c.Renamed[p] != "" {
			continue
		}
		raw, err := c.ReadBase(p)
		if err != nil {
			return nil, fmt.Errorf("inbox integrity: read %s at the base: %w", p, err)
		}
		stem := strings.TrimSuffix(path.Base(p), ".json")
		id := itemID(raw, p)
		if slices.Contains(closed, id) || slices.Contains(closed, stem) {
			continue
		}
		refusals = append(refusals, Refusal{
			Check:   CheckInboxIntegrity,
			Subject: id,
			Reason:  fmt.Sprintf("the commits remove the pending item %s, and no consumed, claimed or moved copy exists at the head and no Closes-Inbox trailer names it", p),
		})
	}
	return refusals, nil
}

func isPending(p string) bool {
	rest, ok := strings.CutPrefix(p, inboxstamps.Pathspec)
	return ok && !strings.Contains(rest, "/") && strings.HasSuffix(rest, ".json")
}

func itemID(raw []byte, itemPath string) string {
	var item struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &item)
	return inboxbatch.ItemID(item.ID, itemPath)
}
