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
// contract (explanationdocs.ValidateReviewedHandoff): an invalid or missing
// handoff must be reviewed as NEEDS_CORRECTION with a concrete correction
// todo, and NEEDS_CORRECTION is accepted when carryover-todos.json backs it.
func validateExplanationReview(report string, req core.PhaseRequest) error {
	if req.ExplanationDocumentationVersion == 0 {
		return nil
	}
	if req.BuildExplanationState == core.BuildExplanationNotYetBuilt {
		return nil
	}
	body, found, err := reportdoc.Section(report, "Explanation Documentation Review")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("retrospective-report.md is missing ## Explanation Documentation Review")
	}
	fields, err := reportdoc.Fields(body, "Status", "Build status", "Document", "Document SHA256", "Evidence", "Correction todo")
	if err != nil {
		return err
	}
	if err := reportdoc.RequirePathLineEvidence(fields["evidence"]); err != nil {
		return err
	}
	if strings.TrimSpace(fields["correction todo"]) == "" {
		return fmt.Errorf("explanation documentation review requires a Correction todo")
	}
	if req.BuildExplanationState == core.BuildExplanationInvalid || req.BuildExplanation == nil {
		if fields["status"] != "NEEDS_CORRECTION" || strings.EqualFold(fields["correction todo"], "none") {
			return fmt.Errorf("missing Build explanation requires NEEDS_CORRECTION and a concrete Correction todo")
		}
		return requireCorrectionTodo(req.Workspace, fields["correction todo"])
	}
	status, err := explanationdocs.ValidateReviewedHandoff(context.Background(), fields, req.BuildExplanation, req.Worktree, req.WorktreeBaseSHA)
	if err != nil {
		return err
	}
	if status == "NEEDS_CORRECTION" {
		if strings.EqualFold(fields["correction todo"], "none") {
			return fmt.Errorf("status NEEDS_CORRECTION requires a concrete Correction todo")
		}
		return requireCorrectionTodo(req.Workspace, fields["correction todo"])
	}
	if !strings.EqualFold(fields["correction todo"], "none") {
		return fmt.Errorf("verified explanation review must use Correction todo: none")
	}
	return nil
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
