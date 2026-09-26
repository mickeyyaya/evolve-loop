package triagecap

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// prosePathRE requires a slash and a known source extension, so ordinary prose is never read as a path.
var prosePathRE = regexp.MustCompile(`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+\.(?:go|md|json|ya?ml|sh|ts|js|html)\b`)

// proseMetadataRE, unlike metadataFieldRE, drops the evidence= value: the contract requires one on
// every card and it is usually a path, so keeping it would warn on nearly every footprint-free card.
var proseMetadataRE = regexp.MustCompile(`\bdefer_reason=[^\n]*|\b(?:source|priority|evidence)=\S+`)

type cardShape struct {
	ID     string   `json:"id"`
	Action string   `json:"action"`
	Files  []string `json:"files"`
}

// cardCheck keeps usable and declaredRaw apart: a declaration that filters to nothing looks compliant yet matches no card.
type cardCheck struct {
	id          string
	text        string
	usable      []string
	declaredRaw int
}

// MissingCardFilesWarning names committed cards that cite a repo path but declare no usable files= footprint.
// It returns "" when there is nothing to say, including when there is no top_n (the contract gate owns presence).
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
			// Undeclared cards offend only when their own text names a path; footprint-free work stays silent.
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

// committedCards prefers a parseable companion, which is what the lane planner reads; otherwise it reads the
// report items, which ProjectDecisionJSON will project. ok is false only when neither source has a top_n.
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

// companionCards applies the same repo-relative filter as the report path: a JSON array is not automatically usable.
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
