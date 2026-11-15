//go:build !windows

package install

import "errors"

// Uninstall understøttes kun på Windows.
func Uninstall(dataDir string) error {
	return errors.New("afinstallation understøttes kun på Windows")
}

// MissingVCRuntime er kun relevant på Windows.
func MissingVCRuntime() []string { return nil }
