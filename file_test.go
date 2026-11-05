package ballofilesystem

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
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

// --- New tests for task requirements ---

func TestMemoryFS_OpenReturnsReadCloser(t *testing.T) {
	fs := NewMemory()
	if err := fs.Write("a.txt", []byte("abc")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	r, err := fs.Open("a.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read from Open failed: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("Open data mismatch: got %q want %q", data, "abc")
	}
}

func TestMemoryFS_OpenNotFound(t *testing.T) {
	fs := NewMemory()
	_, err := fs.Open("missing")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %T: %v", err, err)
	}
}

func TestLocalFS_OpenReturnsReadCloser(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)
	path := filepath.Join(tmp, "b.txt")
	if err := os.WriteFile(path, []byte("local"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	r, err := fs.Open("b.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read from Open failed: %v", err)
	}
	if string(data) != "local" {
		t.Fatalf("Open data mismatch: got %q want %q", data, "local")
	}
}

func TestLocalFS_OpenNotFound(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)
	_, err := fs.Open("missing")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %T: %v", err, err)
	}
}

func TestMemoryFS_ReadCopiaData(t *testing.T) {
	fs := NewMemory()
	original := []byte("secret")
	if err := fs.Write("f.bin", original); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	read, err := fs.Read("f.bin")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if &read[0] == &original[0] {
		t.Fatal("Read returned alias to internal slice")
	}

	read[0] = 'X'
	read2, err := fs.Read("f.bin")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(read2) != "secret" {
		t.Fatalf("internal data was mutated by external alias")
	}
}

func TestMemoryFS_WriteCopiesData(t *testing.T) {
	fs := NewMemory()
	input := []byte("payload")
	if err := fs.Write("g.bin", input); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	input[0] = 'Z'
	read, err := fs.Read("g.bin")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(read) != "payload" {
		t.Fatalf("Write stored alias instead of copy")
	}
}

