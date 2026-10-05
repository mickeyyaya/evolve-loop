package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func (m *Mover) VerifyPremise(taskID, evidence string) (string, error) {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return "", fmt.Errorf("%w: verify requires evidence", ErrBadArgs)
	}
	loc, err := m.locatePending("verify", taskID)
	if err != nil {
		return "", err
	}
	sha, err := m.verifiedAgainst()
	if err != nil {
		return "", err
	}
	stamp := m.now().UTC().Format(time.RFC3339)
	if err := UpdateItemJSON(loc.Path, func(item map[string]json.RawMessage) {
		item[inboxbatch.PremiseVerifiedAtField] = jsonString(stamp)
		item[inboxbatch.PremiseVerifiedSHAField] = jsonString(sha)
		item[inboxbatch.PremiseVerifiedEvidenceField] = jsonString(evidence)
	}); err != nil {
		return "", fmt.Errorf("verify: rewrite %s: %w", loc.Path, err)
	}
	m.linef("verified the premise of %s against %s", taskID, sha)
	m.ledgerLine(ledgerEntry{Action: "verify-premise", TaskID: taskID, GitSHA: &sha, Reason: evidence})
	return loc.Path, nil
}

func (m *Mover) verifiedAgainst() (string, error) {
	sha, err := m.mainHead()
	sha = strings.TrimSpace(sha)
	switch {
	case err != nil:
		return "", fmt.Errorf("verify: resolve the main head: %w", err)
	case sha == "":
		return "", errors.New("verify: resolve the main head: it read as empty")
	}
	return sha, nil
}
