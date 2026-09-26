package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// promotedConfidence sits below LLM-authored lessons (>=0.9) and matches faillearn's deterministic artifacts.
const promotedConfidence = "0.5"

// minPromotedSubstrLen keeps short substrings, which match healthy output, out of the kill path.
const minPromotedSubstrLen = 12

// FailureAdvice is the advisor's unvalidated verdict on a CauseUnknown state; PromoteAdvice validates it.
type FailureAdvice struct {
	Cause         string `json:"cause"`
	PaneSubstr    string `json:"pane_substr"`
	Justification string `json:"justification"`
}

// validCauses is the promotion vocabulary; CauseUnknown is absent because promoting it is noise.
var validCauses = map[TerminalCause]struct{}{
	CauseModelInvalid:   {},
	CauseCLISelfUpdated: {},
	CauseDeadShell:      {},
}

// Promote appends sig after the seeds, so it can never shadow one; nil-receiver safe.
// Not concurrency-safe: never call it on a detector another goroutine is using.
func (d *FatalPaneDetector) Promote(sig FatalSignature) {
	if d == nil || sig.Substr == "" {
		return
	}
	d.sigs = append(d.sigs, sig)
}

// promotionID is a content hash, so re-promoting a substring targets the same file.
func promotionID(substr string) string {
	sum := sha256.Sum256([]byte(substr))
	return "sig-" + hex.EncodeToString(sum[:6])
}

// PromoteSignature writes sig under dir as <id>.yaml unless that file exists, and returns the id.
func PromoteSignature(dir string, sig FatalSignature) (string, error) {
	if sig.Substr == "" {
		return "", fmt.Errorf("recovery: empty signature substring")
	}
	id := promotionID(sig.Substr)
	path := filepath.Join(dir, id+".yaml")
	if _, err := os.Stat(path); err == nil {
		return id, nil // an existing, possibly operator-edited file wins
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# fatal-pane signature promoted into the deterministic registry (ADR-0044 Slice 5)\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "substr: %s\n", strconv.Quote(sig.Substr))
	fmt.Fprintf(&b, "cause: %s\n", sig.Cause)
	fmt.Fprintf(&b, "confidence: %s\n", promotedConfidence)
	fmt.Fprintf(&b, "note: %s\n", strconv.Quote(sig.Note))
	// atomicwrite gives each writer a unique temp, so concurrent promotions of
	// one path cannot tear it. See ADR-0049.
	if err := atomicwrite.Bytes(path, []byte(b.String())); err != nil {
		return "", err
	}
	return id, nil
}

// loadPromotedSignatures is best-effort: a missing dir or corrupt file must never block boot.
func loadPromotedSignatures(dir string) []FatalSignature {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var sigs []FatalSignature
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if sig, ok := parsePromotion(string(data)); ok {
			sigs = append(sigs, sig)
		}
	}
	return sigs
}

// parsePromotion reads the fixed-key format PromoteSignature writes.
func parsePromotion(data string) (FatalSignature, bool) {
	var sig FatalSignature
	for _, line := range strings.Split(data, "\n") {
		key, val, found := strings.Cut(line, ": ")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "substr":
			if s, err := strconv.Unquote(strings.TrimSpace(val)); err == nil {
				sig.Substr = s
			}
		case "cause":
			sig.Cause = TerminalCause(strings.TrimSpace(val))
		case "note":
			if s, err := strconv.Unquote(strings.TrimSpace(val)); err == nil {
				sig.Note = s
			}
		}
	}
	if sig.Substr == "" {
		return FatalSignature{}, false
	}
	if _, ok := validCauses[sig.Cause]; !ok {
		return FatalSignature{}, false
	}
	return sig, true
}

// SeedDetectorWithPromotions returns the seeds plus every promotion replayed from dir.
func SeedDetectorWithPromotions(dir string) *FatalPaneDetector {
	d := SeedDetector()
	for _, sig := range loadPromotedSignatures(dir) {
		d.Promote(sig)
	}
	return d
}

// PromoteAdvice validates advice, then promotes it in memory and under dir; invalid advice is an error.
func PromoteAdvice(d *FatalPaneDetector, dir string, advice FailureAdvice) error {
	cause := TerminalCause(advice.Cause)
	if _, ok := validCauses[cause]; !ok {
		return fmt.Errorf("recovery: advice cause %q outside the typed vocabulary", advice.Cause)
	}
	if len(advice.PaneSubstr) < minPromotedSubstrLen {
		return fmt.Errorf("recovery: advice substring %q too short to promote safely (min %d chars — short substrings are false-positive bombs)", advice.PaneSubstr, minPromotedSubstrLen)
	}
	// The advisor reads a neutralized digest but Detect matches raw panes, so a
	// neutralization artifact could never fire. See ADR-0045.
	for _, artifact := range []string{"[REDACTED]", "[untrusted]", "'''"} {
		if strings.Contains(advice.PaneSubstr, artifact) {
			return fmt.Errorf("recovery: pane_substr %q contains neutralization artifact %q — quoted from the digest view, not the raw pane; promotion rejected", advice.PaneSubstr, artifact)
		}
	}
	sig := FatalSignature{Substr: advice.PaneSubstr, Cause: cause, Note: advice.Justification}
	if _, err := PromoteSignature(dir, sig); err != nil {
		return fmt.Errorf("recovery: durable promotion: %w", err)
	}
	d.Promote(sig)
	return nil
}
