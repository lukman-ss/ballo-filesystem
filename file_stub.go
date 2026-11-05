//go:build windows || js || plan9 || aix || android

package ballofilesystem

import (
	"io"
	"os"
)

// openFileNoFollow is a stub for platforms that lack O_NOFOLLOW.
// Falls back to normal OpenFile; TOCTOU safety not guaranteed.
func openFileNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm)
}
