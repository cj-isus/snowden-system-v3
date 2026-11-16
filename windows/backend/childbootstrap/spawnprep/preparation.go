// Package spawnprep retains original suspended-process/Job handles and attribute
// preparation, pipe I/O and native Job limits. It does not choose roles,
// authenticate images/parents or authorize network access. Callers serialize
// original owners, supply the trusted suspended-create adapter and their compiled
// resource values, and await actual I/O before whole-process retirement.
package spawnprep

import "errors"

var ErrResources = errors.New("owned process preparation retirement unconfirmed")
var ErrProcess = errors.New("original native process custody unavailable")
var ErrFileClose = errors.New("owned process file close unconfirmed")
var ErrLimits = errors.New("owned job resource policy unavailable")
