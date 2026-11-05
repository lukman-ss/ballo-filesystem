# ballo-filesystem

Minimal filesystem abstraction for local and in-memory operations.

## Prerequisites

- Go 1.21+

## Installation

```bash
go get github.com/lukman-ss/ballo-filesystem
```

## Usage

```go
import "github.com/lukman-ss/ballo-filesystem"

// Local filesystem
fs := ballofilesystem.NewLocal("/path/to/root")

// In-memory filesystem
mem := ballofilesystem.NewMemory()

// Basic operations
err := fs.Write("data.txt", []byte("hello"))
data, err := fs.Read("data.txt")
exists, err := fs.Exists("data.txt")
err = fs.Delete("data.txt")
url := fs.URL("data.txt")
```

## Limitations

- Go 1.23+ required.
- Local filesystem only. S3/cloud storage not implemented.
- Path validation prevents traversal but does not enforce case-sensitivity on case-insensitive systems.
- Open returns io.ReadCloser; for local filesystem, underlying type is *os.File (implements ReadCloser).
- Symlink safety and TOCTOU protection:
  - Unix-like platforms (Linux, macOS, BSD, etc.): LocalFS resolves all path components relative to the root directory file descriptor using atomic `openat`/`unlinkat` with `O_NOFOLLOW` and `O_DIRECTORY`. Intermediate directory components replaced by symlinks are blocked atomically at kernel level.
  - Platforms without `openat`/`O_NOFOLLOW` support (Windows, JavaScript/WASM, AIX, Android): TOCTOU protection cannot be guaranteed. Path validation and `filepath.EvalSymlinks` are applied as best effort.
  - Final component symlinks pointing inside root are safely followed.
  - Final component symlinks escaping root are strictly blocked.

## Running tests

```bash
go test ./...
go vet ./...
gofmt -l .
```
