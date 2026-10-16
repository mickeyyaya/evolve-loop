package ciparity

import (
	"reflect"
	"testing"
)

func TestNewUngraduatedPackagesCoverfix_EmptyChangedYieldsNil(t *testing.T) {
	t.Parallel()
	if got := NewUngraduatedPackages(nil, []byte("./internal/a\n")); got != nil {
		t.Fatalf("empty changed must yield nil, got %v", got)
	}
}

func TestNewUngraduatedPackagesCoverfix_EnforceFileCommentsAndBlanksIgnored(t *testing.T) {
	t.Parallel()
	enforce := []byte("# header comment\n\n   \n./internal/a\n#./internal/b\n")
	got := NewUngraduatedPackages([]string{"./internal/a", "./internal/b", "./cmd/x"}, enforce)
	want := []string{"./internal/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commented enforce entry must not graduate its package: got %v want %v", got, want)
	}
}
