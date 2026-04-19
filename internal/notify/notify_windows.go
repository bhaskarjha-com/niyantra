//go:build windows

package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// sendWindowsToast sends a toast notification using PowerShell and native WinRT APIs.
// No external modules needed — uses Windows.UI.Notifications directly.
// Requires Windows 10+ and PowerShell 5.1 (both standard on modern Windows).
func sendWindowsToast(title, body string) error {
	// Escape for XML and PowerShell
	title = escapeXML(title)
	body = escapeXML(body)

	script := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null
$xml = @"
<toast><visual><binding template="ToastGeneric">
<text>%s</text><text>%s</text>
</binding></visual></toast>
"@
$doc = New-Object Windows.Data.Xml.Dom.XmlDocument
$doc.LoadXml($xml)
$appId = '{1AC14E77-02E7-4E5D-B744-2EB3AE51BB4B}\WindowsPowerShell\v1.0\powershell.exe'
$n = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appId)
$n.Show([Windows.UI.Notifications.ToastNotification]::new($doc))
`, title, body)

	return exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).Run()
}

// escapeXML escapes special XML characters.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
