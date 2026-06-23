package recovery

// promote.go — ADR-0044 Slice 5: the Reflexion-style promotion loop. A novel
// fatal pane state the LLM failure-advisor classifies ONCE becomes a
// deterministic registry entry forever: in-memory for the running batch and
// durably under .evolve/instincts/fatal-signatures/ for every later boot.
// The deterministic frontier grows; the LLM never re-pays for a known
// failure (judgment at the frontier, determinism in the core).
//
// File format: a minimal fixed-key YAML subset written AND parsed here with
// zero dependencies (the leaf stays dependency-free; faillearn's lessons use
// the same write-if-absent atomic posture). Promotion ids are deterministic
// (content hash of the substring), so re-promotion is idempotent and the
// absent-only write means an operator-edited file always wins.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/atomicwrite"
)

// promotedConfidence is recorded in every promoted-signature file: below the
// ≥0.9 of LLM-authored lessons, matching faillearn's deterministic-artifact
// confidence so downstream corpus consumers can weight accordingly.
const promotedConfidence = "0.5"

// minPromotedSubstrLen rejects substrings too short to be safe in the hot
// loop: a tiny substring is a false-positive bomb (it would match healthy
// agent output), and a kill-path trigger must never be that cheap to hit.
const minPromotedSubstrLen = 12

// FailureAdvice is the typed verdict the LLM failure-advisor returns for a
// CauseUnknown terminal state: the cause it diagnosed, the novel pane
// substring that identifies the state, and a human-readable justification.
// Cause is a string (not TerminalCause) because it arrives from parsed model
// output — PromoteAdvice validates it against the typed vocabulary before
// anything enters the registry.
type FailureAdvice struct {
	Cause         string `json:"cause"`
	PaneSubstr    string `json:"pane_substr"`
	Justification string `json:"justification"`
}

// validCauses is the promotion vocabulary: only causes the recovery pipeline
// knows how to reason about may enter the registry. CauseUnknown is
// deliberately absent — promoting "unknown" would be noise.
var validCauses = map[TerminalCause]struct{}{
	CauseModelInvalid:   {},
	CauseCLISelfUpdated: {},
	CauseDeadShell:      {},
}

// Promote appends a signature to the in-memory registry. Promotions land
// AFTER the vetted seeds (first match wins), so a promoted signature can
// never shadow a seed's classification. Nil-receiver safe no-op.
// NOT concurrency-safe: never Promote on an instance another goroutine is
// concurrently Detect-ing (today each tmux launch builds its own detector;
// the C3 integration must build a fresh detector post-promotion or add a
// mutex if that changes).
func (d *FatalPaneDetector) Promote(sig FatalSignature) {
	if d == nil || sig.Substr == "" {
		return
	}
	d.sigs = append(d.sigs, sig)
}

// promotionID derives the stable file id for a substring: a short content
// hash, so the same novel signature promotes to the same file forever
// (idempotent re-promotion, convergent with the absent-only write).
func promotionID(substr string) string {
	sum := sha256.Sum256([]byte(substr))
	return "sig-" + hex.EncodeToString(sum[:6])
}

// PromoteSignature durably persists a signature under dir as <id>.yaml,
// absent-only (an existing file — possibly operator-edited — always wins).
// Returns the stable id.
func PromoteSignature(dir string, sig FatalSignature) (string, error) {
	if sig.Substr == "" {
		return "", fmt.Errorf("recovery: empty signature substring")
	}
	id := promotionID(sig.Substr)
	path := filepath.Join(dir, id+".yaml")
	if _, err := os.Stat(path); err == nil {
		return id, nil // existing promotion wins (idempotent)
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
	// ADR-0049 N14: route through the atomicwrite SSOT, whose per-call os.CreateTemp
	// gives every writer a UNIQUE temp. Two concurrent fleet cycles classifying the
	// SAME novel pane resolve to the same content-addressed path; the old hand-rolled
	// `path + ".tmp"` was therefore SHARED, so their write/rename interleaved — the
	// loser's rename hit ENOENT (a lost promotion) and a partial write could tear the
	// entry that every later boot replays. The SSOT also mkdirs the parent, so the
	// explicit MkdirAll is gone. This is what the package header's "same write-if-absent
	// atomic posture as faillearn's lessons" claim always meant; now it is true.
	if err := atomicwrite.Bytes(path, []byte(b.String())); err != nil {
		return "", err
	}
	return id, nil
}

// loadPromotedSignatures reads every parseable *.yaml promotion in dir.
// Best-effort by design: an absent dir or a corrupt file yields what IS
// loadable — a bad promotion must never brick boot. (The write side is
// strict; this is the read side of a crash-safe loop.)
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

// parsePromotion reads the fixed-key subset PromoteSignature writes. A file
// missing substr or carrying an out-of-vocabulary cause is skipped (ok=false).
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

// SeedDetectorWithPromotions returns the seeded registry plus every durable
// promotion replayed from dir (typically
// <projectRoot>/.evolve/instincts/fatal-signatures). Seeds keep precedence;
// an unreadable dir degrades to seeds only.
func SeedDetectorWithPromotions(dir string) *FatalPaneDetector {
	d := SeedDetector()
	for _, sig := range loadPromotedSignatures(dir) {
		d.Promote(sig)
	}
	return d
}

// PromoteAdvice validates an LLM failure-advisor verdict and, when sound,
// promotes it both in-memory and durably. Validation is the trust boundary:
// an out-of-vocabulary cause or a dangerously short substring is REJECTED
// with an error (the caller escalates) — hallucinated judgment never enters
// the hot loop.
func PromoteAdvice(d *FatalPaneDetector, dir string, advice FailureAdvice) error {
	cause := TerminalCause(advice.Cause)
	if _, ok := validCauses[cause]; !ok {
		return fmt.Errorf("recovery: advice cause %q outside the typed vocabulary", advice.Cause)
	}
	if len(advice.PaneSubstr) < minPromotedSubstrLen {
		return fmt.Errorf("recovery: advice substring %q too short to promote safely (min %d chars — short substrings are false-positive bombs)", advice.PaneSubstr, minPromotedSubstrLen)
	}
	// ADR-0045 I5 backstop: the advisor reads a NEUTRALIZED pane digest, but
	// Detect matches RAW panes — a substring carrying a neutralization
	// artifact can never fire. Reject loudly (the caller escalates) instead
	// of promoting a permanently-dead signature.
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
