//go:build !windows

package notify

// sendWindowsToast is a no-op on non-Windows platforms.
func sendWindowsToast(title, body string) error {
	return nil
}
