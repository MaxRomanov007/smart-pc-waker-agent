package updater

import (
	"os"
	"syscall"
)

// restart replaces the current process image with a fresh copy of the binary
// via syscall.Exec (execve). Never returns on success.
//
// Because the PID stays the same:
//   - procd sees respawn as normal and does nothing extra
//   - SysV init.d and rc.local launched the binary directly, the new image
//     takes over transparently
//
// This is the same mechanism used by restart_linux.go in smart-pc-agent.
func restart() {
	exe, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}

	// execve — never returns on success.
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		os.Exit(0)
	}
}
