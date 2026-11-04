package evidence

import (
	"errors"
	"testing"
)

func TestParseFixJourney_ReadsTheLastFixJourneyLineOfTheMessage(t *testing.T) {
	t.Parallel()
	cases := map[string]FixJourney{
		"fix: a\n\nbody\n\n" + KeyFixJourney + ": family-lane\nCo-Authored-By: A <a@b>\n": {Present: true, ID: "family-lane"},
		"fix: a\n\nFix-Journey: none the fix changes only a message\n":                    {Present: true, Reason: "the fix changes only a message"},
		"fix: a\n\nFix-Journey: Bad_ID\n":                                                 {Present: true, ID: "Bad_ID"},
		"fix: a\r\n\r\nFix-Journey: family-lane\r\n\r\n":                                  {Present: true, ID: "family-lane"},
		"fix: a\n\nCloses-Inbox: item\n":                                                  {},
		"fix: a\n\nFix-Journey: family-lane\n\nbody after the trailer\n":                  {Present: true, ID: "family-lane"},
		"fix: a\n\nFix-Journey: none first\nFix-Journey: family-lane\n":                   {Present: true, ID: "family-lane"},
		"fix: a\n\nbody names Fix-Journey: family-lane inline\n":                          {},
	}
	for msg, want := range cases {
		got, err := ParseFixJourney(msg)
		if err != nil || got != want {
			t.Errorf("ParseFixJourney(%q) = %+v, %v; want %+v", msg, got, err, want)
		}
	}
}

func TestParseFixJourney_RefusesNoneWithNoReasonAndTwoWords(t *testing.T) {
	t.Parallel()
	for _, msg := range []string{"fix: a\n\nFix-Journey: none\n", "fix: a\n\nFix-Journey: none   \n", "fix: a\n\nFix-Journey: two words\n", "fix: a\n\nFix-Journey: \n"} {
		if _, err := ParseFixJourney(msg); !errors.Is(err, ErrFixJourneyInvalid) {
			t.Errorf("ParseFixJourney(%q) = %v, want ErrFixJourneyInvalid", msg, err)
		}
	}
}

func TestFixJourney_NoneIsAPresentTrailerWithNoID(t *testing.T) {
	t.Parallel()
	for f, want := range map[FixJourney]bool{{Present: true, Reason: "r"}: true, {Present: true, ID: "a"}: false, {}: false} {
		if got := f.None(); got != want {
			t.Errorf("%+v.None() = %v, want %v", f, got, want)
		}
	}
}

func TestParseFixJourney_TheTrailerSurvivesTheFooterThatShipAppends(t *testing.T) {
	t.Parallel()
	operator := "fix: resume the paused cycles\n\nBody.\n\nFix-Journey: concurrent-resume\nCloses-Inbox: item\nCo-Authored-By: A <a@example.com>\n"
	landed := operator + "\n\n---\n## Actual diff (v8.34.0+)\n\nFiles modified (1):\n- M\tgo/x.go\n\n 1 file changed, 1 insertion(+)" + "\n\nReviewed-by: code-simplifier\nReviewed-by: code-reviewer"

	got, err := ParseFixJourney(landed)

	if err != nil || got != (FixJourney{Present: true, ID: "concurrent-resume"}) {
		t.Fatalf("ParseFixJourney(landed) = %+v, %v; want the operator trailer after the ship footer", got, err)
	}
}
