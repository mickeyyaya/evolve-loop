//go:build !darwin && !linux

package explanationdocs

import "os"

func openRegularNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
