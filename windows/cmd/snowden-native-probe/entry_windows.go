//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Pipe/Job presence is an invocation guard, not authentication of a local owner
// or proof of WFP/route protection. The parent must verify image and scope.
func pipeJobReady() bool {
	for _, file := range []*os.File{os.Stdin, os.Stdout} {
		kind, err := windows.GetFileType(windows.Handle(file.Fd()))
		if err != nil || kind != windows.FILE_TYPE_PIPE {
			return false
		}
	}
	var member int32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
	result, _, _ := proc.Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&member)))
	return result != 0 && member != 0
}
