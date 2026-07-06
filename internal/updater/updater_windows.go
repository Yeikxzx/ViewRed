//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// ReplaceOnExit schedules replacement of the current binary with the
// new one on process exit. On Windows, this writes a batch file that
// waits for the process to exit, then moves the new binary over the old
// one and restarts.
func (u *Updater) ReplaceOnExit(newPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	return u.replaceWindows(exe, newPath)
}

func (u *Updater) replaceWindows(exe, newPath string) error {
	batchPath := filepath.Join(os.TempDir(), "viewred-update.bat")
	script := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
move /y "%s" "%s"
start "" "%s"
del "%%~f0"
`, newPath, exe, exe)

	if err := os.WriteFile(batchPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("write batch: %w", err)
	}
	cmd := exec.Command("cmd", "/c", batchPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start batch: %w", err)
	}
	return nil
}
