// SPDX-License-Identifier: Apache-2.0

package updater

// Storage replaces the pinned upstream updater's local filesystem boundaries.
// It is required: there is no default disk implementation or OS-path fallback.
// The caller owns synchronization, limits, exact byte ownership and retirement.
// Reads must return owned bytes. Only a genuine cache miss may look missing;
// consumers must also observe sticky faults, since upstream can ignore a local
// read error while trying a remote source. Paths are identifiers for Storage,
// never authority to open a file. Target storage is separate from metadata.
// No signature, threshold, role, version, expiry or delegation policy lives here.
type Storage interface {
	EnsurePaths(metadataDir, targetsDir string) error
	ReadMetadata(path string) ([]byte, error)
	WriteMetadata(path string, raw []byte) error
	ReadTarget(path string) ([]byte, error)
	WriteTarget(path string, raw []byte) error
}
