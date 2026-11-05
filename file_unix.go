//go:build !windows && !js && !plan9 && !aix && !android

package ballofilesystem

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// openAtRoot opens a path relative to the root directory handle,
// traversing intermediate directory components with O_NOFOLLOW to prevent
// intermediate symlink substitution (TOCTOU) attacks.
func openAtRoot(root, name string, flag int, perm os.FileMode) (*os.File, error) {
	components := splitPath(name)
	if len(components) == 0 {
		return nil, ErrEmptyPath
	}

	rootFd, err := syscall.Open(root, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(rootFd)

	curFd := rootFd
	for i := 0; i < len(components)-1; i++ {
		c := components[i]
		if c == "" || c == "." {
			continue
		}
		if c == ".." {
			if curFd != rootFd {
				syscall.Close(curFd)
			}
			return nil, ErrPathTraversal
		}

		nextFd, err := syscall.Openat(curFd, c, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if curFd != rootFd {
			syscall.Close(curFd)
		}
		if err != nil {
			if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOTDIR) {
				return nil, ErrPathTraversal
			}
			if errors.Is(err, syscall.ENOENT) {
				return nil, os.ErrNotExist
			}
			return nil, err
		}
		curFd = nextFd
	}
	defer func() {
		if curFd != rootFd {
			syscall.Close(curFd)
		}
	}()

	finalComp := components[len(components)-1]
	if finalComp == "" || finalComp == "." {
		return nil, ErrEmptyPath
	}
	if finalComp == ".." {
		return nil, ErrPathTraversal
	}

	// Try opening the final component with O_NOFOLLOW
	finalFd, err := syscall.Openat(curFd, finalComp, flag|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, uint32(perm))
	if err == nil {
		return os.NewFile(uintptr(finalFd), name), nil
	}

	// If ELOOP, final component is a symlink
	if errors.Is(err, syscall.ELOOP) {
		fullPath := filepath.Join(root, filepath.Clean(name))
		resolved, rerr := filepath.EvalSymlinks(fullPath)
		if rerr != nil || !isSubPath(root, resolved) {
			return nil, ErrPathTraversal
		}
		// Final component is a symlink pointing inside root
		rel, rerr := filepath.Rel(root, resolved)
		if rerr != nil || containsParentTraversal(rel) {
			return nil, ErrPathTraversal
		}
		// Re-open target safely relative to root
		return openAtRoot(root, rel, flag, perm)
	}

	if errors.Is(err, syscall.ENOENT) {
		return nil, os.ErrNotExist
	}
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		return nil, os.ErrPermission
	}
	return nil, err
}

// deleteAtRoot deletes a path relative to the root directory handle,
// verifying intermediate directory components with O_NOFOLLOW.
func deleteAtRoot(root, name string) error {
	components := splitPath(name)
	if len(components) == 0 {
		return ErrEmptyPath
	}

	rootFd, err := syscall.Open(root, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(rootFd)

	curFd := rootFd
	for i := 0; i < len(components)-1; i++ {
		c := components[i]
		if c == "" || c == "." {
			continue
		}
		if c == ".." {
			if curFd != rootFd {
				syscall.Close(curFd)
			}
			return ErrPathTraversal
		}

		nextFd, err := syscall.Openat(curFd, c, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if curFd != rootFd {
			syscall.Close(curFd)
		}
		if err != nil {
			if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOTDIR) {
				return ErrPathTraversal
			}
			if errors.Is(err, syscall.ENOENT) {
				return os.ErrNotExist
			}
			return err
		}
		curFd = nextFd
	}
	defer func() {
		if curFd != rootFd {
			syscall.Close(curFd)
		}
	}()

	finalComp := components[len(components)-1]
	if finalComp == "" || finalComp == "." {
		return ErrEmptyPath
	}
	if finalComp == ".." {
		return ErrPathTraversal
	}

	// Check if final component is a symlink
	fullPath := filepath.Join(root, filepath.Clean(name))
	info, lerr := os.Lstat(fullPath)
	if lerr != nil {
		if os.IsNotExist(lerr) {
			return os.ErrNotExist
		}
		return lerr
	}

	if info.Mode()&os.ModeSymlink != 0 {
		resolved, rerr := filepath.EvalSymlinks(fullPath)
		if rerr != nil || !isSubPath(root, resolved) {
			return ErrPathTraversal
		}
		// If symlink points inside root, remove the target as expected by tests
		rel, rerr := filepath.Rel(root, resolved)
		if rerr != nil || containsParentTraversal(rel) {
			return ErrPathTraversal
		}
		return deleteAtRoot(root, rel)
	}

	// Normal file: remove atomically relative to curFd
	err = syscall.Unlinkat(curFd, finalComp)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return os.ErrNotExist
		}
		return err
	}
	return nil
}
