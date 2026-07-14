//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const cpUTF8 = 65001

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP       = kernel32.NewProc("SetConsoleCP")
	procGetConsoleMode     = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode     = kernel32.NewProc("SetConsoleMode")
)

const enableVirtualTerminalProcessing uint32 = 0x0004

func init() {
	// Force the Windows console to interpret stdout/stdin bytes as UTF-8.
	// Without this, programs that emit UTF-8 (this app, Conbit, pterm) get
	// rendered through the legacy code page (936/GBK in zh-CN), producing
	// mojibake like "瀵煎叆瀹屾垚" instead of "导入完成".
	_, _, _ = procSetConsoleOutputCP.Call(uintptr(cpUTF8))
	_, _, _ = procSetConsoleCP.Call(uintptr(cpUTF8))

	// Enable ANSI escape sequence processing so pterm colors/progress bars
	// render correctly on classic conhost; Windows Terminal already supports
	// this by default but the call is harmless there.
	enableVT(syscall.Stdout)
	enableVT(syscall.Stderr)
}

func enableVT(handle syscall.Handle) {
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return
	}
	_, _, _ = procSetConsoleMode.Call(uintptr(handle), uintptr(mode|enableVirtualTerminalProcessing))
}

