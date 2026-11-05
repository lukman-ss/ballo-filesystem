package ballofilesystem

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func TestLocalFS_SymlinkToInsideRootAllowed(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	target := filepath.Join(tmp, "target.txt")
	if err := os.WriteFile(target, []byte("inside"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	link := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("setup symlink failed: %v", err)
	}

	// Final component symlink pointing inside root is allowed.
	data, err := fs.Read("link.txt")
	if err != nil {
		t.Fatalf("expected read through inside-root symlink to succeed: %v", err)
	}
	if string(data) != "inside" {
		t.Fatalf("expected inside content, got %q", string(data))
	}

	exists, err := fs.Exists("link.txt")
	if err != nil || !exists {
		t.Fatalf("Exists should follow local symlinks inside root")
	}

	if err := fs.Write("link.txt", []byte("updated")); err != nil {
		t.Fatalf("write through inside-root symlink failed: %v", err)
	}
	data, err = fs.Read("link.txt")
	if err != nil || string(data) != "updated" {
		t.Fatalf("expected updated content, got %q err=%v", string(data), err)
	}

	if err := fs.Delete("link.txt"); err != nil {
		t.Fatalf("delete through inside-root symlink failed: %v", err)
	}
	if exists, _ = fs.Exists("target.txt"); exists {
		t.Fatal("target file should have been removed through inside-root symlink")
	}
}

func TestLocalFS_IntermediateDirectorySymlinkBlocked(t *testing.T) {
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	if err := fs.Write("safe.txt", []byte("ok")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	outside := filepath.Join(os.TempDir(), "intermediate_escape")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside failed: %v", err)
	}
	outsideFile := filepath.Join(outside, "leak.txt")
	if err := os.WriteFile(outsideFile, []byte("leaked"), 0o644); err != nil {
		t.Fatalf("write outside failed: %v", err)
	}
	defer os.RemoveAll(outside)

	dirLink := filepath.Join(tmp, "dir")
	if err := os.Symlink(outside, dirLink); err != nil {
		t.Fatalf("setup directory symlink failed: %v", err)
	}

	// Directory symlink followed by more path components must be blocked.
	_, err := fs.Read("dir/leak.txt")
	if err == nil {
		t.Fatal("expected error reading through directory symlink")
	}
	if !isPathTraversalError(err) {
		t.Fatalf("expected path traversal error, got %T: %v", err, err)
	}
}

func platformSupportsO_NOFOLLOW() bool {
	return runtime.GOOS != "windows" && runtime.GOOS != "js" && runtime.GOOS != "plan9" && runtime.GOOS != "aix" && runtime.GOOS != "android"
}

func TestLocalFS_ConcurrentSymlinkSubstitutionFinal(t *testing.T) {
	if !platformSupportsO_NOFOLLOW() {
		t.Skip("TOCTOU protection requires O_NOFOLLOW; not supported on this platform")
	}
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	validPath := filepath.Join(tmp, "valid.txt")
	if err := os.WriteFile(validPath, []byte("safe"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	outside := filepath.Join(os.TempDir(), "leak_final.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("setup outside failed: %v", err)
	}
	defer os.Remove(outside)

	var wg sync.WaitGroup
	var leaked int32
	var blocked int32

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := fs.Read("valid.txt")
			if err != nil {
				atomic.AddInt32(&blocked, 1)
				return
			}
			if string(data) != "safe" {
				atomic.AddInt32(&leaked, 1)
			}
		}()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			os.Remove(validPath)
			os.Symlink(outside, validPath)
		}()
	}

	wg.Wait()

	if leaked > 0 {
		t.Fatalf("data leaked: %d", leaked)
	}
	t.Logf("blocked: %d, leaked: %d", blocked, leaked)
}

