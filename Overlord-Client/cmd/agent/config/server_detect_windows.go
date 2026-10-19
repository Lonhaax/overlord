//go:build windows

package config

import (
	"syscall"
	"unsafe"
)

type osVersionInfoExW struct {
	dwOSVersionInfoSize uint32
	dwMajorVersion     uint32
	dwMinorVersion     uint32
	dwBuildNumber      uint32
	dwPlatformId       uint32
	szCSDVersion       [128]uint16
	wServicePackMajor  uint16
	wServicePackMinor  uint16
	wSuiteMask         uint16
	wProductType       uint8
	wReserved          uint8
}

var (
	ntdll             = syscall.NewLazyDLL("ntdll.dll")
	procRtlGetVersion = ntdll.NewProc("RtlGetVersion")
)

func isWindowsServerOS() bool {
	var info osVersionInfoExW
	info.dwOSVersionInfoSize = uint32(unsafe.Sizeof(info))
	r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&info)))
	if r != 0 {
		return false
	}
	// wProductType:
	// VER_NT_WORKSTATION = 1
	// VER_NT_DOMAIN_CONTROLLER = 2
	// VER_NT_SERVER = 3
	return info.wProductType == 2 || info.wProductType == 3
}
