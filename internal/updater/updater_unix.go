//go:build !windows

package updater

import (
	"fmt"
	"os"
)

// ReplaceOnExit schedules replacement of the current binary with the
// new one on process exit. On Linux/macOS, this directly renames the
// new binary over the old one (the running binary's inode stays valid
// until all file descriptors are closed).
func (u *Updater) ReplaceOnExit(newPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	return u.replaceLinux(exe, newPath)
}

func (u *Updater) replaceLinux(exe, newPath string) error {
	if err := os.Rename(newPath, exe); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
