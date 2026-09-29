package core

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
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
