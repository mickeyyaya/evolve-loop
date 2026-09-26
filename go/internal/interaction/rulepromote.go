package interaction

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/keyspec"
)

// minRulePatternLen keeps short patterns, which match healthy output, from firing keystrokes.
const minRulePatternLen = 12

// Rule stages; the per-rule stage rides inside the registry file.
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
	Stage        string // RuleStageShadow or RuleStageEnforce
}

// ValidateRule is the rejecting trust-boundary gate; a non-nil error means the rule is unsafe to promote.
// corpus is the immutable healthy-pane fixture no promoted pattern may match.
func ValidateRule(regex, responseKeys string, corpus []string) error {
	if len(regex) < minRulePatternLen {
		return fmt.Errorf("interaction: rule pattern %q too short to promote safely (min %d — short patterns are false-positive bombs)", regex, minRulePatternLen)
	}
	re, err := regexp.Compile(regex)
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
		// Stricter than keyspec.Validate, which warns but sends: a promoted rule fires keystrokes.
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

// ruleID hashes the pattern, so re-promotion targets the same file.
func ruleID(regex string) string {
	sum := sha256.Sum256([]byte(regex))
	return "rule-" + hex.EncodeToString(sum[:6])
}

// PromoteRule validates a rule, then writes it under dir as <id>.yaml at stage shadow unless
// that file exists. It returns the id; a validation failure writes nothing.
func PromoteRule(dir, regex, responseKeys, note string, corpus []string) (string, error) {
	if err := ValidateRule(regex, responseKeys, corpus); err != nil {
		return "", err
	}
	id := ruleID(regex)
	path := filepath.Join(dir, id+".yaml")
	if _, err := os.Stat(path); err == nil {
		return id, nil // an existing, possibly operator-edited file wins
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# auto-respond rule promoted into the interaction registry (ADR-0045 I4)\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "regex: %s\n", strconv.Quote(regex))
	fmt.Fprintf(&b, "response_keys: %s\n", strconv.Quote(responseKeys))
	fmt.Fprintf(&b, "note: %s\n", strconv.Quote(note))
	fmt.Fprintf(&b, "stage: %s\n", RuleStageShadow)
	// atomicwrite gives each writer a unique temp, so concurrent promotions of one path cannot tear it.
	// See ADR-0049.
	if err := atomicwrite.Bytes(path, []byte(b.String())); err != nil {
		return "", err
	}
	return id, nil
}

// EnforceRule flips a promoted rule from shadow to enforce after re-validating it against the
// current corpus. A missing rule is an error; an already-enforced rule is a no-op.
func EnforceRule(dir, id string, corpus []string) error {
	path := filepath.Join(dir, id+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("interaction: enforce %s: %w", id, err)
	}
	r, ok := parseRule(string(data))
	if !ok {
		return fmt.Errorf("interaction: enforce %s: rule file unparseable", id)
	}
	if r.Stage == RuleStageEnforce {
		return nil
	}
	if err := ValidateRule(r.Regex, r.ResponseKeys, corpus); err != nil {
		return fmt.Errorf("interaction: enforce %s: re-validation failed (corpus rot since promotion?): %w", id, err)
	}
	// Rewrite only the stage line, so operator-added fields survive the flip.
	lines := strings.Split(string(data), "\n")
	flipped := false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "stage:") {
			lines[i] = "stage: " + RuleStageEnforce
			flipped = true
			break
		}
	}
	if !flipped {
		lines = append(lines, "stage: "+RuleStageEnforce)
	}
	// Concurrent flips each get a unique temp; last writer wins, and all write enforce.
	if err := atomicwrite.Bytes(path, []byte(strings.Join(lines, "\n"))); err != nil {
		return fmt.Errorf("interaction: enforce %s: %w", id, err)
	}
	return nil
}

// LoadRules replays every parseable rule in dir, dropping any that fails re-validation against
// the current corpus; a corrupt file is skipped so boot never bricks.
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
		if err := ValidateRule(r.Regex, r.ResponseKeys, corpus); err != nil {
			continue
		}
		rules = append(rules, r)
	}
	return rules
}

// parseRule reads the fixed-key subset PromoteRule writes; it needs regex and response_keys,
// and any stage other than enforce reads as shadow, so a typo never escalates a rule.
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
