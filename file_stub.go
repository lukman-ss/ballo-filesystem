//go:build windows || js || plan9 || aix || android

package ballofilesystem

import (
	"os"
	"path/filepath"
)

// openAtRoot fallback for platforms without atomic openat / O_NOFOLLOW support.
// Security boundary limitation: TOCTOU protection on intermediate directories
// cannot be guaranteed on these platforms.
func openAtRoot(root, name string, flag int, perm os.FileMode) (*os.File, error) {
	full := filepath.Join(root, filepath.Clean(name))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) && (flag&os.O_CREATE != 0) {
			dir := filepath.Dir(full)
			resDir, derr := filepath.EvalSymlinks(dir)
			if derr == nil && isSubPath(root, resDir) {
				return os.OpenFile(full, flag, perm)
			}
		}
		return nil, ErrPathTraversal
	}
	if !isSubPath(root, resolved) {
		return nil, ErrPathTraversal
	}
	return os.OpenFile(resolved, flag, perm)
}

// deleteAtRoot fallback for platforms without atomic unlinkat.
func deleteAtRoot(root, name string) error {
	full := filepath.Join(root, filepath.Clean(name))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return ErrPathTraversal
	}
	if !isSubPath(root, resolved) {
		return ErrPathTraversal
	}
	return os.Remove(resolved)
}
