package spawnprep

import "time"

const UserTimeTick = 100 * time.Nanosecond

// JobLimits carries the platform's compiled resource values. It does not select
// an executable/role, authenticate a verifier, or grant network/SDK authority.
type JobLimits struct {
	Processes    uint32
	MemoryBytes  uintptr
	UserTime     time.Duration
	SuccessorGUI bool
}
