//go:build windows

package capture

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	wtsapi32                         = syscall.NewLazyDLL("wtsapi32.dll")
	procWTSConnectSessionW           = wtsapi32.NewProc("WTSConnectSessionW")
	procWTSGetActiveConsoleSessionId = kernel32.NewProc("WTSGetActiveConsoleSessionId")
	procProcessIdToSessionId         = kernel32.NewProc("ProcessIdToSessionId")
	procGetCurrentProcessId          = kernel32.NewProc("GetCurrentProcessId")
	procOpenInputDesktop             = user32.NewProc("OpenInputDesktop")
	procSetPriorityClass             = kernel32.NewProc("SetPriorityClass")

	serverWatchdogOnce sync.Once
	lastReconnectTime  atomic.Int64
)

const (
	DESKTOP_READOBJECTS_SERVER   = 0x0001
	DESKTOP_SWITCHDESKTOP_SERVER = 0x0100
	DESKTOP_WRITEOBJECTS_SERVER  = 0x0080

	BELOW_NORMAL_PRIORITY_CLASS = 0x00004000
)

// AttachActiveInputDesktop ensures the calling OS thread is attached to the currently
// active input desktop (e.g. Default or Winlogon/Lock screen). This allows GDI BitBlt
// to succeed even when the session is locked or at the logon screen.
func AttachActiveInputDesktop() (uintptr, func()) {
	r, _, _ := procOpenInputDesktop.Call(0, 0, DESKTOP_READOBJECTS_SERVER|DESKTOP_SWITCHDESKTOP_SERVER|DESKTOP_WRITEOBJECTS_SERVER)
	if r == 0 {
		return 0, func() {}
	}
	tid, _, _ := procGetCurrentThreadId.Call()
	prevDesktop, _, _ := procGetThreadDesktop.Call(tid)
	procSetThreadDesktop.Call(r)
	return r, func() {
		if prevDesktop != 0 {
			procSetThreadDesktop.Call(prevDesktop)
		}
		procCloseDesktop.Call(r)
	}
}

// ReconnectSessionToConsole attaches the current user/RDP session directly to the
// console display surface. This prevents Windows Server from turning the display pipeline
// black or freezing capture when the RDP client is closed.
func ReconnectSessionToConsole() error {
	now := time.Now().Unix()
	last := lastReconnectTime.Load()
	if now-last < 5 {
		// Rate limit to once every 5 seconds
		return nil
	}
	lastReconnectTime.Store(now)

	var currentSession uint32
	pid, _, _ := procGetCurrentProcessId.Call()
	r, _, err := procProcessIdToSessionId.Call(pid, uintptr(unsafe.Pointer(&currentSession)))
	if r == 0 {
		return fmt.Errorf("ProcessIdToSessionId failed: %w", err)
	}

	consoleSession, _, _ := procWTSGetActiveConsoleSessionId.Call()

	// 1. Try Win32 API WTSConnectSessionW (preferred, zero-process in-memory call)
	r, _, _ = procWTSConnectSessionW.Call(uintptr(currentSession), consoleSession, 0, 0)
	if r != 0 {
		log.Printf("[server] WTSConnectSession reconnected session %d to console %d", currentSession, consoleSession)
		ResetMonitorCache()
		return nil
	}

	// 2. Fallback to connecting to console session 0/1
	r, _, _ = procWTSConnectSessionW.Call(uintptr(currentSession), 0, 0, 0)
	if r != 0 {
		log.Printf("[server] WTSConnectSession reconnected session %d to console 0", currentSession)
		ResetMonitorCache()
		return nil
	}

	// 3. Fallback to tscon.exe
	cmd := exec.Command("tscon.exe", fmt.Sprintf("%d", currentSession), "/dest:console")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err == nil {
		log.Printf("[server] tscon reconnected session %d to console", currentSession)
		ResetMonitorCache()
		return nil
	}

	return nil
}

// SetServerProcessPriority lowers the process scheduling priority class so agent capture
// and encoding do not saturate 1-2 vCPU cloud instances.
func SetServerProcessPriority() {
	p, _, _ := procGetCurrentProcess.Call()
	if p != 0 {
		procSetPriorityClass.Call(p, BELOW_NORMAL_PRIORITY_CLASS)
	}
}

// StartServerKeepAliveWatchdog starts a background monitor that detects disconnected RDP
// sessions and automatically reconnects them to the console to maintain screen capture.
func StartServerKeepAliveWatchdog(ctx context.Context) {
	serverWatchdogOnce.Do(func() {
		SetServerProcessPriority()
		// Disable DXGI by default on Windows Server VMs to avoid futile D3D11 crash loops
		SetDesktopDuplication(false)

		go func() {
			ticker := time.NewTicker(6 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// If no monitors are detected or screen resolution is collapsed, reconnect to console
					mons := monitorList()
					if len(mons) == 0 || (mons[0].rect.Dx() <= 0 || mons[0].rect.Dy() <= 0) {
						_ = ReconnectSessionToConsole()
					}
				}
			}
		}()
	})
}
