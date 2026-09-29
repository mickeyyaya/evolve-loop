package core

import (
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const explanationNeedsCorrection = "explanation-needs-correction"

const defectTokenPunctuation = "`'\"()[]{}<>,;:."

var defectLineLocator = regexp.MustCompile(`(?::\d+(?:[-,:]\d+)*|#L\d+(?:-L?\d+)?)$`)

func explanationCorrectionDocument(cs CycleState, fb *phasecontract.FailureBlock) (string, bool) {
	if fb == nil || len(fb.Defects) == 0 || runnerDiagnosedAudit(cs) {
		return "", false
	}
	document, err := explanationdocs.DocumentPath(cs.CycleID, cs.RunID)
	if err != nil {
		return "", false
	}
	for _, defect := range fb.Defects {
		if !namesDocument(defectLocation(defect), document) {
			return "", false
		}
	}
	return document, true
}

func defectLocation(defect string) string {
	for _, field := range strings.Fields(defect) {
		token := strings.Trim(field, defectTokenPunctuation)
		token = strings.Trim(defectLineLocator.ReplaceAllString(token, ""), defectTokenPunctuation)
		if pathLike(token) {
			return token
		}
	}
	return ""
}

func pathLike(token string) bool {
	ext := path.Ext(token)
	stem := strings.TrimSuffix(token, ext)
	return strings.Contains(token, "/") || (len(stem) > 1 && len(ext) > 1 && unicode.IsLetter(rune(ext[1])))
}

func namesDocument(location, document string) bool {
	return location == document || strings.HasSuffix(location, "/"+document)
}

func explanationCorrectionEnvelope(env retryEnvelope) retryEnvelope {
	if !slices.Contains(env.Legal, retryActionRetryBuild) {
		return env
	}
	return retryEnvelope{
		Legal:  []retryAction{retryActionReauthorExplanation, retryActionDecline},
		Reason: env.Reason + "; every defect names the cycle's explanation document (" + explanationNeedsCorrection + "): Build re-authors it, TDD and the code build are skipped",
	}
}

func explanationReauthorScope(next Phase, cs CycleState) string {
	if next != PhaseBuild {
		return ""
	}
	fb, _ := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit))
	document, _ := explanationCorrectionDocument(cs, fb)
	return document
}
