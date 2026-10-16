package failurelearning

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	remediationSlugMaxRunes  = 60
	remediationMaxItems      = 32
	remediationTitleMaxRunes = 500
)

func (e *Engine) remediationItems(f Failure, defects []string, weight float64) []faillearn.InboxItem {
	items := make([]faillearn.InboxItem, 0, len(defects))
	for _, d := range defects {
		if len(items) >= remediationMaxItems {
			e.warn(f, CodeRemediationTruncated,
				fmt.Sprintf("cycle-%d self-reported %d defects; filing the first %d as remediation items and dropping the rest — fix the emitter or raise remediationMaxItems", f.Cycle, len(defects), remediationMaxItems),
				map[string]string{"step": "floor", "reported": strconv.Itoa(len(defects)), "filed": strconv.Itoa(remediationMaxItems)})
			break
		}
		title := carryover.TruncateRunes(strings.TrimSpace(d), remediationTitleMaxRunes)
		slug := remediationSlug(title)
		if title == "" || slug == "" {
			continue
		}
		items = append(items, faillearn.InboxItem{
			ID:            fmt.Sprintf("retro-%d-%s-%s", f.Cycle, slug, remediationFingerprint(title)),
			Title:         title,
			Weight:        weight,
			Kind:          "bug",
			Priority:      "H",
			PriorityClass: inboxbatch.ClassCorrectness,
			InjectedBy:    "faillearn-failure-floor",
		})
	}
	return items
}

func remediationFingerprint(title string) string {
	sum := sha256.Sum256([]byte(title))
	return hex.EncodeToString(sum[:])[:8]
}

func remediationSlug(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(s) {
		if b.Len() >= remediationSlugMaxRunes {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
