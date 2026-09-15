package oauth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openBrowser asks the desktop to open url in the default browser. It returns
// once the request has been handed off, without waiting for the browser to
// exit, because some launchers block until the browser window closes.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start browser: %w", err)
	}

	// Reap the launcher in the background so it does not linger as a zombie.
	go func() { _ = cmd.Wait() }()

	return nil
}
