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

- Local filesystem only. S3/cloud storage not implemented.
- Path validation prevents traversal but does not enforce case-sensitivity on case-insensitive systems.
- Open returns io.ReadCloser; for local filesystem, underlying type is *os.File (implements ReadCloser).
- Symlink safety:
  - Unix-like platforms (Linux, macOS, BSD, etc.): LocalFS uses `os.OpenFile` with `O_NOFOLLOW` to prevent TOCTOU symlink substitution attacks on final component files. The `resolveChecked` method validates paths and blocks directory symlinks followed by additional path components to prevent directory traversal through symlinks.
  - Platforms without `O_NOFOLLOW` (Windows, JavaScript/WASM, AIX, Android): TOCTOU protection via `O_NOFOLLOW` is not available. Path validation and symlink resolution (`filepath.EvalSymlinks`) are still applied, but full TOCTOU protection cannot be guaranteed.
  - Intermediate directory symlinks are always blocked regardless of platform.
  - Final component symlinks pointing inside root are allowed and followed.
  - Final component symlinks pointing outside root are blocked.

## Running tests

```bash
go test ./...
go vet ./...
gofmt -l .
```
