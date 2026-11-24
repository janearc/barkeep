//go:build !darwin

package main

import "os"

// houndsOn asks for nothing where there is no SIGINFO; -hounds still
// opens the column with them.
func houndsOn(chan os.Signal) {}
