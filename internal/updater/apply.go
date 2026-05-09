package updater

import (
	"fmt"
	"os"

	goUpdate "github.com/inconshreveable/go-update"
)

// apply downloads release and atomically replaces the running binary.
// The path on disk stays the same, so the init system's autostart entry
// (procd script, SysV init.d, rc.local) remains valid after the update.
func apply(release ReleaseInfo) error {
	binary, err := downloadBinary(release)
	if err != nil {
		return err
	}
	defer binary.Close()

	if err := goUpdate.Apply(binary, goUpdate.Options{}); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	// go-update leaves a .old backup — remove it best-effort.
	// On routers storage is scarce, so we always clean up.
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}

	return nil
}
