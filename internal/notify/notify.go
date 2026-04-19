// Package notify provides OS-native desktop notifications for Niyantra.
// Zero external dependencies — uses os/exec to call platform-native commands.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send sends an OS-native desktop notification.
// Returns nil if sent, error if the platform is unsupported or the command fails.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript", "-e",
			fmt.Sprintf(`display notification "%s" with title "%s"`, escapeOsascript(body), escapeOsascript(title)),
		).Run()
	case "linux":
		return exec.Command("notify-send", title, body).Run()
	case "windows":
		return sendWindowsToast(title, body)
	default:
		return fmt.Errorf("notifications not supported on %s", runtime.GOOS)
	}
}

// IsSupported returns whether notifications are available on this platform.
func IsSupported() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("osascript")
		return err == nil
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil
	case "windows":
		// PowerShell is always available on Windows 10+
		_, err := exec.LookPath("powershell")
		return err == nil
	default:
		return false
	}
}

// escapeOsascript escapes characters for AppleScript string literals.
func escapeOsascript(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
