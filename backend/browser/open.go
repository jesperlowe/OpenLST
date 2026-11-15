// Package browser åbner standardbrowseren.
package browser

import (
	"os/exec"
	"runtime"

	"openlst/backend/procutil"
)

// Open åbner url i brugerens standardbrowser. url skal være vores egen
// localhost-adresse; funktionen kaldes aldrig med ekstern input.
func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	procutil.Hide(cmd)
	return cmd.Start()
}
