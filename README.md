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
- Symlink safety: LocalFS uses `filepath.EvalSymlinks` to resolve symlinks and validates the full resolved path stays within root. File operations use `os.OpenFile` with `O_NOFOLLOW` (on supported platforms) to prevent TOCTOU symlink substitution attacks. Directory symlinks followed by additional path components are blocked to prevent directory traversal through symlinks.

## Running tests

```bash
go test ./...
go vet ./...
gofmt -l .
```
