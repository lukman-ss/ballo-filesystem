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
- `MemoryFS.Open()` returns error: no real `*os.File`. Use `Read`/`Write` instead.
- Path validation prevents traversal but does not enforce case-sensitivity on case-insensitive systems.

## Running tests

```bash
go test ./...
go vet ./...
gofmt -l .
```
