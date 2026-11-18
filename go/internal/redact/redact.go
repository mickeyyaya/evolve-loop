// Package redact is the one redaction leaf of the repository: secret patterns and the entropy rule.
// See docs/architecture/packages/internal-redact.md.
package redact

import (
	"math"
	"regexp"
)

const Token = "[REDACTED]"

const (
	entropyMinBits    = 4.0
	hexCommitLen      = 40
	hexDigestLen      = 64
	entropyRunPattern = `[A-Za-z0-9+/=_-]{32,}`
)

var secretREs = []*regexp.Regexp{
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{10,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs)_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bxox[a-z]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\bxapp-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]*`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
}

var kvSecretRE = regexp.MustCompile(`(?i)\b(access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|private[_-]?key|api[_-]?key|credential|secret|token|password|passwd|authorization)(\s*[:=]\s*)\S+`)

var (
	entropyRunRE = regexp.MustCompile(entropyRunPattern)
	hexRunRE     = regexp.MustCompile(`^[0-9a-fA-F]+$`)
)

func Secrets(s string) string {
	for _, re := range secretREs {
		s = re.ReplaceAllString(s, Token)
	}
	return kvSecretRE.ReplaceAllString(s, "${1}${2}"+Token)
}

func EntropyRuns(s string) []string {
	var flagged []string
	for _, run := range entropyRunRE.FindAllString(s, -1) {
		if isHexDigest(run) || shannonBits(run) < entropyMinBits {
			continue
		}
		flagged = append(flagged, run)
	}
	return flagged
}

func isHexDigest(run string) bool {
	return (len(run) == hexCommitLen || len(run) == hexDigestLen) && hexRunRE.MatchString(run)
}

func shannonBits(run string) float64 {
	counts := map[byte]int{}
	for i := 0; i < len(run); i++ {
		counts[run[i]]++
	}
	n := float64(len(run))
	bits := 0.0
	for _, c := range counts {
		p := float64(c) / n
		bits -= p * math.Log2(p)
	}
	return bits
}
