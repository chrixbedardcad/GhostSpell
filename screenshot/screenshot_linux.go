//go:build linux

package screenshot

import (
	"fmt"
	"os/exec"
	"strings"
)

// CaptureActiveWindow captures the active window as PNG bytes on Linux.
// Uses xdotool + import (ImageMagick) as the primary method, with
// gnome-screenshot as fallback.
func CaptureActiveWindow() ([]byte, error) {
	// Try xdotool + ImageMagick import first.
	winID, err := exec.Command("xdotool", "getactivewindow").Output()
	if err == nil {
		id := strings.TrimSpace(string(winID))
		out, err := exec.Command("import", "-window", id, "png:-").Output()
		if err == nil && len(out) > 0 {
			return out, nil
		}
	}

	// Fallback: gnome-screenshot to stdout.
	out, err := exec.Command("gnome-screenshot", "-w", "-f", "/dev/stdout").Output()
	if err == nil && len(out) > 0 {
		return out, nil
	}

	return nil, fmt.Errorf("screenshot: no capture tool available (install xdotool+imagemagick or gnome-screenshot)")
}
