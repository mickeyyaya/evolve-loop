package triage

import (
	"context"
	"strings"
	"testing"
)

const verifiedAfterTheContestedCommit = `{"id":"warn-gap","title":"t","kind":"bug","created_at":"2026-08-16T19:30:00Z",
 "premise_verified_at":"2026-09-05T00:00:00Z",
 "files":["go/internal/ship/consume.go (gate condition, both ship paths)","go/internal/ship/gone.go"]}`

func TestPremiseDriftSection_ResetsTheDriftWindowAtTheConsoleVerification(t *testing.T) {
	dir := premiseRepo(t, verifiedAfterTheContestedCommit)
	section := premiseDriftSection(context.Background(), dir, "warn-gap")
	if strings.Contains(section, "single-source committed-inbox-id resolution") {
		t.Errorf("a commit from before the console verified the premise is not drift:\n%s", section)
	}
	for _, want := range []string{"fix: restore audit authority (ReadVerdict)", "not at HEAD: go/internal/ship/gone.go", "2026-09-05"} {
		if !strings.Contains(section, want) {
			t.Errorf("section lacks %q:\n%s", want, section)
		}
	}
}

func TestPremiseDriftSection_AVerificationOlderThanTheFilingChangesNothing(t *testing.T) {
	item := `{"id":"warn-gap","kind":"bug","created_at":"2026-08-16T19:30:00Z","premise_verified_at":"2026-08-01T00:00:00Z",
 "files":["go/internal/ship/consume.go"]}`
	section := premiseDriftSection(context.Background(), premiseRepo(t, item), "warn-gap")
	if !strings.Contains(section, "warn-gap (filed 2026-08-16)") || !strings.Contains(section, "single-source committed-inbox-id resolution") {
		t.Errorf("the later of filing and verification bounds the window — here the filing:\n%s", section)
	}
}

func TestPremiseDriftSection_AnUnreadableVerificationStampFallsBackToTheFiling(t *testing.T) {
	item := `{"id":"warn-gap","kind":"bug","created_at":"2026-08-16T19:30:00Z","premise_verified_at":"yesterday",
 "files":["go/internal/ship/consume.go"]}`
	section := premiseDriftSection(context.Background(), premiseRepo(t, item), "warn-gap")
	if !strings.Contains(section, "single-source committed-inbox-id resolution") {
		t.Errorf("a stamp that does not parse must not hide drift:\n%s", section)
	}
}

func TestPremiseDriftSection_SilentWhenTheVerificationPostdatesEveryChange(t *testing.T) {
	item := `{"id":"warn-gap","kind":"bug","created_at":"2026-08-16T19:30:00Z","premise_verified_at":"2026-09-20T00:00:00Z",
 "files":["go/internal/ship/consume.go"]}`
	if section := premiseDriftSection(context.Background(), premiseRepo(t, item), "warn-gap"); section != "" {
		t.Errorf("a premise re-verified after the last change has no drift to show:\n%s", section)
	}
}
