# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- `Filesystem` interface: `Open`, `Read`, `Write`, `Delete`, `Exists`, `URL`
- `NewLocal` implementation with path-traversal validation scoped to root disk
- `NewMemory` in-memory implementation for tests
- `PathError` typed errors: `ErrEmptyPath`, `ErrAbsolutePath`, `ErrPathTraversal`, `ErrNotFound`, `ErrPermission`, `ErrInvalidName`
- Path validation rejects empty, absolute, and parent-traversal paths
- Automatic parent directory creation on `Write`

### Fixed

- None

### Changed

- None

### Removed

- None
