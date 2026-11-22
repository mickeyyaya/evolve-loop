package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func removeUntrackedCopiesOfOrigin(git func(...string) (string, error), root, remoteRef string, stdout io.Writer) {
	added, err := git("diff", "--name-only", "--no-renames", "--diff-filter=A", "HEAD..."+remoteRef)
	if err != nil {
		return
	}
	for _, rel := range strings.Fields(added) {
		if _, tracked := git("ls-files", "--error-unmatch", "--", rel); tracked == nil {
			continue
		}
		local, readErr := os.ReadFile(filepath.Join(root, rel))
		origin, showErr := git("show", remoteRef+":"+rel)
		if readErr != nil || showErr != nil || string(local) != origin {
			continue
		}
		if os.Remove(filepath.Join(root, rel)) == nil {
			fmt.Fprintf(stdout, "sync-main: removed the untracked copy of %s, which equals origin's file\n", rel)
		}
	}
}
