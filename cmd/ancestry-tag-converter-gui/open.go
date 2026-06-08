package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openInFileManager opens dir in the operating system's native file explorer
// (Finder on macOS, File Explorer on Windows, the default handler via xdg-open
// elsewhere). It returns once the explorer has been launched; it does not wait
// for the user to close it.
func openInFileManager(dir string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default: // linux, *bsd, …
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open %s: %w", dir, err)
	}
	// Reap the process in the background so we neither block the UI nor leak a
	// zombie. explorer.exe often exits non-zero even on success, so the result
	// is intentionally ignored.
	go cmd.Wait()
	return nil
}
