//go:build !windows && !js && !plan9 && !aix && !android

package ballofilesystem

import (
	"os"
	"syscall"
)

// openFileNoFollow opens a file without following symlinks using O_NOFOLLOW.
// Available on Unix-like platforms (Linux, macOS, BSD, etc.).
// Provides TOCTOU-safe file operations.
func openFileNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag|syscall.O_NOFOLLOW, perm)
}
