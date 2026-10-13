package faillearn

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderLessonYAML_PanicsWhenTheEncoderFails(t *testing.T) {
	original := marshalYAML
	t.Cleanup(func() { marshalYAML = original })
	marshalYAML = func(any) ([]byte, error) { return nil, errors.New("encoder down") }

	defer func() {
		got, _ := recover().(string)
		if !strings.Contains(got, "yaml.Marshal of lesson entry must not fail") || !strings.Contains(got, "encoder down") {
			t.Errorf("panic = %q, want the fixed message carrying the encoder error", got)
		}
	}()
	RenderLessonYAML(fixtureEvent())
}
