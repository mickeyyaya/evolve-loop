package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
)

const newFamilyManifest = `{"cli":"newfam-tmux","binary":"newfam","default_tier":"balanced","parsers":{"usage":"claude-usage-v1"}}`

func newFamilyNeedsAGoChange() error {
	dec := json.NewDecoder(bytes.NewReader([]byte(newFamilyManifest)))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return fmt.Errorf("the manifest cannot declare its parsers: %w", err)
	}
	if _, ok := clicontrol.ParseUsage(m.Binary, "You've used 40% of your weekly limit"); !ok {
		return errors.New("a family whose manifest names claude-usage-v1 has no usage parser without a Go switch")
	}
	return nil
}

func TestIsolationMatrix_N14_ANewManifestWithKnownFormatIDsNeedsNoGoChange(t *testing.T) {
	t.Parallel()
	note, err := isolation.Judge("N14", newFamilyNeedsAGoChange())
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
}
