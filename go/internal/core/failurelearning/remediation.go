package failurelearning

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
)

// remediationSlugMaxRunes bounds the id's derived tail so a long defect line
// cannot produce an unwieldy filename. remediationMaxItems and
// remediationTitleMaxRunes bound the queue itself (cycle-1282 DEF-6).
const (
	remediationSlugMaxRunes  = 60
	remediationMaxItems      = 32
	remediationTitleMaxRunes = 500
)

// remediationItems turns a failed phase's self-reported defects into inbox
// remediation todos (batch-integrity-review-2026-08-04.md F1(ii)): the 1255
// defect was a retrospective that "filed" two items which never reached the
// queue. Ids are stable per (cycle, defect) so a re-run of the floor is
// idempotent (faillearn's writeIfAbsent keeps the first write). The weight is
// the caller's ONE policy read, never a literal here.
func (e *Engine) remediationItems(f Failure, defects []string, weight float64) []faillearn.InboxItem {
	items := make([]faillearn.InboxItem, 0, len(defects))
	for _, d := range defects {
		// cycle-1282 DEF-6: defects[] and each line are agent-authored and
		// previously unbounded — a queue nobody can triage hides the real item.
		// Both caps are RECORDED, never silent.
		if len(items) >= remediationMaxItems {
			e.warn(f, CodeRemediationTruncated,
				fmt.Sprintf("cycle-%d self-reported %d defects; filing the first %d as remediation items and dropping the rest — fix the emitter or raise remediationMaxItems", f.Cycle, len(defects), remediationMaxItems),
				map[string]string{"step": "floor", "reported": strconv.Itoa(len(defects)), "filed": strconv.Itoa(remediationMaxItems)})
			break
		}
		title := carryover.TruncateRunes(strings.TrimSpace(d), remediationTitleMaxRunes)
		slug := remediationSlug(title)
		if title == "" || slug == "" {
			continue // an unnameable defect yields no addressable item
		}
		items = append(items, faillearn.InboxItem{
			ID:       fmt.Sprintf("retro-%d-%s-%s", f.Cycle, slug, remediationFingerprint(title)),
			Title:    title,
			Weight:   weight,
			Kind:     "bug",
			Priority: "H",
			// Non-empty provenance is load-bearing: inboxbatch.ConsoleRouted
			// treats an empty injected_by as operator-authored.
			InjectedBy: "faillearn-failure-floor",
		})
	}
	return items
}

// remediationFingerprint is the id's injective tail: a short digest of the
// FULL defect title, appended unconditionally (cycle-1285 F1: the slug is
// lossy twice over — it stops at remediationSlugMaxRunes and collapses every
// run of non-alphanumerics — so two defects diverging only after rune 60 or
// only in punctuation minted ONE id, and the DEF-4 collision refusal then
// suppressed the retrospective and the lesson with the second item).
func remediationFingerprint(title string) string {
	sum := sha256.Sum256([]byte(title))
	return hex.EncodeToString(sum[:])[:8]
}

// remediationSlug lowercases a defect line and maps runs of non-alphanumerics
// to single hyphens, matching the inbox's existing id shape. It is NOT
// injective — see remediationFingerprint, which is what makes the id unique.
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
