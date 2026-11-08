//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procMonitorFromWindow        = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
	procGetWindowLongW           = user32.NewProc("GetWindowLongW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

const (
	monitorDefaultToNearest = 2
	wsCaption               = 0x00C00000
)

type rect struct {
	left, top, right, bottom int32
}

func (r rect) width() int32  { return r.right - r.left }
func (r rect) height() int32 { return r.bottom - r.top }

type monitorInfo struct {
	size    int32
	monitor rect
	_       rect
	flags   uint32
}

// isFullscreenAppInFront reports whether the foreground window covers a whole
// monitor without a caption bar — the standard full-screen game signature on
// Windows (exclusive or borderless). Only the foreground window is examined:
// whatever is in front is what the overlay would have to sit over.
func isFullscreenAppInFront() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}

	// 排除自身：主窗口最大化也不是"全屏应用台前"。
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == uint32(os.Getpid()) {
		return false
	}

	// 有标题栏（WS_CAPTION）的窗口是普通应用窗口，不算全屏。
	// GWL_STYLE 是 -16：uintptr 下用按位取反表示（^15 == -16）
	style, _, _ := procGetWindowLongW.Call(hwnd, ^uintptr(15))
	if style&wsCaption != 0 {
		return false
	}

	var wr rect
	if _, _, _ = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); wr.width() <= 0 {
		return false
	}

	mi := monitorInfo{size: int32(unsafe.Sizeof(monitorInfo{}))}
	if ret, _, _ := procGetMonitorInfoW.Call(
		uintptr(monitorFromWindow(hwnd)),
		uintptr(unsafe.Pointer(&mi)),
	); ret == 0 {
		return false
	}

	// 窗口需要铺满整个显示器（2px 容差）。
	return wr.left <= mi.monitor.left+2 &&
		wr.top <= mi.monitor.top+2 &&
		wr.right >= mi.monitor.right-2 &&
		wr.bottom >= mi.monitor.bottom-2
}

// monitorFromWindow resolves the monitor a window is on.
func monitorFromWindow(hwnd uintptr) uintptr {
	ret, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	return ret
}
