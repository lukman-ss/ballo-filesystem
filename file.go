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
//
// Threat model & Symlink safety:
// On Unix-like platforms (Linux, macOS, BSD), operations walk path components
// relative to a root directory file descriptor using openat with O_NOFOLLOW and
// O_DIRECTORY. This prevents TOCTOU symlink substitution attacks on intermediate
// directories. For the final component, symlinks pointing inside root are followed
// safely, while symlinks escaping root are strictly blocked.
// On unsupported platforms (Windows, JS/WASM, Plan 9), TOCTOU protection cannot
// be guaranteed; path validation and EvalSymlinks are used as best effort.
type localFS struct {
	root string
}

// NewLocal returns a local filesystem rooted at root.
// Panics if root is empty or cannot be converted to absolute path.
func NewLocal(root string) Filesystem {
	abs, err := filepath.Abs(root)
	if err != nil {
		panic("invalid root path: " + err.Error())
	}
	return &localFS{root: abs}
}

func (fs *localFS) validatePath(name string) error {
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

type localReadCloser struct {
	*os.File
}

func (fs *localFS) Open(name string) (io.ReadCloser, error) {
	if err := fs.validatePath(name); err != nil {
		return nil, err
	}
	f, err := openAtRoot(fs.root, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return &localReadCloser{File: f}, nil
}

func (fs *localFS) Read(name string) ([]byte, error) {
	if err := fs.validatePath(name); err != nil {
		return nil, err
	}
	f, err := openAtRoot(fs.root, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (fs *localFS) Write(name string, data []byte) error {
	if err := fs.validatePath(name); err != nil {
		return err
	}
	dirPath := filepath.Join(fs.root, filepath.Dir(name))
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}
	f, err := openAtRoot(fs.root, name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (fs *localFS) Delete(name string) error {
	if err := fs.validatePath(name); err != nil {
		return err
	}
	return deleteAtRoot(fs.root, name)
}

func (fs *localFS) Exists(name string) (bool, error) {
	if err := fs.validatePath(name); err != nil {
		return false, err
	}
	f, err := openAtRoot(fs.root, name, os.O_RDONLY, 0)
	if err == nil {
		f.Close()
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (fs *localFS) URL(name string) string {
	if err := fs.validatePath(name); err != nil {
		return ""
	}
	return "file://" + filepath.Join(fs.root, filepath.Clean(name))
}

func splitPath(p string) []string {
	var parts []string
	c := filepath.Clean(p)
	for c != "." && c != "" {
		b := filepath.Base(c)
		d := filepath.Dir(c)
		if b == "" || b == "." {
			break
		}
		parts = append([]string{b}, parts...)
		c = d
	}
	return parts
}

func containsParentTraversal(name string) bool {
	c := filepath.Clean(name)
	return c == ".." || filepath.HasPrefix(c, ".."+string(os.PathSeparator))
}

func isSubPath(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return !filepath.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".."
}

type readCloser struct {
	io.Reader
}

func (r *readCloser) Close() error { return nil }

var (
	ErrEmptyPath     = &PathError{Op: "validate", Path: "", Err: os.ErrInvalid}
	ErrAbsolutePath  = &PathError{Op: "validate", Path: "", Err: os.ErrInvalid}
	ErrPathTraversal = &PathError{Op: "validate", Path: "", Err: os.ErrPermission}
)

type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return e.Op + " " + e.Path + ": " + e.Err.Error()
}

func (e *PathError) Unwrap() error { return e.Err }

// MemoryFS implements Filesystem in memory.
type MemoryFS struct {
	mu    sync.RWMutex
	files map[string][]byte
}

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
	defer m.mu.RUnlock()
	data, ok := m.files[name]
	if !ok {
		return nil, &PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	c := make([]byte, len(data))
	copy(c, data)
	return io.NopCloser(bytes.NewReader(c)), nil
}

func (m *MemoryFS) Read(name string) ([]byte, error) {
	if err := m.validatePath(name); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[name]
	if !ok {
		return nil, &PathError{Op: "read", Path: name, Err: os.ErrNotExist}
	}
	c := make([]byte, len(data))
	copy(c, data)
	return c, nil
}

func (m *MemoryFS) Write(name string, d []byte) error {
	if err := m.validatePath(name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c := make([]byte, len(d))
	copy(c, d)
	m.files[name] = c
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
	defer m.mu.RUnlock()
	_, ok := m.files[name]
	return ok, nil
}

func (m *MemoryFS) URL(name string) string {
	return "memory://" + name
}
