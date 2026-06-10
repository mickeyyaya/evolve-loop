// Package panetrust is the single trust boundary for pane-derived text
// (ADR-0045 I5). Anything captured from a tmux pane is attacker-influenceable
// agent output (OWASP LLM Top-10: segregate untrusted content): a manipulated
// or compromised agent can print text crafted to steer the supervisor — fake
// verdict sentinels, fake channel breadcrumbs, ANSI tricks. Every path that
// carries pane text toward an LLM prompt, a privileged decision, or a
// persisted ledger MUST traverse this package.
//
// Slice 1 (shipped with I1 telemetry) provides Digest: the neutralized,
// length-capped data block used for quarantined LLM consumption and for the
// interaction ledger (threat S10 — a stored-injection vector one hop removed).
// The full typed-extraction surface (Extract, untrusted framing) ships in the
// I5-full slice.
//
// Leaf constraints: imports stdlib only, so bridge, core, and interaction can
// all depend on it without cycles.
package panetrust

import (
	"regexp"
	"strings"
)

// ansiRE matches the CSI / OSC escape sequences stripped from captured panes.
// Copied verbatim from bridge/tmux.go (the panestream precedent: the leaf
// duplicates the two-alternative regex rather than importing bridge).
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

// defangTag is inserted inside a house control marker so the strict parsers
// (phasecontract.ParseVerdictSentinelFull, the channel-breadcrumb correlator)
// can no longer parse it, while a human reading the digest still sees what
// the agent printed and that it was defused.
const defangTag = "[untrusted]"

// markerBreaks defangs the house control markers an agent could spoof to
// steer the supervisor (threat S1) or to poison the persisted ledger one hop
// removed (S10). Each entry: the marker as the strict parser requires it →
// the broken form. Substring replacement (not regex) keeps the neutralizer
// trivially auditable; defanging runs AFTER ANSI stripping so an escape-split
// marker cannot reassemble into a parseable form.
var markerBreaks = [...][2]string{
	// phasecontract sentinel: `<!-- evolve-verdict: {...} -->`.
	{"evolve-verdict:", "evolve-verdict" + defangTag + ":"},
	// ADR-0037 channel breadcrumb: `{"evolve_channel":...,"corr_id":...}`.
	{`"evolve_channel"`, `"evolve_channel` + defangTag + `"`},
}

// redactedToken replaces secret-shaped strings in digests. Digests persist
// (the interaction ledger) and ship into prompts — possibly to a
// different-vendor fallback CLI — so a credential an agent echoed must never
// survive (threat S6).
const redactedToken = "[REDACTED]"

// secretREs are the secret-shaped patterns redacted WHOLE; kvSecretRE
// additionally redacts the VALUE of key:value lines whose key names a
// credential. RE2-only (no backtracking blowups by construction).
var secretREs = []*regexp.Regexp{
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{10,}`),                                        // OpenAI/Anthropic-style keys
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),                                           // AWS access key id
	regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs)_[A-Za-z0-9]{20,}\b`),                       // GitHub tokens
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),                               // GitHub fine-grained PAT
	regexp.MustCompile(`\bxox[a-z]-[A-Za-z0-9-]{10,}`),                                   // Slack tokens (all xox? families)
	regexp.MustCompile(`\bxapp-[A-Za-z0-9-]{10,}`),                                       // Slack app-level tokens
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]*`), // JWTs (header.payload.sig)
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),                             // PEM headers
}

// kvSecretRE redacts the VALUE of a `key: value` / `key=value` line whose key
// names a credential. The compound forms (access_token, private_key, …) are
// listed BEFORE the bare token/secret/key so the longer key name is the one
// captured — a bare `token` alternative cannot match inside `access_token`
// anyway (no word boundary before `token`), which is the gap this closes (S6).
var kvSecretRE = regexp.MustCompile(`(?i)\b(access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|private[_-]?key|api[_-]?key|credential|secret|token|password|passwd|authorization)(\s*[:=]\s*)\S+`)

// redactSecrets applies the secret patterns to already-ANSI-stripped text.
func redactSecrets(s string) string {
	for _, re := range secretREs {
		s = re.ReplaceAllString(s, redactedToken)
	}
	return kvSecretRE.ReplaceAllString(s, "${1}${2}"+redactedToken)
}

// Digest returns a neutralized digest of pane text, safe by construction to
// persist or to embed in an LLM prompt (under the caller's untrusted-content
// framing): ANSI/OSC stripped, secrets redacted, house markers defanged,
// capped at maxLines from the TAIL (recency beats volume) and maxCols runes
// per line (rune-safe truncation). maxLines <= 0 requests nothing;
// maxCols <= 0 means no column cap. Digest never joins pane text with
// anything else — no env, no templates — so nothing beyond what the agent
// printed (minus secrets) can leak out (S6).
func Digest(pane string, maxLines, maxCols int) string {
	if pane == "" || maxLines <= 0 {
		return ""
	}
	s := ansiRE.ReplaceAllString(pane, "")
	s = redactSecrets(s)
	for _, mb := range markerBreaks {
		s = strings.ReplaceAll(s, mb[0], mb[1])
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	if maxCols > 0 {
		for i, ln := range lines {
			lines[i] = truncateRunes(ln, maxCols)
		}
	}
	return strings.Join(lines, "\n")
}

// truncateRunes caps s at max runes without splitting a multibyte sequence.
func truncateRunes(s string, max int) string {
	if len(s) <= max { // fast path: byte length is an upper bound on rune count
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
