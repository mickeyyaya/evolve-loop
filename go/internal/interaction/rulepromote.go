package interaction

// rulepromote.go — ADR-0045 I4: Reflexion for the auto-respond registry. The
// escalation report tells a HUMAN to run `evolve bridge add-rule`; the fatal-
// pane registry already self-expands (ADR-0044 Slice 5), and the interaction
// registry should too — more carefully, because a bad auto-respond rule ACTS
// (keystrokes) rather than just classifies.
//
// This is a thin payload specialization of the recovery/promote.go mechanism
// (one promotion idiom, two payloads): absent-only content-hash YAML,
// corrupt-safe replay, operator-edit-wins. The payload here is
// {regex, response_keys, note} + a per-rule stage, instead of {cause, substr}.
//
// Validation is the trust boundary and is DELIBERATELY STRICTER than the
// operator hatch (keyspec.Validate WARNs-but-sends): an auto-promoted rule
// that fires keystrokes must clear a REJECTING gate —
//   - the regex compiles under Go's RE2 (no catastrophic backtracking by
//     construction) and is >= minRulePatternLen (a tiny pattern is a
//     false-positive bomb that would inject keystrokes into healthy work);
//   - every response key passes keyspec.Classify as NON-suspect (a single
//     ClassSuspect token refuses the whole rule);
//   - the pattern must NOT match any line of the IMMUTABLE healthy-pane corpus
//     (a rule that fires on normal output is a DoS).
//
// Promoted rules land `shadow` (log would-respond only); auto-enforce is a
// MEASURED step (zero false fires observed via I1), never assumed.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/keyspec"
)

// minRulePatternLen is the floor on a promoted regex's source length: short
// patterns match too much and a keystroke-firing rule must never be that cheap
// to trigger (mirrors recovery.minPromotedSubstrLen's reasoning).
const minRulePatternLen = 12

// Rule stages (the per-rule rollout rides INSIDE the registry file, not a flag).
const (
	RuleStageShadow  = "shadow"
	RuleStageEnforce = "enforce"
)

// InteractionRule is one promoted auto-respond rule.
type InteractionRule struct {
	ID           string
	Regex        string
	ResponseKeys string // CSV, same shape as ManifestPrompt.ResponseKeys
	Note         string
	Stage        string // shadow (promoted) | enforce (measured-clean)
}

// ValidateRule is the REJECTING trust-boundary gate. corpus is the immutable
// healthy-pane fixture every promoted pattern must NOT match. A nil/err return
// means the rule is unsafe to promote — the caller escalates, never relaxes.
func ValidateRule(regex, responseKeys string, corpus []string) error {
	if len(regex) < minRulePatternLen {
		return fmt.Errorf("interaction: rule pattern %q too short to promote safely (min %d — short patterns are false-positive bombs)", regex, minRulePatternLen)
	}
	re, err := regexp.Compile(regex) // Go RE2 — no catastrophic backtracking by construction
	if err != nil {
		return fmt.Errorf("interaction: rule pattern does not compile: %w", err)
	}
	keys := strings.Split(responseKeys, ",")
	nonEmpty := false
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		nonEmpty = true
		// REJECTING form: keyspec.Validate WARNs-but-sends (operator hatch);
		// an auto-promoted rule gets the hard gate — any suspect token refuses.
		if keyspec.Classify(k) == keyspec.ClassSuspect {
			return fmt.Errorf("interaction: response key %q is a suspected typo (ClassSuspect) — refusing to auto-promote a keystroke rule", k)
		}
	}
	if !nonEmpty {
		return fmt.Errorf("interaction: rule has no response keys to send")
	}
	for _, line := range corpus {
		if line != "" && re.MatchString(line) {
			return fmt.Errorf("interaction: rule pattern matches the healthy-pane corpus (%q) — would fire on normal output", line)
		}
	}
	return nil
}

// ruleID derives the stable file id from the pattern (idempotent re-promotion,
// convergent with the absent-only write — the recovery.promotionID idiom).
func ruleID(regex string) string {
	sum := sha256.Sum256([]byte(regex))
	return "rule-" + hex.EncodeToString(sum[:6])
}

// PromoteRule validates then durably persists a rule under dir as <id>.yaml,
// absent-only (an existing — possibly operator-edited — file always wins),
// landing at stage `shadow`. Validation failure returns an error WITHOUT
// writing (the trust boundary). Returns the stable id.
func PromoteRule(dir, regex, responseKeys, note string, corpus []string) (string, error) {
	if err := ValidateRule(regex, responseKeys, corpus); err != nil {
		return "", err
	}
	id := ruleID(regex)
	path := filepath.Join(dir, id+".yaml")
	if _, err := os.Stat(path); err == nil {
		return id, nil // existing promotion wins (idempotent)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# auto-respond rule promoted into the interaction registry (ADR-0045 I4)\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "regex: %s\n", strconv.Quote(regex))
	fmt.Fprintf(&b, "response_keys: %s\n", strconv.Quote(responseKeys))
	fmt.Fprintf(&b, "note: %s\n", strconv.Quote(note))
	fmt.Fprintf(&b, "stage: %s\n", RuleStageShadow)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return id, nil
}

// LoadRules replays every parseable rule in dir, RE-VALIDATING each against
// the current corpus (a corpus update — e.g. a new CLI version's healthy
// banner — DEMOTES a rule that now matches, so promotion is never validated
// only once against a corpus that can rot). A rule failing re-validation is
// dropped from the loaded set (its file stays for the operator to inspect);
// a corrupt file is skipped (boot never bricks). enforceOnly returns only the
// rules cleared to enforce.
func LoadRules(dir string, corpus []string) []InteractionRule {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var rules []InteractionRule
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		r, ok := parseRule(string(data))
		if !ok {
			continue
		}
		// Boot re-validation against the CURRENT corpus (S3 corpus-rot).
		if err := ValidateRule(r.Regex, r.ResponseKeys, corpus); err != nil {
			continue // now-unsafe → demoted out of the active set
		}
		rules = append(rules, r)
	}
	return rules
}

// parseRule reads the fixed-key subset PromoteRule writes. A file missing the
// regex or response_keys is skipped (ok=false). An unknown stage normalizes to
// shadow (the safe default — never auto-escalate on a typo).
func parseRule(data string) (InteractionRule, bool) {
	var r InteractionRule
	for _, line := range strings.Split(data, "\n") {
		key, val, found := strings.Cut(line, ": ")
		if !found {
			continue
		}
		v := strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "id":
			r.ID = v
		case "regex":
			if s, err := strconv.Unquote(v); err == nil {
				r.Regex = s
			}
		case "response_keys":
			if s, err := strconv.Unquote(v); err == nil {
				r.ResponseKeys = s
			}
		case "note":
			if s, err := strconv.Unquote(v); err == nil {
				r.Note = s
			}
		case "stage":
			r.Stage = v
		}
	}
	if r.Regex == "" || r.ResponseKeys == "" {
		return InteractionRule{}, false
	}
	if r.Stage != RuleStageEnforce {
		r.Stage = RuleStageShadow
	}
	return r, true
}
