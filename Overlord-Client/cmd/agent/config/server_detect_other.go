//go:build !windows

package config

func isWindowsServerOS() bool {
	return false
}
