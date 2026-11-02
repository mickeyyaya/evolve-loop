package auditfixports

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const reviewDir = "audit-fix-review"

type Reviewer struct {
	Run         AgentRun
	Route       Route
	ProjectRoot string
	Profiles    []string
}

var _ auditfix.Reviewer = Reviewer{}

func (r Reviewer) ReviewSeat(_ context.Context, avoid auditfix.Seat) (auditfix.Seat, error) {
	var errs []error
	for _, profile := range r.Profiles {
		seat, err := freshSeat(r.Route, profile)
		switch {
		case err != nil:
			errs = append(errs, err)
		case seat.Family != "" && seat.Family != avoid.Family:
			return seat, nil
		}
	}
	return auditfix.Seat{}, errors.Join(errs...)
}

func (r Reviewer) Review(ctx context.Context, req auditfix.ReviewRequest) (auditfix.Verdict, error) {
	files, err := writeReviewInputs(req)
	if err != nil {
		return auditfix.Verdict{}, err
	}
	args := []string{req.Seat.Profile, "--json", "--project-root", r.ProjectRoot}
	for _, f := range files {
		args = append(args, "--file", f)
	}
	env, err := r.Run(ctx, args, reviewBrief)
	if err != nil && (env.Verdict != core.VerdictFAIL || env.Error != "") {
		return auditfix.Verdict{}, fmt.Errorf("auditfixports: review: %w", err)
	}
	return reviewVerdict(env), nil
}

const reviewBrief = "Review only the attached patch against the attached prescriptions of a failed audit. " +
	"The patch must do what each prescription says and nothing more. It must add no regression, no empty test and no new production logic. " +
	"Give each defect as a finding with its severity, and end with the verdict.\n"

func reviewVerdict(env SeatResult) auditfix.Verdict {
	if env.Verdict == core.VerdictPASS || env.Verdict == core.VerdictWARN {
		return auditfix.Verdict{Pass: true}
	}
	return auditfix.Verdict{Reasons: []string{fmt.Sprintf("review %s (%s) verdict %q with %d critical and %d high findings",
		env.RunLabel, env.Family, env.Verdict, env.Critical, env.High)}}
}

func writeReviewInputs(req auditfix.ReviewRequest) ([]string, error) {
	dir := filepath.Join(req.Workspace, reviewDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("auditfixports: review inputs: %w", err)
	}
	var files []string
	for _, in := range []struct{ name, body string }{
		{"patch.diff", req.Patch.Diff},
		{"prescriptions.md", prescriptionsDoc(req.Prescriptions)},
	} {
		path := filepath.Join(dir, in.name)
		if err := os.WriteFile(path, []byte(in.body), 0o644); err != nil {
			return nil, fmt.Errorf("auditfixports: review inputs: %w", err)
		}
		files = append(files, path)
	}
	return files, nil
}

func prescriptionsDoc(prescriptions []auditfix.Prescription) string {
	var b strings.Builder
	b.WriteString("# Prescriptions\n\n")
	for _, p := range prescriptions {
		fmt.Fprintf(&b, "- Finding: %s\n  Prescription: %s\n", p.Defect, p.Text)
	}
	return b.String()
}
