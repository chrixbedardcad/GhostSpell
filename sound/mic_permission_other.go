//go:build !darwin

package sound

// CheckMicPermission is a no-op on non-macOS platforms.
// Windows and Linux do not enforce microphone entitlements at the OS level.
func CheckMicPermission() error {
	return nil
}
