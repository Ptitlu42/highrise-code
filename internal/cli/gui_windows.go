//go:build windows

package cli

import (
	"syscall"
	"unsafe"
)

var proc_console_process_list = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")

func launched_from_gui(_ func(string) string) bool {
	var ids [2]uint32
	count, _, _ := proc_console_process_list.Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	return count <= 1
}
