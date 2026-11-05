# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- `Open` now returns `io.ReadCloser` instead of concrete `*File`
- MemoryFS uses mutex for thread-safe concurrent operations
- MemoryFS returns data copies to prevent slice aliasing
- LocalFS prevents escape through symlinks via `filepath.EvalSymlinks`
- Comprehensive tests for symlink escape, concurrent access, slice copying, Open behavior

### Fixed

- MemoryFS.Open now returns valid `io.ReadCloser` for existing files
- MemoryFS Read/Write clone data to protect internal map from mutation
- LocalFS validates final resolved path after symlink evaluation
- LocalFS Write/Delete/Delete through escaping symlinks are blocked

### Changed

- Filesystem interface: `Open(name string) (*File, error)` → `Open(name string) (io.ReadCloser, error)`

### Removed

- LocalFS.File embedded type (no longer needed)
