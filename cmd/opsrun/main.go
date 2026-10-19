// SPDX-License-Identifier: Apache-2.0

// Command opsrun turns Markdown runbooks into safe, concurrent incident response
// workflows executed over SSH.
package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	err := newRootCmd().Execute()
	if err == nil {
		os.Exit(ExitOK)
	}

	var ece *exitCodeError
	if errors.As(err, &ece) {
		fmt.Fprintln(os.Stderr, "Error:", ece.Error())
		os.Exit(ece.code)
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(ExitGenericError)
}
