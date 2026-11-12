package store

import "os"

// File modes the store uses. Private is Owner-only; shared is world-readable.
const (
	privateDirMode  os.FileMode = 0o700
	privateFileMode os.FileMode = 0o600
	sharedDirMode   os.FileMode = 0o755
	sharedFileMode  os.FileMode = 0o644
)
