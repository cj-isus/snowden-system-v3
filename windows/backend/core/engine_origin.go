package core

// Negative origin pin only, captured before arbitrary context preparation.
// Read-only protocol progress is allowed; a different process/graph is not.
type engineProcessOrigin struct {
	process  *engineProcess
	instance uint64
	raw      []byte
}

func (origin engineProcessOrigin) currentLocked(engine *Engine) bool {
	if !engine.isolated || engine.process != origin.process || len(engine.rawJSON) != len(origin.raw) ||
		len(origin.raw) != 0 && &engine.rawJSON[0] != &origin.raw[0] {
		return false
	}
	return origin.process == nil || origin.process.instance == origin.instance
}
