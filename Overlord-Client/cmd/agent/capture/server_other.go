//go:build !windows

package capture

import "context"

func AttachActiveInputDesktop() (uintptr, func()) {
	return 0, func() {}
}

func ReconnectSessionToConsole() error {
	return nil
}

func SetServerProcessPriority() {}

func StartServerKeepAliveWatchdog(ctx context.Context) {}
