package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const explanationNeedsCorrection = "explanation-needs-correction"

const defectTokenPunctuation = "`'\"()[]{}<>,;:."

var defectLineLocator = regexp.MustCompile(`(?::\d+(?:[-,:]\d+)*|#L\d+(?:-L?\d+)?)$`)

func explanationCorrectionDocument(cs CycleState, fb *phasecontract.FailureBlock) (string, bool) {
	if fb == nil || len(fb.Defects) == 0 || len(cs.AuditFailReasons) > 0 {
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
		if strings.Contains(token, "/") || path.Ext(token) != "" {
			return token
		}
	}
	return ""
}

func namesDocument(location, document string) bool {
	return location == document || strings.HasSuffix(location, "/"+document)
}

func recordExplanationCorrection(workspace string, defects []string) error {
	reasons := make([]string, len(defects))
	for i, defect := range defects {
		reasons[i] = explanationNeedsCorrection + ": " + defect
	}
	b, err := json.Marshal(auditFailReason{SchemaVersion: 1, Phase: string(PhaseAudit), Reasons: reasons})
	if err != nil {
		return fmt.Errorf("record explanation correction: %w", err)
	}
	if err := writeArtifactAtomically(floorFailReasonPath(workspace, PhaseAudit), b); err != nil {
		return fmt.Errorf("record explanation correction: %w", err)
	}
	return nil
}

const retryActionReauthorExplanation retryAction = "retry@explanation"

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

func recordExplanationRound(cs CycleState, fb *phasecontract.FailureBlock) {
	if err := recordExplanationCorrection(cs.WorkspacePath, fb.Defects); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d %s: %v; the re-author reads the audit report alone\n", cs.CycleID, explanationNeedsCorrection, err)
	}
}
