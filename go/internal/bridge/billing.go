package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// billing.go — credential-isolation hygiene (Go port of
// lib/billing-snapshot.sh): snapshot the credential-resolution state
// before/after a call and compare for an unintended auth path (env-var
// leak, proxy injection, credential rotation). Vendor-agnostic operator
// tooling, not on the cycle path.
//
// Simplification vs bash: file + env signals only (the macOS Keychain /
// `claude usage` / statsig branches are platform/exec-specific — the
// credentials-file fingerprint covers the common case).

type billingSnapshot struct {
	TS                    string `json:"ts"`
	Label                 string `json:"label"`
	CredSize              int64  `json:"cred_size"`
	CredTokenHash         string `json:"cred_token_hash"`
	AnthropicAPIKeyInEnv  string `json:"anthropic_api_key_in_env"`  // "yes" | "no"
	AnthropicBaseURLInEnv string `json:"anthropic_base_url_in_env"` // "" when unset
}

// BillingSnapshot writes a credential-state snapshot to dir and returns
// its path. The access token is never stored — only a salted-prefix hash.
func (e *Engine) BillingSnapshot(dir, label string) (string, error) {
	if dir == "" || label == "" {
		return "", fmt.Errorf("bridge:billing: snapshot requires dir and label")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("bridge:billing: mkdir %s: %w", dir, err)
	}
	snap := billingSnapshot{
		TS:                   e.deps.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Label:                label,
		CredTokenHash:        "absent",
		AnthropicAPIKeyInEnv: "no",
	}
	if b, err := os.ReadFile(filepath.Join(e.doctorHome(), ".claude", ".credentials.json")); err == nil {
		snap.CredSize = int64(len(b))
		snap.CredTokenHash = billingTokenHash(b)
	}
	if v, _ := lookupEnv(e.deps, "ANTHROPIC_API_KEY"); v != "" {
		snap.AnthropicAPIKeyInEnv = "yes"
	}
	if v, _ := lookupEnv(e.deps, "ANTHROPIC_BASE_URL"); v != "" {
		snap.AnthropicBaseURLInEnv = v
	}
	b, err := marshalIndent(snap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("bridge:billing: marshal: %w", err)
	}
	out := filepath.Join(dir, fmt.Sprintf("snap-%s-%d.json", label, e.deps.Now().Unix()))
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("bridge:billing: write %s: %w", out, err)
	}
	return out, nil
}

// billingTokenHash returns a sha256 of the access-token prefix (never the
// token itself), or a sentinel when absent / token-less.
func billingTokenHash(b []byte) string {
	s := string(b)
	i := strings.Index(s, `"accessToken":"`)
	if i < 0 {
		return "present-but-no-token-field"
	}
	end := i + 60
	if end > len(s) {
		end = len(s)
	}
	h := sha256.Sum256([]byte(s[i:end]))
	return hex.EncodeToString(h[:])
}

// BillingCompare diffs two snapshots and returns (verdict, exitCode):
// 0 PASS (credential isolation held), 1 FAIL (override env leak),
// 2 INCONCLUSIVE.
func BillingCompare(beforePath, afterPath string) (string, int) {
	var before, after billingSnapshot
	if !readBillingSnap(beforePath, &before) || !readBillingSnap(afterPath, &after) {
		return "INCONCLUSIVE: missing or invalid snapshot(s)", 2
	}
	if after.AnthropicAPIKeyInEnv == "yes" {
		return "FAIL: ANTHROPIC_API_KEY was set during the call (override credential path)", 1
	}
	if after.AnthropicBaseURLInEnv != "" {
		return "FAIL: ANTHROPIC_BASE_URL was set: " + after.AnthropicBaseURLInEnv, 1
	}
	if before.CredTokenHash != after.CredTokenHash &&
		after.CredTokenHash != "absent" && after.CredTokenHash != "present-but-no-token-field" {
		return "PASS: credentials token rotated between snapshots", 0
	}
	if after.CredTokenHash != "absent" {
		return "PASS: credentials present, no env-leak", 0
	}
	return "INCONCLUSIVE: no credential evidence (consult console manually)", 2
}

func readBillingSnap(path string, dst *billingSnapshot) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, dst) == nil
}
