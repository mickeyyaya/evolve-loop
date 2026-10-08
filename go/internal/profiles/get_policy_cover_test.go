package profiles

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestGet_AnUnexpandableToolPolicyIsAnError(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"disallowed_tools", "allowed_tools"} {
		l := NewFromFS(fstest.MapFS{"x.json": {Data: []byte(`{"name":"x","` + field + `":["$include_policy:nope"]}`)}})

		p, err := l.Get("x")

		if err == nil || !strings.HasPrefix(err.Error(), "profiles: expand policies in x.json: ") || p.Name != "" {
			t.Errorf("%s: Get = %+v, %v, want the expand error and no profile", field, p, err)
		}
	}
}
