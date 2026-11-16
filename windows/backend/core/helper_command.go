package core

import "context"

// HelperCommandResult is a completed command fact, not forwarding/probe
// authority. A nil capture error confirms original I/O/process/Job retirement;
// callers must still inspect ExitCode for the command's actual outcome.
type HelperCommandResult struct {
	Output   []byte
	ExitCode uint32
}

// Provisional exit metadata never leaves this synchronous capture before its
// original cleanup/source checks. No earlier caller can inspect this receipt.
type helperCommandExit struct {
	code     uint32
	returned bool
}

// CaptureProtectedCommand uses the existing single protected lane and its exact
// native/pipe owners. Nonzero exit is returned as data only after full original
// retirement; timeout/overflow/unknown cleanup returns no command result.
func CaptureProtectedCommand(parent context.Context, executable string, arguments []string, maxOutput int64) (HelperCommandResult, error) {
	exit := &helperCommandExit{}
	output, err := captureOwnedHelper(&protectedHelper, parent, executable, arguments, maxOutput, helperCaptureOptions{commandExit: exit})
	if err != nil {
		return HelperCommandResult{}, err
	}
	if !exit.returned {
		return HelperCommandResult{}, ErrHelperExit
	}
	return HelperCommandResult{Output: output, ExitCode: exit.code}, nil
}