func TestMemoryFS_Concurrent(t *testing.T) {
	fs := NewMemory()
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("c/%d", n%10)
			if err := fs.Write(key, []byte("x")); err != nil {
				t.Errorf("write failed: %v", err)
				return
			}
			if _, err := fs.Read(key); err != nil {
				t.Errorf("read failed: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestLocalFS_SymlinkEscape(t *testing.T) {
	tmp := t.TempDir()
	outside := filepath.Join(os.TempDir(), "escapexxx.txt")
	if err := os.WriteFile(outside, []byte("leaked"), 0o644); err != nil {
		t.Fatalf("setup outside file failed: %v", err)
	}
	defer os.Remove(outside)

	fs := NewLocal(tmp)
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("setup symlink failed: %v", err)
	}

	_, err := fs.Read("link")
	if err == nil {
		t.Fatal("expected error reading through symlink that escapes root")
	}
	if !isPathTraversalError(err) {
		t.Fatalf("expected path traversal error, got %T: %v", err, err)
	}
}

func TestLocalFS_SymlinkDirectoryEscape(t *testing.T) {
	tmp := t.TempDir()
	secretDir := filepath.Join(tmp, "escapesecret")
	if err := os.Mkdir(secretDir, 0o755); err != nil {
		t.Fatalf("mkdir secret failed: %v", err)
	}
	secretFile := filepath.Join(secretDir, "x.txt")
	if err := os.WriteFile(secretFile, []byte("yes"), 0o644); err != nil {
		t.Fatalf("write secret failed: %v", err)
	}

	fs := NewLocal(tmp)
	insideDir := filepath.Join(tmp, "inside")
	if err := os.Mkdir(insideDir, 0o755); err != nil {
		t.Fatalf("mkdir inside failed: %v", err)
	}
	link := filepath.Join(insideDir, "esc")
	if err := os.Symlink(secretDir, link); err != nil {
		t.Fatalf("setup directory symlink failed: %v", err)
	}

	_, err := fs.Read("inside/esc/x.txt")
	if err == nil {
		t.Fatal("expected error reading through directory symlink that escapes root")
	}
	if !isPathTraversalError(err) {
		t.Fatalf("expected path traversal error, got %T: %v", err, err)
	}
}

func TestLocalFS_WriteThroughSymlink(t *testing.T) {
	tmp := t.TempDir()
	outside := filepath.Join(os.TempDir(), "escapewrite.txt")
	if err := os.WriteFile(outside, []byte("orig"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer os.Remove(outside)

	fs := NewLocal(tmp)
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("setup symlink failed: %v", err)
	}

	err := fs.Write("link", []byte("hijacked"))
	if err == nil {
		t.Fatal("expected error writing through escaping symlink")
	}
	if !isPathTraversalError(err) {
		t.Fatalf("expected path traversal error, got %T: %v", err, err)
	}

	content, _ := os.ReadFile(outside)
	if string(content) != "orig" {
		t.Fatalf("symlink write unexpectedly changed outside file")
	}
}

func TestLocalFS_CreateEscapingSymlink(t *testing.T) {
	tmp := t.TempDir()
	outside := filepath.Join(os.TempDir(), "escapetarget.txt")
	if err := os.WriteFile(outside, []byte("ok"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer os.Remove(outside)

	fs := NewLocal(tmp)
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("setup symlink failed: %v", err)
	}

	// Direct reference to escaping symlink should be blocked.
	_, err := fs.Open("link")
	if err == nil {
		t.Fatal("expected error opening escaping symlink")
	}
	if !isPathTraversalError(err) {
		t.Fatalf("expected path traversal error, got %T: %v", err, err)
	}
}

func TestMemoryFS_RaceReadCloser(t *testing.T) {
	fs := NewMemory()
	if err := fs.Write("race.txt", []byte("stable")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := fs.Open("race.txt")
			if err != nil {
				t.Errorf("Open failed: %v", err)
				return
			}
			defer r.Close()
			buf := make([]byte, 6)
			if _, err := io.ReadFull(r, buf); err != nil {
				t.Errorf("read failed: %v", err)
			}
			if string(buf) != "stable" {
				t.Errorf("concurrent Open returned corrupted data")
			}
		}()
	}
	wg.Wait()
}

func TestLocalFS_ExistsFollowsSymlink(t *testing.T) {
	tmp := t.TempDir()
	hidden := filepath.Join(tmp, "hidden.txt")
	if err := os.WriteFile(hidden, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	fs := NewLocal(tmp)
	link := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(hidden, link); err != nil {
		t.Fatalf("setup symlink failed: %v", err)
	}

	exists, err := fs.Exists("link.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("Exists should follow local symlinks inside root")
	}
}

func TestLocalFS_DeleteThroughSymlink(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(os.TempDir(), "escdeleted.txt")
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer os.Remove(target)

	fs := NewLocal(tmp)
	link := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("setup symlink failed: %v", err)
	}

	err := fs.Delete("link.txt")
	if err == nil {
		t.Fatal("expected error deleting through escaping symlink")
	}
	if !isPathTraversalError(err) {
		t.Fatalf("expected path traversal error, got %T: %v", err, err)
	}
}

func TestLocalFS_NestedSymlinkSafe(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)
	if err := fs.Write("safe.txt", []byte("ok")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	r, err := fs.Open("safe.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("read mismatch")
	}
}

func TestMemoryFS_NegativeTimeout(t *testing.T) {
	fs := NewMemory()
	if err := fs.Write("t.txt", []byte("tt")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	r, err := fs.Open("t.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer r.Close()

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 2)
		io.ReadFull(r, buf)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Open ReadCloser should be consumable")
	}
}

func isPathTraversalError(err error) bool {
	var pe *PathError
	if asPathError(err, &pe) {
		return pe.Err == ErrPathTraversal.Err
	}
	return false
}
