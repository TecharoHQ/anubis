//go:build !windows

package main

import "context"

// Hooks for Windows service management. These are no-ops outside of a
// Windows environment.

func platformStartup()                                 {}
func handleBootstrapFlag() bool                        { return false }
func runPlatformService(fn func(context.Context)) bool { return false }
