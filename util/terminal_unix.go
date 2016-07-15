// +build !windows

package util

import (
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

func IsInteractive() bool {
	return terminal.IsTerminal(syscall.Stdin)
}
