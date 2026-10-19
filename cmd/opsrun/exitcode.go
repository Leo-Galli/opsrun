// SPDX-License-Identifier: Apache-2.0

package main

// Documented exit codes. See README.md.
const (
	ExitOK               = 0
	ExitGenericError     = 1   // config, parse, or I/O error before any execution started
	ExitValidationFailed = 2   // the runbook failed validation
	ExitRunFailed        = 3   // the run failed and (if applicable) rollback ran
	ExitAborted          = 4   // the operator aborted via an approval prompt
	ExitInterrupted      = 130 // the run was interrupted by SIGINT/SIGTERM (128 + SIGINT)
)

// exitCodeError pairs an error with the process exit code it should produce.
type exitCodeError struct {
	code int
	err  error
}

func withExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitCodeError{code: code, err: err}
}

// Error implements the error interface.
func (e *exitCodeError) Error() string { return e.err.Error() }

// Unwrap allows errors.Is / errors.As to see through exitCodeError.
func (e *exitCodeError) Unwrap() error { return e.err }
