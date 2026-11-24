package inboxcmd

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	current.Store(ptr(testDeps()))
	os.Exit(m.Run())
}
