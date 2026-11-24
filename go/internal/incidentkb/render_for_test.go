package incidentkb_test

import (
	"bytes"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

func TestRenderFor_RefusesAnIdThatDisagreesWithTheExpectedOne(t *testing.T) {
	t.Parallel()
	if _, err := incidentkb.RenderFor("2026-11-19-sample", []byte(sampleRecordDoc())); err != nil {
		t.Fatalf("control: the matching id must render: %v", err)
	}
	if out, err := incidentkb.RenderFor("2026-11-19-other", []byte(sampleRecordDoc())); err == nil || out != nil {
		t.Fatalf("an id that disagrees must render nothing: out=%d bytes, err=%v", len(out), err)
	}
}

func TestRenderFor_GivesTheBytesOfRenderWhenTheIdMatches(t *testing.T) {
	t.Parallel()
	want, err := incidentkb.Render([]byte(sampleRecordDoc()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := incidentkb.RenderFor("2026-11-19-sample", []byte(sampleRecordDoc()))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("RenderFor differs from Render: %v", err)
	}
}
