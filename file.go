package ballofilesystem

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Filesystem defines minimal operations.
type Filesystem interface {
	Open(name string) (io.ReadCloser, error)
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
	Delete(name string) error
	Exists(name string) (bool, error)
	URL(name string) string
}

// localFS implements Filesystem using local disk.
type localFS struct {
	root string
}

// NewLocal returns a local filesystem rooted at root.
// Panics if root is empty or cannot be converted to absolute path.
func NewLocal(root string) Filesystem {
	abs, err := filepath.Abs(root)
	if err != nil {
		panic(err)
	}
	return &localFS{root: abs}
}

// validatePath ensures path is inside root and has no parent traversal.
func (fs *localFS) validatePath(name string) (string, error) {
	if name == "" {
		return "", ErrEmptyPath
	}
	if filepath.IsAbs(name) {
		return "", ErrAbsolutePath
	}
	if containsParentTraversal(name) {
		return "", ErrPathTraversal
	}
	joined := filepath.Join(fs.root, filepath.Clean(name))
	if !isSubPath(fs.root, joined) {
		return "", ErrPathTraversal
	}
	return joined, nil
}

// resolveChecked verifies that the final resolved path, including any
// existing symlinks in the path chain, remains inside fs.root.
// It also blocks writes/deletes that would traverse through escaping symlinks.
func (fs *localFS) resolveChecked(name string) (string, error) {
	joined, err := fs.validatePath(name)
	if err != nil {
		return "", err
	}

	// Walk path components, resolving and validating each symlink
	components := splitPath(name)
	basePath := fs.root

	for i, comp := range components {
		// Skip empty components
		if comp == "" || comp == "." {
			continue
		}

		if comp == ".." {
			return "", ErrPathTraversal
		}

		checkPath := filepath.Join(basePath, comp)

		// Check if this component is a symlink
		info, err := os.Lstat(checkPath)
		if err != nil {
			// Component doesn't exist yet (for write/create), stop here
			if os.IsNotExist(err) {
				break
			}
			return "", err
		}

		if info.IsDir() {
			// For directories, continue accumulating
			basePath = checkPath
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// Component is a symlink - resolve it and check if it escapes
			resolved, err := filepath.EvalSymlinks(checkPath)
			if err != nil {
				return "", ErrPathTraversal
			}
			if !isSubPath(fs.root, resolved) {
				return "", ErrPathTraversal
			}

			// For files (not dirs), stop - remaining components should be empty
			// (symlink target should be complete path)
			if i < len(components)-1 {
				return "", ErrPathTraversal
			}
			return joined, nil
		}

		// Regular file or other - stop
		basePath = checkPath
		break
	}

	return joined, nil
}

// splitPath splits a path into components.
func splitPath(path string) []string {
	var parts []string
	clean := filepath.Clean(path)
	for clean != "." && clean != "" {
		base := filepath.Base(clean)
		dir := filepath.Dir(clean)
		if base == "" || base == "." {
			break
		}
		parts = append([]string{base}, parts...)
		clean = dir
	}
	return parts
}

func containsParentTraversal(name string) bool {
	// Normalize separators for consistent checks
	clean := filepath.Clean(name)
	// Only block paths that start with .. or explicitly escape root
	return clean == ".." || filepath.HasPrefix(clean, ".."+string(os.PathSeparator))
}

func isSubPath(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !filepath.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".."
}

// readCloser wraps an io.Reader with a no-op Close.
type readCloser struct {
	io.Reader
}

func (r *readCloser) Close() error { return nil }

// Open opens the named file and returns an io.ReadCloser.
func (fs *localFS) Open(name string) (io.ReadCloser, error) {
	p, err := fs.resolveChecked(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	return &localReadCloser{File: f}, nil
}

// localReadCloser closes the underlying *os.File on Close.
type localReadCloser struct {
	*os.File
}

// Read reads the named file.
func (fs *localFS) Read(name string) ([]byte, error) {
	p, err := fs.resolveChecked(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// Write writes data to the named file.
func (fs *localFS) Write(name string, data []byte) error {
	p, err := fs.resolveChecked(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// Delete removes the named file.
func (fs *localFS) Delete(name string) error {
	p, err := fs.resolveChecked(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// Exists reports whether the named file exists.
func (fs *localFS) Exists(name string) (bool, error) {
	p, err := fs.resolveChecked(name)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// URL returns a file:// URL for the named file.
func (fs *localFS) URL(name string) string {
	p, err := fs.resolveChecked(name)
	if err != nil {
		return ""
	}
	return "file://" + p
}

// Errors.
var (
	ErrEmptyPath     = &PathError{Op: "validate", Path: "", Err: os.ErrInvalid}
	ErrAbsolutePath  = &PathError{Op: "validate", Path: "", Err: os.ErrInvalid}
	ErrPathTraversal = &PathError{Op: "validate", Path: "", Err: os.ErrPermission}
	ErrNotFound      = &PathError{Op: "read", Path: "", Err: os.ErrNotExist}
	ErrPermission    = &PathError{Op: "write", Path: "", Err: os.ErrPermission}
	ErrInvalidName   = &PathError{Op: "validate", Path: "", Err: os.ErrInvalid}
)

type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	if e.Path == "" {
		return e.Op + ": " + e.Err.Error()
	}
	return e.Op + " " + e.Path + ": " + e.Err.Error()
}

func (e *PathError) Unwrap() error {
	return e.Err
}

// MemoryFS implements Filesystem in memory.
type MemoryFS struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemory returns a new in-memory filesystem.
func NewMemory() Filesystem {
	return &MemoryFS{files: make(map[string][]byte)}
}

func (m *MemoryFS) validatePath(name string) error {
	if name == "" {
		return ErrEmptyPath
	}
	if filepath.IsAbs(name) {
		return ErrAbsolutePath
	}
	if containsParentTraversal(name) {
		return ErrPathTraversal
	}
	return nil
}

func (m *MemoryFS) Open(name string) (io.ReadCloser, error) {
	if err := m.validatePath(name); err != nil {
		return nil, err
	}
	m.mu.RLock()
	data, ok := m.files[name]
	m.mu.RUnlock()
	if !ok {
		return nil, &PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	clone := make([]byte, len(data))
	copy(clone, data)
	return io.NopCloser(bytes.NewReader(clone)), nil
}

func (m *MemoryFS) Read(name string) ([]byte, error) {
	if err := m.validatePath(name); err != nil {
		return nil, err
	}
	m.mu.RLock()
	data, ok := m.files[name]
	m.mu.RUnlock()
	if !ok {
		return nil, &PathError{Op: "read", Path: name, Err: os.ErrNotExist}
	}
	clone := make([]byte, len(data))
	copy(clone, data)
	return clone, nil
}

func (m *MemoryFS) Write(name string, data []byte) error {
	if err := m.validatePath(name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := make([]byte, len(data))
	copy(clone, data)
	m.files[name] = clone
	return nil
}

func (m *MemoryFS) Delete(name string) error {
	if err := m.validatePath(name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[name]; !ok {
		return &PathError{Op: "delete", Path: name, Err: os.ErrNotExist}
	}
	delete(m.files, name)
	return nil
}

func (m *MemoryFS) Exists(name string) (bool, error) {
	if err := m.validatePath(name); err != nil {
		return false, err
	}
	m.mu.RLock()
	_, ok := m.files[name]
	m.mu.RUnlock()
	return ok, nil
}

func (m *MemoryFS) URL(name string) string {
	return "memory://" + name
}
