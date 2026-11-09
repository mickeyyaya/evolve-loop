package logcompress

import (
	"strings"
	"testing"
)

func TestCompress_ElidesAPassingRunAndReportsThroughResult(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("--- PASS: TestSomething\n", 30) + "FAIL\n"

	var res Result = Compress(in, settings())

	if !res.Applied || res.BytesIn != len(in) || res.BytesOut != len(res.Text) || res.TokensOut >= res.TokensIn {
		t.Errorf("Result = %+v", res)
	}
	if want := "[logcompress: 30 lines elided: passing tests]\nFAIL\n"; res.Text != want {
		t.Errorf("Text = %q, want %q", res.Text, want)
	}
}
