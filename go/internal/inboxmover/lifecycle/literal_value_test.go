package lifecycle

import (
	"encoding/json"
	"testing"
)

func TestLiteralValueKeepsOtherEscapesAndDecodesHTMLOnes(t *testing.T) {
	cases := map[string]string{
		`"a\nb\"c"`: `"a\nb\"c"`,
		`"x<y&z>"`:  `"x<y&z>"`,
		`"tail\`:    `"tail\`,
		`"é"`:       `"é"`,
	}
	for in, want := range cases {
		if got := string(literalValue(json.RawMessage(in))); got != want {
			t.Errorf("literalValue(%s) = %s, want %s", in, got, want)
		}
	}
}
