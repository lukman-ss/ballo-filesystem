package ballofilesystem

import (
	"os"
	"path/filepath"
)

// Filesystem defines minimal operations.
type Filesystem interface {
	Open(name string) (*File, error)
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
	Delete(name string) error
	Exists(name string) (bool, error)
	URL(name string) string
}

// File wraps os.File with safe path handling.
type File struct {
	*os.File
	fs *localFS
}

func (f *File) Close() error {
	return f.File.Close()
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

// Open opens the named file.
func (fs *localFS) Open(name string) (*File, error) {
	p, err := fs.validatePath(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	return &File{File: f, fs: fs}, nil
}

// Read reads the named file.
func (fs *localFS) Read(name string) ([]byte, error) {
	p, err := fs.validatePath(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// Write writes data to the named file.
func (fs *localFS) Write(name string, data []byte) error {
	p, err := fs.validatePath(name)
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
	p, err := fs.validatePath(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// Exists reports whether the named file exists.
func (fs *localFS) Exists(name string) (bool, error) {
	p, err := fs.validatePath(name)
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
	p, err := fs.validatePath(name)
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

func (m *MemoryFS) Open(name string) (*File, error) {
	if err := m.validatePath(name); err != nil {
		return nil, err
	}
	_, ok := m.files[name]
	if !ok {
		return nil, &PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	// MemoryFS cannot return real *os.File, so return nil.
	// Callers should avoid using File methods beyond ReadCloser-like behavior.
	return nil, &PathError{Op: "open", Path: name, Err: ErrInvalidName}
}

func (m *MemoryFS) Read(name string) ([]byte, error) {
	if err := m.validatePath(name); err != nil {
		return nil, err
	}
	data, ok := m.files[name]
	if !ok {
		return nil, &PathError{Op: "read", Path: name, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *MemoryFS) Write(name string, data []byte) error {
	if err := m.validatePath(name); err != nil {
		return err
	}
	m.files[name] = data
	return nil
}

func (m *MemoryFS) Delete(name string) error {
	if err := m.validatePath(name); err != nil {
		return err
	}
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
	_, ok := m.files[name]
	return ok, nil
}

func (m *MemoryFS) URL(name string) string {
	return "memory://" + name
}