func TestLocalFS_ConcurrentDirectorySymlinkSubstitution(t *testing.T) {
	if !platformSupportsO_NOFOLLOW() {
		t.Skip("TOCTOU protection requires O_NOFOLLOW; not supported on this platform")
	}
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	safeDir := filepath.Join(tmp, "safe")
	if err := os.Mkdir(safeDir, 0o755); err != nil {
		t.Fatalf("mkdir safe failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(safeDir, "file.txt"), []byte("safe"), 0o644); err != nil {
		t.Fatalf("write safe failed: %v", err)
	}

	outside := filepath.Join(os.TempDir(), "dir_escape")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside failed: %v", err)
	}
	outsideFile := filepath.Join(outside, "file.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside failed: %v", err)
	}
	defer os.RemoveAll(outside)

	dirLink := filepath.Join(tmp, "dir")
	var wg sync.WaitGroup
	var leaked int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			os.Remove(dirLink)
			os.Symlink(outside, dirLink)
		}()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := fs.Read("dir/file.txt")
			if err != nil {
				return
			}
			if string(data) == "secret" {
				atomic.AddInt32(&leaked, 1)
			}
		}()
	}

	wg.Wait()

	if leaked > 0 {
		t.Fatalf("directory symlink substitution leaked outside data: %d", leaked)
	}
}

func TestLocalFS_ConcurrentDirectorySymlinkSubstitutionWrite(t *testing.T) {
	if !platformSupportsO_NOFOLLOW() {
		t.Skip("TOCTOU protection requires O_NOFOLLOW; not supported on this platform")
	}
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	safeDir := filepath.Join(tmp, "safe")
	if err := os.Mkdir(safeDir, 0o755); err != nil {
		t.Fatalf("mkdir safe failed: %v", err)
	}

	outside := filepath.Join(os.TempDir(), "dir_escape_write")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside failed: %v", err)
	}
	defer os.RemoveAll(outside)

	dirLink := filepath.Join(tmp, "dir")
	var wg sync.WaitGroup
	var leaked int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			os.Remove(dirLink)
			os.Symlink(outside, dirLink)
		}()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fs.Write("dir/file.txt", []byte("safe"))
			if err != nil {
				return
			}
			data, err := os.ReadFile(filepath.Join(outside, "file.txt"))
			if err == nil && string(data) == "safe" {
				atomic.AddInt32(&leaked, 1)
			}
		}()
	}

	wg.Wait()

	if leaked > 0 {
		t.Fatalf("directory symlink substitution leaked on write: %d", leaked)
	}
}

func TestLocalFS_ConcurrentDirectorySymlinkSubstitutionDelete(t *testing.T) {
	if !platformSupportsO_NOFOLLOW() {
		t.Skip("TOCTOU protection requires O_NOFOLLOW; not supported on this platform")
	}
	tmp := t.TempDir()
	fs := NewLocal(tmp)

	safeDir := filepath.Join(tmp, "safe")
	if err := os.Mkdir(safeDir, 0o755); err != nil {
		t.Fatalf("mkdir safe failed: %v", err)
	}
	safeFile := filepath.Join(safeDir, "file.txt")
	if err := os.WriteFile(safeFile, []byte("safe"), 0o644); err != nil {
		t.Fatalf("write safe failed: %v", err)
	}

	outside := filepath.Join(os.TempDir(), "dir_escape_delete")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside failed: %v", err)
	}
	outsideFile := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside failed: %v", err)
	}
	defer os.RemoveAll(outside)

	dirLink := filepath.Join(tmp, "dir")
	var wg sync.WaitGroup
	var leaked int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			os.Remove(dirLink)
			os.Symlink(outside, dirLink)
		}()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Try to delete through potentially swapped dir
			_ = fs.Delete("dir/victim.txt")
			if _, err := os.Stat(outsideFile); os.IsNotExist(err) {
				atomic.AddInt32(&leaked, 1)
			}
		}()
	}

	wg.Wait()

	if leaked > 0 {
		t.Fatalf("directory symlink substitution deleted outside file: %d", leaked)
	}
}
