package ballofilesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFS_Basic(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	// Write
	if err := fs.Write("test.txt", []byte("hello")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read
	data, err := fs.Read("test.txt")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected hello, got %s", string(data))
	}

	// Exists
	exists, err := fs.Exists("test.txt")
	if err != nil || !exists {
		t.Fatalf("exists check failed")
	}

	// Delete
	if err := fs.Delete("test.txt"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Not found after delete
	if exists, err = fs.Exists("test.txt"); err != nil || exists {
		t.Fatalf("file should not exist after delete")
	}
}

func TestLocalFS_PathTraversal(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	tests := []string{
		"../etc/passwd",
		"../../etc/passwd",
	}

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			_, err := fs.Read(tc)
			if err == nil {
				t.Fatalf("expected error for path traversal: %s", tc)
			}
			var pe *PathError
			if !asPathError(err, &pe) || pe.Err != ErrPathTraversal.Err {
				t.Fatalf("expected ErrPathTraversal, got %v", err)
			}
		})
	}
}

func TestLocalFS_EmptyPath(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	_, err := fs.Read("")
	if err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestLocalFS_AbsolutePath(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	_, err := fs.Read("/etc/passwd")
	if err == nil {
		t.Fatalf("expected error for absolute path")
	}
}

func TestMemoryFS_Basic(t *testing.T) {
	fs := NewMemory()

	if err := fs.Write("test.txt", []byte("hello")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	data, err := fs.Read("test.txt")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected hello, got %s", string(data))
	}

	exists, err := fs.Exists("test.txt")
	if err != nil || !exists {
		t.Fatalf("exists check failed")
	}

	if err := fs.Delete("test.txt"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	exists, _ = fs.Exists("test.txt")
	if exists {
		t.Fatalf("file should not exist after delete")
	}
}

func TestMemoryFS_PathTraversal(t *testing.T) {
	fs := NewMemory()

	tests := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"foo/../..",
	}

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			_, err := fs.Read(tc)
			if err == nil {
				t.Fatalf("expected error for path traversal: %s", tc)
			}
		})
	}
}

func TestURL(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	fs.Write("test.txt", []byte("hello"))
	url := fs.URL("test.txt")
	if url == "" || !filepath.HasPrefix(url, "file://") {
		t.Fatalf("invalid file URL: %s", url)
	}

	mfs := NewMemory()
	url = mfs.URL("test.txt")
	if url != "memory://test.txt" {
		t.Fatalf("invalid memory URL: %s", url)
	}
}

func TestLocalFS_ReadNotFound(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	exists, err := fs.Exists("tidak-ada.txt")
	if err != nil || exists {
		t.Fatalf("file tidak boleh ada: %v", err)
	}

	_, err = fs.Read("tidak-ada.txt")
	if err == nil {
		t.Fatalf("expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestLocalFS_WriteNestedDir(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	// Direktori belum ada, Write harus membuat parent otomatis.
	if err := fs.Write("nested/sub/tidak-ada.txt", []byte("data")); err != nil {
		t.Fatalf("write to non-existent dir failed: %v", err)
	}

	exists, err := fs.Exists("nested/sub/tidak-ada.txt")
	if err != nil || !exists {
		t.Fatalf("file should exist: %v", err)
	}

	data, err := fs.Read("nested/sub/tidak-ada.txt")
	if err != nil || string(data) != "data" {
		t.Fatalf("read mismatch: %v", string(data))
	}
}

func TestLocalFS_DeleteNotFound(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	if err := fs.Delete("tidak-ada.txt"); err == nil {
		t.Fatalf("expected error deleting non-existent file")
	}
}

// Helpers
func asPathError(err error, out **PathError) bool {
	switch e := err.(type) {
	case *PathError:
		*out = e
		return true
	default:
		return false
	}
}

func TestLocalFS_Permission(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	// Create file
	if err := fs.Write("test.txt", []byte("data")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Make directory read-only to simulate permission issue
	dir := filepath.Join(tmp, "subdir")
	if err := os.Mkdir(dir, 0o000); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Write to subdirectory (may fail with permission, or succeed if root).
	err := fs.Write("subdir/nested.txt", []byte("data"))
	if err != nil {
		t.Logf("write failed: %v", err)
	}

	// Cleanup
	os.Chmod(dir, 0o755)
	os.RemoveAll(dir)
}

func TestMemoryFS_Permission(t *testing.T) {
	fs := NewMemory()

	// MemoryFS never returns permission errors, but validates paths
	if err := fs.Write("../evil.txt", []byte("data")); err == nil {
		t.Fatalf("expected error for path traversal in memory fs")
	}
}
