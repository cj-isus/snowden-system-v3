package quic

// A failed convenience call must not hide the cleanup error or lose its exact
// original transport while the error is retained. This is not a global retired
// inventory or permission to retry native cleanup with another transport.
type transportSetupError struct {
	transport *Transport
	cause     error
	cleanup   error
}

func (*transportSetupError) Error() string {
	return "quic: setup failed; transport cleanup returned an error"
}

func (e *transportSetupError) Unwrap() []error { return []error{e.cause, e.cleanup} }

// The cached original Close owns all cleanup. In particular a borrowed socket
// is interrupted and restored through that owner, never closed as an owned one.
func (t *Transport) closeAfterSetupError(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &transportSetupError{transport: t, cause: cause, cleanup: cleanup}
	}
	return cause
}
