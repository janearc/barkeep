//go:build darwin

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// houndsOn sends on the channel when the hounds are asked for: SIGINFO,
// which macOS has and Linux does not.
func houndsOn(asked chan os.Signal) { signal.Notify(asked, syscall.SIGINFO) }
