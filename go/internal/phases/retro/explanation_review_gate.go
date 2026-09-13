package retro

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// validateExplanationReview applies retro's policy around the shared review
// contract (explanationdocs.ValidateReviewedHandoff). Since 2026-09-13
// (ADR-0102, operator decision) the review's shape — the section, its
// fields, the correction-todo bookkeeping, the handoff echoes and the
// citations — is advisory: the findings ride the retrospective's record as
// warnings. The error, the only blocking outcome, is the reasoning floor (a
// token Evidence; a missing or duplicated review section is no review text
// at all) and host-side defects in the handoff itself.
func validateExplanationReview(report string, req core.PhaseRequest) (advisories []string, err error) {
	if req.ExplanationDocumentationVersion == 0 {
		return nil, nil
	}
	if req.BuildExplanationState == core.BuildExplanationNotYetBuilt {
		return nil, nil
	}
	fields, advisories, err := reportdoc.ReasonedReview(report, "retrospective-report.md", "Explanation Documentation Review", "Status", "Build status", "Document", "Document SHA256", "Evidence", "Correction todo")
	if err != nil || fields == nil {
		return advisories, err
	}
	todo := strings.TrimSpace(fields["correction todo"])
	if todo == "" {
		advisories = append(advisories, "explanation documentation review requires a Correction todo")
	}
	if req.BuildExplanationState == core.BuildExplanationInvalid || req.BuildExplanation == nil {
		if fields["status"] != "NEEDS_CORRECTION" || strings.EqualFold(todo, "none") {
			return append(advisories, "missing Build explanation requires NEEDS_CORRECTION and a concrete Correction todo"+explanationdocs.HostReason(req.BuildExplanationError)), nil
		}
		return adviseUnbackedTodo(advisories, req.Workspace, todo), nil
	}
	review, err := explanationdocs.ValidateReviewedHandoff(context.Background(), fields, req.BuildExplanation, req.Worktree, req.WorktreeBaseSHA)
	if err != nil {
		return append(advisories, review.Advisories...), err // the findings made before the host defect still ride the record
	}
	advisories = append(advisories, review.Advisories...)
	if review.Status == "NEEDS_CORRECTION" {
		if strings.EqualFold(todo, "none") {
			return append(advisories, "status NEEDS_CORRECTION requires a concrete Correction todo"), nil
		}
		return adviseUnbackedTodo(advisories, req.Workspace, todo), nil
	}
	if !strings.EqualFold(todo, "none") {
		advisories = append(advisories, "verified explanation review must use Correction todo: none")
	}
	return advisories, nil
}

// adviseUnbackedTodo records, as an advisory, a correction todo that
// carryover-todos.json does not back.
func adviseUnbackedTodo(advisories []string, workspace, todo string) []string {
	if err := requireCorrectionTodo(workspace, todo); err != nil {
		return append(advisories, err.Error())
	}
	return advisories
}

func requireCorrectionTodo(workspace, id string) error {
	id = strings.TrimSpace(id)
	path := filepath.Join(workspace, "carryover-todos.json")
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("correction todo %q is not backed by carryover-todos.json: %w", id, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 1<<20 {
		return fmt.Errorf("carryover-todos.json must be a regular file no larger than 1 MiB")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read carryover-todos.json: %w", err)
	}
	var todos []struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(body, &todos); err != nil {
		return fmt.Errorf("parse carryover-todos.json: %w", err)
	}
	for _, todo := range todos {
		if strings.TrimSpace(todo.ID) == id && strings.TrimSpace(todo.Action) != "" {
			return nil
		}
	}
	return fmt.Errorf("correction todo %q is absent from carryover-todos.json", id)
}
