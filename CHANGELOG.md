# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- `Open` now returns `io.ReadCloser` instead of concrete `*File`
- MemoryFS uses mutex for thread-safe concurrent operations
- MemoryFS returns data copies to prevent slice aliasing
- LocalFS prevents escape through symlinks via `filepath.EvalSymlinks`
- TOCTOU-safe file operations using `os.OpenFile` with `O_NOFOLLOW` on supported platforms
- Comprehensive tests for symlink escape, concurrent access, slice copying, Open behavior, and concurrent symlink substitution

### Fixed

- MemoryFS.Open now returns valid `io.ReadCloser` for existing files
- MemoryFS Read/Write clone data to protect internal map from mutation
- LocalFS validates final resolved path after symlink evaluation
- LocalFS Write/Delete through escaping symlinks are blocked
- Component-level symlink check prevents directory traversal through internal symlinks

### Changed

- Filesystem interface: `Open(name string) (*File, error)` → `Open(name string) (io.ReadCloser, error)`

### Removed

- LocalFS.File embedded type (no longer needed)
