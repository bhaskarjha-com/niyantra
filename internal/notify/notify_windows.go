//go:build windows

package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// sendWindowsToast sends a desktop notification using .NET BalloonTip.
// Uses System.Windows.Forms.NotifyIcon which is universally supported on
// Windows 7+ without requiring WinRT app registration or notification
// settings changes. Requires ~3s for the balloon to render before cleanup.
func sendWindowsToast(title, body string) error {
	// Escape single quotes for PowerShell string literals
	title = escapePSString(title)
	body = escapePSString(body)

	script := fmt.Sprintf(
		"Add-Type -AssemblyName System.Windows.Forms;"+
			"$n = New-Object System.Windows.Forms.NotifyIcon;"+
			"$n.Icon = [System.Drawing.SystemIcons]::Information;"+
			"$n.BalloonTipTitle = '%s';"+
			"$n.BalloonTipText = '%s';"+
			"$n.BalloonTipIcon = 'Info';"+
			"$n.Visible = $true;"+
			"$n.ShowBalloonTip(5000);"+
			"Start-Sleep -Seconds 3;"+
			"$n.Dispose()",
		title, body,
	)

	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell: %w: %s", err, string(out))
	}
	return nil
}

// escapePSString escapes characters for PowerShell single-quoted strings.
func escapePSString(s string) string {
	// In PowerShell single-quoted strings, only single quotes need escaping (doubled)
	s = strings.ReplaceAll(s, "'", "''")
	return s
}

