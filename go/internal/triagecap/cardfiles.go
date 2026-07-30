package triagecap

// cardfiles.go — the loud half of triage-cards-carry-files. project.go carries a
// DECLARED `files=` footprint from the report item into the companion's top_n
// card; this file makes an OMITTED footprint visible.
//
// Silence was the whole defect: cycle-1130 committed a card whose action prose
// named go/internal/phases/audit/audit.go and whose files[] was absent, so
// fleet.TodosFromTriage saw an id island and let a second lane take the same
// file. Nothing warned, in any log, for a whole batch. The check is deliberately
// NARROW — it fires only when the card's own prose already names a repo path, so
// legitimately footprint-free work (research, docs sweeps) stays silent and the
// warning keeps meaning something.

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// prosePathRE matches a repo-relative source path inside free prose: at least
// one separator (a bare "audit.go" cannot be located) plus a known source
// extension. Anchored on a closed extension set rather than "anything with a
// slash" so ordinary prose ("selection→dispatch handoff") is never read as a
// path — the standing rule for parsing agent-authored text.
var prosePathRE = regexp.MustCompile(`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+\.(?:go|md|json|ya?ml|sh|ts|js|html)\b`)

// proseMetadataRE strips the bullet contract's metadata VALUES before the prose
// scan. Unlike metadataFieldRE (which deliberately keeps the evidence= value so
// the floor counters can read the package it names), this drops evidence= whole:
// the contract REQUIRES evidence={pointer} on every card and pointers are
// routinely file paths, so keeping it would warn on nearly every genuinely
// footprint-free card (research, a doc read) — and a warning that fires on
// compliant work is the fastest way to train an agent to ignore it.
var proseMetadataRE = regexp.MustCompile(`\bdefer_reason=[^\n]*|\b(?:source|priority|evidence)=\S+`)

// cardShape is the subset of a companion's top_n card this check reads.
type cardShape struct {
	ID     string   `json:"id"`
	Action string   `json:"action"`
	Files  []string `json:"files"`
}

// cardCheck is one committed card reduced to what the check needs: the usable
// footprint, and whether the card declared ANYTHING at all. The two are distinct
// offences — an absent declaration is an omission, a declaration that survives the
// repo-relative filter as nothing is a card that LOOKS compliant while matching no
// other card's files, which is strictly worse.
type cardCheck struct {
	id          string
	text        string
	usable      []string
	declaredRaw int
}

// MissingCardFilesWarning returns a non-empty operator/agent warning when a
// committed (## top_n) card names a repo path but declares no usable footprint.
// Two offences, because both produce the same invisible card: no declaration at
// all, and a declaration whose tokens are all unusable (a template placeholder, a
// glob, an absolute path — the mangled case is WORSE than an absent one, since the
// card looks compliant while matching nothing).
//
// companionPath is the authority when it exists and parses: ship/postship prefers
// an agent-authored triage-decision.json over the projection, so the companion is
// what the lane planner will actually read. Absent/unparseable ⇒ the report items
// are checked, which is exactly what ProjectDecisionJSON will project.
//
// Empty string means nothing to say: every path-naming card declared its files, no
// card names a path, or there is no top_n at all (absent input is never an alarm —
// the contract gate owns presence).
func MissingCardFilesWarning(artifact, companionPath string) string {
	cards, ok := committedCards(artifact, companionPath)
	if !ok {
		return ""
	}
	var offenders []string
	for _, c := range cards {
		switch {
		case len(c.usable) > 0:
			continue
		case c.declaredRaw > 0:
			offenders = append(offenders, fmt.Sprintf("%s (declared %d footprint token(s), none of them a usable repo-relative path)", c.id, c.declaredRaw))
		default:
			// No declaration at all: the card is an offender only if its own text
			// already names a repo path (footprint-free work stays silent).
			paths := prosePathRE.FindAllString(proseMetadataRE.ReplaceAllString(c.text, " "), -1)
			if len(paths) == 0 {
				continue
			}
			offenders = append(offenders, fmt.Sprintf("%s (names %s)", c.id, strings.Join(dedupe(paths), ", ")))
		}
	}
	if len(offenders) == 0 {
		return ""
	}
	return fmt.Sprintf("%d committed top_n card(s) carry no usable files= footprint: %s. "+
		"The fleet disjointness planner reads files[] ONLY (exact repo-relative overlap), so such a card becomes an "+
		"id island and a concurrent lane may edit the same file — add `files=path1;path2` to the item's metadata "+
		"tail (repo-relative, no globs or placeholders). Do NOT let the planner infer paths from prose: a wrong "+
		"inferred file is worse than an island.",
		len(offenders), strings.Join(offenders, "; "))
}

// committedCards resolves the cards to check. The companion wins when it exists
// and parses — ship/postship prefers an agent-authored triage-decision.json over
// the projection, so that file is what the lane planner will read. An absent,
// unreadable, malformed or top_n-less companion falls back to the report items,
// which is exactly what ProjectDecisionJSON will project. ok is false only when
// NEITHER source has a top_n (nothing to say; the contract gate owns presence, and
// MalformedCommittedFloorWarning owns malformed-companion reporting).
func committedCards(artifact, companionPath string) ([]cardCheck, bool) {
	if cards, ok := companionCards(companionPath); ok {
		return cards, true
	}
	body, found := topNSection(artifact)
	if !found {
		return nil, false
	}
	var cards []cardCheck
	for _, it := range parseItems(body) {
		declared, _ := splitDeclaredFiles(it.rest)
		cards = append(cards, cardCheck{
			id:          it.id,
			text:        it.rest,
			usable:      filesOf(it.rest),
			declaredRaw: len(declared),
		})
	}
	return cards, len(cards) > 0
}

// companionCards reads top_n from an agent-authored companion, applying the SAME
// repo-relative filter as the report path: a JSON array is not automatically a
// usable footprint.
func companionCards(companionPath string) ([]cardCheck, bool) {
	if companionPath == "" {
		return nil, false
	}
	data, err := os.ReadFile(companionPath)
	if err != nil {
		return nil, false
	}
	var decision struct {
		TopN []cardShape `json:"top_n"`
	}
	if json.Unmarshal(data, &decision) != nil || len(decision.TopN) == 0 {
		return nil, false
	}
	cards := make([]cardCheck, 0, len(decision.TopN))
	for _, c := range decision.TopN {
		if c.ID == "" {
			continue
		}
		var usable []string
		for _, f := range c.Files {
			if p, ok := declaredFilePath(f); ok {
				usable = append(usable, p)
			}
		}
		cards = append(cards, cardCheck{id: c.ID, text: c.Action, usable: usable, declaredRaw: len(c.Files)})
	}
	return cards, len(cards) > 0
}

// dedupe returns paths with duplicates removed, order preserved — one path cited
// twice in one action is one path.
func dedupe(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}
